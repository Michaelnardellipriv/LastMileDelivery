package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"lastmiledelivery/backend/internal/autenticazione"
	"lastmiledelivery/backend/internal/httpx"
	"lastmiledelivery/backend/internal/identita"
)

var firmatarioTest = autenticazione.NuovoFirmatario([]byte("segreto-di-prova-lungo-almeno-32-caratteri"), time.Hour)

func serverSenzaDatabase(t *testing.T, ammettiHeaderID bool) *httptest.Server {
	t.Helper()
	db, err := pgxpool.New(context.Background(), "postgres://x:x@127.0.0.1:1/x?connect_timeout=2")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	cors, err := httpx.NuovoCORS("http://localhost:5173")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(Nuovo(db, Config{
		MaxTentativi: 3, Firmatario: firmatarioTest, Postino: autenticazione.PostinoLog{}, AmmettiHeaderID: ammettiHeaderID,
		CORS: cors,
	}))
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
	srv := serverSenzaDatabase(t, true)
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
	srv := serverSenzaDatabase(t, true)
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

func TestTokenBearer(t *testing.T) {
	srv := serverSenzaDatabase(t, false)
	token, _, err := firmatarioTest.Firma(9, identita.Richiedente{Ruolo: identita.Corriere, ID: 1})
	if err != nil {
		t.Fatal(err)
	}
	stato, _, _ := chiama(t, srv, http.MethodGet, "/api/spedizioni", "", map[string]string{"Authorization": "Bearer " + token})
	if stato != http.StatusServiceUnavailable {
		t.Fatalf("token valido: la richiesta arriva al database (503 qui), ottenuto %d", stato)
	}
	for nome, h := range map[string]map[string]string{
		"header X-Id spenti":    {"X-Id-Corriere": "1"},
		"token manomesso":       {"Authorization": "Bearer " + token + "x"},
		"schema non Bearer":     {"Authorization": "Basic abc"},
		"token di altro server": {"Authorization": "Bearer " + tokenAltroSegreto(t)},
	} {
		stato, header, _ := chiama(t, srv, http.MethodGet, "/api/spedizioni", "", h)
		if stato != http.StatusUnauthorized || header.Get("WWW-Authenticate") != "Bearer" {
			t.Errorf("%s: atteso 401 con WWW-Authenticate, ottenuto %d", nome, stato)
		}
	}
}

func tokenAltroSegreto(t *testing.T) string {
	t.Helper()
	altro := autenticazione.NuovoFirmatario([]byte("un-altro-segreto-lungo-almeno-32-caratteri"), time.Hour)
	token, _, err := altro.Firma(9, identita.Richiedente{Ruolo: identita.Corriere, ID: 1})
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestCORSSulServer(t *testing.T) {
	srv := serverSenzaDatabase(t, false)
	for _, percorso := range []string{"/api/spedizioni/1/stato", "/api/auth/login", "/api/corrieri/posizioni"} {
		stato, header, _ := chiama(t, srv, http.MethodOptions, percorso, "", map[string]string{
			"Origin": "http://localhost:5173", "Access-Control-Request-Method": "PATCH",
			"Access-Control-Request-Headers": "authorization, content-type",
		})
		if stato != http.StatusNoContent || header.Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
			t.Errorf("preflight su %s: atteso 204 con l'origine ammessa (non 405), ottenuto %d %v", percorso, stato, header)
		}
	}
	stato, header, _ := chiama(t, srv, http.MethodGet, "/api/spedizioni", "", map[string]string{"Origin": "http://localhost:5173"})
	if stato != http.StatusUnauthorized || header.Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("anche le risposte di errore hanno gli header CORS, così il browser le legge: %d %v", stato, header)
	}
}

func TestPercorsiNonPrevisti(t *testing.T) {
	srv := serverSenzaDatabase(t, true)
	if stato, _, _ := chiama(t, srv, http.MethodGet, "/api/inesistente", "", nil); stato != http.StatusNotFound {
		t.Fatalf("atteso 404, ottenuto %d", stato)
	}
	for _, c := range []struct{ metodo, percorso, ammessi string }{
		{http.MethodDelete, "/api/spedizioni/1", "GET, PATCH"},
		{http.MethodGet, "/api/spedizioni/1/note", "POST"},
		{http.MethodDelete, "/api/spedizioni", "GET, POST"},
		{http.MethodPost, "/api/spedizioni/1/stato", "PATCH"},
		{http.MethodGet, "/api/auth/login", "POST"},
	} {
		stato, header, _ := chiama(t, srv, c.metodo, c.percorso, "", nil)
		if stato != http.StatusMethodNotAllowed || header.Get("Allow") != c.ammessi {
			t.Fatalf("%s %s: atteso 405 con Allow %q, ottenuto %d %q", c.metodo, c.percorso, c.ammessi, stato, header.Get("Allow"))
		}
	}
}
