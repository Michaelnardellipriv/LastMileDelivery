package autenticazione

import (
	"net/http"
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
	IDUtente int32  `json:"id_utente"`
	Ruolo    string `json:"ruolo"`
	ID       int32  `json:"id"`
	Nome     string `json:"nome"`
	Cognome  string `json:"cognome"`
	Email    string `json:"email"`
}

type sessioneDTO struct {
	Token   string    `json:"token"`
	Tipo    string    `json:"tipo"`
	ScadeIl time.Time `json:"scade_il"`
	Utente  utenteDTO `json:"utente"`
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
	w.Header().Set("Cache-Control", "no-store")
	httpx.ScriviJSON(w, http.StatusOK, sessioneDTO{
		Token: s.Token, Tipo: "Bearer", ScadeIl: s.Scadenza.UTC(),
		Utente: utenteDTO{
			IDUtente: s.Account.IDUtente, Ruolo: string(s.Ruolo.Ruolo), ID: s.Ruolo.ID,
			Nome: s.Account.Nome, Cognome: s.Account.Cognome, Email: s.Account.Email,
		},
	})
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
