package anagrafiche

import (
	"context"

	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

// Utente è un corriere o un operatore logistico, gestito dall'amministratore.
// ID è l'id dell'utente (tabella utenti), IDRuolo quello nella tabella del ruolo
// (corrieri o operatori_logistici), cioè l'id usato nelle spedizioni.
type Utente struct {
	ID         int32
	Ruolo      identita.Ruolo
	IDRuolo    int32
	Nome       string
	Cognome    string
	Email      string
	Prefisso   string
	NrTelefono string
	Attivo     bool
}

type NuovoUtente struct {
	Ruolo *string
	NuovoCliente
}

type EsitoUtente struct {
	Utente      Utente
	MailInviata bool
}

type RicercaUtenti struct {
	Ricerca
	Ruolo  *identita.Ruolo
	Attivo *bool
}

// ruoliGestiti sono i ruoli che l'amministratore crea, abilita e disattiva.
var ruoliGestiti = map[identita.Ruolo]string{
	identita.Corriere:  "corriere",
	identita.Operatore: "operatore logistico",
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

// CercaUtenti restituisce corrieri e operatori, attivi e disattivati, con il totale.
func (s *Servizio) CercaUtenti(ctx context.Context, r identita.Richiedente, ric Ricerca, ruolo *string, attivo *bool) ([]Utente, int, error) {
	if err := soloAmministratore(r, "consultare gli utenti"); err != nil {
		return nil, 0, err
	}
	if err := validaRicerca(ric); err != nil {
		return nil, 0, err
	}
	e := &errori.Validazione{}
	filtro := RicercaUtenti{Ricerca: ric, Ruolo: controllaRuolo(e, ruolo, false), Attivo: attivo}
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

// ImpostaUtenteAttivo abilita o disattiva un corriere o un operatore. L'utente
// disattivato non entra più e i token che ha già smettono di valere; un corriere
// disattivato non riceve nuove spedizioni.
func (s *Servizio) ImpostaUtenteAttivo(ctx context.Context, r identita.Richiedente, idUtente int32, attivo *bool) (Utente, error) {
	if err := soloAmministratore(r, "abilitare o disattivare gli utenti"); err != nil {
		return Utente{}, err
	}
	if attivo == nil {
		e := &errori.Validazione{}
		e.Aggiungi("attivo", "obbligatorio")
		return Utente{}, e
	}
	return s.archivio.ImpostaUtenteAttivo(ctx, idUtente, *attivo)
}
