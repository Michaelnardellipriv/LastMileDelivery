package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"lastmiledelivery/backend/internal/httpx"
)

type dbFinto struct{ errore error }

func (d dbFinto) Ping(context.Context) error { return d.errore }

func stato(t *testing.T, h http.Handler, percorso string) int {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, percorso, nil))
	return w.Code
}

func TestSaluteConDatabaseRaggiungibile(t *testing.T) {
	rotte := httpx.NuoveRotte()
	registraSalute(rotte, dbFinto{})
	h := rotte.Handler()
	if s := stato(t, h, "/health"); s != http.StatusOK {
		t.Errorf("/health: atteso 200, ottenuto %d", s)
	}
	if s := stato(t, h, "/ready"); s != http.StatusOK {
		t.Errorf("/ready: atteso 200, ottenuto %d", s)
	}
}

// Con il database giù il Pod resta vivo (non va riavviato) ma esce dal Service.
func TestDatabaseGiuPodVivoMaNonPronto(t *testing.T) {
	rotte := httpx.NuoveRotte()
	registraSalute(rotte, dbFinto{errore: errors.New("connessione rifiutata")})
	h := rotte.Handler()
	if s := stato(t, h, "/health"); s != http.StatusOK {
		t.Errorf("/health non dipende dal database: atteso 200, ottenuto %d", s)
	}
	if s := stato(t, h, "/ready"); s != http.StatusServiceUnavailable {
		t.Errorf("/ready senza database: atteso 503, ottenuto %d", s)
	}
}
