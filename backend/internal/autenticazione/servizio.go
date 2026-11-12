package autenticazione

import (
	"context"
	"log"
	"net/url"
	"strings"
	"time"

	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

const (
	durataRecupero       = 30 * time.Minute
	maxRecuperiPerOra    = 3
	messaggioCredenziali = "email o password non corrette"
	oggettoMailRecupero  = "Recupero password LastMileDelivery"
)

type archivio interface {
	PerEmail(ctx context.Context, email string) (Account, bool, error)
	PerRuolo(ctx context.Context, r identita.Richiedente) (Account, error)
	AggiornaPassword(ctx context.Context, idUtente int32, hash string) error
	RecuperiRecenti(ctx context.Context, idUtente int32, dal time.Time) (int, error)
	CreaRecupero(ctx context.Context, idUtente int32, tokenHash string, scadenza time.Time) error
	UsaRecupero(ctx context.Context, tokenHash, nuovoHash string) (bool, error)
}

type Servizio struct {
	archivio     archivio
	firmatario   *Firmatario
	postino      Postino
	linkRecupero string
	adesso       func() time.Time
}

func NuovoServizio(a archivio, f *Firmatario, p Postino, linkRecupero string) *Servizio {
	return &Servizio{archivio: a, firmatario: f, postino: p, linkRecupero: linkRecupero, adesso: time.Now}
}

type Sessione struct {
	Token    string
	Scadenza time.Time
	Account  Account
	Ruolo    identita.Richiedente
}

func (s *Servizio) Login(ctx context.Context, email, password, ruolo *string) (Sessione, error) {
	e := &errori.Validazione{}
	if email == nil || strings.TrimSpace(*email) == "" {
		e.Aggiungi("email", "obbligatoria")
	}
	if password == nil || *password == "" {
		e.Aggiungi("password", "obbligatoria")
	}
	if err := e.Err(); err != nil {
		return Sessione{}, err
	}

	acc, trovato, err := s.archivio.PerEmail(ctx, strings.TrimSpace(*email))
	if err != nil {
		return Sessione{}, err
	}
	if !trovato {
		confrontoFittizio(*password)
		return Sessione{}, &errori.NonAutenticato{Messaggio: messaggioCredenziali}
	}
	if !passwordCorretta(acc.PwdHash, *password) || len(acc.Ruoli) == 0 {
		return Sessione{}, &errori.NonAutenticato{Messaggio: messaggioCredenziali}
	}

	scelto, err := scegliRuolo(acc.Ruoli, ruolo)
	if err != nil {
		return Sessione{}, err
	}
	token, scadenza, err := s.firmatario.Firma(acc.IDUtente, scelto)
	if err != nil {
		return Sessione{}, err
	}
	return Sessione{Token: token, Scadenza: scadenza, Account: acc, Ruolo: scelto}, nil
}

func scegliRuolo(ruoli []identita.Richiedente, richiesto *string) (identita.Richiedente, error) {
	if richiesto == nil {
		if len(ruoli) == 1 {
			return ruoli[0], nil
		}
		nomi := make([]string, len(ruoli))
		for i, r := range ruoli {
			nomi[i] = string(r.Ruolo)
		}
		e := &errori.Validazione{}
		e.Aggiungi("ruolo", "l'utente ha più ruoli: indicarne uno tra "+strings.Join(nomi, ", "))
		return identita.Richiedente{}, e
	}
	for _, r := range ruoli {
		if string(r.Ruolo) == *richiesto {
			return r, nil
		}
	}
	return identita.Richiedente{}, &errori.NonAutorizzato{Messaggio: "l'utente non ha il ruolo " + *richiesto}
}

func (s *Servizio) CambiaPassword(ctx context.Context, r identita.Richiedente, attuale, nuova *string) error {
	e := &errori.Validazione{}
	if attuale == nil || *attuale == "" {
		e.Aggiungi("password_attuale", "obbligatoria")
	}
	controllaPassword(e, "nuova_password", nuova)
	if attuale != nil && nuova != nil && *attuale == *nuova {
		e.Aggiungi("nuova_password", "deve essere diversa da quella attuale")
	}
	if err := e.Err(); err != nil {
		return err
	}

	acc, err := s.archivio.PerRuolo(ctx, r)
	if err != nil {
		return err
	}
	if !passwordCorretta(acc.PwdHash, *attuale) {
		return &errori.NonAutorizzato{Messaggio: "password attuale non corretta"}
	}
	hash, err := hashPassword(*nuova)
	if err != nil {
		return err
	}
	return s.archivio.AggiornaPassword(ctx, acc.IDUtente, hash)
}

func (s *Servizio) RichiediRecupero(ctx context.Context, email *string) error {
	e := &errori.Validazione{}
	if email == nil || strings.TrimSpace(*email) == "" {
		e.Aggiungi("email", "obbligatoria")
	}
	if err := e.Err(); err != nil {
		return err
	}

	acc, trovato, err := s.archivio.PerEmail(ctx, strings.TrimSpace(*email))
	if err != nil || !trovato {
		return err
	}
	recenti, err := s.archivio.RecuperiRecenti(ctx, acc.IDUtente, s.adesso().Add(-time.Hour))
	if err != nil {
		return err
	}
	if recenti >= maxRecuperiPerOra {
		log.Printf("recupero password: troppe richieste nell'ultima ora per l'utente %d, mail non inviata", acc.IDUtente)
		return nil
	}

	token, hash, err := nuovoTokenRecupero()
	if err != nil {
		return err
	}
	if err := s.archivio.CreaRecupero(ctx, acc.IDUtente, hash, s.adesso().Add(durataRecupero)); err != nil {
		return err
	}
	link := s.linkRecupero + "?token=" + url.QueryEscape(token)
	testo := "Ciao " + acc.Nome + ",\n\n" +
		"abbiamo ricevuto una richiesta di recupero della password del tuo account LastMileDelivery.\n" +
		"Per sceglierne una nuova apri questo link entro 30 minuti:\n\n" + link + "\n\n" +
		"Se non hai chiesto tu il recupero, ignora questa mail: la password attuale resta valida.\n"
	if err := s.postino.Invia(ctx, acc.Email, oggettoMailRecupero, testo); err != nil {
		log.Printf("recupero password: invio della mail all'utente %d fallito: %v", acc.IDUtente, err)
	}
	return nil
}

func (s *Servizio) ReimpostaPassword(ctx context.Context, token, nuova *string) error {
	e := &errori.Validazione{}
	if token == nil || strings.TrimSpace(*token) == "" {
		e.Aggiungi("token", "obbligatorio")
	}
	controllaPassword(e, "nuova_password", nuova)
	if err := e.Err(); err != nil {
		return err
	}
	hash, err := hashPassword(*nuova)
	if err != nil {
		return err
	}
	usato, err := s.archivio.UsaRecupero(ctx, hashToken(strings.TrimSpace(*token)), hash)
	if err != nil {
		return err
	}
	if !usato {
		e.Aggiungi("token", "non valido, scaduto o già usato: richiedere un nuovo recupero")
		return e
	}
	return nil
}
