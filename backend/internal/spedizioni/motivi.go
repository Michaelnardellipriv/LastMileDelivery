package spedizioni

import (
	"slices"

	"lastmiledelivery/backend/internal/errori"
)

// Motivo è la parte pubblica di una nota: l'unica che vede il cliente.
type Motivo string

const (
	MotivoMittenteAssente       Motivo = "mittente assente"
	MotivoPaccoNonPronto        Motivo = "pacco non pronto"
	MotivoIndirizzoRitiroErrato Motivo = "indirizzo di ritiro errato"
	MotivoDestinatarioAssente   Motivo = "destinatario assente"
	MotivoIndirizzoErrato       Motivo = "indirizzo errato"
	MotivoRifiutata             Motivo = "rifiutata dal destinatario"
	MotivoAccessoImpossibile    Motivo = "accesso impossibile"
	MotivoRichiestaMittente     Motivo = "richiesta del mittente"
	MotivoRichiestaDestinatario Motivo = "richiesta del destinatario"
	MotivoDatiErrati            Motivo = "dati della spedizione errati"
	MotivoRitiroNonRiuscito     Motivo = "ritiro non riuscito"
	MotivoSmarritaInDeposito    Motivo = "smarrita in deposito"
	MotivoSmarritaInTrasporto   Motivo = "smarrita durante il trasporto"
	MotivoFurto                 Motivo = "furto"
	MotivoAltro                 Motivo = "altro"
)

// Motivi è l'elenco unico dei motivi ammessi. Vale per tutti i tentativi falliti e
// per i cambi di stato in annullata e persa: chi li inserisce sceglie quello adatto,
// il sistema non li filtra in base allo stato o al tipo di tentativo.
var Motivi = []Motivo{
	MotivoMittenteAssente, MotivoPaccoNonPronto, MotivoIndirizzoRitiroErrato,
	MotivoDestinatarioAssente, MotivoIndirizzoErrato, MotivoRifiutata, MotivoAccessoImpossibile,
	MotivoRichiestaMittente, MotivoRichiestaDestinatario, MotivoDatiErrati, MotivoRitiroNonRiuscito,
	MotivoSmarritaInDeposito, MotivoSmarritaInTrasporto, MotivoFurto,
	MotivoAltro,
}

// azioniConMotivo sono i cambi di stato che chiedono motivo pubblico e nota interna.
var azioniConMotivo = map[Stato]string{
	Annullata: "annullare una spedizione",
	Persa:     "segnalare una spedizione persa",
}

// controllaMotivo verifica che il motivo, se c'è, sia uno di quelli dell'elenco.
func controllaMotivo(e *errori.Validazione, motivo *string, obbligatorio string) {
	switch {
	case motivo == nil:
		if obbligatorio != "" {
			e.Aggiungi("motivo", obbligatorio)
		}
	case !slices.Contains(Motivi, Motivo(*motivo)):
		e.Aggiungi("motivo", "valore non previsto: usare uno dei motivi di GET /api/motivi")
	}
}
