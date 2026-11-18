package server

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"lastmiledelivery/backend/internal/anagrafiche"
	"lastmiledelivery/backend/internal/autenticazione"
	"lastmiledelivery/backend/internal/configurazione"
	"lastmiledelivery/backend/internal/httpx"
	"lastmiledelivery/backend/internal/idempotenza"
	"lastmiledelivery/backend/internal/identita"
	"lastmiledelivery/backend/internal/posizioni"
	"lastmiledelivery/backend/internal/spedizioni"
)

type Config struct {
	Firmatario           *autenticazione.Firmatario
	Postino              autenticazione.Postino
	LinkRecuperoPassword string
	AmmettiHeaderID      bool
	CORS                 *httpx.CORS
}

func Nuovo(db *pgxpool.Pool, cfg Config) http.Handler {
	richiediUtente := identita.Richiedi(identita.NuovoRepository(db), cfg.Firmatario, cfg.AmmettiHeaderID)
	idempotente := idempotenza.NuovoRegistro().Middleware

	rotte := httpx.NuoveRotte()

	servizioAutenticazione := autenticazione.NuovoServizio(autenticazione.NuovoRepository(db), cfg.Firmatario, cfg.Postino, cfg.LinkRecuperoPassword)
	autenticazione.NuovoHandler(servizioAutenticazione).Registra(rotte, richiediUtente)

	// tentativi, giorni di visibilità e motivi li sceglie l'amministratore (tabelle impostazioni e motivi)
	repositoryConfigurazione := configurazione.NuovoRepository(db)
	configurazione.NuovoHandler(configurazione.NuovoServizio(repositoryConfigurazione)).Registra(rotte, richiediUtente, idempotente)

	servizioSpedizioni := spedizioni.NuovoServizio(spedizioni.NuovoRepository(db), repositoryConfigurazione)
	spedizioni.NuovoHandler(servizioSpedizioni).Registra(rotte, richiediUtente, idempotente)

	servizioAnagrafiche := anagrafiche.NuovoServizio(anagrafiche.NuovoRepository(db), cfg.Postino)
	anagrafiche.NuovoHandler(servizioAnagrafiche).Registra(rotte, richiediUtente, idempotente)

	servizioPosizioni := posizioni.NuovoServizio(posizioni.NuovoRepository(db))
	posizioni.NuovoHandler(servizioPosizioni).Registra(rotte, richiediUtente)

	if cfg.CORS == nil {
		return rotte.Handler()
	}
	return cfg.CORS.Middleware(rotte.Handler())
}
