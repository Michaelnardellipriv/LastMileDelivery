package identita

import (
	"context"

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

func (r *Repository) Esiste(ctx context.Context, ruolo Ruolo, id int32) (bool, error) {
	tabella, ok := tabellaDelRuolo[ruolo]
	if !ok {
		return false, nil
	}
	var esiste bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM `+tabella+` WHERE id = $1)`, id).Scan(&esiste)
	return esiste, err
}
