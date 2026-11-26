// Server di sviluppo: i due microservizi (utenti e spedizioni) nello stesso processo,
// ognuno con il suo database, su una sola porta. Smista i percorsi /api come l'Ingress
// di Kubernetes e le chiamate /interno tra i due restano in memoria. Serve per lavorare
// in locale con Flutter e per i test di integrazione; in Kubernetes girano
// cmd/utenti e cmd/spedizioni, ognuno nel suo Deployment.
//
// Variabili: DATABASE_URL (database globale) e DATABASE_SPEDIZIONI_URL (database
// delle spedizioni), più quelle comuni descritte in docs/configuration.md.
package main

import (
	"log"

	"lastmiledelivery/backend/internal/avvio"
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

	cfg, err := avvio.Config(ctx, true)
	if err != nil {
		return err
	}
	globale, err := avvio.Database(ctx, "DATABASE_URL")
	if err != nil {
		return err
	}
	defer globale.Close()
	// DATABASE_SPEDIZIONI_URL è il gruppo 0; DATABASE_SPEDIZIONI_URL_1, ... gli altri gruppi
	sped, _, err := avvio.GruppiSpedizioni(ctx, "DATABASE_SPEDIZIONI_URL")
	if err != nil {
		return err
	}
	defer func() {
		for _, db := range sped {
			db.Close()
		}
	}()

	return avvio.Servi(ctx, "utenti + spedizioni (sviluppo)", server.NuovoTutto(globale, sped, cfg))
}
