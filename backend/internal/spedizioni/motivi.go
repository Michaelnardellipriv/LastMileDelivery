package spedizioni

import (
	"slices"

	"lastmiledelivery/backend/internal/errori"
)

// I motivi sono la parte pubblica di una nota: l'unica che vede il cliente.
// L'elenco è unico e lo gestisce l'amministratore (tabella motivi): vale per tutti
// i tentativi falliti e per i cambi di stato in annullata e persa. Chi li inserisce
// sceglie quello adatto, il sistema non li filtra in base allo stato o al tipo di tentativo.

// azioniConMotivo sono i cambi di stato che chiedono motivo pubblico e nota interna.
var azioniConMotivo = map[Stato]string{
	Annullata: "annullare una spedizione",
	Persa:     "segnalare una spedizione persa",
}

// controllaMotivo verifica che il motivo, se c'è, sia uno dei motivi attivi.
func controllaMotivo(e *errori.Validazione, motivi []string, motivo *string, obbligatorio string) {
	switch {
	case motivo == nil:
		if obbligatorio != "" {
			e.Aggiungi("motivo", obbligatorio)
		}
	case !slices.Contains(motivi, *motivo):
		e.Aggiungi("motivo", "valore non previsto: usare uno dei motivi di GET /api/motivi")
	}
}
