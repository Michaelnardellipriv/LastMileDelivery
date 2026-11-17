package identita

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var tabellaDelRuolo = map[Ruolo]string{
	Operatore:      "operatori_logistici",
	Corriere:       "corrieri",
	Cliente:        "clienti",
	Amministratore: "amministratori",
}

type Repository struct {
	db *pgxpool.Pool
}

func NuovoRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Stato dice se l'utente con quel ruolo esiste e se l'amministratore non l'ha disattivato.
func (r *Repository) Stato(ctx context.Context, ruolo Ruolo, id int32) (esiste, attivo bool, err error) {
	tabella, ok := tabellaDelRuolo[ruolo]
	if !ok {
		return false, false, nil
	}
	err = r.db.QueryRow(ctx, `
		SELECT u.attivo FROM `+tabella+` AS t JOIN utenti AS u ON u.id = t.id_utente
		WHERE t.id = $1`, id).Scan(&attivo)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false, nil
	}
	return err == nil, attivo, err
}
