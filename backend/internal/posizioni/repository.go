package posizioni

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lastmiledelivery/backend/internal/errori"
)

const (
	toleranzaIntervalloSecondi = 2.0
	condizioneSpedizioneAttiva = `s.stato = 'in consegna'`
)

type Repository struct {
	db *pgxpool.Pool
}

func NuovoRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

type interrogabile interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func leggiIntervallo(ctx context.Context, q interrogabile) (int32, error) {
	var secondi int32
	err := q.QueryRow(ctx, `SELECT intervallo_posizione_sec FROM impostazioni`).Scan(&secondi)
	return secondi, err
}

func (r *Repository) Intervallo(ctx context.Context) (int32, error) {
	return leggiIntervallo(ctx, r.db)
}

func (r *Repository) AggiornaIntervallo(ctx context.Context, secondi, idAmministratore int32) error {
	_, err := r.db.Exec(ctx, `
		UPDATE impostazioni
		SET intervallo_posizione_sec   = $1,
		    modificate_il              = now(),
		    id_amministratore_modifica = $2`,
		secondi, idAmministratore)
	return err
}

func (r *Repository) Registra(ctx context.Context, idCorriere int32, lat, lon float64) (int32, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT 1 FROM corrieri WHERE id = $1 FOR UPDATE`, idCorriere); err != nil {
		return 0, err
	}

	var attivo bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM spedizioni AS s
			JOIN assegnazioni AS a ON a.id = s.id_assegnazione_corrente
			WHERE a.id_corriere = $1 AND `+condizioneSpedizioneAttiva+`
		)`, idCorriere).Scan(&attivo); err != nil {
		return 0, err
	}
	if !attivo {
		return 0, &errori.Conflitto{Messaggio: "il corriere non ha spedizioni in consegna: la posizione non viene registrata"}
	}

	intervallo, err := leggiIntervallo(ctx, tx)
	if err != nil {
		return 0, err
	}

	var trascorsi *float64
	if err := tx.QueryRow(ctx, `
		SELECT EXTRACT(EPOCH FROM now() - max(rilevata_il))::float8
		FROM posizioni_corrieri
		WHERE id_corriere = $1`, idCorriere).Scan(&trascorsi); err != nil {
		return 0, err
	}
	if trascorsi != nil && *trascorsi < float64(intervallo)-toleranzaIntervalloSecondi {
		return intervallo, &errori.TroppoPresto{
			Messaggio:         "posizione inviata troppo presto: rispetta l'intervallo impostato dall'amministratore",
			RiprovaTraSecondi: int(intervallo),
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO posizioni_corrieri (id_corriere, latitudine, longitudine)
		VALUES ($1, $2, $3)`, idCorriere, lat, lon); err != nil {
		return 0, err
	}
	return intervallo, tx.Commit(ctx)
}

func (r *Repository) Posizioni(ctx context.Context) ([]Posizione, error) {
	righe, err := r.db.Query(ctx, `
		WITH attivi AS (
			SELECT a.id_corriere, array_agg(s.id ORDER BY s.id) AS spedizioni
			FROM spedizioni AS s
			JOIN assegnazioni AS a ON a.id = s.id_assegnazione_corrente
			WHERE `+condizioneSpedizioneAttiva+`
			GROUP BY a.id_corriere
		)
		SELECT c.id, u.nome, u.cognome,
		       p.latitudine, p.longitudine, p.rilevata_il,
		       at.spedizioni
		FROM attivi AS at
		JOIN corrieri AS c ON c.id = at.id_corriere
		JOIN utenti   AS u ON u.id = c.id_utente
		JOIN LATERAL (
			SELECT latitudine, longitudine, rilevata_il
			FROM posizioni_corrieri AS pc
			WHERE pc.id_corriere = c.id
			ORDER BY rilevata_il DESC
			LIMIT 1
		) AS p ON TRUE
		ORDER BY c.id`)
	if err != nil {
		return nil, err
	}
	posizioni, err := pgx.CollectRows(righe, func(riga pgx.CollectableRow) (Posizione, error) {
		var p Posizione
		err := riga.Scan(&p.IDCorriere, &p.Nome, &p.Cognome,
			&p.Latitudine, &p.Longitudine, &p.RilevataIl, &p.Spedizioni)
		return p, err
	})
	if err != nil {
		return nil, err
	}
	if posizioni == nil {
		posizioni = []Posizione{}
	}
	return posizioni, nil
}

var _ archivio = (*Repository)(nil)
