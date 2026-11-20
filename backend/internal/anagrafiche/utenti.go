package anagrafiche

import (
	"context"

	"lastmiledelivery/backend/internal/autenticazione"
	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

// Utente è un corriere, un operatore logistico o un cliente, gestito
// dall'amministratore (i dati dei clienti li modifica anche l'operatore).
// ID è l'id dell'utente (tabella utenti), IDRuolo quello nella tabella del ruolo
// (corrieri, operatori_logistici o clienti), cioè l'id usato nelle spedizioni.
// Destinatario c'è solo per i clienti: true se il cliente è stato destinatario di
// almeno una spedizione (accede all'app), false se è solo mittente.
type Utente struct {
	ID           int32
	Ruolo        identita.Ruolo
	IDRuolo      int32
	Nome         string
	Cognome      string
	Email        string
	Prefisso     string
	NrTelefono   string
	Attivo       bool
	Destinatario *bool
}

// soloMittente: un cliente che non è mai stato destinatario non accede all'app.
func (u Utente) soloMittente() bool {
	return u.Ruolo == identita.Cliente && (u.Destinatario == nil || !*u.Destinatario)
}

type NuovoUtente struct {
	Ruolo *string
	NuovoCliente
}

type EsitoUtente struct {
	Utente      Utente
	MailInviata bool
}

// ModificaUtente contiene i campi da cambiare (nil = invariato).
type ModificaUtente struct {
	Nome       *string
	Cognome    *string
	Prefisso   *string
	NrTelefono *string
	Email      *string
	Password   *string
	Attivo     *bool
}

func (m ModificaUtente) vuota() bool {
	return m.Nome == nil && m.Cognome == nil && m.Prefisso == nil && m.NrTelefono == nil &&
		m.Email == nil && m.Password == nil && m.Attivo == nil
}

// DatiUtente sono i campi già validati da salvare; la password è già l'hash.
type DatiUtente struct {
	Nome       *string
	Cognome    *string
	Prefisso   *string
	NrTelefono *string
	Email      *string
	PwdHash    *string
	Attivo     *bool
}

type RicercaUtenti struct {
	Ricerca
	Ruolo  *identita.Ruolo
	Attivo *bool
}

// ruoliGestiti sono i ruoli che l'amministratore crea: i clienti li crea l'operatore.
var ruoliGestiti = map[identita.Ruolo]string{
	identita.Corriere:  "corriere",
	identita.Operatore: "operatore logistico",
}

// ruoliInElenco sono i ruoli di GET /api/utenti e di PATCH /api/utenti/{id}.
var ruoliInElenco = map[identita.Ruolo]string{
	identita.Corriere:  "corriere",
	identita.Operatore: "operatore logistico",
	identita.Cliente:   "cliente",
}

func soloAmministratore(r identita.Richiedente, azione string) error {
	if r.Ruolo != identita.Amministratore {
		return &errori.NonAutorizzato{Messaggio: "solo un amministratore può " + azione}
	}
	return nil
}

func controllaRuolo(e *errori.Validazione, ruolo *string, obbligatorio bool) *identita.Ruolo {
	if ruolo == nil {
		if obbligatorio {
			e.Aggiungi("ruolo", "obbligatorio: corriere oppure operatore")
		}
		return nil
	}
	r := identita.Ruolo(*ruolo)
	if _, ok := ruoliGestiti[r]; !ok {
		e.Aggiungi("ruolo", "valore non previsto: ammessi corriere, operatore")
		return nil
	}
	return &r
}

func controllaRuoloElenco(e *errori.Validazione, ruolo *string) *identita.Ruolo {
	if ruolo == nil {
		return nil
	}
	r := identita.Ruolo(*ruolo)
	if _, ok := ruoliInElenco[r]; !ok {
		e.Aggiungi("ruolo", "valore non previsto: ammessi corriere, operatore, cliente")
		return nil
	}
	return &r
}

// CercaUtenti restituisce corrieri, operatori e clienti, attivi e disattivati, con il totale.
func (s *Servizio) CercaUtenti(ctx context.Context, r identita.Richiedente, ric Ricerca, ruolo *string, attivo *bool) ([]Utente, int, error) {
	if err := soloAmministratore(r, "consultare gli utenti"); err != nil {
		return nil, 0, err
	}
	if err := validaRicerca(ric); err != nil {
		return nil, 0, err
	}
	e := &errori.Validazione{}
	filtro := RicercaUtenti{Ricerca: ric, Ruolo: controllaRuoloElenco(e, ruolo), Attivo: attivo}
	if err := e.Err(); err != nil {
		return nil, 0, err
	}
	return s.archivio.CercaUtenti(ctx, filtro)
}

// CreaUtente crea un corriere o un operatore con la password scelta dall'amministratore.
func (s *Servizio) CreaUtente(ctx context.Context, r identita.Richiedente, n NuovoUtente) (EsitoUtente, error) {
	if err := soloAmministratore(r, "creare corrieri e operatori"); err != nil {
		return EsitoUtente{}, err
	}
	e := &errori.Validazione{}
	ruolo := controllaRuolo(e, n.Ruolo, true)
	c := validaPersona(e, n.NuovoCliente)
	if err := e.Err(); err != nil {
		return EsitoUtente{}, err
	}
	hash, err := s.preparaCredenziali(ctx, c.Email, *n.Password)
	if err != nil {
		return EsitoUtente{}, err
	}
	creato, err := s.archivio.CreaUtente(ctx, *ruolo, c, hash)
	if err != nil {
		return EsitoUtente{}, err
	}
	mail := s.inviaCredenziali(ctx, creato.Nome, creato.Email, *n.Password,
		"Veloce24 ha creato il tuo account da "+ruoliGestiti[creato.Ruolo]+".")
	return EsitoUtente{Utente: creato, MailInviata: mail}, nil
}

// ModificaUtente cambia solo i campi presenti di un corriere, di un operatore o di un
// cliente.
//   - L'amministratore cambia dati, password e stato di tutti; di un cliente solo
//     mittente (non accede all'app) solo i dati.
//   - L'operatore cambia solo i dati dei clienti: niente password né disattivazione.
//
// Con attivo=false l'utente non entra più e i token che ha già smettono di valere; un
// corriere disattivato non riceve nuove spedizioni, un cliente disattivato non può
// essere destinatario di nuove spedizioni. Con una nuova password, l'utente la riceve
// per mail.
func (s *Servizio) ModificaUtente(ctx context.Context, r identita.Richiedente, idUtente int32, m ModificaUtente) (Utente, error) {
	switch r.Ruolo {
	case identita.Amministratore:
	case identita.Operatore:
		if m.Attivo != nil || m.Password != nil {
			return Utente{}, &errori.NonAutorizzato{Messaggio: "solo un amministratore può disattivare un utente o reimpostarne la password"}
		}
	default:
		return Utente{}, &errori.NonAutorizzato{Messaggio: "solo amministratori e operatori logistici possono modificare gli utenti"}
	}
	e := &errori.Validazione{}
	if m.vuota() {
		e.Aggiungi("corpo", "indicare almeno un campo da modificare")
		return Utente{}, e
	}
	campo := func(nome string, v *string, massimo int) *string {
		if v == nil {
			return nil
		}
		t := testo(e, nome, v, massimo)
		return &t
	}
	d := DatiUtente{
		Nome:       campo("nome", m.Nome, 100),
		Cognome:    campo("cognome", m.Cognome, 100),
		Prefisso:   campo("prefisso", m.Prefisso, 5),
		NrTelefono: campo("nr_telefono", m.NrTelefono, 15),
		Email:      campo("email", m.Email, 254),
		Attivo:     m.Attivo,
	}
	valore := func(v *string) string {
		if v == nil {
			return ""
		}
		return *v
	}
	controllaFormati(e, Cliente{Prefisso: valore(d.Prefisso), NrTelefono: valore(d.NrTelefono), Email: valore(d.Email)})
	if m.Password != nil {
		autenticazione.ControllaPassword(e, "password", m.Password)
	}
	if err := e.Err(); err != nil {
		return Utente{}, err
	}

	bersaglio, err := s.archivio.UtentePerID(ctx, idUtente)
	if err != nil {
		return Utente{}, err
	}
	if r.Ruolo == identita.Operatore && bersaglio.Ruolo != identita.Cliente {
		return Utente{}, &errori.NonAutorizzato{Messaggio: "l'operatore logistico può modificare solo i dati dei clienti"}
	}
	if bersaglio.soloMittente() && (m.Attivo != nil || m.Password != nil) {
		return Utente{}, &errori.Conflitto{Messaggio: "un cliente solo mittente non accede all'app: si possono modificare solo i dati"}
	}

	if d.Email != nil {
		usata, err := s.archivio.EmailUsata(ctx, *d.Email, idUtente)
		if err != nil {
			return Utente{}, err
		}
		if usata {
			return Utente{}, errEmailUsata
		}
	}
	if m.Password != nil {
		hash, err := autenticazione.HashPassword(*m.Password)
		if err != nil {
			return Utente{}, err
		}
		d.PwdHash = &hash
	}
	u, err := s.archivio.ModificaUtente(ctx, idUtente, d)
	if err != nil {
		return Utente{}, err
	}
	if m.Password != nil {
		s.inviaCredenziali(ctx, u.Nome, u.Email, *m.Password,
			"L'amministratore di Veloce24 ha reimpostato la password del tuo account da "+ruoliInElenco[u.Ruolo]+".")
	}
	return u, nil
}
