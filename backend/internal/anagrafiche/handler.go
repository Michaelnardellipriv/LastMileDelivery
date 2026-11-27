package anagrafiche

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"lastmiledelivery/backend/internal/audit"
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
	rotte.Gestisci(http.MethodGet, "/api/utenti", http.HandlerFunc(h.cercaUtenti), identifica)
	rotte.Gestisci(http.MethodPost, "/api/utenti", http.HandlerFunc(h.creaUtente), identifica, idempotente)
	rotte.Gestisci(http.MethodPatch, "/api/utenti/{id}", http.HandlerFunc(h.modificaUtente), identifica, idempotente)
	rotte.Gestisci(http.MethodGet, "/api/sedi", http.HandlerFunc(h.cercaSedi), identifica)
	rotte.Gestisci(http.MethodPost, "/api/sedi", http.HandlerFunc(h.creaSede), identifica, idempotente)
	rotte.Gestisci(http.MethodGet, "/api/sedi/{id}", http.HandlerFunc(h.sede), identifica)
	rotte.Gestisci(http.MethodPatch, "/api/sedi/{id}", http.HandlerFunc(h.modificaSede), identifica, idempotente)
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
	Ruolo  *string `json:"ruolo"`
	IDSede *int64  `json:"id_sede"`
	richiestaCliente
}

type richiestaModificaUtente struct {
	richiestaCliente
	Attivo *bool  `json:"attivo"`
	IDSede *int64 `json:"id_sede"`
	// il ruolo non si cambia: se c'è, la richiesta è rifiutata con un errore sul campo
	// (e nel database un trigger impedisce comunque di spostare un utente di ruolo)
	Ruolo *string `json:"ruolo"`
}

type richiestaSede struct {
	Nome      *string            `json:"nome"`
	Indirizzo richiestaIndirizzo `json:"indirizzo"`
	// database delle spedizioni della sede (facoltativo, 0 = il principale)
	GruppoSpedizioni *int `json:"gruppo_spedizioni"`
}

// Modifica di una sede: solo i campi presenti. L'indirizzo, se c'è, va dato intero e
// deve essere nella città della sede.
type richiestaModificaSede struct {
	Nome      *string             `json:"nome"`
	Indirizzo *richiestaIndirizzo `json:"indirizzo"`
	Attiva    *bool               `json:"attiva"`
	// non si cambia: c'è solo per rispondere 400 con un messaggio chiaro
	GruppoSpedizioni *int `json:"gruppo_spedizioni"`
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
	ID         int64  `json:"id"`
	IDUtente   int64  `json:"id_utente"`
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
	ID         int64  `json:"id"`
	IDUtente   int64  `json:"id_utente"`
	IDSede     int64  `json:"id_sede"`
	Nome       string `json:"nome"`
	Cognome    string `json:"cognome"`
	Email      string `json:"email"`
	Prefisso   string `json:"prefisso"`
	NrTelefono string `json:"nr_telefono"`
}

type utenteDTO struct {
	ID      int64  `json:"id"`
	Ruolo   string `json:"ruolo"`
	IDRuolo int64  `json:"id_ruolo"`
	// sede in cui lavora il corriere o l'operatore; null per i clienti
	IDSede     *int64  `json:"id_sede"`
	Sede       *string `json:"sede"`
	Nome       string  `json:"nome"`
	Cognome    string  `json:"cognome"`
	Email      string  `json:"email"`
	Prefisso   string  `json:"prefisso"`
	NrTelefono string  `json:"nr_telefono"`
	Attivo     bool    `json:"attivo"`
	// solo per i clienti: true se è stato destinatario di almeno una spedizione
	Destinatario *bool `json:"destinatario"`
}

type esitoUtenteDTO struct {
	Utente      utenteDTO `json:"utente"`
	MailInviata bool      `json:"mail_inviata"`
}

type indirizzoDTO struct {
	ID      int64  `json:"id"`
	IDCitta int64  `json:"id_citta"`
	Via     string `json:"via"`
	Civico  string `json:"civico"`
	Citta   string `json:"citta"`
	CAP     string `json:"cap"`
	Regione string `json:"regione"`
	Nazione string `json:"nazione"`
}

type sedeDTO struct {
	ID               int64        `json:"id"`
	Nome             string       `json:"nome"`
	Attiva           bool         `json:"attiva"`
	Indirizzo        indirizzoDTO `json:"indirizzo"`
	GruppoSpedizioni int          `json:"gruppo_spedizioni"`
	CreataIl         time.Time    `json:"creata_il"`
}

func versoSedeDTO(s Sede) sedeDTO {
	return sedeDTO{ID: s.ID, Nome: s.Nome, Attiva: s.Attiva, Indirizzo: versoIndirizzoDTO(s.Indirizzo),
		GruppoSpedizioni: s.Gruppo, CreataIl: s.CreataIl.UTC()}
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
	return utenteDTO{ID: u.ID, Ruolo: string(u.Ruolo), IDRuolo: u.IDRuolo, IDSede: u.IDSede, Sede: u.NomeSede,
		Nome: u.Nome, Cognome: u.Cognome, Email: u.Email, Prefisso: u.Prefisso, NrTelefono: u.NrTelefono,
		Attivo: u.Attivo, Destinatario: u.Destinatario}
}

func versoIndirizzoDTO(i Indirizzo) indirizzoDTO {
	return indirizzoDTO{ID: i.ID, IDCitta: i.IDCitta, Via: i.Via, Civico: i.Civico, Citta: i.Citta, CAP: i.CAP, Regione: i.Regione, Nazione: i.Nazione}
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
	if v := q.Get("id_sede"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			e.Aggiungi("id_sede", "deve essere un numero intero")
		} else {
			ric.IDSede = &n
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
		risposta.Dati[i] = corriereDTO{ID: c.ID, IDUtente: c.IDUtente, IDSede: c.IDSede, Nome: c.Nome, Cognome: c.Cognome,
			Email: c.Email, Prefisso: c.Prefisso, NrTelefono: c.NrTelefono}
	}
	httpx.ScriviJSON(w, http.StatusOK, risposta)
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
	attivo, ok := leggiBooleano(w, r, "attivo")
	if !ok {
		return
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
		Ruolo: req.Ruolo, IDSede: req.IDSede,
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
	if req.Ruolo != nil {
		audit.Evento(r.Context(), "cambio_ruolo_rifiutato", "id_utente", id)
		httpx.RispondiErrore(w, r, &errori.Validazione{Dettagli: []errori.ErroreCampo{{
			Campo: "ruolo", Messaggio: "il ruolo non si cambia: creare un nuovo utente con il ruolo giusto e disattivare questo"}}})
		return
	}
	u, err := h.servizio.ModificaUtente(r.Context(), identita.DalContesto(r.Context()), id, ModificaUtente{
		Nome: req.Nome, Cognome: req.Cognome, Prefisso: req.Prefisso, NrTelefono: req.NrTelefono,
		Email: req.Email, Password: req.Password, Attivo: req.Attivo, IDSede: req.IDSede,
	})
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	// solo i nomi dei campi cambiati, mai i valori (password, dati personali); stato e
	// sede sì, perché sono le decisioni di accesso
	var campi []string
	for nome, presente := range map[string]bool{"nome": req.Nome != nil, "cognome": req.Cognome != nil,
		"telefono": req.Prefisso != nil || req.NrTelefono != nil, "email": req.Email != nil,
		"password": req.Password != nil, "attivo": req.Attivo != nil, "id_sede": req.IDSede != nil} {
		if presente {
			campi = append(campi, nome)
		}
	}
	sort.Strings(campi)
	evento := []any{"id_utente", id, "ruolo", u.Ruolo, "campi", strings.Join(campi, ",")}
	if req.Attivo != nil {
		evento = append(evento, "attivo", *req.Attivo)
	}
	if req.IDSede != nil {
		evento = append(evento, "id_sede", *req.IDSede)
	}
	audit.Evento(r.Context(), "utente_modificato", evento...)
	httpx.ScriviJSON(w, http.StatusOK, versoUtenteDTO(u))
}

// leggiBooleano legge un filtro facoltativo true/false dalla query (nil = assente).
func leggiBooleano(w http.ResponseWriter, r *http.Request, nome string) (*bool, bool) {
	v := r.URL.Query().Get(nome)
	if v == "" {
		return nil, true
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		httpx.ScriviErrore(w, http.StatusBadRequest, "dati non validi", errori.ErroreCampo{Campo: nome, Messaggio: "ammessi true, false"})
		return nil, false
	}
	return &b, true
}

func (h *Handler) cercaSedi(w http.ResponseWriter, r *http.Request) {
	ric, ok := leggiRicerca(w, r)
	if !ok {
		return
	}
	attiva, ok := leggiBooleano(w, r, "attiva")
	if !ok {
		return
	}
	sedi, totale, err := h.servizio.CercaSedi(r.Context(), identita.DalContesto(r.Context()), RicercaSedi{Ricerca: ric, Attiva: attiva})
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	risposta := nuovaPagina[sedeDTO](len(sedi), ric, totale)
	for i, s := range sedi {
		risposta.Dati[i] = versoSedeDTO(s)
	}
	httpx.ScriviJSON(w, http.StatusOK, risposta)
}

func (h *Handler) sede(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.IDDalPercorso(w, r)
	if !ok {
		return
	}
	s, err := h.servizio.Sede(r.Context(), identita.DalContesto(r.Context()), id)
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	httpx.ScriviJSON(w, http.StatusOK, versoSedeDTO(s))
}

func (h *Handler) creaSede(w http.ResponseWriter, r *http.Request) {
	var req richiestaSede
	if !httpx.LeggiCorpo(w, r, &req) {
		return
	}
	ind := req.Indirizzo
	s, err := h.servizio.CreaSede(r.Context(), identita.DalContesto(r.Context()), NuovaSede{
		Nome: req.Nome,
		Indirizzo: NuovoIndirizzo{Via: ind.Via, Civico: ind.Civico, Citta: ind.Citta, CAP: ind.CAP,
			Regione: ind.Regione, Nazione: ind.Nazione},
		Gruppo: req.GruppoSpedizioni,
	})
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	// il gruppo delle spedizioni non si cambia più: resta traccia di chi l'ha scelto
	audit.Evento(r.Context(), "sede_creata", "sede", s.ID, "gruppo_spedizioni", s.Gruppo)
	httpx.ScriviJSON(w, http.StatusCreated, versoSedeDTO(s))
}

func (h *Handler) modificaSede(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.IDDalPercorso(w, r)
	if !ok {
		return
	}
	var req richiestaModificaSede
	if !httpx.LeggiCorpo(w, r, &req) {
		return
	}
	if req.GruppoSpedizioni != nil {
		e := &errori.Validazione{}
		e.Aggiungi("gruppo_spedizioni", "non si cambia: le spedizioni della sede sono nel database del suo gruppo")
		httpx.RispondiErrore(w, r, e)
		return
	}
	m := ModificaSede{Nome: req.Nome, Attiva: req.Attiva}
	if i := req.Indirizzo; i != nil {
		m.Indirizzo = &NuovoIndirizzo{Via: i.Via, Civico: i.Civico, Citta: i.Citta, CAP: i.CAP, Regione: i.Regione, Nazione: i.Nazione}
	}
	s, err := h.servizio.ModificaSede(r.Context(), identita.DalContesto(r.Context()), id, m)
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	var campi []string
	if m.Nome != nil {
		campi = append(campi, "nome")
	}
	if m.Indirizzo != nil {
		campi = append(campi, "indirizzo")
	}
	if m.Attiva != nil {
		campi = append(campi, "attiva")
	}
	audit.Evento(r.Context(), "sede_modificata", "sede", s.ID, "campi", strings.Join(campi, ","), "attiva", s.Attiva)
	httpx.ScriviJSON(w, http.StatusOK, versoSedeDTO(s))
}
