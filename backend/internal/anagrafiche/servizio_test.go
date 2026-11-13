package anagrafiche

import (
	"context"
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

type archivioFinto struct {
	emailUsate map[string]bool
	hash       string
	creati     int
	ricerca    Ricerca
	indirizzo  Indirizzo
}

func (a *archivioFinto) CercaClienti(_ context.Context, r Ricerca) ([]Cliente, error) {
	a.ricerca = r
	return []Cliente{{ID: 1}}, nil
}

func (a *archivioFinto) EmailUsata(_ context.Context, email string) (bool, error) {
	return a.emailUsate[strings.ToLower(email)], nil
}

func (a *archivioFinto) CreaCliente(_ context.Context, c Cliente, pwdHash string) (Cliente, error) {
	a.hash = pwdHash
	a.creati++
	c.ID, c.IDUtente = 9, 19
	return c, nil
}

func (a *archivioFinto) CercaCorrieri(_ context.Context, r Ricerca) ([]Corriere, error) {
	a.ricerca = r
	return []Corriere{{ID: 2, Nome: "Carlo"}}, nil
}

func (a *archivioFinto) CercaIndirizzi(_ context.Context, r Ricerca) ([]Indirizzo, error) {
	a.ricerca = r
	return nil, nil
}

func (a *archivioFinto) TrovaOCreaIndirizzo(_ context.Context, i Indirizzo) (Indirizzo, bool, error) {
	a.indirizzo = i
	i.ID = 5
	return i, true, nil
}

type postinoFinto struct {
	destinatario, testo string
	errore              error
}

func (p *postinoFinto) Invia(_ context.Context, destinatario, _, testo string) error {
	p.destinatario, p.testo = destinatario, testo
	return p.errore
}

func p[T any](v T) *T { return &v }

func tipoErrore(err error) string {
	var validazione *errori.Validazione
	var nonAutorizzato *errori.NonAutorizzato
	var conflitto *errori.Conflitto
	switch {
	case err == nil:
		return "nessuno"
	case errors.As(err, &validazione):
		return "400"
	case errors.As(err, &nonAutorizzato):
		return "403"
	case errors.As(err, &conflitto):
		return "409"
	}
	return err.Error()
}

var (
	operatore      = identita.Richiedente{Ruolo: identita.Operatore, ID: 1}
	amministratore = identita.Richiedente{Ruolo: identita.Amministratore, ID: 1}
	corriere       = identita.Richiedente{Ruolo: identita.Corriere, ID: 1}
	cliente        = identita.Richiedente{Ruolo: identita.Cliente, ID: 1}
)

func nuovoCliente(email string, password *string) NuovoCliente {
	return NuovoCliente{Nome: p(" Dora "), Cognome: p("Destinataria"), Prefisso: p("+39"),
		NrTelefono: p("3331234567"), Email: p(email), Password: password}
}

func TestCreaCliente(t *testing.T) {
	a := &archivioFinto{emailUsate: map[string]bool{}}
	postino := &postinoFinto{}
	s := NuovoServizio(a, postino)

	esito, err := s.CreaCliente(context.Background(), operatore, nuovoCliente("dora@test.it", p("Password123")))
	if err != nil || !esito.MailInviata || esito.Cliente.Nome != "Dora" {
		t.Fatalf("nuovo cliente creato e mail inviata: %+v %v", esito, err)
	}
	if bcrypt.CompareHashAndPassword([]byte(a.hash), []byte("Password123")) != nil {
		t.Fatal("nel database va l'hash bcrypt della password")
	}
	if postino.destinatario != "dora@test.it" || !strings.Contains(postino.testo, "Password123") || !strings.Contains(postino.testo, "dora@test.it") {
		t.Fatalf("la mail contiene email e password: %q", postino.testo)
	}

	postino.errore = errors.New("smtp giù")
	esito, err = s.CreaCliente(context.Background(), operatore, nuovoCliente("altro@test.it", p("Password123")))
	if err != nil || esito.MailInviata {
		t.Fatalf("se la mail non parte il cliente resta creato e mail_inviata è false: %+v %v", esito, err)
	}
}

func TestEmailUnicaTraIRuoli(t *testing.T) {
	a := &archivioFinto{emailUsate: map[string]bool{"corriere@test.it": true}}
	s := NuovoServizio(a, &postinoFinto{})
	if _, err := s.CreaCliente(context.Background(), operatore, nuovoCliente("Corriere@Test.it", p("Password123"))); tipoErrore(err) != "409" {
		t.Fatalf("email già usata da un altro utente (anche con un altro ruolo): atteso 409, ottenuto %v", err)
	}
	if a.creati != 0 {
		t.Fatal("nessun utente creato")
	}
}

func TestCreaClienteValidazione(t *testing.T) {
	s := NuovoServizio(&archivioFinto{emailUsate: map[string]bool{}}, &postinoFinto{})
	casi := map[string]func(*NuovoCliente){
		"senza nome":            func(n *NuovoCliente) { n.Nome = nil },
		"prefisso senza +":      func(n *NuovoCliente) { n.Prefisso = p("39") },
		"telefono con spazi":    func(n *NuovoCliente) { n.NrTelefono = p("333 123") },
		"email non valida":      func(n *NuovoCliente) { n.Email = p("dora.test.it") },
		"senza password":        func(n *NuovoCliente) { n.Password = nil },
		"password troppo corta": func(n *NuovoCliente) { n.Password = p("corta") },
	}
	for nome, cambia := range casi {
		n := nuovoCliente("nuovo@test.it", p("Password123"))
		cambia(&n)
		if _, err := s.CreaCliente(context.Background(), operatore, n); tipoErrore(err) != "400" {
			t.Errorf("%s: atteso 400, ottenuto %v", nome, err)
		}
	}
	for _, r := range []identita.Richiedente{amministratore, corriere, cliente} {
		if _, err := s.CreaCliente(context.Background(), r, nuovoCliente("x@test.it", p("Password123"))); tipoErrore(err) != "403" {
			t.Errorf("%s non crea clienti: %v", r.Ruolo, err)
		}
	}
}

func TestIndirizzi(t *testing.T) {
	a := &archivioFinto{}
	s := NuovoServizio(a, &postinoFinto{})
	ind, creato, err := s.TrovaOCreaIndirizzo(context.Background(), operatore, NuovoIndirizzo{
		Via: p(" Via Roma "), Civico: p("1"), Citta: p("Trento"), CAP: p("38121"), Regione: p("Trentino-Alto Adige"), Nazione: p("Italia"),
	})
	if err != nil || !creato || ind.ID != 5 || a.indirizzo.Via != "Via Roma" {
		t.Fatalf("indirizzo creato con i campi ripuliti dagli spazi: %+v %v", ind, err)
	}
	if _, _, err := s.TrovaOCreaIndirizzo(context.Background(), operatore, NuovoIndirizzo{Via: p("Via Roma")}); tipoErrore(err) != "400" {
		t.Fatalf("tutti i campi sono obbligatori: %v", err)
	}
	if _, _, err := s.TrovaOCreaIndirizzo(context.Background(), corriere, NuovoIndirizzo{}); tipoErrore(err) != "403" {
		t.Fatalf("il corriere non inserisce indirizzi: %v", err)
	}
}

func TestFormatoIndirizzo(t *testing.T) {
	s := NuovoServizio(&archivioFinto{}, &postinoFinto{})
	valido := func() NuovoIndirizzo {
		return NuovoIndirizzo{Via: p("Via Roma"), Civico: p("12/B"), Citta: p("Sant'Orsola Terme"), CAP: p("38121"),
			Regione: p("Trentino-Alto Adige"), Nazione: p("Italia")}
	}
	if _, _, err := s.TrovaOCreaIndirizzo(context.Background(), operatore, valido()); err != nil {
		t.Fatalf("indirizzo valido rifiutato: %v", err)
	}
	estero := valido()
	estero.Nazione, estero.CAP, estero.Civico = p("Regno Unito"), p("SW1A 1AA"), p("SNC")
	if _, _, err := s.TrovaOCreaIndirizzo(context.Background(), operatore, estero); err != nil {
		t.Fatalf("CAP estero e civico SNC sono ammessi: %v", err)
	}
	casi := map[string]struct {
		campo  string
		cambia func(*NuovoIndirizzo)
	}{
		"via solo numeri":      {"via", func(n *NuovoIndirizzo) { n.Via = p("123") }},
		"civico con parole":    {"civico", func(n *NuovoIndirizzo) { n.Civico = p("dodici") }},
		"città con cifre":      {"citta", func(n *NuovoIndirizzo) { n.Citta = p("Trento2") }},
		"regione con simboli":  {"regione", func(n *NuovoIndirizzo) { n.Regione = p("Trentino#") }},
		"nazione con cifre":    {"nazione", func(n *NuovoIndirizzo) { n.Nazione = p("1talia") }},
		"CAP italiano a 4":     {"cap", func(n *NuovoIndirizzo) { n.CAP = p("3812") }},
		"CAP italiano lettere": {"cap", func(n *NuovoIndirizzo) { n.CAP = p("38A21") }},
		"CAP estero con #":     {"cap", func(n *NuovoIndirizzo) { n.Nazione, n.CAP = p("Francia"), p("75#01") }},
	}
	for nome, c := range casi {
		n := valido()
		c.cambia(&n)
		_, _, err := s.TrovaOCreaIndirizzo(context.Background(), operatore, n)
		v, ok := err.(*errori.Validazione)
		if !ok || !v.Segnalato(c.campo) {
			t.Errorf("%s: atteso errore sul campo %s, ottenuto %v", nome, c.campo, err)
		}
	}
}

func TestRicercheSoloOperatoreEAmministratore(t *testing.T) {
	a := &archivioFinto{}
	s := NuovoServizio(a, &postinoFinto{})
	ric := Ricerca{Testo: "dora", Pagina: 1, PerPagina: 20}
	for _, r := range []identita.Richiedente{operatore, amministratore} {
		if _, err := s.CercaClienti(context.Background(), r, ric); err != nil {
			t.Errorf("%s cerca clienti: %v", r.Ruolo, err)
		}
		if _, err := s.CercaIndirizzi(context.Background(), r, ric); err != nil {
			t.Errorf("%s cerca indirizzi: %v", r.Ruolo, err)
		}
		if c, err := s.CercaCorrieri(context.Background(), r, ric); err != nil || len(c) != 1 {
			t.Errorf("%s cerca corrieri: %v %v", r.Ruolo, c, err)
		}
	}
	for _, r := range []identita.Richiedente{corriere, cliente} {
		if _, err := s.CercaClienti(context.Background(), r, ric); tipoErrore(err) != "403" {
			t.Errorf("%s non cerca clienti: %v", r.Ruolo, err)
		}
		if _, err := s.CercaCorrieri(context.Background(), r, ric); tipoErrore(err) != "403" {
			t.Errorf("%s non vede l'elenco dei corrieri: %v", r.Ruolo, err)
		}
	}
	if _, err := s.CercaCorrieri(context.Background(), operatore, Ricerca{Pagina: 0, PerPagina: 20}); tipoErrore(err) != "400" {
		t.Errorf("pagina 0: %v", err)
	}
}
