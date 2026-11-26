package identita

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"lastmiledelivery/backend/internal/database"
	"lastmiledelivery/backend/internal/errori"
)

// Motivi per cui una sessione si chiude (colonna motivo_revoca / motivo): compaiono nel
// log di audit e, per la disattivazione, fanno rispondere 403 invece di 401.
const (
	MotivoDisattivato      = "utente disattivato"
	MotivoCambioSede       = "cambio di sede"
	MotivoPasswordCambiata = "password cambiata"
	MotivoLogout           = "logout"
)

var (
	errSessioneChiusa = &errori.NonAutenticato{Messaggio: "sessione chiusa o scaduta: rifare il login"}
	errDisattivato    = &errori.NonAutorizzato{Messaggio: messaggioDisattivato}
)

// Controlla (servizio utenti, database globale): la sessione del token esiste, è dello
// stesso utente e ruolo, non è stata chiusa né è scaduta, e l'utente è attivo. Restituisce
// il richiedente con la sede che ha adesso.
func (r *Repository) Controlla(ctx context.Context, rich Richiedente) (Richiedente, error) {
	t, ok := tabellaDelRuolo[rich.Ruolo]
	if !ok {
		return rich, errSessioneChiusa
	}
	var aperta, attivo bool
	var idSede int64
	err := r.db.QueryRow(ctx, `
		SELECT s.revocata_il IS NULL AND s.scade_il > now(), u.attivo, `+t.sede+`
		FROM sessioni s
		JOIN utenti u ON u.id = s.id_utente
		JOIN `+t.tabella+` t ON t.id_utente = u.id AND t.id = s.id_ruolo
		WHERE s.id = $1 AND s.ruolo = $2 AND s.id_ruolo = $3`,
		rich.Sessione, string(rich.Ruolo), rich.ID).Scan(&aperta, &attivo, &idSede)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return rich, errSessioneChiusa
	case err != nil:
		return rich, err
	case !attivo:
		return rich, errDisattivato
	case !aperta:
		return rich, errSessioneChiusa
	}
	rich.IDSede = idSede
	return rich, nil
}

// ControlloRevoche (servizio spedizioni): la sessione del token non è tra quelle chiuse.
// Il servizio utenti comunica qui ogni sessione che chiude (disattivazione, cambio di
// sede, nuova password, logout) prima di confermare l'operazione, quindi la revoca vale
// subito; il controllo è una lettura nel database delle spedizioni, senza chiamate di rete.
//
// Le revoche sono in ogni gruppo di database (il servizio utenti le manda a tutti): si
// legge quello della sede di chi chiede, il principale per chi non ha una sede.
type ControlloRevoche struct {
	db *database.Gruppi
}

func NuovoControlloRevoche(db *database.Gruppi) *ControlloRevoche {
	return &ControlloRevoche{db: db}
}

func (c *ControlloRevoche) Controlla(ctx context.Context, rich Richiedente) (Richiedente, error) {
	db := c.db.Principale()
	if rich.Ruolo.HaSede() {
		var err error
		if db, err = c.db.PerSede(ctx, rich.IDSede); err != nil {
			return rich, err
		}
	}
	var motivo string
	err := db.QueryRow(ctx, `SELECT motivo FROM sessioni_revocate WHERE id = $1`, rich.Sessione).Scan(&motivo)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return rich, nil
	case err != nil:
		return rich, err
	case motivo == MotivoDisattivato:
		return rich, errDisattivato
	}
	return rich, errSessioneChiusa
}
