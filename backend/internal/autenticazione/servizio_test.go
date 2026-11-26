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

// sessioniFinte: le sessioni del login, come nella tabella sessioni del database.
type sessioniFinte struct {
	aperte map[string]bool
	nuove  int
	fine   time.Time // fine delle sessioni (zero: nessun limite)
}

func (s *sessioniFinte) Apri(_ context.Context, _ int64, _ identita.Richiedente, _ string) (string, error) {
	s.nuove++
	id := "sessione-" + string(rune('0'+s.nuove))
	s.aperte[id] = true
	return id, nil
}

func (s *sessioniFinte) Valida(_ context.Context, r identita.Richiedente) (time.Time, error) {
	if !s.aperte[r.Sessione] {
		return time.Time{}, &errori.NonAutenticato{Messaggio: "sessione chiusa o scaduta: rifare il login"}
	}
	return s.fine, nil
}

func (s *sessioniFinte) Chiudi(_ context.Context, id, _ string) error {
	s.aperte[id] = false
	return nil
}

type archivioFinto struct {
	tranne      string         // sessione lasciata aperta dall'ultimo cambio di password
	destinatari map[int64]bool // clienti con almeno una spedizione in arrivo
	account     map[string]Account
	recuperi    map[string]int64
	recenti     int
	nuovoHash   string
}

func (a *archivioFinto) PerEmail(_ context.Context, email string) (Account, bool, error) {
	acc, ok := a.account[strings.ToLower(email)]
	return acc, ok, nil
}

func (a *archivioFinto) DestinatarioDiSpedizioni(_ context.Context, idCliente int64) (bool, error) {
	return a.destinatari[idCliente], nil
}

func (a *archivioFinto) PerRuolo(_ context.Context, r identita.Richiedente) (Account, error) {
	for _, acc := range a.account {
		for _, ruolo := range acc.Ruoli {
			if ruolo.Ruolo == r.Ruolo && ruolo.ID == r.ID {
				return acc, nil
			}
		}
	}
	return Account{}, &errori.NonAutenticato{Messaggio: "utente non riconosciuto"}
}

func (a *archivioFinto) AggiornaPassword(_ context.Context, idUtente int64, hash, tranne string) error {
	a.tranne = tranne
	for email, acc := range a.account {
		if acc.IDUtente == idUtente {
			acc.PwdHash = hash
			a.account[email] = acc
		}
	}
	return nil
}

func (a *archivioFinto) RecuperiRecenti(context.Context, int64, time.Time) (int, error) {
	return a.recenti, nil
}

func (a *archivioFinto) CreaRecupero(_ context.Context, idUtente int64, tokenHash string, _ time.Time) error {
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

var sessioniDiProva *sessioniFinte

func servizioDiProva(t *testing.T) (*Servizio, *archivioFinto, *postinoFinto) {
	t.Helper()
	hash, err := hashPassword("Password123")
	if err != nil {
		t.Fatal(err)
	}
	a := &archivioFinto{
		account: map[string]Account{
			"operatore@test.it":    {IDUtente: 10, Nome: "Oscar", Email: "operatore@test.it", PwdHash: hash, Attivo: true, Ruoli: []identita.Richiedente{operatore}},
			"destinatario@test.it": {IDUtente: 11, Nome: "Dora", Email: "destinatario@test.it", PwdHash: hash, Attivo: true, Ruoli: []identita.Richiedente{cliente}},
			"doppio@test.it":       {IDUtente: 12, Nome: "Carlo", Email: "doppio@test.it", PwdHash: hash, Attivo: true, Ruoli: []identita.Richiedente{corriere, {Ruolo: identita.Cliente, ID: 8}}},
			"bloccato@test.it":     {IDUtente: 13, Nome: "Bloccato", Email: "bloccato@test.it", PwdHash: "!", Attivo: true, Ruoli: []identita.Richiedente{{Ruolo: identita.Cliente, ID: 9}}},
			"disattivato@test.it":  {IDUtente: 15, Nome: "Dario", Email: "disattivato@test.it", PwdHash: hash, Ruoli: []identita.Richiedente{{Ruolo: identita.Corriere, ID: 30}}},
			"mittente@test.it":     {IDUtente: 14, Nome: "Marco", Email: "mittente@test.it", PwdHash: hash, Attivo: true, Ruoli: []identita.Richiedente{{Ruolo: identita.Cliente, ID: 20}}},
		},
		destinatari: map[int64]bool{cliente.ID: true},
		recuperi:    map[string]int64{},
	}
	postino := &postinoFinto{}
	sessioniDiProva = &sessioniFinte{aperte: map[string]bool{}}
	s := NuovoServizio(a, sessioniDiProva, firmatarioDiProva(time.Now()), postino, "https://app.test/reimposta")
	return s, a, postino
}

func TestLogin(t *testing.T) {
	ctx := context.Background()
	s, _, _ := servizioDiProva(t)

	for email, atteso := range map[string]identita.Richiedente{"operatore@test.it": operatore, "Destinatario@Test.it": cliente} {
		sess, err := s.Login(ctx, p(email), p("Password123"), nil)
		if err != nil || sess.Ruolo.Ruolo != atteso.Ruolo || sess.Ruolo.ID != atteso.ID || sess.Token == "" {
			t.Fatalf("%s: login riuscito con il suo ruolo: %+v %v", email, sess, err)
		}
		if r, err := s.firmatario.Verifica(sess.Token); err != nil || r.Ruolo != atteso.Ruolo || r.ID != atteso.ID ||
			r.Sessione == "" || !sessioniDiProva.aperte[r.Sessione] {
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

func TestLoginSoloClientiDestinatari(t *testing.T) {
	ctx := context.Background()
	s, a, _ := servizioDiProva(t)

	// credenziali giuste, ma nessuna spedizione in arrivo: è solo un mittente
	if _, err := s.Login(ctx, p("mittente@test.it"), p("Password123"), nil); tipoErrore(err) != "403" {
		t.Fatalf("il cliente solo mittente non entra: atteso 403, ottenuto %v", err)
	}
	// la password sbagliata resta 401: non si rivela nulla a chi non conosce le credenziali
	if _, err := s.Login(ctx, p("mittente@test.it"), p("Sbagliata1"), nil); tipoErrore(err) != "401" {
		t.Fatalf("password sbagliata: atteso 401, ottenuto %v", err)
	}
	// dalla prima spedizione in arrivo il cliente può entrare
	a.destinatari[20] = true
	if sess, err := s.Login(ctx, p("mittente@test.it"), p("Password123"), nil); err != nil || sess.Ruolo.Ruolo != identita.Cliente {
		t.Fatalf("il cliente diventato destinatario entra: %+v %v", sess, err)
	}
}

func TestUtenteDisattivato(t *testing.T) {
	ctx := context.Background()
	s, _, postino := servizioDiProva(t)

	if _, err := s.Login(ctx, p("disattivato@test.it"), p("Password123"), nil); tipoErrore(err) != "403" {
		t.Fatalf("l'utente disattivato non entra: atteso 403, ottenuto %v", err)
	}
	// con la password sbagliata resta 401: chi non conosce le credenziali non scopre che l'account esiste
	if _, err := s.Login(ctx, p("disattivato@test.it"), p("Sbagliata1"), nil); tipoErrore(err) != "401" {
		t.Fatalf("password sbagliata: atteso 401, ottenuto %v", err)
	}
	if err := s.RichiediRecupero(ctx, p("disattivato@test.it")); err != nil || postino.destinatario != "" {
		t.Fatalf("all'utente disattivato non parte la mail di recupero: %v %q", err, postino.destinatario)
	}
}

func TestLoginConPiuRuoli(t *testing.T) {
	ctx := context.Background()
	s, _, _ := servizioDiProva(t)
	if _, err := s.Login(ctx, p("doppio@test.it"), p("Password123"), nil); tipoErrore(err) != "400" {
		t.Fatalf("con più ruoli va indicato quale: %v", err)
	}
	sess, err := s.Login(ctx, p("doppio@test.it"), p("Password123"), p("corriere"))
	if err != nil || sess.Ruolo.Ruolo != corriere.Ruolo || sess.Ruolo.ID != corriere.ID {
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

// Il rinnovo dà un nuovo token, con la stessa sessione, solo se la sessione è ancora
// aperta (anche con un token appena scaduto) e l'utente è attivo; il logout chiude la
// sessione e da lì il token non si rinnova più.
func TestRinnovaEdEsci(t *testing.T) {
	ctx := context.Background()
	s, _, _ := servizioDiProva(t)
	login := time.Date(2026, 11, 24, 8, 0, 0, 0, time.UTC)
	adesso := login
	s.firmatario.adesso = func() time.Time { return adesso }
	s.firmatario.ConDurataSessione(12 * time.Hour)

	sess, err := s.Login(ctx, p("operatore@test.it"), p("Password123"), nil)
	if err != nil {
		t.Fatal(err)
	}
	adesso = login.Add(3 * time.Hour) // il token (1 ora) è scaduto, la sessione no
	nuovo, err := s.Rinnova(ctx, sess.Token)
	if err != nil || nuovo.Token == "" || nuovo.Ruolo.Sessione != sess.Ruolo.Sessione {
		t.Fatalf("token scaduto, sessione aperta: nuovo token della stessa sessione, ottenuto %+v %v", nuovo, err)
	}
	if r, err := s.firmatario.Verifica(nuovo.Token); err != nil || r.Sessione != sess.Ruolo.Sessione {
		t.Fatalf("il nuovo token porta la sessione: %+v %v", r, err)
	}
	// un token scaduto da più della durata di una sessione non si rinnova (la fine della
	// sessione nel database è controllata dai test di integrazione)
	adesso = login.Add(17 * time.Hour)
	if _, err := s.Rinnova(ctx, nuovo.Token); tipoErrore(err) != "401" {
		t.Fatalf("token scaduto da più della durata della sessione: atteso 401, ottenuto %v", err)
	}

	// il logout accetta un token scaduto (dentro la durata della sessione) e chiude la sessione
	adesso = login.Add(3 * time.Hour)
	if err := s.Esci(ctx, sess.Token); err != nil {
		t.Fatalf("logout con token scaduto: %v", err)
	}
	if err := s.Esci(ctx, "abc.def.ghi"); tipoErrore(err) != "401" {
		t.Fatalf("logout con token non valido: atteso 401, ottenuto %v", err)
	}
	adesso = login
	if _, err := s.Rinnova(ctx, sess.Token); tipoErrore(err) != "401" {
		t.Fatalf("dopo il logout: atteso 401, ottenuto %v", err)
	}
	if _, err := s.Rinnova(ctx, "abc.def.ghi"); tipoErrore(err) != "401" {
		t.Fatalf("token non valido: atteso 401, ottenuto %v", err)
	}

	// utente disattivato con una sessione ancora aperta: 403
	disattivato := identita.Richiedente{Ruolo: identita.Corriere, ID: 30, Sessione: "aperta"}
	sessioniDiProva.aperte["aperta"] = true
	tok, _, _ := s.firmatario.Firma(15, disattivato)
	if _, err := s.Rinnova(ctx, tok); tipoErrore(err) != "403" {
		t.Fatalf("utente disattivato: atteso 403, ottenuto %v", err)
	}
}

// Cambiando la password, l'utente chiude le altre sue sessioni (un token rubato smette di
// valere) ma resta dentro con quella da cui l'ha cambiata.
func TestCambioPasswordChiudeLeAltreSessioni(t *testing.T) {
	ctx := context.Background()
	s, archivio, _ := servizioDiProva(t)
	sess, err := s.Login(ctx, p("operatore@test.it"), p("Password123"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CambiaPassword(ctx, sess.Ruolo, p("Password123"), p("PasswordNuova9")); err != nil {
		t.Fatal(err)
	}
	if archivio.tranne != sess.Ruolo.Sessione {
		t.Fatalf("resta aperta solo la sessione da cui si cambia la password: %q", archivio.tranne)
	}
}
