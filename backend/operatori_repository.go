package main

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func trovaIDOperatorePerEmail(ctx context.Context, db *pgxpool.Pool, email string) (int32, error) {
	const query = `
		SELECT ol.id
		FROM operatori_logistici AS ol
		JOIN utenti AS u ON u.id = ol.id_utente
		WHERE u.email = $1
	`

	var idOperatore int32

	err := db.QueryRow(ctx, query, email).Scan(&idOperatore)

	if err != nil {
		return 0, err
	}

	return idOperatore, nil
}