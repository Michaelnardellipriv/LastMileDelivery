package configurazione

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lastmiledelivery/backend/internal/database"
	"lastmiledelivery/backend/internal/errori"
)

type Repository struct {
	db *pgxpool.Pool
}

func NuovoRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

const colonneImpostazioni = `
	giorni_visibilita_cliente, max_tentativi_ritiro, max_tentativi_consegna,
	intervallo_posizione_sec, modificate_il, id_amministratore_modifica`

func (i *Impostazioni) campi() []any {
	return []any{&i.GiorniVisibilitaCliente, &i.MaxTentativiRitiro, &i.MaxTentativiConsegna,
		&i.IntervalloPosizioneSec, &i.ModificateIl, &i.IDAmministratoreModifica}
}

// Regole legge impostazioni e motivi attivi con una sola query: la usano tutte le
// operazioni sulle spedizioni, così una modifica dell'amministratore vale subito.
func (r *Repository) Regole(ctx context.Context) (Regole, error) {
	var reg Regole
	err := r.db.QueryRow(ctx, `
		SELECT `+colonneImpostazioni+`,
		       COALESCE((SELECT array_agg(descrizione ORDER BY id) FROM motivi WHERE attivo), '{}')
		FROM impostazioni`).Scan(append(reg.campi(), &reg.Motivi)...)
	return reg, err
}

func (r *Repository) AggiornaImpostazioni(ctx context.Context, m ModificaImpostazioni, idAmministratore int32) (Impostazioni, error) {
	var imp Impostazioni
	err := r.db.QueryRow(ctx, `
		UPDATE impostazioni SET
			giorni_visibilita_cliente  = COALESCE($1, giorni_visibilita_cliente),
			max_tentativi_ritiro       = COALESCE($2, max_tentativi_ritiro),
			max_tentativi_consegna     = COALESCE($3, max_tentativi_consegna),
			intervallo_posizione_sec   = COALESCE($4, intervallo_posizione_sec),
			modificate_il              = now(),
			id_amministratore_modifica = $5
		RETURNING `+colonneImpostazioni,
		m.GiorniVisibilitaCliente, m.MaxTentativiRitiro, m.MaxTentativiConsegna, m.IntervalloPosizioneSec,
		idAmministratore).Scan(imp.campi()...)
	return imp, err
}

const selezioneMotivo = `SELECT id, descrizione, attivo, creato_il FROM motivi`

func scansionaMotivo(riga pgx.CollectableRow) (Motivo, error) {
	var m Motivo
	err := riga.Scan(&m.ID, &m.Descrizione, &m.Attivo, &m.CreatoIl)
	return m, err
}

func (r *Repository) Motivi(ctx context.Context) ([]Motivo, error) {
	righe, err := r.db.Query(ctx, selezioneMotivo+` ORDER BY id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(righe, scansionaMotivo)
}

func (r *Repository) CreaMotivo(ctx context.Context, descrizione string) (Motivo, error) {
	righe, err := r.db.Query(ctx, `
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
	return m, err
}

func (r *Repository) ImpostaMotivoAttivo(ctx context.Context, id int32, attivo bool) (Motivo, error) {
	tx, err := r.db.Begin(ctx)
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
	attivi, err := pgx.CollectRows(righe, pgx.RowTo[int32])
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
	return m, tx.Commit(ctx)
}

var _ archivio = (*Repository)(nil)
