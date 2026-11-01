package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	CodiceChiaveEsterna = "23503"
	CodiceUnicita       = "23505"
)

func Collega(ctx context.Context, connessione string) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	db, err := pgxpool.New(ctx, connessione)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func NonDisponibile(err error) bool {
	var errConnessione *pgconn.ConnectError
	return errors.As(err, &errConnessione) ||
		errors.Is(err, context.DeadlineExceeded) ||
		pgconn.Timeout(err)
}

func ErroreVincolo(err error) (codice, vincolo string, ok bool) {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return "", "", false
	}
	return pgErr.Code, pgErr.ConstraintName, true
}
