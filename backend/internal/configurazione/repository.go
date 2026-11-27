package configurazione

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lastmiledelivery/backend/internal/database"
	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/interno"
)

// anagrafe: le sedi sono nel database globale (servizio utenti).
type anagrafe interface {
	Risolvi(ctx context.Context, r interno.Richiesta) (interno.Risposta, error)
}

type Repository struct {
	db       *database.Gruppi
	anagrafe anagrafe
}

// NuovoRepository: db sono i database delle spedizioni (uno per gruppo di sedi).
//   - Le impostazioni di una sede stanno nel database del suo gruppo.
//   - I motivi sono uguali per tutte le sedi: l'amministratore li cambia nel database
//     principale (gruppo 0) e da lì si copiano negli altri (SincronizzaMotivi), perché
//     ogni operazione li legge nel database della sua sede, insieme alle impostazioni.
func NuovoRepository(db *database.Gruppi, a anagrafe) *Repository {
	return &Repository{db: db, anagrafe: a}
}

// assicuraImpostazioni: la sede nasce nel database globale, la sua riga di impostazioni
// qui si crea (con i valori predefiniti) la prima volta che serve. Una sede che non
// esiste risponde 404.
func (r *Repository) assicuraImpostazioni(ctx context.Context, db *pgxpool.Pool, idSede int64) error {
	var presente bool
	if err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM impostazioni WHERE id_sede = $1)`, idSede).Scan(&presente); err != nil || presente {
		return err
	}
	ris, err := r.anagrafe.Risolvi(ctx, interno.Richiesta{Sedi: []int64{idSede}})
	if err != nil {
		return err
	}
	if _, ok := ris.Sedi[idSede]; !ok {
		return errSedeNonTrovata
	}
	_, err = db.Exec(ctx, `INSERT INTO impostazioni (id_sede) VALUES ($1) ON CONFLICT (id_sede) DO NOTHING`, idSede)
	return err
}

var errSedeNonTrovata = &errori.NonTrovato{Messaggio: "sede non trovata"}

const colonneImpostazioni = `
	id_sede, giorni_visibilita_cliente, max_tentativi_ritiro, max_tentativi_consegna,
	intervallo_posizione_sec, modificate_il, id_amministratore_modifica`

func (i *Impostazioni) campi() []any {
	return []any{&i.IDSede, &i.GiorniVisibilitaCliente, &i.MaxTentativiRitiro, &i.MaxTentativiConsegna,
		&i.IntervalloPosizioneSec, &i.ModificateIl, &i.IDAmministratoreModifica}
}

// Regole legge le impostazioni della sede e i motivi attivi con una sola query: la usano
// tutte le operazioni sulle spedizioni, così una modifica dell'amministratore vale subito.
func (r *Repository) Regole(ctx context.Context, idSede int64) (Regole, error) {
	db, err := r.db.PerSede(ctx, idSede)
	if err != nil {
		return Regole{}, err
	}
	if err := r.assicuraImpostazioni(ctx, db, idSede); err != nil {
		return Regole{}, err
	}
	var reg Regole
	err = db.QueryRow(ctx, `
		SELECT `+colonneImpostazioni+`,
		       COALESCE((SELECT array_agg(descrizione ORDER BY id) FROM motivi WHERE attivo), '{}')
		FROM impostazioni
		WHERE id_sede = $1`, idSede).Scan(append(reg.campi(), &reg.Motivi)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Regole{}, errSedeNonTrovata
	}
	return reg, err
}

// MotiviAttivi sono uguali per tutte le sedi: si leggono nel database della sede di chi
// chiede (il principale per chi non ha una sede).
func (r *Repository) MotiviAttivi(ctx context.Context) ([]string, error) {
	db, err := r.db.PerAmbito(ctx)
	if err != nil {
		return nil, err
	}
	var motivi []string
	err = db.QueryRow(ctx, `SELECT COALESCE(array_agg(descrizione ORDER BY id), '{}') FROM motivi WHERE attivo`).Scan(&motivi)
	return motivi, err
}

func (r *Repository) AggiornaImpostazioni(ctx context.Context, idSede int64, m ModificaImpostazioni, idAmministratore int64) (Impostazioni, error) {
	db, err := r.db.PerSede(ctx, idSede)
	if err != nil {
		return Impostazioni{}, err
	}
	if err := r.assicuraImpostazioni(ctx, db, idSede); err != nil {
		return Impostazioni{}, err
	}
	var imp Impostazioni
	err = db.QueryRow(ctx, `
		UPDATE impostazioni SET
			giorni_visibilita_cliente  = COALESCE($2, giorni_visibilita_cliente),
			max_tentativi_ritiro       = COALESCE($3, max_tentativi_ritiro),
			max_tentativi_consegna     = COALESCE($4, max_tentativi_consegna),
			intervallo_posizione_sec   = COALESCE($5, intervallo_posizione_sec),
			modificate_il              = now(),
			id_amministratore_modifica = $6
		WHERE id_sede = $1
		RETURNING `+colonneImpostazioni,
		idSede, m.GiorniVisibilitaCliente, m.MaxTentativiRitiro, m.MaxTentativiConsegna, m.IntervalloPosizioneSec,
		idAmministratore).Scan(imp.campi()...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Impostazioni{}, errSedeNonTrovata
	}
	return imp, err
}

const selezioneMotivo = `SELECT id, descrizione, attivo, creato_il FROM motivi`

func scansionaMotivo(riga pgx.CollectableRow) (Motivo, error) {
	var m Motivo
	err := riga.Scan(&m.ID, &m.Descrizione, &m.Attivo, &m.CreatoIl)
	return m, err
}

func (r *Repository) Motivi(ctx context.Context) ([]Motivo, error) {
	righe, err := r.db.Principale().Query(ctx, selezioneMotivo+` ORDER BY id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(righe, scansionaMotivo)
}

func (r *Repository) CreaMotivo(ctx context.Context, descrizione string) (Motivo, error) {
	righe, err := r.db.Principale().Query(ctx, `
		INSERT INTO motivi (descrizione) VALUES ($1)
		RETURNING id, descrizione, attivo, creato_il`, descrizione)
	if err != nil {
		return Motivo{}, err
	}
	m, err := pgx.CollectExactlyOneRow(righe, scansionaMotivo)
	if codice, vincolo, ok := database.ErroreVincolo(err); ok && codice == database.CodiceUnicita && vincolo == "uq_motivi_descrizione" {
		return Motivo{}, &errori.Conflitto{
			Messaggio: "esiste già un motivo con questa descrizione: se è disattivato, riattivarlo con PATCH /api/impostazioni/motivi/{id}",
			Dettagli:  []errori.ErroreCampo{{Campo: "descrizione", Messaggio: "già presente"}},
		}
	}
	if err == nil {
		r.copiaMotivi(ctx)
	}
	return m, err
}

func (r *Repository) ImpostaMotivoAttivo(ctx context.Context, id int64, attivo bool) (Motivo, error) {
	tx, err := r.db.Principale().Begin(ctx)
	if err != nil {
		return Motivo{}, err
	}
	defer tx.Rollback(ctx)

	// Il blocco sui motivi attivi mette in fila due disattivazioni contemporanee:
	// senza, entrambe vedrebbero un altro motivo attivo e l'elenco resterebbe vuoto.
	righe, err := tx.Query(ctx, `SELECT id FROM motivi WHERE attivo FOR UPDATE`)
	if err != nil {
		return Motivo{}, err
	}
	attivi, err := pgx.CollectRows(righe, pgx.RowTo[int64])
	if err != nil {
		return Motivo{}, err
	}
	if !attivo && len(attivi) == 1 && attivi[0] == id {
		return Motivo{}, &errori.Conflitto{Messaggio: "è l'ultimo motivo attivo: crearne o riattivarne un altro prima di disattivarlo"}
	}

	righe, err = tx.Query(ctx, `
		UPDATE motivi SET attivo = $2 WHERE id = $1
		RETURNING id, descrizione, attivo, creato_il`, id, attivo)
	if err != nil {
		return Motivo{}, err
	}
	m, err := pgx.CollectExactlyOneRow(righe, scansionaMotivo)
	if errors.Is(err, pgx.ErrNoRows) {
		return Motivo{}, &errori.NonTrovato{Messaggio: "motivo non trovato"}
	}
	if err != nil {
		return Motivo{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Motivo{}, err
	}
	r.copiaMotivi(ctx)
	return m, nil
}

// copiaMotivi: dopo una modifica dei motivi si copiano subito negli altri gruppi. Se un
// gruppo non risponde la modifica resta valida (è nel principale) e la copia la ripete
// SincronizzazionePeriodica: per poco quel gruppo vede i motivi di prima.
func (r *Repository) copiaMotivi(ctx context.Context) {
	if err := r.SincronizzaMotivi(ctx); err != nil {
		log.Printf("motivi non ancora copiati in tutti i gruppi (si riprova tra poco): %v", err)
	}
}

// SincronizzaMotivi rende i motivi di ogni gruppo uguali a quelli del principale: stessi
// id, descrizioni e stato. Un motivo che nel principale non c'è viene disattivato (le
// spedizioni lo citano per descrizione, quindi non si cancella).
func (r *Repository) SincronizzaMotivi(ctx context.Context) error {
	codici := r.db.Codici()
	if len(codici) == 1 {
		return nil
	}
	var errs []error
	for _, c := range codici[1:] {
		// un gruppo che non risponde non deve tenere ferma la risposta all'amministratore
		ctxGruppo, annulla := context.WithTimeout(ctx, 5*time.Second)
		db, _, err := r.db.Gruppo(ctxGruppo, c)
		if err == nil {
			err = copiaMotiviIn(ctxGruppo, db, r.db.Principale())
		}
		annulla()
		if err != nil {
			errs = append(errs, fmt.Errorf("gruppo %d: %w", c, err))
		}
	}
	return errors.Join(errs...)
}

// copiaMotiviIn: due copie contemporanee verso lo stesso gruppo (repliche diverse, una
// modifica e la copia periodica) si mettono in fila con un lock del gruppo, e ognuna legge i
// motivi del principale solo dopo averlo preso: chi scrive per ultimo ha letto per ultimo,
// quindi una copia vecchia non può annullare una modifica appena copiata.
func copiaMotiviIn(ctx context.Context, db, principale *pgxpool.Pool) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, lockCopiaMotivi); err != nil {
		return err
	}
	righe, err := principale.Query(ctx, selezioneMotivo+` ORDER BY id`)
	if err != nil {
		return err
	}
	motivi, err := pgx.CollectRows(righe, scansionaMotivo)
	if err != nil {
		return err
	}
	ids := make([]int64, len(motivi))
	for i, m := range motivi {
		ids[i] = m.ID
		if _, err := tx.Exec(ctx, `
			INSERT INTO motivi (id, descrizione, attivo, creato_il) OVERRIDING SYSTEM VALUE
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (id) DO UPDATE SET descrizione = EXCLUDED.descrizione, attivo = EXCLUDED.attivo
			WHERE (motivi.descrizione, motivi.attivo) IS DISTINCT FROM (EXCLUDED.descrizione, EXCLUDED.attivo)`,
			m.ID, m.Descrizione, m.Attivo, m.CreatoIl); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE motivi SET attivo = false WHERE attivo AND id <> ALL($1)`, ids); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// lockCopiaMotivi: chiave dell'advisory lock che mette in fila le copie dei motivi
const lockCopiaMotivi = 7_204_001

// SincronizzazionePeriodica copia i motivi all'avvio e poi ogni minuto, finché ctx non si
// chiude: riallinea un gruppo che non rispondeva quando l'amministratore li ha cambiati.
func (r *Repository) SincronizzazionePeriodica(ctx context.Context) {
	if len(r.db.Codici()) == 1 {
		return
	}
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		r.copiaMotivi(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

var _ archivio = (*Repository)(nil)
