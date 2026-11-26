// Package sessioni registra nel database globale le sessioni aperte con il login. Ogni
// token porta l'id della sua sessione (claim "sid") e vale solo finché la sessione è
// aperta: chiuderla invalida subito il token, su tutti e due i servizi.
//
// Una sessione si chiude:
//   - con il logout (POST /api/auth/esci);
//   - quando l'amministratore disattiva l'utente o lo sposta di sede;
//   - quando la password cambia: cambio da parte dell'utente (tranne la sessione con cui
//     la cambia), recupero, nuova password data dall'amministratore.
//
// Il servizio utenti legge le sessioni nel suo database a ogni richiesta. Il servizio
// spedizioni ha un altro database: Revoca gli comunica le sessioni chiuse (rotta interna
// /interno/sessioni/revocate) dentro la stessa transazione, prima del commit. Se il
// servizio spedizioni non risponde l'operazione fallisce (503) e niente cambia: non può
// succedere che un utente risulti disattivato ma possa ancora lavorare sulle spedizioni.
package sessioni

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lastmiledelivery/backend/internal/audit"
	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
	"lastmiledelivery/backend/internal/interno"
)

// propagatore comunica le sessioni chiuse al servizio spedizioni.
type propagatore interface {
	RevocaSessioni(ctx context.Context, sessioni []interno.SessioneRevocata) error
}

type Archivio struct {
	db         *pgxpool.Pool
	spedizioni propagatore
	durata     time.Duration // DURATA_SESSIONE: dopo, si rifà il login
}

func Nuovo(db *pgxpool.Pool, spedizioni propagatore, durata time.Duration) *Archivio {
	return &Archivio{db: db, spedizioni: spedizioni, durata: durata}
}

var (
	errChiusa      = &errori.NonAutenticato{Messaggio: "sessione chiusa o scaduta: rifare il login"}
	errCambiato    = &errori.NonAutenticato{Messaggio: "l'account è cambiato durante l'accesso: rifare il login"}
	errDisattivato = &errori.NonAutorizzato{Messaggio: "account disattivato: contattare l'amministratore"}
)

// Apri registra la sessione di un login e ne restituisce l'id (casuale, 128 bit).
// pwdHash è l'hash con cui il login ha verificato la password.
//
// Il login ha letto l'account prima di arrivare qui: nel frattempo l'amministratore
// potrebbe averlo disattivato o spostato di sede, o la password potrebbe essere cambiata.
// Per non aprire una sessione che nessuno revocherà, si blocca la riga dell'utente (FOR
// SHARE: aspetta le transazioni che la stanno modificando, e quelle che arrivano dopo
// aspettano questa) e si ricontrolla lo stato prima di inserire. Così la sessione o
// nasce dopo la revoca, e la vede, o nasce prima, e la revoca la chiude.
func (a *Archivio) Apri(ctx context.Context, idUtente int64, r identita.Richiedente, pwdHash string) (string, error) {
	if r.Ruolo == "" || r.ID <= 0 {
		return "", errors.New("sessione senza utente")
	}
	tabella, sede, ok := identita.TabellaDelRuolo(r.Ruolo)
	if !ok {
		return "", errors.New("ruolo sconosciuto")
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	id := hex.EncodeToString(b)
	// le sessioni finite da più di 30 giorni non servono più (nemmeno per ricostruire un
	// accesso): si tolgono qui, una manciata per volta
	if _, err := a.db.Exec(ctx, `
		DELETE FROM sessioni WHERE id IN (
			SELECT id FROM sessioni WHERE scade_il < now() - interval '30 days' LIMIT 100)`); err != nil {
		return "", err
	}

	tx, err := a.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var attivo, invariato bool
	err = tx.QueryRow(ctx, `
		SELECT u.attivo, u.pwd_hash = $3 AND `+sede+` = $4
		FROM utenti u JOIN `+tabella+` t ON t.id_utente = u.id AND t.id = $2
		WHERE u.id = $1
		FOR SHARE OF u`, idUtente, r.ID, pwdHash, r.IDSede).Scan(&attivo, &invariato)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", errCambiato
	case err != nil:
		return "", err
	case !attivo:
		return "", errDisattivato
	case !invariato:
		return "", errCambiato
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO sessioni (id, id_utente, ruolo, id_ruolo, id_sede, scade_il)
		VALUES ($1, $2, $3, $4, NULLIF($5, 0), now() + make_interval(secs => $6))`,
		id, idUtente, string(r.Ruolo), r.ID, r.IDSede, a.durata.Seconds()); err != nil {
		return "", err
	}
	return id, tx.Commit(ctx)
}

// Valida (rinnovo del token): la sessione è aperta, non scaduta, dello stesso ruolo.
// Restituisce la fine della sessione, oltre la quale il nuovo token non deve valere.
// Una sessione chiusa perché l'utente è stato disattivato dà 403, come il login.
func (a *Archivio) Valida(ctx context.Context, r identita.Richiedente) (time.Time, error) {
	var aperta bool
	var fine time.Time
	var motivo *string
	err := a.db.QueryRow(ctx, `
		UPDATE sessioni SET rinnovata_il = now()
		WHERE id = $1 AND ruolo = $2 AND id_ruolo = $3
		RETURNING revocata_il IS NULL AND scade_il > now(), scade_il, motivo_revoca`,
		r.Sessione, string(r.Ruolo), r.ID).Scan(&aperta, &fine, &motivo)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return time.Time{}, errChiusa
	case err != nil:
		return time.Time{}, err
	case !aperta && motivo != nil && *motivo == identita.MotivoDisattivato:
		return time.Time{}, errDisattivato
	case !aperta:
		return time.Time{}, errChiusa
	}
	return fine, nil
}

// Chiudi chiude una sessione (logout).
func (a *Archivio) Chiudi(ctx context.Context, idSessione, motivo string) error {
	tx, err := a.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	righe, err := tx.Query(ctx, `
		UPDATE sessioni SET revocata_il = now(), motivo_revoca = $2
		WHERE id = $1 AND revocata_il IS NULL
		RETURNING id, scade_il, ruolo, COALESCE(id_sede, 0)`, idSessione, motivo)
	if err != nil {
		return err
	}
	chiuse, err := raccogli(righe, motivo)
	if err != nil {
		return err
	}
	if err := a.spedizioni.RevocaSessioni(ctx, chiuse); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RevocaUtente chiude, dentro tx, tutte le sessioni aperte dell'utente tranne tranne
// (vuoto: tutte) e le comunica al servizio spedizioni prima che chi chiama faccia il
// commit. Restituisce quante ne ha chiuse.
func (a *Archivio) RevocaUtente(ctx context.Context, tx pgx.Tx, idUtente int64, tranne, motivo string) (int, error) {
	righe, err := tx.Query(ctx, `
		UPDATE sessioni SET revocata_il = now(), motivo_revoca = $3
		WHERE id_utente = $1 AND revocata_il IS NULL AND scade_il > now() AND id <> $2
		RETURNING id, scade_il, ruolo, COALESCE(id_sede, 0)`, idUtente, tranne, motivo)
	if err != nil {
		return 0, err
	}
	chiuse, err := raccogli(righe, motivo)
	if err != nil {
		return 0, err
	}
	if err := a.spedizioni.RevocaSessioni(ctx, chiuse); err != nil {
		return 0, err
	}
	if len(chiuse) > 0 {
		audit.Evento(ctx, "sessioni_chiuse", "id_utente", idUtente, "numero", len(chiuse), "motivo", motivo)
	}
	return len(chiuse), nil
}

// raccogli: le sessioni appena chiuse (id, fine, ruolo e sede del login), da comunicare al
// servizio spedizioni.
func raccogli(righe pgx.Rows, motivo string) ([]interno.SessioneRevocata, error) {
	return pgx.CollectRows(righe, func(r pgx.CollectableRow) (interno.SessioneRevocata, error) {
		var s interno.SessioneRevocata
		err := r.Scan(&s.ID, &s.ScadeIl, &s.Ruolo, &s.IDSede)
		s.Motivo = motivo
		return s, err
	})
}
