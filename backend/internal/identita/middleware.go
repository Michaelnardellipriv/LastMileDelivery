package identita

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"lastmiledelivery/backend/internal/audit"
	"lastmiledelivery/backend/internal/httpx"
)

type verificaUtente interface {
	Stato(ctx context.Context, ruolo Ruolo, id int64) (esiste, attivo bool, idSede int64, err error)
}

const messaggioDisattivato = "account disattivato: contattare l'amministratore"

type verificaToken interface {
	Verifica(token string) (Richiedente, error)
}

var intestazioni = []struct {
	nome  string
	ruolo Ruolo
}{
	{"X-Id-Operatore", Operatore},
	{"X-Id-Corriere", Corriere},
	{"X-Id-Cliente", Cliente},
	{"X-Id-Amministratore", Amministratore},
}

func nonAutenticato(w http.ResponseWriter, messaggio string) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	httpx.ScriviErrore(w, http.StatusUnauthorized, messaggio)
}

// controlloSessione: il token è firmato, ma vale solo se la sua sessione è ancora aperta
// nel database. Restituisce il richiedente con i dati attuali (sede) o un errore: 401 se la
// sessione è chiusa, scaduta o non esiste, 403 se l'utente è disattivato.
type controlloSessione interface {
	Controlla(ctx context.Context, r Richiedente) (Richiedente, error)
}

// Richiedi identifica chi chiama e lo controlla nel database a ogni richiesta, così una
// revoca vale subito:
//   - con un token: firma e scadenza, poi la sessione (sessioni). Sul servizio utenti la
//     sessione si legge nel database globale (Repository.Controlla: aperta, utente attivo,
//     sede attuale); sul servizio spedizioni nel suo database (ControlloRevoche: la sessione
//     non è tra quelle chiuse, che il servizio utenti gli comunica prima di confermare
//     disattivazioni, cambi di sede, nuove password e logout);
//   - con un header X-Id-* (solo sviluppo): l'utente deve esistere ed essere attivo (utenti).
func Richiedi(sessioni controlloSessione, utenti verificaUtente, token verificaToken, ammettiHeaderID bool) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if autorizzazione := r.Header.Get("Authorization"); autorizzazione != "" {
				valore, ok := strings.CutPrefix(autorizzazione, "Bearer ")
				if !ok || strings.TrimSpace(valore) == "" {
					nonAutenticato(w, "intestazione Authorization non valida: usare Bearer <token>")
					return
				}
				rich, err := token.Verifica(strings.TrimSpace(valore))
				if err != nil {
					nonAutenticato(w, "token non valido o scaduto: rifare il login")
					return
				}
				if rich.Sessione == "" {
					nonAutenticato(w, "token senza sessione: rifare il login")
					return
				}
				rich, err = sessioni.Controlla(r.Context(), rich)
				if err != nil {
					audit.Evento(audit.ConUtente(r.Context(), rich.String()), "accesso_rifiutato",
						"percorso", r.URL.Path, "motivo", err.Error())
					httpx.RispondiErrore(w, r, err)
					return
				}
				next.ServeHTTP(w, r.WithContext(NelContesto(r.Context(), rich)))
				return
			}
			if !ammettiHeaderID {
				nonAutenticato(w, "utente non identificato: fare il login e usare Authorization: Bearer <token>")
				return
			}

			var rich Richiedente
			var valore string
			trovate := 0
			for _, h := range intestazioni {
				if v := r.Header.Get(h.nome); v != "" {
					trovate++
					rich.Ruolo, valore = h.ruolo, v
				}
			}
			switch {
			case trovate > 1:
				httpx.ScriviErrore(w, http.StatusBadRequest,
					"indicare una sola identità tra X-Id-Operatore, X-Id-Corriere, X-Id-Cliente e X-Id-Amministratore")
				return
			case trovate == 0:
				nonAutenticato(w, "utente non identificato")
				return
			}

			id, err := strconv.ParseInt(valore, 10, 64)
			if err != nil || id <= 0 {
				nonAutenticato(w, "utente non identificato")
				return
			}
			rich.ID = id

			rich, ok := utenteAttivo(w, r, utenti, rich)
			if !ok {
				return
			}

			next.ServeHTTP(w, r.WithContext(NelContesto(r.Context(), rich)))
		})
	}
}

// utenteAttivo controlla che l'utente esista e sia attivo e gli assegna la sede che ha
// adesso nel database (quella del token potrebbe essere vecchia).
func utenteAttivo(w http.ResponseWriter, r *http.Request, utenti verificaUtente, rich Richiedente) (Richiedente, bool) {
	esiste, attivo, idSede, err := utenti.Stato(r.Context(), rich.Ruolo, rich.ID)
	switch {
	case err != nil:
		httpx.RispondiErrore(w, r, err)
	case !esiste:
		nonAutenticato(w, "utente non riconosciuto")
	case !attivo:
		httpx.ScriviErrore(w, http.StatusForbidden, messaggioDisattivato)
	}
	rich.IDSede = idSede
	return rich, err == nil && esiste && attivo
}
