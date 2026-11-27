package posizioni

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"lastmiledelivery/backend/internal/database"
	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/interno"
)

const (
	toleranzaIntervalloSecondi = 2.0
	condizioneSpedizioneAttiva = `s.stato = 'in consegna'`
)

// anagrafe: nome e cognome dei corrieri sono nel database globale (servizio utenti).
type anagrafe interface {
	Risolvi(ctx context.Context, r interno.Richiesta) (interno.Risposta, error)
}

type Repository struct {
	db       *database.Gruppi
	anagrafe anagrafe
}

// NuovoRepository: db sono i database delle spedizioni; ogni operazione usa quello della
// sede del corriere o dell'operatore.
func NuovoRepository(db *database.Gruppi, a anagrafe) *Repository {
	return &Repository{db: db, anagrafe: a}
}

type interrogabile interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// intervalloPredefinito è il DEFAULT di impostazioni.intervallo_posizione_sec.
const intervalloPredefinito = 30

// leggiIntervallo: se la sede non ha ancora la riga delle impostazioni (la sede nasce nel
// database globale, la riga qui si crea al primo uso) vale il valore predefinito.
func leggiIntervallo(ctx context.Context, q interrogabile, idSede int64) (int64, error) {
	var secondi int64
	err := q.QueryRow(ctx, `
		SELECT COALESCE((SELECT intervallo_posizione_sec FROM impostazioni WHERE id_sede = $1), $2)`,
		idSede, intervalloPredefinito).Scan(&secondi)
	return secondi, err
}

func (r *Repository) Intervallo(ctx context.Context, idSede int64) (int64, error) {
	db, err := r.db.PerSede(ctx, idSede)
	if err != nil {
		return 0, err
	}
	return leggiIntervallo(ctx, db, idSede)
}

// Registra salva l'ultima posizione del corriere: una riga per corriere, sovrascritta a
// ogni invio, così la tabella non cresce con il tempo.
func (r *Repository) Registra(ctx context.Context, idSede, idCorriere int64, lat, lon float64) (int64, error) {
	db, err := r.db.PerSede(ctx, idSede)
	if err != nil {
		return 0, err
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	// due invii contemporanei dello stesso corriere passano uno alla volta (i corrieri sono
	// nel database globale: qui un lock sul numero del corriere, fino alla fine della transazione)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, idCorriere); err != nil {
		return 0, err
	}

	var attivo bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM spedizioni AS s
			JOIN assegnazioni AS a ON a.id = s.id_assegnazione_corrente
			WHERE a.id_corriere = $1 AND s.id_sede = $2 AND `+condizioneSpedizioneAttiva+`
		)`, idCorriere, idSede).Scan(&attivo); err != nil {
		return 0, err
	}
	if !attivo {
		return 0, &errori.Conflitto{Messaggio: "il corriere non ha spedizioni in consegna: la posizione non viene registrata"}
	}

	intervallo, err := leggiIntervallo(ctx, tx, idSede)
	if err != nil {
		return 0, err
	}

	var trascorsi *float64
	err = tx.QueryRow(ctx, `
		SELECT EXTRACT(EPOCH FROM now() - rilevata_il)::float8
		FROM ultime_posizioni_corrieri
		WHERE id_corriere = $1`, idCorriere).Scan(&trascorsi)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	if trascorsi != nil && *trascorsi < float64(intervallo)-toleranzaIntervalloSecondi {
		return intervallo, &errori.TroppoPresto{
			Messaggio:         "posizione inviata troppo presto: rispetta l'intervallo impostato dall'amministratore",
			RiprovaTraSecondi: int(intervallo),
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO ultime_posizioni_corrieri (id_corriere, id_sede, latitudine, longitudine, rilevata_il)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (id_corriere) DO UPDATE
		SET id_sede = EXCLUDED.id_sede, latitudine = EXCLUDED.latitudine,
		    longitudine = EXCLUDED.longitudine, rilevata_il = EXCLUDED.rilevata_il`,
		idCorriere, idSede, lat, lon); err != nil {
		return 0, err
	}
	return intervallo, tx.Commit(ctx)
}

// Posizioni: l'ultima posizione dei corrieri della sede che hanno spedizioni in consegna.
func (r *Repository) Posizioni(ctx context.Context, idSede int64) ([]Posizione, error) {
	db, err := r.db.PerSede(ctx, idSede)
	if err != nil {
		return nil, err
	}
	righe, err := db.Query(ctx, `
		WITH attivi AS (
			SELECT a.id_corriere, array_agg(s.id ORDER BY s.id) AS spedizioni
			FROM spedizioni AS s
			JOIN assegnazioni AS a ON a.id = s.id_assegnazione_corrente
			WHERE s.id_sede = $1 AND `+condizioneSpedizioneAttiva+`
			GROUP BY a.id_corriere
		)
		SELECT at.id_corriere,
		       p.latitudine, p.longitudine, p.rilevata_il,
		       at.spedizioni
		FROM attivi AS at
		JOIN ultime_posizioni_corrieri AS p ON p.id_corriere = at.id_corriere AND p.id_sede = $1
		ORDER BY at.id_corriere`, idSede)
	if err != nil {
		return nil, err
	}
	posizioni, err := pgx.CollectRows(righe, func(riga pgx.CollectableRow) (Posizione, error) {
		var p Posizione
		err := riga.Scan(&p.IDCorriere, &p.Latitudine, &p.Longitudine, &p.RilevataIl, &p.Spedizioni)
		return p, err
	})
	if err != nil {
		return nil, err
	}
	if len(posizioni) == 0 {
		return []Posizione{}, nil
	}
	// nome e cognome dei corrieri: database globale
	ids := make([]int64, len(posizioni))
	for i, p := range posizioni {
		ids[i] = p.IDCorriere
	}
	ris, err := r.anagrafe.Risolvi(ctx, interno.Richiesta{Corrieri: ids})
	if err != nil {
		return nil, err
	}
	for i := range posizioni {
		c := ris.Corrieri[posizioni[i].IDCorriere]
		posizioni[i].Nome, posizioni[i].Cognome = c.Nome, c.Cognome
	}
	return posizioni, nil
}

var _ archivio = (*Repository)(nil)
