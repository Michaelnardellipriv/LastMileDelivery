package autenticazione

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

type Account struct {
	IDUtente int64
	Nome     string
	Cognome  string
	Email    string
	PwdHash  string
	Attivo   bool
	Ruoli    []identita.Richiedente
}

// revocaSessioni chiude le sessioni dell'utente dentro la transazione (package sessioni).
type revocaSessioni interface {
	RevocaUtente(ctx context.Context, tx pgx.Tx, idUtente int64, tranne, motivo string) (int, error)
}

type Repository struct {
	db       *pgxpool.Pool
	sessioni revocaSessioni
}

func NuovoRepository(db *pgxpool.Pool, sessioni revocaSessioni) *Repository {
	return &Repository{db: db, sessioni: sessioni}
}

const selezioneAccount = `
	SELECT u.id, u.nome, u.cognome, u.email, u.pwd_hash, u.attivo,
	       o.id, o.id_sede, c.id, c.id_sede, cl.id, a.id
	FROM utenti u
	LEFT JOIN operatori_logistici o ON o.id_utente = u.id
	LEFT JOIN corrieri c            ON c.id_utente = u.id
	LEFT JOIN clienti cl            ON cl.id_utente = u.id
	LEFT JOIN amministratori a      ON a.id_utente = u.id`

func leggiAccount(riga pgx.Row) (Account, error) {
	var acc Account
	var op, sedeOp, co, sedeCo, cl, am *int64
	if err := riga.Scan(&acc.IDUtente, &acc.Nome, &acc.Cognome, &acc.Email, &acc.PwdHash, &acc.Attivo,
		&op, &sedeOp, &co, &sedeCo, &cl, &am); err != nil {
		return Account{}, err
	}
	nessuna := new(int64)
	for _, r := range []struct {
		ruolo identita.Ruolo
		id    *int64
		sede  *int64
	}{{identita.Operatore, op, sedeOp}, {identita.Corriere, co, sedeCo}, {identita.Cliente, cl, nessuna}, {identita.Amministratore, am, nessuna}} {
		if r.id != nil && r.sede != nil {
			acc.Ruoli = append(acc.Ruoli, identita.Richiedente{Ruolo: r.ruolo, ID: *r.id, IDSede: *r.sede})
		}
	}
	return acc, nil
}

func (r *Repository) PerEmail(ctx context.Context, email string) (Account, bool, error) {
	acc, err := leggiAccount(r.db.QueryRow(ctx, selezioneAccount+`
		WHERE lower(u.email) = lower($1)
		ORDER BY u.id
		LIMIT 1`, email))
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, false, nil
	}
	return acc, err == nil, err
}

var colonnaDelRuolo = map[identita.Ruolo]string{
	identita.Operatore:      "o.id",
	identita.Corriere:       "c.id",
	identita.Cliente:        "cl.id",
	identita.Amministratore: "a.id",
}

func (r *Repository) PerRuolo(ctx context.Context, rich identita.Richiedente) (Account, error) {
	colonna, ok := colonnaDelRuolo[rich.Ruolo]
	if !ok {
		return Account{}, &errori.NonAutenticato{Messaggio: "utente non riconosciuto"}
	}
	acc, err := leggiAccount(r.db.QueryRow(ctx, selezioneAccount+` WHERE `+colonna+` = $1`, rich.ID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, &errori.NonAutenticato{Messaggio: "utente non riconosciuto"}
	}
	return acc, err
}

// DestinatarioDiSpedizioni: le spedizioni sono nell'altro database; quando un cliente
// diventa destinatario il servizio spedizioni lo segna qui (clienti.destinatario).
func (r *Repository) DestinatarioDiSpedizioni(ctx context.Context, idCliente int64) (bool, error) {
	var destinatario bool
	err := r.db.QueryRow(ctx, `SELECT destinatario FROM clienti WHERE id = $1`, idCliente).Scan(&destinatario)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return destinatario, err
}

// AggiornaPassword cambia la password e, nella stessa transazione, chiude le sessioni
// dell'utente tranne tranne (quella con cui l'utente l'ha cambiata).
func (r *Repository) AggiornaPassword(ctx context.Context, idUtente int64, hash, tranne string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE utenti SET pwd_hash = $2 WHERE id = $1`, idUtente, hash); err != nil {
		return err
	}
	if _, err := r.sessioni.RevocaUtente(ctx, tx, idUtente, tranne, identita.MotivoPasswordCambiata); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) RecuperiRecenti(ctx context.Context, idUtente int64, dal time.Time) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM recuperi_password WHERE id_utente = $1 AND creato_il >= $2`,
		idUtente, dal).Scan(&n)
	return n, err
}

func (r *Repository) CreaRecupero(ctx context.Context, idUtente int64, tokenHash string, scadenza time.Time) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		UPDATE recuperi_password SET usato_il = now()
		WHERE id_utente = $1 AND usato_il IS NULL`, idUtente); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO recuperi_password (id_utente, token_hash, scade_il) VALUES ($1, $2, $3)`,
		idUtente, tokenHash, scadenza); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) UsaRecupero(ctx context.Context, tokenHash, nuovoHash string) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var idUtente int64
	err = tx.QueryRow(ctx, `
		UPDATE recuperi_password SET usato_il = now()
		WHERE token_hash = $1 AND usato_il IS NULL AND scade_il > now()
		RETURNING id_utente`, tokenHash).Scan(&idUtente)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `UPDATE utenti SET pwd_hash = $2 WHERE id = $1`, idUtente, nuovoHash); err != nil {
		return false, err
	}
	// chi ha la vecchia password (o un suo token) non entra più
	if _, err := r.sessioni.RevocaUtente(ctx, tx, idUtente, "", identita.MotivoPasswordCambiata); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

var _ archivio = (*Repository)(nil)
