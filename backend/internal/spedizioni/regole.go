package spedizioni

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

type passaggio struct {
	Da, A Stato
}

var transizioni = map[passaggio]identita.Ruolo{
	{Creata, Annullata}:      identita.Operatore,
	{Assegnata, Ritirata}:    identita.Corriere,
	{Assegnata, Annullata}:   identita.Operatore,
	{Ritirata, InConsegna}:   identita.Corriere,
	{Ritirata, Persa}:        identita.Operatore,
	{InConsegna, Consegnata}: identita.Corriere,
	{InConsegna, Ritirata}:   identita.Operatore,
	{InConsegna, Persa}:      identita.Operatore,
}

var altroEndpoint = map[Stato]string{
	Creata:        "non si imposta a mano: ogni spedizione nasce creata",
	Assegnata:     "si imposta assegnando un corriere: POST /api/spedizioni/{id}/assegnazioni",
	NonConsegnata: "si raggiunge registrando i tentativi di consegna falliti: POST /api/spedizioni/{id}/tentativi",
}

const (
	maxCodOrdine       = 50
	maxNota            = 2000
	tolleranzaOrologio = 5 * time.Minute
)

var formatoCodOrdine = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)

func raggiungibile(stato Stato, ruolo identita.Ruolo) bool {
	for p, r := range transizioni {
		if p.A == stato && r == ruolo {
			return true
		}
	}
	return false
}

func controllaCodOrdine(e *errori.Validazione, cod string) {
	switch {
	case cod == "":
		e.Aggiungi("cod_ordine", "obbligatorio")
	case utf8.RuneCountInString(cod) > maxCodOrdine:
		e.Aggiungi("cod_ordine", "al massimo 50 caratteri")
	case !formatoCodOrdine.MatchString(cod):
		e.Aggiungi("cod_ordine", "solo lettere, cifre e . _ / - senza spazi")
	}
}

func controllaID(e *errori.Validazione, campo string, v int32) {
	if v <= 0 {
		e.Aggiungi(campo, "deve essere un id positivo")
	}
}

func controllaIndirizzi(e *errori.Validazione, ritiro, spedizione int32) {
	if ritiro > 0 && ritiro == spedizione {
		e.Aggiungi("id_indirizzo_spedizione", "deve essere diverso dall'indirizzo di ritiro")
	}
}

func controllaData(e *errori.Validazione, data, adesso time.Time) {
	if !data.IsZero() && data.Before(adesso.Add(-tolleranzaOrologio)) {
		e.Aggiungi("dataora_richiesta_spedizione", "non può essere nel passato")
	}
}

func controllaNota(e *errori.Validazione, nota *string, obbligatoria string) {
	if nota == nil {
		if obbligatoria != "" {
			e.Aggiungi("nota", obbligatoria)
		}
		return
	}
	switch testo := strings.TrimSpace(*nota); {
	case testo == "":
		e.Aggiungi("nota", "non può essere vuota")
	case utf8.RuneCountInString(testo) > maxNota:
		e.Aggiungi("nota", "al massimo 2000 caratteri")
	}
}

func notaDaSalvare(nota, motivo *string) *NotaDaSalvare {
	if nota == nil {
		return nil
	}
	return &NotaDaSalvare{Testo: strings.TrimSpace(*nota), Motivo: motivo}
}

func stessoCorriere(a, b *int32) bool {
	return a != nil && b != nil && *a == *b
}

func elencoStati() string {
	nomi := make([]string, len(Stati))
	for i, s := range Stati {
		nomi[i] = s.String()
	}
	return strings.Join(nomi, ", ")
}
