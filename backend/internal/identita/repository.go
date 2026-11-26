package identita

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// tabella del ruolo e colonna della sede (vuota per i ruoli senza sede)
var tabellaDelRuolo = map[Ruolo]struct{ tabella, sede string }{
	Operatore:      {"operatori_logistici", "t.id_sede"},
	Corriere:       {"corrieri", "t.id_sede"},
	Cliente:        {"clienti", "0"},
	Amministratore: {"amministratori", "0"},
}

type Repository struct {
	db *pgxpool.Pool
}

func NuovoRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Stato dice se l'utente con quel ruolo esiste, se l'amministratore non l'ha disattivato
// e in quale sede lavora (0 per clienti e amministratori).
func (r *Repository) Stato(ctx context.Context, ruolo Ruolo, id int64) (esiste, attivo bool, idSede int64, err error) {
	t, ok := tabellaDelRuolo[ruolo]
	if !ok {
		return false, false, 0, nil
	}
	err = r.db.QueryRow(ctx, `
		SELECT u.attivo, `+t.sede+` FROM `+t.tabella+` AS t JOIN utenti AS u ON u.id = t.id_utente
		WHERE t.id = $1`, id).Scan(&attivo, &idSede)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false, 0, nil
	}
	return err == nil, attivo, idSede, err
}

// TabellaDelRuolo: la tabella del ruolo e l'espressione SQL della sede (alias t; "0" per i
// ruoli senza sede). ok è falso per un ruolo sconosciuto.
func TabellaDelRuolo(ruolo Ruolo) (tabella, sede string, ok bool) {
	t, ok := tabellaDelRuolo[ruolo]
	return t.tabella, t.sede, ok
}
