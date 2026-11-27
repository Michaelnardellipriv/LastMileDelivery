package autenticazione

import (
	"context"
	"log"
	"net/url"
	"strings"
	"time"

	"lastmiledelivery/backend/internal/audit"
	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

const (
	durataRecupero        = 30 * time.Minute
	maxRecuperiPerOra     = 3
	messaggioCredenziali  = "email o password non corrette"
	messaggioSoloMittente = "accesso riservato ai clienti destinatari: non risulta nessuna spedizione destinata a te"
	messaggioDisattivato  = "account disattivato: contattare l'amministratore"
	oggettoMailRecupero   = "Recupero password LastMileDelivery"
)

type archivio interface {
	PerEmail(ctx context.Context, email string) (Account, bool, error)
	PerRuolo(ctx context.Context, r identita.Richiedente) (Account, error)
	// AggiornaPassword cambia la password e chiude le sessioni dell'utente tranne tranne
	AggiornaPassword(ctx context.Context, idUtente int64, hash, tranne string) error
	RecuperiRecenti(ctx context.Context, idUtente int64, dal time.Time) (int, error)
	CreaRecupero(ctx context.Context, idUtente int64, tokenHash string, scadenza time.Time) error
	// UsaRecupero imposta la nuova password e chiude tutte le sessioni dell'utente
	UsaRecupero(ctx context.Context, tokenHash, nuovoHash string) (bool, error)
	// DestinatarioDiSpedizioni dice se il cliente è (o è stato) destinatario di almeno una spedizione.
	DestinatarioDiSpedizioni(ctx context.Context, idCliente int64) (bool, error)
}

// registroSessioni: le sessioni del login nel database globale (package sessioni).
type registroSessioni interface {
	// Apri ricontrolla l'account (attivo, stessa password e sede) mentre apre la sessione
	Apri(ctx context.Context, idUtente int64, r identita.Richiedente, pwdHash string) (string, error)
	// Valida: la sessione è aperta; restituisce quando finisce
	Valida(ctx context.Context, r identita.Richiedente) (time.Time, error)
	Chiudi(ctx context.Context, idSessione, motivo string) error
}

type Servizio struct {
	archivio     archivio
	sessioni     registroSessioni
	firmatario   *Firmatario
	postino      Postino
	linkRecupero string
	adesso       func() time.Time
}

func NuovoServizio(a archivio, s registroSessioni, f *Firmatario, p Postino, linkRecupero string) *Servizio {
	return &Servizio{archivio: a, sessioni: s, firmatario: f, postino: p, linkRecupero: linkRecupero, adesso: time.Now}
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
		// nel log non c'è l'email (dato personale): basta sapere che l'accesso è fallito
		audit.Evento(ctx, "login_fallito", "motivo", "email_sconosciuta")
		return Sessione{}, &errori.NonAutenticato{Messaggio: messaggioCredenziali}
	}
	if !passwordCorretta(acc.PwdHash, *password) || len(acc.Ruoli) == 0 {
		audit.Evento(ctx, "login_fallito", "id_utente", acc.IDUtente, "motivo", "password")
		return Sessione{}, &errori.NonAutenticato{Messaggio: messaggioCredenziali}
	}
	// lo si dice solo dopo la password giusta, per non rivelare quali email sono registrate
	if !acc.Attivo {
		audit.Evento(ctx, "login_fallito", "id_utente", acc.IDUtente, "motivo", "disattivato")
		return Sessione{}, &errori.NonAutorizzato{Messaggio: messaggioDisattivato}
	}

	scelto, err := scegliRuolo(acc.Ruoli, ruolo)
	if err != nil {
		return Sessione{}, err
	}
	// Il sistema è per i clienti destinatari: chi è solo mittente (o non ha ancora avuto
	// spedizioni in arrivo) non entra.
	if scelto.Ruolo == identita.Cliente {
		destinatario, err := s.archivio.DestinatarioDiSpedizioni(ctx, scelto.ID)
		if err != nil {
			return Sessione{}, err
		}
		if !destinatario {
			return Sessione{}, &errori.NonAutorizzato{Messaggio: messaggioSoloMittente}
		}
	}
	// la sessione del login: il token vale solo finché resta aperta nel database
	if scelto.Sessione, err = s.sessioni.Apri(ctx, acc.IDUtente, scelto, acc.PwdHash); err != nil {
		return Sessione{}, err
	}
	token, scadenza, err := s.firmatario.Firma(acc.IDUtente, scelto)
	if err != nil {
		return Sessione{}, err
	}
	audit.Evento(audit.ConUtente(ctx, scelto.String()), "login")
	return Sessione{Token: token, Scadenza: scadenza, Account: acc, Ruolo: scelto}, nil
}

// Rinnova emette un nuovo token a partire da quello attuale, anche se è appena scaduto
// (app rimasta senza rete), purché la sua sessione sia ancora aperta nel database (non
// chiusa da logout, disattivazione, cambio di sede o nuova password, e non oltre
// DURATA_SESSIONE dal login) e l'utente sia attivo (altrimenti 403). Il nuovo token ha la
// stessa sessione e la sede che l'utente ha adesso.
func (s *Servizio) Rinnova(ctx context.Context, token string) (Sessione, error) {
	r, err := s.firmatario.VerificaPerRinnovo(token)
	if err != nil {
		return Sessione{}, &errori.NonAutenticato{Messaggio: err.Error()}
	}
	fineSessione, err := s.sessioni.Valida(ctx, r)
	if err != nil {
		audit.Evento(audit.ConUtente(ctx, r.String()), "rinnovo_rifiutato", "motivo", err.Error())
		return Sessione{}, err
	}
	acc, err := s.archivio.PerRuolo(ctx, r)
	if err != nil {
		return Sessione{}, err
	}
	if !acc.Attivo {
		return Sessione{}, &errori.NonAutorizzato{Messaggio: messaggioDisattivato}
	}
	attuale, trovato := r, false
	for _, ruolo := range acc.Ruoli {
		if ruolo.Ruolo == r.Ruolo && ruolo.ID == r.ID {
			attuale, trovato = ruolo, true // con la sede di adesso
		}
	}
	if !trovato {
		return Sessione{}, &errori.NonAutenticato{Messaggio: "utente non riconosciuto"}
	}
	attuale.Sessione = r.Sessione
	nuovo, scadenza, err := s.firmatario.FirmaFinoA(acc.IDUtente, attuale, fineSessione)
	if err != nil {
		return Sessione{}, err
	}
	return Sessione{Token: nuovo, Scadenza: scadenza, Account: acc, Ruolo: attuale}, nil
}

// Esci chiude la sessione del token (logout): da subito il token non vale più, su tutti
// e due i servizi. Come il rinnovo accetta anche un token appena scaduto (app rimasta
// senza rete, orologio del dispositivo avanti): altrimenti la sessione resterebbe aperta,
// e quel token rinnovabile, fino a DURATA_SESSIONE. Una sessione già chiusa non è un
// errore.
func (s *Servizio) Esci(ctx context.Context, token string) error {
	r, err := s.firmatario.VerificaPerRinnovo(token)
	if err != nil {
		return &errori.NonAutenticato{Messaggio: err.Error()}
	}
	ctx = audit.ConUtente(ctx, r.String())
	if err := s.sessioni.Chiudi(ctx, r.Sessione, identita.MotivoLogout); err != nil {
		return err
	}
	audit.Evento(ctx, "logout")
	return nil
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
	// le altre sessioni dell'utente (es. un token rubato) si chiudono; quella con cui ha
	// cambiato la password resta aperta
	if err := s.archivio.AggiornaPassword(ctx, acc.IDUtente, hash, r.Sessione); err != nil {
		return err
	}
	audit.Evento(ctx, "password_cambiata")
	return nil
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
	if !acc.Attivo {
		log.Printf("recupero password: l'utente %d è disattivato, mail non inviata", acc.IDUtente)
		return nil
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
	audit.Evento(ctx, "password_reimpostata")
	return nil
}
