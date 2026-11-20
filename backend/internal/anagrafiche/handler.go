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
	rotte.Gestisci(http.MethodGet, "/api/utenti", http.HandlerFunc(h.cercaUtenti), identifica)
	rotte.Gestisci(http.MethodPost, "/api/utenti", http.HandlerFunc(h.creaUtente), identifica, idempotente)
	rotte.Gestisci(http.MethodPatch, "/api/utenti/{id}", http.HandlerFunc(h.modificaUtente), identifica, idempotente)
}

type richiestaCliente struct {
	Nome       *string `json:"nome"`
	Cognome    *string `json:"cognome"`
	Prefisso   *string `json:"prefisso"`
	NrTelefono *string `json:"nr_telefono"`
	Email      *string `json:"email"`
	Password   *string `json:"password"`
}

type richiestaUtente struct {
	Ruolo *string `json:"ruolo"`
	richiestaCliente
}

type richiestaModificaUtente struct {
	richiestaCliente
	Attivo *bool `json:"attivo"`
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
	Attivo     bool   `json:"attivo"`
	// true se ha ricevuto almeno una spedizione, false se è un mittente (anche appena creato)
	Destinatario bool `json:"destinatario"`
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

type utenteDTO struct {
	ID         int32  `json:"id"`
	Ruolo      string `json:"ruolo"`
	IDRuolo    int32  `json:"id_ruolo"`
	Nome       string `json:"nome"`
	Cognome    string `json:"cognome"`
	Email      string `json:"email"`
	Prefisso   string `json:"prefisso"`
	NrTelefono string `json:"nr_telefono"`
	Attivo     bool   `json:"attivo"`
	// solo per i clienti: true se è stato destinatario di almeno una spedizione
	Destinatario *bool `json:"destinatario"`
}

type esitoUtenteDTO struct {
	Utente      utenteDTO `json:"utente"`
	MailInviata bool      `json:"mail_inviata"`
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
	Dati           []T `json:"dati"`
	Pagina         int `json:"pagina"`
	PerPagina      int `json:"per_pagina"`
	TotaleElementi int `json:"totale_elementi"`
	TotalePagine   int `json:"totale_pagine"`
}

func nuovaPagina[T any](n int, ric Ricerca, totale int) pagina[T] {
	pagine := 0
	if totale > 0 && ric.PerPagina > 0 {
		pagine = (totale + ric.PerPagina - 1) / ric.PerPagina
	}
	return pagina[T]{Dati: make([]T, n), Pagina: ric.Pagina, PerPagina: ric.PerPagina, TotaleElementi: totale, TotalePagine: pagine}
}

func versoClienteDTO(c Cliente) clienteDTO {
	return clienteDTO{ID: c.ID, IDUtente: c.IDUtente, Nome: c.Nome, Cognome: c.Cognome,
		Email: c.Email, Prefisso: c.Prefisso, NrTelefono: c.NrTelefono, Attivo: c.Attivo, Destinatario: c.Destinatario}
}

func versoUtenteDTO(u Utente) utenteDTO {
	return utenteDTO{ID: u.ID, Ruolo: string(u.Ruolo), IDRuolo: u.IDRuolo, Nome: u.Nome, Cognome: u.Cognome,
		Email: u.Email, Prefisso: u.Prefisso, NrTelefono: u.NrTelefono, Attivo: u.Attivo, Destinatario: u.Destinatario}
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
	clienti, totale, err := h.servizio.CercaClienti(r.Context(), identita.DalContesto(r.Context()), ric)
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	risposta := nuovaPagina[clienteDTO](len(clienti), ric, totale)
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
	corrieri, totale, err := h.servizio.CercaCorrieri(r.Context(), identita.DalContesto(r.Context()), ric)
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	risposta := nuovaPagina[corriereDTO](len(corrieri), ric, totale)
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
	indirizzi, totale, err := h.servizio.CercaIndirizzi(r.Context(), identita.DalContesto(r.Context()), ric)
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	risposta := nuovaPagina[indirizzoDTO](len(indirizzi), ric, totale)
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

func (h *Handler) cercaUtenti(w http.ResponseWriter, r *http.Request) {
	ric, ok := leggiRicerca(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	var ruolo *string
	if v := q.Get("ruolo"); v != "" {
		ruolo = &v
	}
	var attivo *bool
	if v := q.Get("attivo"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			httpx.ScriviErrore(w, http.StatusBadRequest, "dati non validi", errori.ErroreCampo{Campo: "attivo", Messaggio: "ammessi true, false"})
			return
		}
		attivo = &b
	}
	utenti, totale, err := h.servizio.CercaUtenti(r.Context(), identita.DalContesto(r.Context()), ric, ruolo, attivo)
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	risposta := nuovaPagina[utenteDTO](len(utenti), ric, totale)
	for i, u := range utenti {
		risposta.Dati[i] = versoUtenteDTO(u)
	}
	httpx.ScriviJSON(w, http.StatusOK, risposta)
}

func (h *Handler) creaUtente(w http.ResponseWriter, r *http.Request) {
	var req richiestaUtente
	if !httpx.LeggiCorpo(w, r, &req) {
		return
	}
	esito, err := h.servizio.CreaUtente(r.Context(), identita.DalContesto(r.Context()), NuovoUtente{
		Ruolo: req.Ruolo,
		NuovoCliente: NuovoCliente{
			Nome: req.Nome, Cognome: req.Cognome, Prefisso: req.Prefisso,
			NrTelefono: req.NrTelefono, Email: req.Email, Password: req.Password,
		},
	})
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	httpx.ScriviJSON(w, http.StatusCreated, esitoUtenteDTO{Utente: versoUtenteDTO(esito.Utente), MailInviata: esito.MailInviata})
}

func (h *Handler) modificaUtente(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.IDDalPercorso(w, r)
	if !ok {
		return
	}
	var req richiestaModificaUtente
	if !httpx.LeggiCorpo(w, r, &req) {
		return
	}
	u, err := h.servizio.ModificaUtente(r.Context(), identita.DalContesto(r.Context()), id, ModificaUtente{
		Nome: req.Nome, Cognome: req.Cognome, Prefisso: req.Prefisso, NrTelefono: req.NrTelefono,
		Email: req.Email, Password: req.Password, Attivo: req.Attivo,
	})
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	httpx.ScriviJSON(w, http.StatusOK, versoUtenteDTO(u))
}
