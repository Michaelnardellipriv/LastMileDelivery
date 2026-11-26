package autenticazione

import (
	"context"
	"errors"
	"fmt"
	"log"
	"mime"
	"net"
	"net/smtp"
	"net/textproto"
	"strings"
)

type Postino interface {
	Invia(ctx context.Context, destinatario, oggetto, testo string) error
}

type PostinoSMTP struct {
	Host     string
	Porta    string
	Utente   string
	Password string
	Mittente string
}

func (p PostinoSMTP) Invia(_ context.Context, destinatario, oggetto, testo string) error {
	if strings.ContainsAny(destinatario, "\r\n") {
		return fmt.Errorf("indirizzo destinatario non valido")
	}
	var auth smtp.Auth
	if p.Utente != "" {
		auth = smtp.PlainAuth("", p.Utente, p.Password, p.Host)
	}
	messaggio := "From: " + p.Mittente + "\r\n" +
		"To: " + destinatario + "\r\n" +
		"Subject: " + mime.QEncoding.Encode("utf-8", oggetto) + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"Content-Transfer-Encoding: 8bit\r\n" +
		"\r\n" + strings.ReplaceAll(testo, "\n", "\r\n")
	err := smtp.SendMail(net.JoinHostPort(p.Host, p.Porta), auth, p.Mittente, []string{destinatario}, []byte(messaggio))
	var risposta *textproto.Error
	if errors.As(err, &risposta) {
		// il testo della risposta del server SMTP può contenere l'indirizzo del
		// destinatario (es. "550 <mario@...>: mailbox unavailable"): chi chiama scrive
		// l'errore nel log, quindi si tiene solo il codice
		return fmt.Errorf("il server SMTP ha rifiutato la mail (codice %d)", risposta.Code)
	}
	return err // connessione, TLS: non contengono il destinatario
}

// PostinoLog sostituisce l'invio quando SMTP_HOST non c'è. Il testo di una mail può
// contenere una password o un link di recupero: lo scrive nel log solo in sviluppo
// (MostraTesto, header X-Id-* attivi); altrimenti solo l'oggetto.
type PostinoLog struct {
	MostraTesto bool
}

func (p PostinoLog) Invia(_ context.Context, destinatario, oggetto, testo string) error {
	if !p.MostraTesto {
		log.Printf("mail NON inviata (SMTP_HOST non impostata), oggetto %q: testo omesso (contiene credenziali)", oggetto)
		return nil
	}
	log.Printf("mail NON inviata (SMTP_HOST non impostata, SOLO SVILUPPO), a %s, oggetto %q:\n%s", destinatario, oggetto, testo)
	return nil
}
