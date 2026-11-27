package database

import (
	"context"
	"errors"
	"log"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
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

	db, err := Prepara(ctx, connessione)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, err
	}
	avvisaSeIgnoraRLS(ctx, db)
	return db, nil
}

// Prepara crea il pool senza collegarsi: le connessioni si aprono al primo uso. Serve per i
// database che all'avvio possono non rispondere (i gruppi delle spedizioni diversi dal 0).
func Prepara(ctx context.Context, connessione string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(connessione)
	if err != nil {
		return nil, err
	}
	// un host che non risponde (nodo spento) non deve tenere ferma una richiesta a lungo
	if cfg.ConnConfig.ConnectTimeout == 0 {
		cfg.ConnConfig.ConnectTimeout = 5 * time.Second
	}
	// Prima di dare una connessione a una richiesta le si dice su quale sede lavora
	// (Row-Level Security). Si imposta SEMPRE, anche vuoto: così una connessione non si
	// porta dietro la sede della richiesta precedente.
	cfg.PrepareConn = func(ctx context.Context, conn *pgx.Conn) (bool, error) {
		tutte, sede := AmbitoDalContesto(ctx).variabili()
		_, err := conn.Exec(ctx, `SELECT set_config('app.tutte_le_sedi', $1, false), set_config('app.id_sede', $2, false)`,
			tutte, sede)
		return err == nil, err
	}

	return pgxpool.NewWithConfig(ctx, cfg)
}

// avvisaSeIgnoraRLS: un superutente (o un utente con BYPASSRLS) non è soggetto alla
// Row-Level Security, quindi la rete di sicurezza sulle sedi non scatterebbe.
func avvisaSeIgnoraRLS(ctx context.Context, db *pgxpool.Pool) {
	var ignora bool
	var utente string
	err := db.QueryRow(ctx, `SELECT current_user, rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`).
		Scan(&utente, &ignora)
	if err == nil && ignora {
		log.Printf("ATTENZIONE: l'utente del database %q è superutente o ha BYPASSRLS: la Row-Level Security sulle sedi "+
			"non vale. Usate in DATABASE_URL l'utente del backend (db/ruolo_applicazione.sh)", utente)
	}
}

// Ambito dice a PostgreSQL quali sedi può vedere la richiesta: tutte (amministratori e
// clienti) oppure una sola (operatori e corrieri). L'ambito zero non vede nessuna sede.
type Ambito struct {
	TutteLeSedi bool
	IDSede      int64
}

func TutteLeSedi() Ambito { return Ambito{TutteLeSedi: true} }

func SoloSede(id int64) Ambito { return Ambito{IDSede: id} }

// Ammette dice se l'ambito comprende la sede indicata.
func (a Ambito) Ammette(idSede int64) bool {
	return a.TutteLeSedi || (a.IDSede > 0 && a.IDSede == idSede)
}

func (a Ambito) variabili() (tutte, sede string) {
	if a.TutteLeSedi {
		return "si", ""
	}
	if a.IDSede > 0 {
		return "", strconv.FormatInt(a.IDSede, 10)
	}
	return "", ""
}

type chiaveAmbito struct{}

func ConAmbito(ctx context.Context, a Ambito) context.Context {
	return context.WithValue(ctx, chiaveAmbito{}, a)
}

func AmbitoDalContesto(ctx context.Context) Ambito {
	a, _ := ctx.Value(chiaveAmbito{}).(Ambito)
	return a
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
