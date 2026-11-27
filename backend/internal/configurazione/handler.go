package configurazione

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

func (h *Handler) Registra(rotte *httpx.Rotte, identifica, idempotente httpx.Middleware) {
	// ogni sede ha le sue impostazioni: l'amministratore sceglie la sede e poi i parametri
	rotte.Gestisci(http.MethodGet, "/api/sedi/{id}/impostazioni", http.HandlerFunc(h.impostazioni), identifica)
	rotte.Gestisci(http.MethodPatch, "/api/sedi/{id}/impostazioni", http.HandlerFunc(h.modificaImpostazioni), identifica, idempotente)
	rotte.Gestisci(http.MethodGet, "/api/impostazioni/motivi", http.HandlerFunc(h.motivi), identifica)
	rotte.Gestisci(http.MethodPost, "/api/impostazioni/motivi", http.HandlerFunc(h.creaMotivo), identifica, idempotente)
	rotte.Gestisci(http.MethodPatch, "/api/impostazioni/motivi/{id}", http.HandlerFunc(h.modificaMotivo), identifica, idempotente)
	rotte.Gestisci(http.MethodGet, "/api/motivi", http.HandlerFunc(h.motiviAttivi), identifica)
}

type richiestaImpostazioni struct {
	GiorniVisibilitaCliente *int `json:"giorni_visibilita_cliente"`
	MaxTentativiRitiro      *int `json:"max_tentativi_ritiro"`
	MaxTentativiConsegna    *int `json:"max_tentativi_consegna"`
	IntervalloPosizioneSec  *int `json:"intervallo_posizione_sec"`
}

type impostazioniDTO struct {
	IDSede                   int64     `json:"id_sede"`
	GiorniVisibilitaCliente  int       `json:"giorni_visibilita_cliente"`
	MaxTentativiRitiro       int       `json:"max_tentativi_ritiro"`
	MaxTentativiConsegna     int       `json:"max_tentativi_consegna"`
	IntervalloPosizioneSec   int       `json:"intervallo_posizione_sec"`
	ModificateIl             time.Time `json:"modificate_il"`
	IDAmministratoreModifica *int64    `json:"id_amministratore_modifica"`
}

type richiestaMotivo struct {
	Descrizione *string `json:"descrizione"`
}

type richiestaStatoMotivo struct {
	Attivo *bool `json:"attivo"`
}

type motivoDTO struct {
	ID          int64     `json:"id"`
	Descrizione string    `json:"descrizione"`
	Attivo      bool      `json:"attivo"`
	CreatoIl    time.Time `json:"creato_il"`
}

type elenco[T any] struct {
	Dati []T `json:"dati"`
}

func versoImpostazioniDTO(i Impostazioni) impostazioniDTO {
	return impostazioniDTO{
		IDSede: i.IDSede, GiorniVisibilitaCliente: i.GiorniVisibilitaCliente, MaxTentativiRitiro: i.MaxTentativiRitiro,
		MaxTentativiConsegna: i.MaxTentativiConsegna, IntervalloPosizioneSec: i.IntervalloPosizioneSec,
		ModificateIl: i.ModificateIl.UTC(), IDAmministratoreModifica: i.IDAmministratoreModifica,
	}
}

func versoMotivoDTO(m Motivo) motivoDTO {
	return motivoDTO{ID: m.ID, Descrizione: m.Descrizione, Attivo: m.Attivo, CreatoIl: m.CreatoIl.UTC()}
}

func (h *Handler) impostazioni(w http.ResponseWriter, r *http.Request) {
	idSede, ok := httpx.IDDalPercorso(w, r)
	if !ok {
		return
	}
	imp, err := h.servizio.Impostazioni(r.Context(), identita.DalContesto(r.Context()), idSede)
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	httpx.ScriviJSON(w, http.StatusOK, versoImpostazioniDTO(imp))
}

func (h *Handler) modificaImpostazioni(w http.ResponseWriter, r *http.Request) {
	idSede, ok := httpx.IDDalPercorso(w, r)
	if !ok {
		return
	}
	var req richiestaImpostazioni
	if !httpx.LeggiCorpo(w, r, &req) {
		return
	}
	imp, err := h.servizio.ModificaImpostazioni(r.Context(), identita.DalContesto(r.Context()), idSede, ModificaImpostazioni(req))
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	httpx.ScriviJSON(w, http.StatusOK, versoImpostazioniDTO(imp))
}

// motiviAttivi restituisce l'elenco dei motivi pubblici ammessi: i client lo
// mostrano in un menu a tendina invece di far scrivere il motivo a mano.
func (h *Handler) motiviAttivi(w http.ResponseWriter, r *http.Request) {
	motivi, err := h.servizio.MotiviAttivi(r.Context())
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	httpx.ScriviJSON(w, http.StatusOK, elenco[string]{Dati: motivi})
}

func (h *Handler) motivi(w http.ResponseWriter, r *http.Request) {
	motivi, err := h.servizio.Motivi(r.Context(), identita.DalContesto(r.Context()))
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	risposta := elenco[motivoDTO]{Dati: make([]motivoDTO, len(motivi))}
	for i, m := range motivi {
		risposta.Dati[i] = versoMotivoDTO(m)
	}
	httpx.ScriviJSON(w, http.StatusOK, risposta)
}

func (h *Handler) creaMotivo(w http.ResponseWriter, r *http.Request) {
	var req richiestaMotivo
	if !httpx.LeggiCorpo(w, r, &req) {
		return
	}
	m, err := h.servizio.CreaMotivo(r.Context(), identita.DalContesto(r.Context()), req.Descrizione)
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	httpx.ScriviJSON(w, http.StatusCreated, versoMotivoDTO(m))
}

func (h *Handler) modificaMotivo(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.IDDalPercorso(w, r)
	if !ok {
		return
	}
	var req richiestaStatoMotivo
	if !httpx.LeggiCorpo(w, r, &req) {
		return
	}
	m, err := h.servizio.ImpostaMotivoAttivo(r.Context(), identita.DalContesto(r.Context()), id, req.Attivo)
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	httpx.ScriviJSON(w, http.StatusOK, versoMotivoDTO(m))
}
