package spedizioni

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"lastmiledelivery/backend/internal/errori"
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
	rotte.Gestisci(http.MethodGet, "/api/spedizioni", http.HandlerFunc(h.elenca), identifica)
	rotte.Gestisci(http.MethodPost, "/api/spedizioni", http.HandlerFunc(h.crea), identifica, idempotente)
	rotte.Gestisci(http.MethodPatch, "/api/spedizioni/{id}", http.HandlerFunc(h.modificaDati), identifica, idempotente)
	rotte.Gestisci(http.MethodPost, "/api/spedizioni/{id}/assegnazioni", http.HandlerFunc(h.assegna), identifica, idempotente)
	rotte.Gestisci(http.MethodPost, "/api/spedizioni/{id}/tentativi", http.HandlerFunc(h.registraTentativo), identifica, idempotente)
	rotte.Gestisci(http.MethodPatch, "/api/spedizioni/{id}/stato", http.HandlerFunc(h.cambiaStato), identifica, idempotente)
	rotte.Gestisci(http.MethodGet, "/api/spedizioni/{id}/storico", http.HandlerFunc(h.storico), identifica)
}

func (h *Handler) storico(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.IDDalPercorso(w, r)
	if !ok {
		return
	}
	voci, err := h.servizio.Storico(r.Context(), identita.DalContesto(r.Context()), id)
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	httpx.ScriviJSON(w, http.StatusOK, versoStoricoDTO(voci))
}

func (h *Handler) elenca(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := FiltroElenco{Pagina: 1, PerPagina: PerPaginaPredefinito}
	e := &errori.Validazione{}
	if v := q.Get("stato"); v != "" {
		stato := Stato(v)
		f.Stato = &stato
	}
	intero := func(nome string, dest *int) {
		if v := q.Get(nome); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				e.Aggiungi(nome, "deve essere un numero intero")
				return
			}
			*dest = n
		}
	}
	intero("pagina", &f.Pagina)
	intero("per_pagina", &f.PerPagina)
	if v := q.Get("id_corriere"); v != "" {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			e.Aggiungi("id_corriere", "deve essere un numero intero")
		} else {
			id := int32(n)
			f.IDCorriere = &id
		}
	}
	data := func(nome string) *time.Time {
		v := q.Get(nome)
		if v == "" {
			return nil
		}
		t, err := time.Parse(time.DateOnly, v)
		if err != nil {
			e.Aggiungi(nome, "formato non valido: usare AAAA-MM-GG, es. 2026-11-01")
			return nil
		}
		return &t
	}
	f.Dal, f.Al = data("dal"), data("al")
	if q.Has("cod_ordine") {
		cod := strings.TrimSpace(q.Get("cod_ordine"))
		f.CodOrdine = &cod
	}
	if err := e.Err(); err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}

	voci, err := h.servizio.Elenca(r.Context(), identita.DalContesto(r.Context()), f)
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	httpx.ScriviJSON(w, http.StatusOK, versoElencoDTO(voci, f))
}

func (h *Handler) crea(w http.ResponseWriter, r *http.Request) {
	var req richiestaCrea
	if !httpx.LeggiCorpo(w, r, &req) {
		return
	}
	nuova, e := req.versoDominio()
	if len(e.Dettagli) > 0 {
		for _, regola := range h.servizio.RegoleNuova(nuova) {
			if !e.Segnalato(regola.Campo) {
				e.Aggiungi(regola.Campo, regola.Messaggio)
			}
		}
		httpx.RispondiErrore(w, r, e)
		return
	}

	creata, err := h.servizio.Crea(r.Context(), identita.DalContesto(r.Context()), nuova)
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	httpx.ScriviJSON(w, http.StatusCreated, versoDTO(creata))
}

func (h *Handler) modificaDati(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.IDDalPercorso(w, r)
	if !ok {
		return
	}
	var req richiestaModifica
	if !httpx.LeggiCorpo(w, r, &req) {
		return
	}
	dati, err := req.versoDominio()
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	sped, err := h.servizio.ModificaDati(r.Context(), identita.DalContesto(r.Context()), id, dati)
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	httpx.ScriviJSON(w, http.StatusOK, versoDTO(sped))
}

func (h *Handler) assegna(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.IDDalPercorso(w, r)
	if !ok {
		return
	}
	var req richiestaAssegnazione
	if !httpx.LeggiCorpo(w, r, &req) {
		return
	}
	sped, nuova, err := h.servizio.Assegna(r.Context(), identita.DalContesto(r.Context()), id, req.IDCorriere)
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	stato := http.StatusOK
	if nuova {
		stato = http.StatusCreated
	}
	httpx.ScriviJSON(w, stato, versoDTO(sped))
}

func (h *Handler) registraTentativo(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.IDDalPercorso(w, r)
	if !ok {
		return
	}
	var req richiestaTentativo
	if !httpx.LeggiCorpo(w, r, &req) {
		return
	}
	if req.Tipo == nil {
		httpx.RispondiErrore(w, r, &errori.Validazione{Dettagli: []errori.ErroreCampo{{Campo: "tipo", Messaggio: "obbligatorio"}}})
		return
	}
	t := TentativoFallito{Tipo: TipoTentativo(*req.Tipo), Nota: req.Nota, Motivo: req.Motivo}
	sped, err := h.servizio.RegistraTentativo(r.Context(), identita.DalContesto(r.Context()), id, t)
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	httpx.ScriviJSON(w, http.StatusCreated, versoDTO(sped))
}

func (h *Handler) cambiaStato(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.IDDalPercorso(w, r)
	if !ok {
		return
	}
	var req richiestaStato
	if !httpx.LeggiCorpo(w, r, &req) {
		return
	}
	if req.Stato == nil {
		httpx.RispondiErrore(w, r, &errori.Validazione{Dettagli: []errori.ErroreCampo{{Campo: "stato", Messaggio: "obbligatorio"}}})
		return
	}
	sped, err := h.servizio.CambiaStato(r.Context(), identita.DalContesto(r.Context()), id,
		CambioStato{Stato: Stato(*req.Stato), Nota: req.Nota, Motivo: req.Motivo})
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	httpx.ScriviJSON(w, http.StatusOK, versoDTO(sped))
}
