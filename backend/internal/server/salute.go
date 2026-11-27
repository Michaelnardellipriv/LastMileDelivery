package server

import (
	"context"
	"net/http"
	"time"

	"lastmiledelivery/backend/internal/httpx"
)

type pingabile interface {
	Ping(ctx context.Context) error
}

type statoDTO struct {
	Stato string `json:"stato"`
}

// registraSalute aggiunge i controlli usati da Kubernetes (fuori da /api, quindi non
// passano dall'Ingress):
//   - GET /health: il processo risponde. Non guarda il database: se il database è giù
//     riavviare il backend non serve (liveness probe).
//   - GET /ready: il backend può lavorare, cioè il database risponde. Finché no, il Pod
//     non riceve traffico dal Service (readiness probe).
func registraSalute(rotte *httpx.Rotte, db pingabile) {
	rotte.Gestisci(http.MethodGet, "/health", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		httpx.ScriviJSON(w, http.StatusOK, statoDTO{Stato: "ok"})
	}))
	rotte.Gestisci(http.MethodGet, "/ready", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, annulla := context.WithTimeout(r.Context(), 2*time.Second)
		defer annulla()
		if err := db.Ping(ctx); err != nil {
			w.Header().Set("Retry-After", "5")
			httpx.ScriviJSON(w, http.StatusServiceUnavailable, statoDTO{Stato: "database non raggiungibile"})
			return
		}
		httpx.ScriviJSON(w, http.StatusOK, statoDTO{Stato: "pronto"})
	}))
}
