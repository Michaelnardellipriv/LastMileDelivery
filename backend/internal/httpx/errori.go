package httpx

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"lastmiledelivery/backend/internal/database"
	"lastmiledelivery/backend/internal/errori"
)

func RispondiErrore(w http.ResponseWriter, r *http.Request, err error) {
	var validazione *errori.Validazione
	var riferimento *errori.RiferimentoNonTrovato
	var nonAutorizzato *errori.NonAutorizzato
	var nonTrovato *errori.NonTrovato
	var conflitto *errori.Conflitto
	var troppoPresto *errori.TroppoPresto

	switch {
	case errors.As(err, &validazione):
		ScriviErrore(w, http.StatusBadRequest, "dati non validi", validazione.Dettagli...)
	case errors.As(err, &riferimento):
		ScriviErrore(w, http.StatusBadRequest, "dati non validi",
			errori.ErroreCampo{Campo: riferimento.Campo, Messaggio: riferimento.Messaggio})
	case errors.As(err, &nonAutorizzato):
		ScriviErrore(w, http.StatusForbidden, nonAutorizzato.Messaggio)
	case errors.As(err, &nonTrovato):
		ScriviErrore(w, http.StatusNotFound, nonTrovato.Messaggio)
	case errors.As(err, &conflitto):
		ScriviErrore(w, http.StatusConflict, conflitto.Messaggio, conflitto.Dettagli...)
	case errors.As(err, &troppoPresto):
		w.Header().Set("Retry-After", strconv.Itoa(troppoPresto.RiprovaTraSecondi))
		ScriviErrore(w, http.StatusTooManyRequests, troppoPresto.Messaggio)
	case database.NonDisponibile(err):
		log.Printf("database non disponibile: %s %s: %v", r.Method, r.URL.Path, err)
		w.Header().Set("Retry-After", "5")
		ScriviErrore(w, http.StatusServiceUnavailable, "servizio non disponibile, riprova")
	default:
		log.Printf("errore interno: %s %s: %v", r.Method, r.URL.Path, err)
		ScriviErrore(w, http.StatusInternalServerError, "errore interno")
	}
}
