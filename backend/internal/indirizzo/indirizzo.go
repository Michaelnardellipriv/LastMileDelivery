// Package indirizzo controlla i dati di un indirizzo (via, civico, città, CAP, regione,
// nazione). Lo usano tutti e due i servizi: utenti per l'indirizzo di una sede,
// spedizioni per gli indirizzi di ritiro e consegna (che stanno nel database del gruppo
// della sede).
package indirizzo

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"lastmiledelivery/backend/internal/errori"
)

var (
	contieneLettera  = regexp.MustCompile(`\p{L}`)
	formatoCivico    = regexp.MustCompile(`^(?i:snc|[0-9]{1,5}(?: ?[/-]? ?[a-z0-9]{1,3})?)$`)
	formatoLuogo     = regexp.MustCompile(`^\p{L}[\p{L} '’.-]*$`)
	formatoCAPItalia = regexp.MustCompile(`^[0-9]{5}$`)
	formatoCAPEstero = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9 -]{1,8})[A-Za-z0-9]$`)
)

// Nuovo: i campi come arrivano dalla richiesta (nil = mancante).
type Nuovo struct {
	Via, Civico, Citta, CAP, Regione, Nazione *string
}

// Dati: i campi controllati, senza spazi in più.
type Dati struct {
	Via, Civico, Citta, CAP, Regione, Nazione string
}

// Valida controlla un indirizzo nuovo; prefisso precede il nome dei campi negli errori
// (es. "indirizzo." per l'indirizzo di una sede).
func Valida(e *errori.Validazione, prefisso string, n Nuovo) Dati {
	interno := &errori.Validazione{}
	d := Dati{
		Via:     testo(interno, "via", n.Via, 150),
		Civico:  testo(interno, "civico", n.Civico, 10),
		Citta:   testo(interno, "citta", n.Citta, 100),
		CAP:     testo(interno, "cap", n.CAP, 10),
		Regione: testo(interno, "regione", n.Regione, 100),
		Nazione: testo(interno, "nazione", n.Nazione, 100),
	}
	controlla(interno, d)
	for _, x := range interno.Dettagli {
		e.Aggiungi(prefisso+x.Campo, x.Messaggio)
	}
	return d
}

// StessoLuogo dice se due indirizzi sono nella stessa città (con regione e nazione), senza
// badare a maiuscole e minuscole.
func StessoLuogo(a, b Dati) bool {
	return strings.EqualFold(a.Citta, b.Citta) && strings.EqualFold(a.Regione, b.Regione) &&
		strings.EqualFold(a.Nazione, b.Nazione)
}

func testo(e *errori.Validazione, campo string, v *string, massimo int) string {
	if v == nil || strings.TrimSpace(*v) == "" {
		e.Aggiungi(campo, "obbligatorio")
		return ""
	}
	t := strings.TrimSpace(*v)
	if utf8.RuneCountInString(t) > massimo {
		e.Aggiungi(campo, "troppo lungo")
	}
	return t
}

// controlla verifica il formato dei campi già presenti (quelli mancanti li ha già
// segnalati testo come obbligatori).
func controlla(e *errori.Validazione, d Dati) {
	if d.Via != "" && !contieneLettera.MatchString(d.Via) {
		e.Aggiungi("via", "deve contenere il nome della via, non solo numeri o simboli")
	}
	if d.Civico != "" && !formatoCivico.MatchString(d.Civico) {
		e.Aggiungi("civico", "formato non valido: numero con eventuale lettera o barra, es. 12, 12A, 12/B, oppure SNC")
	}
	for _, c := range []struct{ campo, valore string }{
		{"citta", d.Citta}, {"regione", d.Regione}, {"nazione", d.Nazione},
	} {
		if c.valore != "" && !formatoLuogo.MatchString(c.valore) {
			e.Aggiungi(c.campo, "solo lettere, spazi, apostrofi, punti e trattini")
		}
	}
	switch {
	case d.CAP == "":
	case strings.EqualFold(d.Nazione, "Italia"):
		if !formatoCAPItalia.MatchString(d.CAP) {
			e.Aggiungi("cap", "in Italia il CAP è di 5 cifre, es. 38121")
		}
	case !formatoCAPEstero.MatchString(d.CAP):
		e.Aggiungi("cap", "formato non valido: da 3 a 10 tra lettere, cifre, spazi e trattini")
	}
}
