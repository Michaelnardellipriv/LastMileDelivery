package posizioni

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
	rotte.Gestisci(http.MethodPost, "/api/corrieri/posizione", http.HandlerFunc(h.invia), identifica)
	rotte.Gestisci(http.MethodGet, "/api/corrieri/posizioni", http.HandlerFunc(h.elenca), identifica)
	rotte.Gestisci(http.MethodGet, "/api/impostazioni/intervallo-posizione", http.HandlerFunc(h.intervallo), identifica)
	rotte.Gestisci(http.MethodPut, "/api/impostazioni/intervallo-posizione", http.HandlerFunc(h.modificaIntervallo), identifica)
}

type richiestaPosizione struct {
	Latitudine  *float64 `json:"latitudine"`
	Longitudine *float64 `json:"longitudine"`
}

type richiestaIntervallo struct {
	IntervalloSecondi *int32 `json:"intervallo_secondi"`
}

type intervalloDTO struct {
	IntervalloSecondi int32 `json:"intervallo_secondi"`
}

type posizioneDTO struct {
	IDCorriere  int32     `json:"id_corriere"`
	Nome        string    `json:"nome"`
	Cognome     string    `json:"cognome"`
	Latitudine  float64   `json:"latitudine"`
	Longitudine float64   `json:"longitudine"`
	RilevataIl  time.Time `json:"rilevata_il"`
	Spedizioni  []int32   `json:"spedizioni_attive"`
}

type rispostaPosizioni struct {
	IntervalloSecondi int32          `json:"intervallo_secondi"`
	Corrieri          []posizioneDTO `json:"corrieri"`
}

func (h *Handler) invia(w http.ResponseWriter, r *http.Request) {
	var req richiestaPosizione
	if !httpx.LeggiCorpo(w, r, &req) {
		return
	}
	intervallo, err := h.servizio.Invia(r.Context(), identita.DalContesto(r.Context()), req.Latitudine, req.Longitudine)
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	httpx.ScriviJSON(w, http.StatusCreated, intervalloDTO{IntervalloSecondi: intervallo})
}

func (h *Handler) elenca(w http.ResponseWriter, r *http.Request) {
	intervallo, posizioni, err := h.servizio.Elenca(r.Context(), identita.DalContesto(r.Context()))
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	risposta := rispostaPosizioni{IntervalloSecondi: intervallo, Corrieri: make([]posizioneDTO, len(posizioni))}
	for i, p := range posizioni {
		risposta.Corrieri[i] = posizioneDTO{
			IDCorriere: p.IDCorriere, Nome: p.Nome, Cognome: p.Cognome,
			Latitudine: p.Latitudine, Longitudine: p.Longitudine,
			RilevataIl: p.RilevataIl.UTC(), Spedizioni: p.Spedizioni,
		}
	}
	httpx.ScriviJSON(w, http.StatusOK, risposta)
}

func (h *Handler) intervallo(w http.ResponseWriter, r *http.Request) {
	intervallo, err := h.servizio.Intervallo(r.Context(), identita.DalContesto(r.Context()))
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	httpx.ScriviJSON(w, http.StatusOK, intervalloDTO{IntervalloSecondi: intervallo})
}

func (h *Handler) modificaIntervallo(w http.ResponseWriter, r *http.Request) {
	var req richiestaIntervallo
	if !httpx.LeggiCorpo(w, r, &req) {
		return
	}
	intervallo, err := h.servizio.ModificaIntervallo(r.Context(), identita.DalContesto(r.Context()), req.IntervalloSecondi)
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	httpx.ScriviJSON(w, http.StatusOK, intervalloDTO{IntervalloSecondi: intervallo})
}
