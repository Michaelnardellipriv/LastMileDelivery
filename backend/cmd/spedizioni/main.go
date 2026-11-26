// Microservizio spedizioni: database delle spedizioni (spedizioni, note, storico,
// impostazioni delle sedi, motivi, posizione dei corrieri).
//
// Variabili: DATABASE_URL (database delle spedizioni), URL_SERVIZIO_UTENTI (es.
// http://utenti-svc:8080), CHIAVE_INTERNA e JWT_SEGRETO (le stesse del servizio utenti),
// più quelle comuni descritte in docs/configuration.md.
package main

import (
	"log"

	"lastmiledelivery/backend/internal/avvio"
	"lastmiledelivery/backend/internal/database"
	"lastmiledelivery/backend/internal/interno"
	"lastmiledelivery/backend/internal/server"
)

func main() {
	if err := esegui(); err != nil {
		log.Fatal("avvio fallito: ", err)
	}
}

func esegui() error {
	ctx, stop := avvio.Contesto()
	defer stop()

	cfg, err := avvio.Config(ctx, false)
	if err != nil {
		return err
	}
	urlUtenti, err := avvio.Obbligatoria("URL_SERVIZIO_UTENTI")
	if err != nil {
		return err
	}
	// DATABASE_URL è il gruppo 0; DATABASE_URL_1, _2, ... i database degli altri gruppi
	pool, verificati, err := avvio.GruppiSpedizioni(ctx, "DATABASE_URL")
	if err != nil {
		return err
	}
	utenti := interno.NuovoClient("utenti", urlUtenti, cfg.ChiaveInterna, nil)
	gruppi, err := database.NuoviGruppi(pool, utenti)
	if err != nil {
		return err
	}
	defer gruppi.Chiudi()
	for _, c := range verificati {
		gruppi.Segna(c)
	}

	return avvio.Servi(ctx, "spedizioni", server.NuovoSpedizioni(gruppi, cfg, utenti))
}
