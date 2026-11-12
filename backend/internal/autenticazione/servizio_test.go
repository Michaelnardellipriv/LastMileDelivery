package autenticazione

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

type archivioFinto struct {
	account   map[string]Account
	recuperi  map[string]int32
	recenti   int
	nuovoHash string
}

func (a *archivioFinto) PerEmail(_ context.Context, email string) (Account, bool, error) {
	acc, ok := a.account[strings.ToLower(email)]
	return acc, ok, nil
}

func (a *archivioFinto) PerRuolo(_ context.Context, r identita.Richiedente) (Account, error) {
	for _, acc := range a.account {
		for _, ruolo := range acc.Ruoli {
			if ruolo == r {
				return acc, nil
			}
		}
	}
	return Account{}, &errori.NonAutenticato{Messaggio: "utente non riconosciuto"}
}

func (a *archivioFinto) AggiornaPassword(_ context.Context, idUtente int32, hash string) error {
	for email, acc := range a.account {
		if acc.IDUtente == idUtente {
			acc.PwdHash = hash
			a.account[email] = acc
		}
	}
	return nil
}

func (a *archivioFinto) RecuperiRecenti(context.Context, int32, time.Time) (int, error) {
	return a.recenti, nil
}

func (a *archivioFinto) CreaRecupero(_ context.Context, idUtente int32, tokenHash string, _ time.Time) error {
	a.recuperi[tokenHash] = idUtente
	return nil
}

func (a *archivioFinto) UsaRecupero(_ context.Context, tokenHash, nuovoHash string) (bool, error) {
	if _, ok := a.recuperi[tokenHash]; !ok {
		return false, nil
	}
	delete(a.recuperi, tokenHash)
	a.nuovoHash = nuovoHash
	return true, nil
}

type postinoFinto struct {
	destinatario, testo string
}

func (p *postinoFinto) Invia(_ context.Context, destinatario, _, testo string) error {
	p.destinatario, p.testo = destinatario, testo
	return nil
}

func p[T any](v T) *T { return &v }

func tipoErrore(err error) string {
	var validazione *errori.Validazione
	var nonAutenticato *errori.NonAutenticato
	var nonAutorizzato *errori.NonAutorizzato
	var conflitto *errori.Conflitto
	switch {
	case err == nil:
		return "nessuno"
	case errors.As(err, &validazione):
		return "400"
	case errors.As(err, &nonAutenticato):
		return "401"
	case errors.As(err, &nonAutorizzato):
		return "403"
	case errors.As(err, &conflitto):
		return "409"
	}
	return err.Error()
}

var (
	operatore = identita.Richiedente{Ruolo: identita.Operatore, ID: 1}
	cliente   = identita.Richiedente{Ruolo: identita.Cliente, ID: 7}
	corriere  = identita.Richiedente{Ruolo: identita.Corriere, ID: 3}
)

func servizioDiProva(t *testing.T) (*Servizio, *archivioFinto, *postinoFinto) {
	t.Helper()
	hash, err := hashPassword("Password123")
	if err != nil {
		t.Fatal(err)
	}
	a := &archivioFinto{
		account: map[string]Account{
			"operatore@test.it":    {IDUtente: 10, Nome: "Oscar", Email: "operatore@test.it", PwdHash: hash, Ruoli: []identita.Richiedente{operatore}},
			"destinatario@test.it": {IDUtente: 11, Nome: "Dora", Email: "destinatario@test.it", PwdHash: hash, Ruoli: []identita.Richiedente{cliente}},
			"doppio@test.it":       {IDUtente: 12, Nome: "Carlo", Email: "doppio@test.it", PwdHash: hash, Ruoli: []identita.Richiedente{corriere, {Ruolo: identita.Cliente, ID: 8}}},
			"bloccato@test.it":     {IDUtente: 13, Nome: "Bloccato", Email: "bloccato@test.it", PwdHash: "!", Ruoli: []identita.Richiedente{{Ruolo: identita.Cliente, ID: 9}}},
		},
		recuperi: map[string]int32{},
	}
	postino := &postinoFinto{}
	s := NuovoServizio(a, firmatarioDiProva(time.Now()), postino, "https://app.test/reimposta")
	return s, a, postino
}

func TestLogin(t *testing.T) {
	ctx := context.Background()
	s, _, _ := servizioDiProva(t)

	for email, atteso := range map[string]identita.Richiedente{"operatore@test.it": operatore, "Destinatario@Test.it": cliente} {
		sess, err := s.Login(ctx, p(email), p("Password123"), nil)
		if err != nil || sess.Ruolo != atteso || sess.Token == "" {
			t.Fatalf("%s: login riuscito con il suo ruolo: %+v %v", email, sess, err)
		}
		if r, err := s.firmatario.Verifica(sess.Token); err != nil || r != atteso {
			t.Fatalf("%s: il token identifica l'utente: %+v %v", email, r, err)
		}
	}

	for nome, c := range map[string][2]*string{
		"password sbagliata": {p("operatore@test.it"), p("Sbagliata1")},
		"email inesistente":  {p("nessuno@test.it"), p("Password123")},
		"hash non valido":    {p("bloccato@test.it"), p("Password123")},
	} {
		if _, err := s.Login(ctx, c[0], c[1], nil); tipoErrore(err) != "401" {
			t.Errorf("%s: atteso 401, ottenuto %v", nome, err)
		}
	}
	for nome, c := range map[string][2]*string{
		"senza email":    {nil, p("Password123")},
		"senza password": {p("operatore@test.it"), nil},
	} {
		if _, err := s.Login(ctx, c[0], c[1], nil); tipoErrore(err) != "400" {
			t.Errorf("%s: atteso 400, ottenuto %v", nome, err)
		}
	}
}

func TestLoginConPiuRuoli(t *testing.T) {
	ctx := context.Background()
	s, _, _ := servizioDiProva(t)
	if _, err := s.Login(ctx, p("doppio@test.it"), p("Password123"), nil); tipoErrore(err) != "400" {
		t.Fatalf("con più ruoli va indicato quale: %v", err)
	}
	sess, err := s.Login(ctx, p("doppio@test.it"), p("Password123"), p("corriere"))
	if err != nil || sess.Ruolo != corriere {
		t.Fatalf("login come corriere: %+v %v", sess, err)
	}
	if _, err := s.Login(ctx, p("doppio@test.it"), p("Password123"), p("amministratore")); tipoErrore(err) != "403" {
		t.Fatalf("ruolo che l'utente non ha: atteso 403, ottenuto %v", err)
	}
}

func TestCambiaPassword(t *testing.T) {
	ctx := context.Background()
	s, a, _ := servizioDiProva(t)

	if err := s.CambiaPassword(ctx, cliente, p("Sbagliata1"), p("NuovaPassword1")); tipoErrore(err) != "403" {
		t.Fatalf("password attuale sbagliata: atteso 403, ottenuto %v", err)
	}
	for nome, c := range map[string][2]*string{
		"nuova troppo corta": {p("Password123"), p("corta")},
		"nuova uguale":       {p("Password123"), p("Password123")},
		"senza attuale":      {nil, p("NuovaPassword1")},
		"nuova troppo lunga": {p("Password123"), p(strings.Repeat("a", 73))},
	} {
		if err := s.CambiaPassword(ctx, cliente, c[0], c[1]); tipoErrore(err) != "400" {
			t.Errorf("%s: atteso 400, ottenuto %v", nome, err)
		}
	}
	if err := s.CambiaPassword(ctx, cliente, p("Password123"), p("NuovaPassword1")); err != nil {
		t.Fatal(err)
	}
	if !passwordCorretta(a.account["destinatario@test.it"].PwdHash, "NuovaPassword1") {
		t.Fatal("la nuova password è salvata")
	}
}

func TestRecuperoPassword(t *testing.T) {
	ctx := context.Background()
	s, a, postino := servizioDiProva(t)

	if err := s.RichiediRecupero(ctx, p("nessuno@test.it")); err != nil || postino.destinatario != "" {
		t.Fatalf("email sconosciuta: nessun errore e nessuna mail: %v %q", err, postino.destinatario)
	}
	if err := s.RichiediRecupero(ctx, p("destinatario@test.it")); err != nil || postino.destinatario != "destinatario@test.it" {
		t.Fatalf("mail inviata all'utente: %v %q", err, postino.destinatario)
	}
	_, link, ok := strings.Cut(postino.testo, "https://app.test/reimposta?token=")
	if !ok {
		t.Fatalf("la mail contiene il link: %s", postino.testo)
	}
	token := strings.Fields(link)[0]
	if _, salvato := a.recuperi[hashToken(token)]; !salvato || len(a.recuperi) != 1 {
		t.Fatal("nel database c'è solo l'hash del token")
	}

	if err := s.ReimpostaPassword(ctx, p(token), p("corta")); tipoErrore(err) != "400" {
		t.Fatalf("nuova password troppo corta: %v", err)
	}
	if err := s.ReimpostaPassword(ctx, p(token), p("NuovaPassword1")); err != nil || a.nuovoHash == "" {
		t.Fatalf("reimpostazione con token valido: %v", err)
	}
	if err := s.ReimpostaPassword(ctx, p(token), p("AltraPassword1")); tipoErrore(err) != "400" {
		t.Fatalf("token già usato: atteso 400, ottenuto %v", err)
	}

	postino.destinatario = ""
	a.recenti = maxRecuperiPerOra
	if err := s.RichiediRecupero(ctx, p("operatore@test.it")); err != nil || postino.destinatario != "" {
		t.Fatalf("troppe richieste nell'ultima ora: nessuna mail: %v %q", err, postino.destinatario)
	}
}
