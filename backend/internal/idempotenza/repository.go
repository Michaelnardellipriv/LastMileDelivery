package idempotenza

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lastmiledelivery/backend/internal/database"
)

// Repository tiene le chiavi nel database della richiesta: uno solo nel servizio utenti;
// nel servizio spedizioni quello del gruppo della sede di chi chiede (database.Gruppi).
type Repository struct {
	per   func(ctx context.Context) (*pgxpool.Pool, error)
	tutti func() []*pgxpool.Pool
}

// NuovoRepository: un solo database.
func NuovoRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		per:   func(context.Context) (*pgxpool.Pool, error) { return db, nil },
		tutti: func() []*pgxpool.Pool { return []*pgxpool.Pool{db} },
	}
}

// NuovoRepositoryPerGruppi: le chiavi stanno nel database del gruppo della sede di chi
// chiede (il principale per chi non ha una sede), così le scritture di un gruppo non
// dipendono dal database di un altro. La chiave contiene ruolo e id di chi chiede, quindi
// le sue richieste ripetute tornano sempre allo stesso database.
func NuovoRepositoryPerGruppi(g *database.Gruppi) *Repository {
	return &Repository{per: g.PerAmbito, tutti: g.Verificati}
}

// Prenota prova a riservare la chiave per una nuova richiesta. Se la chiave c'è già
// restituisce cosa ne è stato: ancora in corso, usata con un corpo diverso o completata.
// L'INSERT ... ON CONFLICT è atomico: due repliche che ricevono la stessa chiave insieme
// non possono vincere entrambe.
func (r *Repository) Prenota(ctx context.Context, chiave string, hashCorpo []byte, durata, durataInCorso time.Duration) (Esito, Risposta, error) {
	// due giri: se tra la INSERT e la SELECT un'altra richiesta libera la chiave, si riprova
	db, err := r.per(ctx)
	if err != nil {
		return 0, Risposta{}, err
	}
	for range 2 {
		// una chiave scaduta, o rimasta "in corso" da una richiesta abbandonata, si riusa
		if _, err := db.Exec(ctx, `
			DELETE FROM chiavi_idempotenza
			WHERE chiave = $1
			  AND (creata_il < now() - make_interval(secs => $2)
			       OR (stato_http IS NULL AND creata_il < now() - make_interval(secs => $3)))`,
			chiave, durata.Seconds(), durataInCorso.Seconds()); err != nil {
			return 0, Risposta{}, err
		}
		tag, err := db.Exec(ctx, `
			INSERT INTO chiavi_idempotenza (chiave, hash_corpo) VALUES ($1, $2)
			ON CONFLICT (chiave) DO NOTHING`, chiave, hashCorpo)
		if err != nil {
			return 0, Risposta{}, err
		}
		if tag.RowsAffected() == 1 {
			return Nuova, Risposta{}, nil
		}

		var hash, corpo, intestazioni []byte
		var stato *int
		err = db.QueryRow(ctx, `
			SELECT hash_corpo, stato_http, intestazioni, corpo FROM chiavi_idempotenza WHERE chiave = $1`,
			chiave).Scan(&hash, &stato, &intestazioni, &corpo)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, Risposta{}, err
		}
		switch {
		case !bytes.Equal(hash, hashCorpo):
			return CorpoDiverso, Risposta{}, nil
		case stato == nil:
			return InCorso, Risposta{}, nil
		}
		risposta := Risposta{Stato: *stato, Corpo: corpo, Header: http.Header{}}
		if len(intestazioni) > 0 {
			if err := json.Unmarshal(intestazioni, &risposta.Header); err != nil {
				return 0, Risposta{}, err
			}
		}
		return Completata, risposta, nil
	}
	return InCorso, Risposta{}, nil
}

func (r *Repository) Completa(ctx context.Context, chiave string, risposta Risposta) error {
	intestazioni, err := json.Marshal(risposta.Header)
	if err != nil {
		return err
	}
	db, err := r.per(ctx)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `
		UPDATE chiavi_idempotenza SET stato_http = $2, intestazioni = $3, corpo = $4
		WHERE chiave = $1`, chiave, risposta.Stato, intestazioni, risposta.Corpo)
	return err
}

func (r *Repository) Libera(ctx context.Context, chiave string) error {
	db, err := r.per(ctx)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `DELETE FROM chiavi_idempotenza WHERE chiave = $1 AND stato_http IS NULL`, chiave)
	return err
}

// Pulisci toglie le chiavi scadute da tutti i database.
func (r *Repository) Pulisci(ctx context.Context, durata time.Duration) (int64, error) {
	var tolte int64
	var errs []error
	for _, db := range r.tutti() {
		tag, err := db.Exec(ctx, `DELETE FROM chiavi_idempotenza WHERE creata_il < now() - make_interval(secs => $1)`,
			durata.Seconds())
		tolte += tag.RowsAffected()
		errs = append(errs, err)
	}
	return tolte, errors.Join(errs...)
}

var _ archivio = (*Repository)(nil)
