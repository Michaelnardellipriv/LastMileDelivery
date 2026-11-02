package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func serverSenzaDatabase(t *testing.T) *httptest.Server {
	t.Helper()
	db, err := pgxpool.New(context.Background(), "postgres://x:x@127.0.0.1:1/x?connect_timeout=2")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	srv := httptest.NewServer(Nuovo(db, Config{MaxTentativi: 3}))
	t.Cleanup(srv.Close)
	return srv
}

func chiama(t *testing.T, srv *httptest.Server, metodo, percorso, corpo string, header map[string]string) (int, http.Header, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(metodo, srv.URL+percorso, strings.NewReader(corpo))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range header {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var j map[string]any
	json.NewDecoder(res.Body).Decode(&j)
	return res.StatusCode, res.Header, j
}

func TestDatabaseNonRaggiungibile503(t *testing.T) {
	srv := serverSenzaDatabase(t)
	for _, c := range []struct{ metodo, percorso string }{
		{http.MethodGet, "/api/spedizioni"},
		{http.MethodPost, "/api/spedizioni"},
		{http.MethodPatch, "/api/spedizioni/1"},
		{http.MethodPost, "/api/spedizioni/1/assegnazioni"},
		{http.MethodPost, "/api/spedizioni/1/tentativi"},
		{http.MethodPatch, "/api/spedizioni/1/stato"},
	} {
		stato, header, corpo := chiama(t, srv, c.metodo, c.percorso, `{}`, map[string]string{"X-Id-Corriere": "1"})
		if stato != http.StatusServiceUnavailable || header.Get("Retry-After") == "" {
			t.Fatalf("%s %s: atteso 503 con Retry-After, ottenuto %d %v", c.metodo, c.percorso, stato, corpo)
		}
		if corpo["errore"] != "servizio non disponibile, riprova" {
			t.Fatalf("messaggio inatteso: %v", corpo)
		}
	}
}

func TestSenzaIdentita401(t *testing.T) {
	srv := serverSenzaDatabase(t)
	if stato, _, _ := chiama(t, srv, http.MethodPatch, "/api/spedizioni/1/stato", `{"stato":"ritirata"}`, nil); stato != http.StatusUnauthorized {
		t.Fatalf("atteso 401, ottenuto %d", stato)
	}
	if stato, _, _ := chiama(t, srv, http.MethodGet, "/api/spedizioni", "", nil); stato != http.StatusUnauthorized {
		t.Fatalf("elenco senza identità: atteso 401, ottenuto %d", stato)
	}
	both := map[string]string{"X-Id-Corriere": "1", "X-Id-Cliente": "1"}
	if stato, _, _ := chiama(t, srv, http.MethodPatch, "/api/spedizioni/1/stato", `{"stato":"ritirata"}`, both); stato != http.StatusBadRequest {
		t.Fatalf("con due identità: atteso 400, ottenuto %d", stato)
	}
}

func TestPercorsiNonPrevisti(t *testing.T) {
	srv := serverSenzaDatabase(t)
	if stato, _, _ := chiama(t, srv, http.MethodGet, "/api/inesistente", "", nil); stato != http.StatusNotFound {
		t.Fatalf("atteso 404, ottenuto %d", stato)
	}
	for _, c := range []struct{ metodo, percorso, ammessi string }{
		{http.MethodDelete, "/api/spedizioni/1", "GET, PATCH"},
		{http.MethodGet, "/api/spedizioni/1/note", "POST"},
		{http.MethodDelete, "/api/spedizioni", "GET, POST"},
		{http.MethodPost, "/api/spedizioni/1/stato", "PATCH"},
	} {
		stato, header, _ := chiama(t, srv, c.metodo, c.percorso, "", nil)
		if stato != http.StatusMethodNotAllowed || header.Get("Allow") != c.ammessi {
			t.Fatalf("%s %s: atteso 405 con Allow %q, ottenuto %d %q", c.metodo, c.percorso, c.ammessi, stato, header.Get("Allow"))
		}
	}
}
