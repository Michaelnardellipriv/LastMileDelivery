package main

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNessunaSpedizioneAttiva = errors.New("il corriere non ha spedizioni in consegna: la posizione non viene registrata")
var ErrInvioTroppoFrequente = errors.New("posizione inviata troppo presto: rispetta l'intervallo impostato dall'amministratore")

// Margine sull'intervallo: una richiesta che arriva con un piccolo anticipo
// (rete lenta, timer impreciso) non viene rifiutata.
const toleranzaIntervalloSecondi = 2.0

// La posizione si comunica SOLO mentre la spedizione è in consegna
// (non quando è soltanto ritirata).
const condizioneSpedizioneAttiva = `s.stato = 'in consegna'`

type PosizioneCorriere struct {
	IDCorriere  int32     `json:"id_corriere"`
	Nome        string    `json:"nome"`
	Cognome     string    `json:"cognome"`
	Latitudine  float64   `json:"latitudine"`
	Longitudine float64   `json:"longitudine"`
	RilevataIl  time.Time `json:"rilevata_il"`
	Spedizioni  []int32   `json:"spedizioni_attive"`
}

// Sia il pool sia una transazione hanno QueryRow: così la stessa funzione
// si può usare dentro e fuori da una transazione.
type eseguiQuery interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func leggiIntervalloPosizione(ctx context.Context, db eseguiQuery) (int32, error) {
	var secondi int32
	err := db.QueryRow(ctx, `SELECT intervallo_posizione_sec FROM impostazioni`).Scan(&secondi)
	return secondi, err
}

func aggiornaIntervalloPosizione(ctx context.Context, db *pgxpool.Pool, secondi int32, idAmministratore int32) error {
	const query = `
		UPDATE impostazioni
		SET intervallo_posizione_sec   = $1,
		    modificate_il              = now(),
		    id_amministratore_modifica = $2
	`
	_, err := db.Exec(ctx, query, secondi, idAmministratore)
	return err
}

// registraPosizioneCorriere salva la posizione se il corriere ha spedizioni
// in consegna e se è passato l'intervallo dall'ultimo invio.
// Restituisce l'intervallo attuale, così il client sa quando inviare di nuovo.
func registraPosizioneCorriere(ctx context.Context, db *pgxpool.Pool, idCorriere int32, lat, lon float64) (int32, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx) // non fa nulla se c'è già stato il Commit

	// Blocca la riga del corriere: due invii contemporanei dello stesso
	// corriere vengono controllati uno dopo l'altro, non insieme.
	_, err = tx.Exec(ctx, `SELECT 1 FROM corrieri WHERE id = $1 FOR UPDATE`, idCorriere)
	if err != nil {
		return 0, err
	}

	var attivo bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM spedizioni AS s
			JOIN assegnazioni AS a ON a.id = s.id_assegnazione_corrente
			WHERE a.id_corriere = $1 AND `+condizioneSpedizioneAttiva+`
		)`, idCorriere).Scan(&attivo)
	if err != nil {
		return 0, err
	}
	if !attivo {
		return 0, ErrNessunaSpedizioneAttiva
	}

	intervallo, err := leggiIntervalloPosizione(ctx, tx)
	if err != nil {
		return 0, err
	}

	// Secondi passati dall'ultimo invio, calcolati dal database (NULL se è il primo).
	var trascorsi *float64
	err = tx.QueryRow(ctx, `
		SELECT EXTRACT(EPOCH FROM now() - max(rilevata_il))::float8
		FROM posizioni_corrieri
		WHERE id_corriere = $1`, idCorriere).Scan(&trascorsi)
	if err != nil {
		return 0, err
	}
	if trascorsi != nil && *trascorsi < float64(intervallo)-toleranzaIntervalloSecondi {
		return intervallo, ErrInvioTroppoFrequente
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO posizioni_corrieri (id_corriere, latitudine, longitudine)
		VALUES ($1, $2, $3)`, idCorriere, lat, lon)
	if err != nil {
		return 0, err
	}

	return intervallo, tx.Commit(ctx)
}

// posizioniCorrieriAttivi restituisce l'ultima posizione dei soli corrieri
// che ORA hanno spedizioni in consegna. Chi ha finito le consegne
// sparisce dalla lista, anche se le sue posizioni restano nello storico.
func posizioniCorrieriAttivi(ctx context.Context, db *pgxpool.Pool) ([]PosizioneCorriere, error) {
	rows, err := db.Query(ctx, `
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

	posizioni, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (PosizioneCorriere, error) {
		var p PosizioneCorriere
		err := row.Scan(&p.IDCorriere, &p.Nome, &p.Cognome,
			&p.Latitudine, &p.Longitudine, &p.RilevataIl, &p.Spedizioni)
		return p, err
	})
	if err != nil {
		return nil, err
	}

	// Lista vuota invece di null nel JSON.
	if posizioni == nil {
		posizioni = []PosizioneCorriere{}
	}
	return posizioni, nil
}
