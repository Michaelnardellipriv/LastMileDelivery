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
	maxCodOrdine = 50
	maxNota      = 2000
	// margine per gli orologi non sincronizzati tra client e server
	tolleranzaOrologio = 5 * time.Minute

	// l'operatore riporta in ritirata una spedizione in consegna solo dopo un tentativo di
	// consegna fallito di questa uscita (il corriere torna al magazzino con il pacco)
	messaggioRitiroSenzaTentativo = "si riporta in ritirata solo una spedizione con un tentativo di consegna " +
		"fallito registrato dal corriere in questa uscita"

	// un tentativo di consegna fallito chiude l'uscita: niente altro tentativo né consegna
	// finché l'operatore non la riporta in ritirata e il corriere non la rimette in consegna
	messaggioUscitaFallita = "in questa uscita è già stato registrato un tentativo di consegna fallito: " +
		"l'operatore deve riportare la spedizione in ritirata e il corriere rimetterla in consegna"
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

func controllaID(e *errori.Validazione, campo string, v int64) {
	if v <= 0 {
		e.Aggiungi(campo, "deve essere un id positivo")
	}
}

func controllaIndirizzi(e *errori.Validazione, ritiro, spedizione int64) {
	if ritiro > 0 && ritiro == spedizione {
		e.Aggiungi("id_indirizzo_spedizione", "deve essere diverso dall'indirizzo di ritiro")
	}
}

// controllaDataRichiesta: la spedizione è stata richiesta adesso o in passato, mai nel futuro.
func controllaDataRichiesta(e *errori.Validazione, data, adesso time.Time) {
	if data.After(adesso.Add(tolleranzaOrologio)) {
		e.Aggiungi("dataora_richiesta_spedizione", "non può essere nel futuro: è il momento in cui la spedizione è stata richiesta")
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

func stessoCorriere(a, b *int64) bool {
	return a != nil && b != nil && *a == *b
}

func elencoStati() string {
	nomi := make([]string, len(Stati))
	for i, s := range Stati {
		nomi[i] = s.String()
	}
	return strings.Join(nomi, ", ")
}
