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
	IDUtente int32
	Nome     string
	Cognome  string
	Email    string
	PwdHash  string
	Ruoli    []identita.Richiedente
}

type Repository struct {
	db *pgxpool.Pool
}

func NuovoRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

const selezioneAccount = `
	SELECT u.id, u.nome, u.cognome, u.email, u.pwd_hash, o.id, c.id, cl.id, a.id
	FROM utenti u
	LEFT JOIN operatori_logistici o ON o.id_utente = u.id
	LEFT JOIN corrieri c            ON c.id_utente = u.id
	LEFT JOIN clienti cl            ON cl.id_utente = u.id
	LEFT JOIN amministratori a      ON a.id_utente = u.id`

func leggiAccount(riga pgx.Row) (Account, error) {
	var acc Account
	var op, co, cl, am *int32
	if err := riga.Scan(&acc.IDUtente, &acc.Nome, &acc.Cognome, &acc.Email, &acc.PwdHash, &op, &co, &cl, &am); err != nil {
		return Account{}, err
	}
	for _, r := range []struct {
		ruolo identita.Ruolo
		id    *int32
	}{{identita.Operatore, op}, {identita.Corriere, co}, {identita.Cliente, cl}, {identita.Amministratore, am}} {
		if r.id != nil {
			acc.Ruoli = append(acc.Ruoli, identita.Richiedente{Ruolo: r.ruolo, ID: *r.id})
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

func (r *Repository) AggiornaPassword(ctx context.Context, idUtente int32, hash string) error {
	_, err := r.db.Exec(ctx, `UPDATE utenti SET pwd_hash = $2 WHERE id = $1`, idUtente, hash)
	return err
}

func (r *Repository) RecuperiRecenti(ctx context.Context, idUtente int32, dal time.Time) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM recuperi_password WHERE id_utente = $1 AND creato_il >= $2`,
		idUtente, dal).Scan(&n)
	return n, err
}

func (r *Repository) CreaRecupero(ctx context.Context, idUtente int32, tokenHash string, scadenza time.Time) error {
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
	var idUtente int32
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
	return true, tx.Commit(ctx)
}

var _ archivio = (*Repository)(nil)
