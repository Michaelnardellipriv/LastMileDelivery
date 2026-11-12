package autenticazione

import (
	"context"
	"fmt"
	"log"
	"mime"
	"net"
	"net/smtp"
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
	return smtp.SendMail(net.JoinHostPort(p.Host, p.Porta), auth, p.Mittente, []string{destinatario}, []byte(messaggio))
}

type PostinoLog struct{}

func (PostinoLog) Invia(_ context.Context, destinatario, oggetto, testo string) error {
	log.Printf("mail NON inviata (SMTP_HOST non impostata), a %s, oggetto %q:\n%s", destinatario, oggetto, testo)
	return nil
}
