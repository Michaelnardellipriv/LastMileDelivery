package spedizioni

import (
	"encoding/json"
	"net/http"
	"strconv"

	"lastmiledelivery/backend/internal/httpx"
	"lastmiledelivery/backend/internal/interno"
)

// HandlerInterno: le rotte interne del servizio spedizioni, chiamate solo dal servizio
// utenti (non passano dall'Ingress).
type HandlerInterno struct {
	repo *Repository
}

func NuovoHandlerInterno(repo *Repository) *HandlerInterno {
	return &HandlerInterno{repo: repo}
}

func (h *HandlerInterno) Registra(rotte *httpx.Rotte, protetto httpx.Middleware) {
	rotte.Gestisci(http.MethodGet, interno.PercorsoSpedizioniAperte, http.HandlerFunc(h.spedizioniAperte), protetto)
	rotte.Gestisci(http.MethodPost, interno.PercorsoSessioniRevocate, http.HandlerFunc(h.sessioniRevocate), protetto)
	rotte.Gestisci(http.MethodGet, interno.PercorsoGruppi, http.HandlerFunc(h.gruppi), protetto)
}

// gruppi: i gruppi di sedi che qui hanno un database delle spedizioni. Il servizio utenti
// li chiede prima di creare una sede in un gruppo diverso dal principale.
func (h *HandlerInterno) gruppi(w http.ResponseWriter, _ *http.Request) {
	httpx.ScriviJSON(w, http.StatusOK, interno.Gruppi{Codici: h.repo.Gruppi()})
}

// sessioniRevocate: il servizio utenti ha chiuso queste sessioni; da adesso i loro token
// qui non valgono più (identita.ControlloRevoche).
func (h *HandlerInterno) sessioniRevocate(w http.ResponseWriter, r *http.Request) {
	var ric interno.RichiestaRevoca
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&ric); err != nil {
		httpx.ScriviErrore(w, http.StatusBadRequest, "richiesta non valida")
		return
	}
	if err := h.repo.RegistraSessioniRevocate(r.Context(), ric.Sessioni); err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *HandlerInterno) spedizioniAperte(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.ScriviErrore(w, http.StatusBadRequest, "id non valido")
		return
	}
	aperte, err := h.repo.CorriereHaSpedizioniAperte(r.Context(), id)
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	httpx.ScriviJSON(w, http.StatusOK, interno.SpedizioniAperte{Aperte: aperte})
}
