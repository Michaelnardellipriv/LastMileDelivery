package server

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"lastmiledelivery/backend/internal/httpx"
	"lastmiledelivery/backend/internal/idempotenza"
	"lastmiledelivery/backend/internal/identita"
	"lastmiledelivery/backend/internal/posizioni"
	"lastmiledelivery/backend/internal/spedizioni"
)

type Config struct {
	MaxTentativi            int
	GiorniVisibilitaCliente int
}

func Nuovo(db *pgxpool.Pool, cfg Config) http.Handler {
	richiediUtente := identita.Richiedi(identita.NuovoRepository(db))
	idempotente := idempotenza.NuovoRegistro().Middleware

	rotte := httpx.NuoveRotte()

	servizioSpedizioni := spedizioni.NuovoServizio(spedizioni.NuovoRepository(db), cfg.MaxTentativi, cfg.GiorniVisibilitaCliente)
	spedizioni.NuovoHandler(servizioSpedizioni).Registra(rotte, richiediUtente, idempotente)

	servizioPosizioni := posizioni.NuovoServizio(posizioni.NuovoRepository(db))
	posizioni.NuovoHandler(servizioPosizioni).Registra(rotte, richiediUtente)

	return rotte.Handler()
}
