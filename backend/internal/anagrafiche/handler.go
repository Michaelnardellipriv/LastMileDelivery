package anagrafiche

import (
	"net/http"
	"strconv"
	"strings"

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
	rotte.Gestisci(http.MethodGet, "/api/clienti", http.HandlerFunc(h.cercaClienti), identifica)
	rotte.Gestisci(http.MethodPost, "/api/clienti", http.HandlerFunc(h.creaCliente), identifica, idempotente)
	rotte.Gestisci(http.MethodGet, "/api/corrieri", http.HandlerFunc(h.cercaCorrieri), identifica)
	rotte.Gestisci(http.MethodGet, "/api/indirizzi", http.HandlerFunc(h.cercaIndirizzi), identifica)
	rotte.Gestisci(http.MethodPost, "/api/indirizzi", http.HandlerFunc(h.trovaOCreaIndirizzo), identifica, idempotente)
}

type richiestaCliente struct {
	Nome       *string `json:"nome"`
	Cognome    *string `json:"cognome"`
	Prefisso   *string `json:"prefisso"`
	NrTelefono *string `json:"nr_telefono"`
	Email      *string `json:"email"`
	Password   *string `json:"password"`
}

type richiestaIndirizzo struct {
	Via     *string `json:"via"`
	Civico  *string `json:"civico"`
	Citta   *string `json:"citta"`
	CAP     *string `json:"cap"`
	Regione *string `json:"regione"`
	Nazione *string `json:"nazione"`
}

type clienteDTO struct {
	ID         int32  `json:"id"`
	IDUtente   int32  `json:"id_utente"`
	Nome       string `json:"nome"`
	Cognome    string `json:"cognome"`
	Email      string `json:"email"`
	Prefisso   string `json:"prefisso"`
	NrTelefono string `json:"nr_telefono"`
}

type esitoClienteDTO struct {
	Cliente     clienteDTO `json:"cliente"`
	MailInviata bool       `json:"mail_inviata"`
}

type corriereDTO struct {
	ID         int32  `json:"id"`
	IDUtente   int32  `json:"id_utente"`
	Nome       string `json:"nome"`
	Cognome    string `json:"cognome"`
	Email      string `json:"email"`
	Prefisso   string `json:"prefisso"`
	NrTelefono string `json:"nr_telefono"`
}

type indirizzoDTO struct {
	ID      int32  `json:"id"`
	Via     string `json:"via"`
	Civico  string `json:"civico"`
	Citta   string `json:"citta"`
	CAP     string `json:"cap"`
	Regione string `json:"regione"`
	Nazione string `json:"nazione"`
}

type pagina[T any] struct {
	Dati      []T `json:"dati"`
	Pagina    int `json:"pagina"`
	PerPagina int `json:"per_pagina"`
}

func versoClienteDTO(c Cliente) clienteDTO {
	return clienteDTO{ID: c.ID, IDUtente: c.IDUtente, Nome: c.Nome, Cognome: c.Cognome,
		Email: c.Email, Prefisso: c.Prefisso, NrTelefono: c.NrTelefono}
}

func versoIndirizzoDTO(i Indirizzo) indirizzoDTO {
	return indirizzoDTO{ID: i.ID, Via: i.Via, Civico: i.Civico, Citta: i.Citta, CAP: i.CAP, Regione: i.Regione, Nazione: i.Nazione}
}

func leggiRicerca(w http.ResponseWriter, r *http.Request) (Ricerca, bool) {
	q := r.URL.Query()
	ric := Ricerca{Testo: strings.TrimSpace(q.Get("q")), Pagina: 1, PerPagina: PerPaginaPredefinito}
	e := &errori.Validazione{}
	for _, c := range []struct {
		nome string
		dest *int
	}{{"pagina", &ric.Pagina}, {"per_pagina", &ric.PerPagina}} {
		if v := q.Get(c.nome); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				e.Aggiungi(c.nome, "deve essere un numero intero")
				continue
			}
			*c.dest = n
		}
	}
	if err := e.Err(); err != nil {
		httpx.RispondiErrore(w, r, err)
		return Ricerca{}, false
	}
	return ric, true
}

func (h *Handler) cercaClienti(w http.ResponseWriter, r *http.Request) {
	ric, ok := leggiRicerca(w, r)
	if !ok {
		return
	}
	clienti, err := h.servizio.CercaClienti(r.Context(), identita.DalContesto(r.Context()), ric)
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	risposta := pagina[clienteDTO]{Dati: make([]clienteDTO, len(clienti)), Pagina: ric.Pagina, PerPagina: ric.PerPagina}
	for i, c := range clienti {
		risposta.Dati[i] = versoClienteDTO(c)
	}
	httpx.ScriviJSON(w, http.StatusOK, risposta)
}

func (h *Handler) creaCliente(w http.ResponseWriter, r *http.Request) {
	var req richiestaCliente
	if !httpx.LeggiCorpo(w, r, &req) {
		return
	}
	esito, err := h.servizio.CreaCliente(r.Context(), identita.DalContesto(r.Context()), NuovoCliente{
		Nome: req.Nome, Cognome: req.Cognome, Prefisso: req.Prefisso,
		NrTelefono: req.NrTelefono, Email: req.Email, Password: req.Password,
	})
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	httpx.ScriviJSON(w, http.StatusCreated, esitoClienteDTO{
		Cliente: versoClienteDTO(esito.Cliente), MailInviata: esito.MailInviata,
	})
}

func (h *Handler) cercaCorrieri(w http.ResponseWriter, r *http.Request) {
	ric, ok := leggiRicerca(w, r)
	if !ok {
		return
	}
	corrieri, err := h.servizio.CercaCorrieri(r.Context(), identita.DalContesto(r.Context()), ric)
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	risposta := pagina[corriereDTO]{Dati: make([]corriereDTO, len(corrieri)), Pagina: ric.Pagina, PerPagina: ric.PerPagina}
	for i, c := range corrieri {
		risposta.Dati[i] = corriereDTO{ID: c.ID, IDUtente: c.IDUtente, Nome: c.Nome, Cognome: c.Cognome,
			Email: c.Email, Prefisso: c.Prefisso, NrTelefono: c.NrTelefono}
	}
	httpx.ScriviJSON(w, http.StatusOK, risposta)
}

func (h *Handler) cercaIndirizzi(w http.ResponseWriter, r *http.Request) {
	ric, ok := leggiRicerca(w, r)
	if !ok {
		return
	}
	indirizzi, err := h.servizio.CercaIndirizzi(r.Context(), identita.DalContesto(r.Context()), ric)
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	risposta := pagina[indirizzoDTO]{Dati: make([]indirizzoDTO, len(indirizzi)), Pagina: ric.Pagina, PerPagina: ric.PerPagina}
	for i, ind := range indirizzi {
		risposta.Dati[i] = versoIndirizzoDTO(ind)
	}
	httpx.ScriviJSON(w, http.StatusOK, risposta)
}

func (h *Handler) trovaOCreaIndirizzo(w http.ResponseWriter, r *http.Request) {
	var req richiestaIndirizzo
	if !httpx.LeggiCorpo(w, r, &req) {
		return
	}
	ind, creato, err := h.servizio.TrovaOCreaIndirizzo(r.Context(), identita.DalContesto(r.Context()), NuovoIndirizzo{
		Via: req.Via, Civico: req.Civico, Citta: req.Citta, CAP: req.CAP, Regione: req.Regione, Nazione: req.Nazione,
	})
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	stato := http.StatusOK
	if creato {
		stato = http.StatusCreated
	}
	httpx.ScriviJSON(w, stato, versoIndirizzoDTO(ind))
}
