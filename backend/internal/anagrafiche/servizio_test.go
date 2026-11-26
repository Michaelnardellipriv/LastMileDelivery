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
	utenti     RicercaUtenti
	modifica   *DatiUtente
	// proprietari: di chi è un'email di emailUsate, per la modifica dell'utente stesso
	proprietari map[string]int64
	// sedeUtente: la sede passata a CreaUtente
	sedeUtente int64
	// luogo: città, regione e nazione della sede restituita da Sede (Trento se vuoto)
	luogo         *Indirizzo
	nuovaSede     string
	indirizzoSede Indirizzo
	gruppoSede    int
	ricercaSedi   RicercaSedi
	modificaSede  *DatiSede
}

const sedeTrento, sedeBolzano = 10, 20

func (a *archivioFinto) CercaSedi(_ context.Context, r RicercaSedi) ([]Sede, int, error) {
	a.ricerca = r.Ricerca
	a.ricercaSedi = r
	return []Sede{{ID: sedeTrento, Nome: "Veloce24 Trento"}}, 1, nil
}

func (a *archivioFinto) Sede(_ context.Context, id int64) (Sede, error) {
	luogo := Indirizzo{Citta: "Trento", Regione: "Trentino-Alto Adige", Nazione: "Italia"}
	if a.luogo != nil {
		luogo = *a.luogo
	}
	return Sede{ID: id, Nome: "Veloce24", Indirizzo: luogo}, nil
}

func (a *archivioFinto) CreaSede(_ context.Context, nome string, i Indirizzo, gruppo int) (Sede, error) {
	a.nuovaSede, a.indirizzoSede, a.gruppoSede = nome, i, gruppo
	return Sede{ID: 30, Nome: nome, Indirizzo: i, Gruppo: gruppo}, nil
}

func (a *archivioFinto) ModificaSede(_ context.Context, id int64, d DatiSede) (Sede, error) {
	a.modificaSede = &d
	s := Sede{ID: id, Nome: "Veloce24 Trento", Attiva: true}
	if d.Nome != nil {
		s.Nome = *d.Nome
	}
	if d.Attiva != nil {
		s.Attiva = *d.Attiva
	}
	return s, nil
}

func (a *archivioFinto) CercaClienti(_ context.Context, r Ricerca) ([]Cliente, int, error) {
	a.ricerca = r
	return []Cliente{{ID: 1}}, 1, nil
}

func (a *archivioFinto) EmailUsata(_ context.Context, email string, escludi int64) (bool, error) {
	if proprietario, ok := a.proprietari[strings.ToLower(email)]; ok && proprietario == escludi {
		return false, nil
	}
	return a.emailUsate[strings.ToLower(email)], nil
}

func (a *archivioFinto) CreaCliente(_ context.Context, c Cliente, pwdHash string) (Cliente, error) {
	a.hash = pwdHash
	a.creati++
	c.ID, c.IDUtente = 9, 19
	return c, nil
}

func (a *archivioFinto) CercaCorrieri(_ context.Context, r Ricerca) ([]Corriere, int, error) {
	a.ricerca = r
	return []Corriere{{ID: 2, Nome: "Carlo"}}, 1, nil
}

func (a *archivioFinto) CercaUtenti(_ context.Context, r RicercaUtenti) ([]Utente, int, error) {
	a.utenti = r
	return []Utente{{ID: 3}}, 1, nil
}

func (a *archivioFinto) CreaUtente(_ context.Context, ruolo identita.Ruolo, idSede int64, c Cliente, pwdHash string) (Utente, error) {
	a.hash = pwdHash
	a.creati++
	a.sedeUtente = idSede
	return Utente{ID: 20, Ruolo: ruolo, IDRuolo: 4, Nome: c.Nome, Cognome: c.Cognome, Email: c.Email,
		Prefisso: c.Prefisso, NrTelefono: c.NrTelefono, Attivo: true}, nil
}

// Utenti di prova: 8 operatore, 30 cliente destinatario, 31 cliente solo mittente, 99
// inesistente (o amministratore); gli altri sono corrieri.
func (a *archivioFinto) UtentePerID(_ context.Context, idUtente int64) (Utente, error) {
	si, no := true, false
	switch idUtente {
	case 8:
		return Utente{ID: 8, Ruolo: identita.Operatore}, nil
	case 30:
		return Utente{ID: 30, Ruolo: identita.Cliente, Destinatario: &si}, nil
	case 31:
		return Utente{ID: 31, Ruolo: identita.Cliente, Destinatario: &no}, nil
	case 99:
		return Utente{}, &errori.NonTrovato{Messaggio: "nessun corriere, operatore o cliente con questo id utente"}
	}
	return Utente{ID: idUtente, Ruolo: identita.Corriere}, nil
}

func (a *archivioFinto) ModificaUtente(_ context.Context, idUtente int64, d DatiUtente) (Utente, error) {
	a.modifica = &d
	u := Utente{ID: idUtente, Ruolo: identita.Corriere, Nome: "Carlo", Email: "carlo@test.it", Attivo: true}
	if d.Nome != nil {
		u.Nome = *d.Nome
	}
	if d.Email != nil {
		u.Email = *d.Email
	}
	if d.Attivo != nil {
		u.Attivo = *d.Attivo
	}
	return u, nil
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
	var nonTrovato *errori.NonTrovato
	switch {
	case err == nil:
		return "nessuno"
	case errors.As(err, &validazione):
		return "400"
	case errors.As(err, &nonAutorizzato):
		return "403"
	case errors.As(err, &conflitto):
		return "409"
	case errors.As(err, &nonTrovato):
		return "404"
	}
	return err.Error()
}

var (
	operatore      = identita.Richiedente{Ruolo: identita.Operatore, ID: 1, IDSede: sedeTrento}
	amministratore = identita.Richiedente{Ruolo: identita.Amministratore, ID: 1}
	corriere       = identita.Richiedente{Ruolo: identita.Corriere, ID: 1, IDSede: sedeTrento}
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

func TestRicercheSoloOperatoreEAmministratore(t *testing.T) {
	a := &archivioFinto{}
	s := NuovoServizio(a, &postinoFinto{})
	ric := Ricerca{Testo: "dora", Pagina: 1, PerPagina: 20}
	for _, r := range []identita.Richiedente{operatore, amministratore} {
		if _, _, err := s.CercaClienti(context.Background(), r, ric); err != nil {
			t.Errorf("%s cerca clienti: %v", r.Ruolo, err)
		}
		if c, _, err := s.CercaCorrieri(context.Background(), r, ric); err != nil || len(c) != 1 {
			t.Errorf("%s cerca corrieri: %v %v", r.Ruolo, c, err)
		}
	}
	for _, r := range []identita.Richiedente{corriere, cliente} {
		if _, _, err := s.CercaClienti(context.Background(), r, ric); tipoErrore(err) != "403" {
			t.Errorf("%s non cerca clienti: %v", r.Ruolo, err)
		}
		if _, _, err := s.CercaCorrieri(context.Background(), r, ric); tipoErrore(err) != "403" {
			t.Errorf("%s non vede l'elenco dei corrieri: %v", r.Ruolo, err)
		}
	}
	if _, _, err := s.CercaCorrieri(context.Background(), operatore, Ricerca{Pagina: 0, PerPagina: 20}); tipoErrore(err) != "400" {
		t.Errorf("pagina 0: %v", err)
	}
}

func TestOperatoreCercaSoloNellaSuaSede(t *testing.T) {
	a := &archivioFinto{}
	s := NuovoServizio(a, &postinoFinto{})
	ric := Ricerca{Pagina: 1, PerPagina: 20}

	if _, _, err := s.CercaCorrieri(context.Background(), operatore, ric); err != nil || a.ricerca.IDSede == nil || *a.ricerca.IDSede != sedeTrento {
		t.Fatalf("l'operatore vede solo i corrieri della sua sede: %+v %v", a.ricerca, err)
	}
	altra := ric
	altra.IDSede = p(int64(sedeBolzano))
	if _, _, err := s.CercaCorrieri(context.Background(), operatore, altra); tipoErrore(err) != "403" {
		t.Fatalf("l'operatore non filtra su un'altra sede: %v", err)
	}
}

func TestAmministratoreVedeTutteLeSedi(t *testing.T) {
	a := &archivioFinto{}
	s := NuovoServizio(a, &postinoFinto{})
	ric := Ricerca{Pagina: 1, PerPagina: 20}

	if _, _, err := s.CercaCorrieri(context.Background(), amministratore, ric); err != nil || a.ricerca.IDSede != nil {
		t.Fatalf("senza filtro l'amministratore vede i corrieri di tutte le sedi: %+v %v", a.ricerca, err)
	}
	if _, _, err := s.CercaSedi(context.Background(), amministratore, RicercaSedi{Ricerca: ric}); err != nil || a.ricerca.IDSede != nil {
		t.Fatalf("l'amministratore vede tutte le sedi: %+v %v", a.ricerca, err)
	}
	if _, _, err := s.CercaSedi(context.Background(), amministratore, RicercaSedi{Ricerca: ric, Attiva: p(false)}); err != nil ||
		a.ricercaSedi.Attiva == nil || *a.ricercaSedi.Attiva {
		t.Fatalf("l'amministratore filtra le sedi disattivate: %+v %v", a.ricercaSedi, err)
	}
	filtro := ric
	filtro.IDSede = p(int64(sedeBolzano))
	if _, _, err := s.CercaCorrieri(context.Background(), amministratore, filtro); err != nil || *a.ricerca.IDSede != sedeBolzano {
		t.Fatalf("l'amministratore filtra per sede: %+v %v", a.ricerca, err)
	}
	filtro.IDSede = p(int64(0))
	if _, _, err := s.CercaCorrieri(context.Background(), amministratore, filtro); tipoErrore(err) != "400" {
		t.Fatalf("sede 0: atteso 400, ottenuto %v", err)
	}
}

func TestSediVisibili(t *testing.T) {
	a := &archivioFinto{}
	s := NuovoServizio(a, &postinoFinto{})
	ric := Ricerca{Pagina: 1, PerPagina: 20}

	if _, _, err := s.CercaSedi(context.Background(), operatore, RicercaSedi{Ricerca: ric}); err != nil || *a.ricerca.IDSede != sedeTrento {
		t.Fatalf("l'operatore vede solo la sua sede: %+v %v", a.ricerca, err)
	}
	if _, err := s.Sede(context.Background(), corriere, sedeTrento); err != nil {
		t.Fatalf("il corriere vede la sua sede: %v", err)
	}
	if _, err := s.Sede(context.Background(), corriere, sedeBolzano); tipoErrore(err) != "403" {
		t.Fatalf("il corriere non vede un'altra sede: %v", err)
	}
	if _, err := s.Sede(context.Background(), amministratore, sedeBolzano); err != nil {
		t.Fatalf("l'amministratore vede ogni sede: %v", err)
	}
	if _, _, err := s.CercaSedi(context.Background(), cliente, RicercaSedi{Ricerca: ric}); tipoErrore(err) != "403" {
		t.Fatalf("il cliente non vede le sedi: %v", err)
	}
}

func TestCreaSede(t *testing.T) {
	a := &archivioFinto{}
	s := NuovoServizio(a, &postinoFinto{})
	nuova := func() NuovaSede {
		return NuovaSede{Nome: p("  Veloce24   Verona "), Indirizzo: NuovoIndirizzo{Via: p("Via Mazzini"), Civico: p("5"),
			Citta: p("Verona"), CAP: p("37121"), Regione: p("Veneto"), Nazione: p("Italia")}}
	}

	sede, err := s.CreaSede(context.Background(), amministratore, nuova())
	if err != nil || a.nuovaSede != "Veloce24 Verona" || a.indirizzoSede.Citta != "Verona" || sede.ID != 30 {
		t.Fatalf("l'amministratore crea la sede, nome con gli spazi normalizzati: %+v %q %v", sede, a.nuovaSede, err)
	}
	if a.gruppoSede != 0 {
		t.Errorf("senza gruppo la sede va nel database principale (0), ottenuto %d", a.gruppoSede)
	}
	conGruppo := nuova()
	g := 2
	conGruppo.Gruppo = &g
	if sede, err := s.CreaSede(context.Background(), amministratore, conGruppo); err != nil || a.gruppoSede != 2 || sede.Gruppo != 2 {
		t.Errorf("sede nel gruppo 2: %+v %v", sede, err)
	}
	for _, r := range []identita.Richiedente{operatore, corriere, cliente} {
		if _, err := s.CreaSede(context.Background(), r, nuova()); tipoErrore(err) != "403" {
			t.Errorf("%s non crea sedi: %v", r.Ruolo, err)
		}
	}
	for nome, c := range map[string]struct {
		cambia func(*NuovaSede)
		campo  string
	}{
		"senza nome":        {func(n *NuovaSede) { n.Nome = nil }, "nome"},
		"nome troppo lungo": {func(n *NuovaSede) { n.Nome = p(strings.Repeat("x", 101)) }, "nome"},
		"senza città":       {func(n *NuovaSede) { n.Indirizzo.Citta = nil }, "indirizzo.citta"},
		"CAP italiano a 4":  {func(n *NuovaSede) { n.Indirizzo.CAP = p("3712") }, "indirizzo.cap"},
		"gruppo negativo":   {func(n *NuovaSede) { g := -1; n.Gruppo = &g }, "gruppo_spedizioni"},
		"gruppo oltre 9006": {func(n *NuovaSede) { g := 9007; n.Gruppo = &g }, "gruppo_spedizioni"},
	} {
		n := nuova()
		c.cambia(&n)
		if _, err := s.CreaSede(context.Background(), amministratore, n); tipoErrore(err) != "400" || !campoSegnalato(err, c.campo) {
			t.Errorf("%s: atteso 400 sul campo %s, ottenuto %v", nome, c.campo, err)
		}
	}
}

func TestModificaSede(t *testing.T) {
	ctx := context.Background()
	a := &archivioFinto{}
	s := NuovoServizio(a, &postinoFinto{})

	if sede, err := s.ModificaSede(ctx, amministratore, sedeTrento, ModificaSede{Nome: p(" Trento   Centro ")}); err != nil ||
		*a.modificaSede.Nome != "Trento Centro" || a.modificaSede.Attiva != nil || a.modificaSede.Indirizzo != nil || sede.Nome != "Trento Centro" {
		t.Fatalf("l'amministratore rinomina la sede, il resto non cambia: %+v %v", a.modificaSede, err)
	}
	if sede, err := s.ModificaSede(ctx, amministratore, sedeTrento, ModificaSede{Attiva: p(false)}); err != nil || sede.Attiva ||
		a.modificaSede.Nome != nil {
		t.Fatalf("l'amministratore disattiva la sede: %+v %v", a.modificaSede, err)
	}
	nuovo := NuovoIndirizzo{Via: p("Via Verdi"), Civico: p("3"), Citta: p("Trento"), CAP: p("38122"), Regione: p("Trentino-Alto Adige"), Nazione: p("Italia")}
	if _, err := s.ModificaSede(ctx, amministratore, sedeTrento, ModificaSede{Indirizzo: &nuovo}); err != nil ||
		a.modificaSede.Indirizzo == nil || a.modificaSede.Indirizzo.Via != "Via Verdi" {
		t.Fatalf("l'amministratore sposta la sede a un altro indirizzo: %+v %v", a.modificaSede, err)
	}

	for nome, c := range map[string]struct {
		m     ModificaSede
		campo string
	}{
		"nessun campo":        {ModificaSede{}, "corpo"},
		"nome vuoto":          {ModificaSede{Nome: p("  ")}, "nome"},
		"indirizzo senza via": {ModificaSede{Indirizzo: &NuovoIndirizzo{Civico: p("1"), Citta: p("Trento"), CAP: p("38122"), Regione: p("Trentino-Alto Adige"), Nazione: p("Italia")}}, "indirizzo.via"},
	} {
		a.modificaSede = nil
		if _, err := s.ModificaSede(ctx, amministratore, sedeTrento, c.m); tipoErrore(err) != "400" || !campoSegnalato(err, c.campo) || a.modificaSede != nil {
			t.Errorf("%s: atteso 400 su %s senza salvare, ottenuto %v", nome, c.campo, err)
		}
	}
	for _, r := range []identita.Richiedente{operatore, corriere, cliente} {
		if _, err := s.ModificaSede(ctx, r, sedeTrento, ModificaSede{Attiva: p(false)}); tipoErrore(err) != "403" {
			t.Errorf("%s non modifica le sedi: %v", r.Ruolo, err)
		}
	}
}
