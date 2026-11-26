package autenticazione

import (
	"net/http"
	"strings"
	"time"

	"lastmiledelivery/backend/internal/httpx"
	"lastmiledelivery/backend/internal/identita"
)

type Handler struct {
	servizio *Servizio
}

func NuovoHandler(s *Servizio) *Handler {
	return &Handler{servizio: s}
}

func (h *Handler) Registra(rotte *httpx.Rotte, identifica httpx.Middleware) {
	rotte.Gestisci(http.MethodPost, "/api/auth/login", http.HandlerFunc(h.login))
	// senza il middleware di identità: il rinnovo accetta anche un token appena scaduto
	// e fa da sé i controlli (Servizio.Rinnova)
	rotte.Gestisci(http.MethodPost, "/api/auth/rinnova", http.HandlerFunc(h.rinnova))
	// anche il logout accetta un token appena scaduto (Servizio.Esci)
	rotte.Gestisci(http.MethodPost, "/api/auth/esci", http.HandlerFunc(h.esci))
	rotte.Gestisci(http.MethodPost, "/api/auth/cambio-password", http.HandlerFunc(h.cambiaPassword), identifica)
	rotte.Gestisci(http.MethodPost, "/api/auth/recupero-password", http.HandlerFunc(h.richiediRecupero))
	rotte.Gestisci(http.MethodPost, "/api/auth/reimposta-password", http.HandlerFunc(h.reimpostaPassword))
}

type richiestaLogin struct {
	Email    *string `json:"email"`
	Password *string `json:"password"`
	Ruolo    *string `json:"ruolo"`
}

type richiestaCambio struct {
	PasswordAttuale *string `json:"password_attuale"`
	NuovaPassword   *string `json:"nuova_password"`
}

type richiestaRecupero struct {
	Email *string `json:"email"`
}

type richiestaReimposta struct {
	Token         *string `json:"token"`
	NuovaPassword *string `json:"nuova_password"`
}

type utenteDTO struct {
	IDUtente int64  `json:"id_utente"`
	Ruolo    string `json:"ruolo"`
	ID       int64  `json:"id"`
	// sede di operatori e corrieri; assente per clienti e amministratori
	IDSede  *int64 `json:"id_sede,omitempty"`
	Nome    string `json:"nome"`
	Cognome string `json:"cognome"`
	Email   string `json:"email"`
}

type sessioneDTO struct {
	Token   string    `json:"token"`
	Tipo    string    `json:"tipo"`
	ScadeIl time.Time `json:"scade_il"`
	// durata del token in secondi: l'app programma il rinnovo con il proprio orologio,
	// senza confrontare scade_il con un orologio del dispositivo che può essere sbagliato
	DurataSecondi int       `json:"durata_secondi"`
	Utente        utenteDTO `json:"utente"`
}

type messaggioDTO struct {
	Messaggio string `json:"messaggio"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req richiestaLogin
	if !httpx.LeggiCorpo(w, r, &req) {
		return
	}
	s, err := h.servizio.Login(r.Context(), req.Email, req.Password, req.Ruolo)
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	scriviSessione(w, s)
}

// rinnova: nuovo token a partire da quello in Authorization (anche appena scaduto).
func (h *Handler) rinnova(w http.ResponseWriter, r *http.Request) {
	token, ok := tokenDaRichiesta(w, r)
	if !ok {
		return
	}
	s, err := h.servizio.Rinnova(r.Context(), token)
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	scriviSessione(w, s)
}

// tokenDaRichiesta: il token in Authorization; se manca risponde 401.
func tokenDaRichiesta(w http.ResponseWriter, r *http.Request) (string, bool) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || strings.TrimSpace(token) == "" {
		w.Header().Set("WWW-Authenticate", "Bearer")
		httpx.ScriviErrore(w, http.StatusUnauthorized, "serve il token: Authorization: Bearer <token>")
		return "", false
	}
	return strings.TrimSpace(token), true
}

func scriviSessione(w http.ResponseWriter, s Sessione) {
	w.Header().Set("Cache-Control", "no-store")
	var idSede *int64
	if s.Ruolo.Ruolo.HaSede() {
		idSede = &s.Ruolo.IDSede
	}
	httpx.ScriviJSON(w, http.StatusOK, sessioneDTO{
		Token: s.Token, Tipo: "Bearer", ScadeIl: s.Scadenza.UTC(), DurataSecondi: int(time.Until(s.Scadenza).Round(time.Second).Seconds()),
		Utente: utenteDTO{
			IDUtente: s.Account.IDUtente, Ruolo: string(s.Ruolo.Ruolo), ID: s.Ruolo.ID, IDSede: idSede,
			Nome: s.Account.Nome, Cognome: s.Account.Cognome, Email: s.Account.Email,
		},
	})
}

// esci: logout, la sessione del token si chiude e il token non vale più.
func (h *Handler) esci(w http.ResponseWriter, r *http.Request) {
	token, ok := tokenDaRichiesta(w, r)
	if !ok {
		return
	}
	if err := h.servizio.Esci(r.Context(), token); err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) cambiaPassword(w http.ResponseWriter, r *http.Request) {
	var req richiestaCambio
	if !httpx.LeggiCorpo(w, r, &req) {
		return
	}
	if err := h.servizio.CambiaPassword(r.Context(), identita.DalContesto(r.Context()), req.PasswordAttuale, req.NuovaPassword); err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) richiediRecupero(w http.ResponseWriter, r *http.Request) {
	var req richiestaRecupero
	if !httpx.LeggiCorpo(w, r, &req) {
		return
	}
	if err := h.servizio.RichiediRecupero(r.Context(), req.Email); err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	httpx.ScriviJSON(w, http.StatusAccepted, messaggioDTO{
		Messaggio: "se l'email è registrata riceverai un link per scegliere una nuova password, valido 30 minuti",
	})
}

func (h *Handler) reimpostaPassword(w http.ResponseWriter, r *http.Request) {
	var req richiestaReimposta
	if !httpx.LeggiCorpo(w, r, &req) {
		return
	}
	if err := h.servizio.ReimpostaPassword(r.Context(), req.Token, req.NuovaPassword); err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
