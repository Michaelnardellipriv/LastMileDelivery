// Microservizio utenti: database globale (utenti, clienti, sedi, indirizzi), login e
// amministrazione degli utenti e delle sedi.
//
// Variabili: DATABASE_URL (database globale), URL_SERVIZIO_SPEDIZIONI (es.
// http://spedizioni-svc:8080), CHIAVE_INTERNA e JWT_SEGRETO (le stesse del servizio
// spedizioni), più quelle comuni descritte in docs/configuration.md.
package main

import (
	"log"

	"lastmiledelivery/backend/internal/avvio"
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
	urlSpedizioni, err := avvio.Obbligatoria("URL_SERVIZIO_SPEDIZIONI")
	if err != nil {
		return err
	}
	db, err := avvio.Database(ctx, "DATABASE_URL")
	if err != nil {
		return err
	}
	defer db.Close()

	spedizioni := interno.NuovoClient("spedizioni", urlSpedizioni, cfg.ChiaveInterna, nil)
	return avvio.Servi(ctx, "utenti", server.NuovoUtenti(db, cfg, spedizioni))
}
