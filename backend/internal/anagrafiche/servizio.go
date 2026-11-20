package anagrafiche

import (
	"context"
	"log"
	"regexp"
	"strings"
	"unicode/utf8"

	"lastmiledelivery/backend/internal/autenticazione"
	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

const (
	PerPaginaPredefinito = 20
	PerPaginaMassimo     = 100
	oggettoMailAccount   = "Il tuo account LastMileDelivery"
)

var (
	formatoPrefisso = regexp.MustCompile(`^\+[0-9]{1,4}$`)
	formatoTelefono = regexp.MustCompile(`^[0-9]{4,15}$`)
	formatoEmail    = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

	// indirizzi
	contieneLettera  = regexp.MustCompile(`\p{L}`)
	formatoCivico    = regexp.MustCompile(`^(?i:snc|[0-9]{1,5}(?: ?[/-]? ?[a-z0-9]{1,3})?)$`)
	formatoLuogo     = regexp.MustCompile(`^\p{L}[\p{L} '’.-]*$`)
	formatoCAPItalia = regexp.MustCompile(`^[0-9]{5}$`)
	formatoCAPEstero = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9 -]{1,8})[A-Za-z0-9]$`)
)

type Cliente struct {
	ID         int32
	IDUtente   int32
	Nome       string
	Cognome    string
	Email      string
	Prefisso   string
	NrTelefono string
	Attivo     bool
	// Destinatario: ha ricevuto almeno una spedizione; altrimenti è un mittente.
	Destinatario bool
}

type NuovoCliente struct {
	Nome       *string
	Cognome    *string
	Prefisso   *string
	NrTelefono *string
	Email      *string
	Password   *string
}

type EsitoCliente struct {
	Cliente     Cliente
	MailInviata bool
}

type Corriere struct {
	ID         int32
	IDUtente   int32
	Nome       string
	Cognome    string
	Email      string
	Prefisso   string
	NrTelefono string
}

type Indirizzo struct {
	ID      int32
	Via     string
	Civico  string
	Citta   string
	CAP     string
	Regione string
	Nazione string
}

type NuovoIndirizzo struct {
	Via     *string
	Civico  *string
	Citta   *string
	CAP     *string
	Regione *string
	Nazione *string
}

type Ricerca struct {
	Testo     string
	Pagina    int
	PerPagina int
}

type archivio interface {
	CercaClienti(ctx context.Context, r Ricerca) ([]Cliente, int, error)
	// EmailUsata dice se l'email è già di un utente diverso da escludi (0 = nessuno escluso).
	EmailUsata(ctx context.Context, email string, escludi int32) (bool, error)
	CreaCliente(ctx context.Context, c Cliente, pwdHash string) (Cliente, error)
	CercaCorrieri(ctx context.Context, r Ricerca) ([]Corriere, int, error)
	CercaIndirizzi(ctx context.Context, r Ricerca) ([]Indirizzo, int, error)
	TrovaOCreaIndirizzo(ctx context.Context, i Indirizzo) (Indirizzo, bool, error)
	CercaUtenti(ctx context.Context, r RicercaUtenti) ([]Utente, int, error)
	CreaUtente(ctx context.Context, ruolo identita.Ruolo, c Cliente, pwdHash string) (Utente, error)
	// UtentePerID: corriere, operatore o cliente; NonTrovato per gli altri (amministratori).
	UtentePerID(ctx context.Context, idUtente int32) (Utente, error)
	ModificaUtente(ctx context.Context, idUtente int32, d DatiUtente) (Utente, error)
}

type Servizio struct {
	archivio archivio
	postino  autenticazione.Postino
}

func NuovoServizio(a archivio, p autenticazione.Postino) *Servizio {
	return &Servizio{archivio: a, postino: p}
}

func soloOperatore(r identita.Richiedente, azione string) error {
	if r.Ruolo != identita.Operatore {
		return &errori.NonAutorizzato{Messaggio: "solo un operatore logistico può " + azione}
	}
	return nil
}

func puoConsultare(r identita.Richiedente) error {
	if r.Ruolo != identita.Operatore && r.Ruolo != identita.Amministratore {
		return &errori.NonAutorizzato{Messaggio: "clienti e indirizzi sono consultabili solo da operatori e amministratori"}
	}
	return nil
}

func validaRicerca(r Ricerca) error {
	e := &errori.Validazione{}
	if r.Pagina < 1 {
		e.Aggiungi("pagina", "deve essere almeno 1")
	}
	if r.PerPagina < 1 || r.PerPagina > PerPaginaMassimo {
		e.Aggiungi("per_pagina", "deve essere tra 1 e 100")
	}
	if utf8.RuneCountInString(r.Testo) > 100 {
		e.Aggiungi("q", "al massimo 100 caratteri")
	}
	return e.Err()
}

func testo(e *errori.Validazione, campo string, v *string, massimo int) string {
	if v == nil || strings.TrimSpace(*v) == "" {
		e.Aggiungi(campo, "obbligatorio")
		return ""
	}
	t := strings.TrimSpace(*v)
	if utf8.RuneCountInString(t) > massimo {
		e.Aggiungi(campo, "troppo lungo")
	}
	return t
}

// CercaClienti restituisce i clienti della pagina richiesta e quanti sono in tutto.
func (s *Servizio) CercaClienti(ctx context.Context, r identita.Richiedente, ric Ricerca) ([]Cliente, int, error) {
	if err := puoConsultare(r); err != nil {
		return nil, 0, err
	}
	if err := validaRicerca(ric); err != nil {
		return nil, 0, err
	}
	return s.archivio.CercaClienti(ctx, ric)
}

func (s *Servizio) CreaCliente(ctx context.Context, r identita.Richiedente, n NuovoCliente) (EsitoCliente, error) {
	if err := soloOperatore(r, "creare un cliente"); err != nil {
		return EsitoCliente{}, err
	}
	e := &errori.Validazione{}
	c := validaPersona(e, n)
	if err := e.Err(); err != nil {
		return EsitoCliente{}, err
	}
	hash, err := s.preparaCredenziali(ctx, c.Email, *n.Password)
	if err != nil {
		return EsitoCliente{}, err
	}
	creato, err := s.archivio.CreaCliente(ctx, c, hash)
	if err != nil {
		return EsitoCliente{}, err
	}
	mail := s.inviaCredenziali(ctx, creato.Nome, creato.Email, *n.Password,
		"Veloce24 ha creato il tuo account per seguire le spedizioni a te destinate.")
	return EsitoCliente{Cliente: creato, MailInviata: mail}, nil
}

// validaPersona controlla i dati anagrafici e la password di un nuovo utente.
func validaPersona(e *errori.Validazione, n NuovoCliente) Cliente {
	c := Cliente{
		Nome:       testo(e, "nome", n.Nome, 100),
		Cognome:    testo(e, "cognome", n.Cognome, 100),
		Prefisso:   testo(e, "prefisso", n.Prefisso, 5),
		NrTelefono: testo(e, "nr_telefono", n.NrTelefono, 15),
		Email:      testo(e, "email", n.Email, 254),
	}
	controllaFormati(e, c)
	autenticazione.ControllaPassword(e, "password", n.Password)
	return c
}

// controllaFormati verifica il formato dei campi presenti (quelli vuoti li segnala testo).
func controllaFormati(e *errori.Validazione, c Cliente) {
	if c.Prefisso != "" && !formatoPrefisso.MatchString(c.Prefisso) {
		e.Aggiungi("prefisso", "formato non valido: + seguito da 1 a 4 cifre, es. +39")
	}
	if c.NrTelefono != "" && !formatoTelefono.MatchString(c.NrTelefono) {
		e.Aggiungi("nr_telefono", "da 4 a 15 cifre, senza spazi")
	}
	if c.Email != "" && !formatoEmail.MatchString(c.Email) {
		e.Aggiungi("email", "formato non valido")
	}
}

// preparaCredenziali verifica che l'email sia libera e restituisce l'hash della password.
func (s *Servizio) preparaCredenziali(ctx context.Context, email, password string) (string, error) {
	usata, err := s.archivio.EmailUsata(ctx, email, 0)
	if err != nil {
		return "", err
	}
	if usata {
		return "", errEmailUsata
	}
	return autenticazione.HashPassword(password)
}

var errEmailUsata = &errori.Conflitto{
	Messaggio: "email già usata da un altro utente: ogni email può avere un solo ruolo; se è già un cliente, cercarlo con GET /api/clienti",
	Dettagli:  []errori.ErroreCampo{{Campo: "email", Messaggio: "già usata"}},
}

// CercaCorrieri restituisce i corrieri della pagina richiesta e quanti sono in tutto.
func (s *Servizio) CercaCorrieri(ctx context.Context, r identita.Richiedente, ric Ricerca) ([]Corriere, int, error) {
	if r.Ruolo != identita.Operatore && r.Ruolo != identita.Amministratore {
		return nil, 0, &errori.NonAutorizzato{Messaggio: "l'elenco dei corrieri è consultabile solo da operatori e amministratori"}
	}
	if err := validaRicerca(ric); err != nil {
		return nil, 0, err
	}
	return s.archivio.CercaCorrieri(ctx, ric)
}

func (s *Servizio) inviaCredenziali(ctx context.Context, nome, email, password, presentazione string) bool {
	testo := "Ciao " + nome + ",\n\n" +
		presentazione + "\n\n" +
		"Email: " + email + "\n" +
		"Password: " + password + "\n\n" +
		"Ti consigliamo di cambiare la password dopo il primo accesso.\n"
	if err := s.postino.Invia(ctx, email, oggettoMailAccount, testo); err != nil {
		log.Printf("nuovo account %s: invio della mail con le credenziali fallito: %v", email, err)
		return false
	}
	return true
}

// CercaIndirizzi restituisce gli indirizzi della pagina richiesta e quanti sono in tutto.
func (s *Servizio) CercaIndirizzi(ctx context.Context, r identita.Richiedente, ric Ricerca) ([]Indirizzo, int, error) {
	if err := puoConsultare(r); err != nil {
		return nil, 0, err
	}
	if err := validaRicerca(ric); err != nil {
		return nil, 0, err
	}
	return s.archivio.CercaIndirizzi(ctx, ric)
}

func (s *Servizio) TrovaOCreaIndirizzo(ctx context.Context, r identita.Richiedente, n NuovoIndirizzo) (Indirizzo, bool, error) {
	if err := soloOperatore(r, "inserire un indirizzo"); err != nil {
		return Indirizzo{}, false, err
	}
	e := &errori.Validazione{}
	i := Indirizzo{
		Via:     testo(e, "via", n.Via, 150),
		Civico:  testo(e, "civico", n.Civico, 10),
		Citta:   testo(e, "citta", n.Citta, 100),
		CAP:     testo(e, "cap", n.CAP, 10),
		Regione: testo(e, "regione", n.Regione, 100),
		Nazione: testo(e, "nazione", n.Nazione, 100),
	}
	controllaIndirizzo(e, i)
	if err := e.Err(); err != nil {
		return Indirizzo{}, false, err
	}
	return s.archivio.TrovaOCreaIndirizzo(ctx, i)
}

// controllaIndirizzo verifica il formato dei campi già presenti (quelli mancanti
// li ha già segnalati testo come obbligatori).
func controllaIndirizzo(e *errori.Validazione, i Indirizzo) {
	if i.Via != "" && !contieneLettera.MatchString(i.Via) {
		e.Aggiungi("via", "deve contenere il nome della via, non solo numeri o simboli")
	}
	if i.Civico != "" && !formatoCivico.MatchString(i.Civico) {
		e.Aggiungi("civico", "formato non valido: numero con eventuale lettera o barra, es. 12, 12A, 12/B, oppure SNC")
	}
	for _, c := range []struct{ campo, valore string }{
		{"citta", i.Citta}, {"regione", i.Regione}, {"nazione", i.Nazione},
	} {
		if c.valore != "" && !formatoLuogo.MatchString(c.valore) {
			e.Aggiungi(c.campo, "solo lettere, spazi, apostrofi, punti e trattini")
		}
	}
	switch {
	case i.CAP == "":
	case strings.EqualFold(i.Nazione, "Italia"):
		if !formatoCAPItalia.MatchString(i.CAP) {
			e.Aggiungi("cap", "in Italia il CAP è di 5 cifre, es. 38121")
		}
	case !formatoCAPEstero.MatchString(i.CAP):
		e.Aggiungi("cap", "formato non valido: da 3 a 10 tra lettere, cifre, spazi e trattini")
	}
}
