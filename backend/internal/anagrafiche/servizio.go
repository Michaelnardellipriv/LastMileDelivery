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
	"lastmiledelivery/backend/internal/indirizzo"
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
)

type Cliente struct {
	ID         int64
	IDUtente   int64
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
	ID         int64
	IDUtente   int64
	IDSede     int64
	Nome       string
	Cognome    string
	Email      string
	Prefisso   string
	NrTelefono string
}

type Indirizzo struct {
	ID      int64
	IDCitta int64
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
	// IDSede limita la ricerca a una sede (corrieri, utenti, indirizzi della sua città)
	IDSede *int64
}

type archivio interface {
	CercaClienti(ctx context.Context, r Ricerca) ([]Cliente, int, error)
	// EmailUsata dice se l'email è già di un utente diverso da escludi (0 = nessuno escluso).
	EmailUsata(ctx context.Context, email string, escludi int64) (bool, error)
	CreaCliente(ctx context.Context, c Cliente, pwdHash string) (Cliente, error)
	CercaCorrieri(ctx context.Context, r Ricerca) ([]Corriere, int, error)
	CercaUtenti(ctx context.Context, r RicercaUtenti) ([]Utente, int, error)
	CreaUtente(ctx context.Context, ruolo identita.Ruolo, idSede int64, c Cliente, pwdHash string) (Utente, error)
	// UtentePerID: corriere, operatore o cliente; NonTrovato per gli altri (amministratori).
	UtentePerID(ctx context.Context, idUtente int64) (Utente, error)
	ModificaUtente(ctx context.Context, idUtente int64, d DatiUtente) (Utente, error)
	CercaSedi(ctx context.Context, r RicercaSedi) ([]Sede, int, error)
	Sede(ctx context.Context, id int64) (Sede, error)
	// CreaSede: gruppo è il database delle spedizioni della sede (deve averne uno)
	CreaSede(ctx context.Context, nome string, i Indirizzo, gruppo int) (Sede, error)
	ModificaSede(ctx context.Context, id int64, d DatiSede) (Sede, error)
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
// L'operatore vede solo i corrieri della sua sede (gli unici a cui può assegnare);
// l'amministratore tutti, con il filtro per sede facoltativo.
func (s *Servizio) CercaCorrieri(ctx context.Context, r identita.Richiedente, ric Ricerca) ([]Corriere, int, error) {
	if r.Ruolo != identita.Operatore && r.Ruolo != identita.Amministratore {
		return nil, 0, &errori.NonAutorizzato{Messaggio: "l'elenco dei corrieri è consultabile solo da operatori e amministratori"}
	}
	if err := validaRicerca(ric); err != nil {
		return nil, 0, err
	}
	ric, err := limitaAllaSede(r, ric)
	if err != nil {
		return nil, 0, err
	}
	return s.archivio.CercaCorrieri(ctx, ric)
}

// limitaAllaSede: operatori e corrieri cercano solo nella propria sede; l'amministratore
// può filtrare su qualunque sede o vederle tutte.
func limitaAllaSede(r identita.Richiedente, ric Ricerca) (Ricerca, error) {
	if r.Ruolo == identita.Amministratore {
		if ric.IDSede != nil && *ric.IDSede <= 0 {
			return ric, &errori.Validazione{Dettagli: []errori.ErroreCampo{{Campo: "id_sede", Messaggio: "deve essere un id positivo"}}}
		}
		return ric, nil
	}
	if ric.IDSede != nil && *ric.IDSede != r.IDSede {
		return ric, &errori.NonAutorizzato{Messaggio: "il filtro per sede è per gli amministratori: si vede solo la propria sede"}
	}
	sede := r.IDSede
	ric.IDSede = &sede
	return ric, nil
}

func (s *Servizio) inviaCredenziali(ctx context.Context, nome, email, password, presentazione string) bool {
	testo := "Ciao " + nome + ",\n\n" +
		presentazione + "\n\n" +
		"Email: " + email + "\n" +
		"Password: " + password + "\n\n" +
		"Ti consigliamo di cambiare la password dopo il primo accesso.\n"
	if err := s.postino.Invia(ctx, email, oggettoMailAccount, testo); err != nil {
		// nel log niente email (dato personale) né password
		log.Printf("nuovo account: invio della mail con le credenziali fallito: %v", err)
		return false
	}
	return true
}

// validaIndirizzo controlla i campi dell'indirizzo di una sede; prefisso precede il nome
// dei campi negli errori (es. "indirizzo."). Le regole sono in internal/indirizzo, comuni
// con gli indirizzi delle spedizioni.
func validaIndirizzo(e *errori.Validazione, prefisso string, n NuovoIndirizzo) Indirizzo {
	d := indirizzo.Valida(e, prefisso, indirizzo.Nuovo{Via: n.Via, Civico: n.Civico, Citta: n.Citta, CAP: n.CAP,
		Regione: n.Regione, Nazione: n.Nazione})
	return Indirizzo{Via: d.Via, Civico: d.Civico, Citta: d.Citta, CAP: d.CAP, Regione: d.Regione, Nazione: d.Nazione}
}
