package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNuovoCORS(t *testing.T) {
	for valore, messaggio := range map[string]string{
		"":                       "CORS_ORIGIN non impostata",
		"http://localhost:5173/": `non deve avere percorso né barra finale, usare http://localhost:5173`,
		"http://x.test/app":      "non deve avere percorso",
		"localhost:5173":         "non è un'origine valida",
		"ftp://x.test":           "non è un'origine valida",
	} {
		if _, err := NuovoCORS(valore); err == nil || !strings.Contains(err.Error(), messaggio) {
			t.Errorf("%q: atteso errore con %q, ottenuto %v", valore, messaggio, err)
		}
	}
	for _, valore := range []string{"http://localhost:5173", "http://a.test, https://b.test:8443", "*"} {
		if _, err := NuovoCORS(valore); err != nil {
			t.Errorf("%q è valida: %v", valore, err)
		}
	}
}

func chiamaCORS(t *testing.T, c *CORS, metodo, origine string, preflight bool) *httptest.ResponseRecorder {
	t.Helper()
	arrivata := false
	h := c.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		arrivata = true
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(metodo, "/api/spedizioni", nil)
	if origine != "" {
		req.Header.Set("Origin", origine)
	}
	if preflight {
		req.Header.Set("Access-Control-Request-Method", "PATCH")
		req.Header.Set("Access-Control-Request-Headers", "authorization, content-type")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if preflight && arrivata {
		t.Error("la richiesta preflight non deve arrivare agli handler")
	}
	return rec
}

func TestCORSOrigineAmmessa(t *testing.T) {
	c, _ := NuovoCORS("http://localhost:5173, http://lastmile.test")

	rec := chiamaCORS(t, c, http.MethodOptions, "http://lastmile.test", true)
	h := rec.Header()
	if rec.Code != http.StatusNoContent || h.Get("Access-Control-Allow-Origin") != "http://lastmile.test" ||
		!strings.Contains(h.Get("Access-Control-Allow-Methods"), "PATCH") ||
		!strings.Contains(h.Get("Access-Control-Allow-Headers"), "Authorization") ||
		!strings.Contains(h.Get("Access-Control-Allow-Headers"), "Idempotency-Key") {
		t.Fatalf("preflight ammessa: %d %v", rec.Code, h)
	}

	rec = chiamaCORS(t, c, http.MethodGet, "http://localhost:5173", false)
	if rec.Code != http.StatusOK || rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" ||
		!strings.Contains(rec.Header().Get("Access-Control-Expose-Headers"), "WWW-Authenticate") {
		t.Fatalf("richiesta normale da origine ammessa: %d %v", rec.Code, rec.Header())
	}
}

func TestCORSOrigineNonAmmessa(t *testing.T) {
	c, _ := NuovoCORS("http://localhost:5173")

	if rec := chiamaCORS(t, c, http.MethodOptions, "http://cattivo.test", true); rec.Code != http.StatusForbidden ||
		rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("preflight da origine non ammessa: %d %v", rec.Code, rec.Header())
	}
	if rec := chiamaCORS(t, c, http.MethodGet, "http://cattivo.test", false); rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("nessun header CORS per un'origine non ammessa: il browser blocca la risposta")
	}
	if rec := chiamaCORS(t, c, http.MethodGet, "", false); rec.Code != http.StatusOK || rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("senza Origin (app mobile, curl) la richiesta passa senza header CORS")
	}
}

func TestCORSTutte(t *testing.T) {
	c, _ := NuovoCORS("*")
	if rec := chiamaCORS(t, c, http.MethodGet, "http://qualsiasi.test:1234", false); rec.Header().Get("Access-Control-Allow-Origin") != "http://qualsiasi.test:1234" {
		t.Fatalf("con * ogni origine è ammessa: %v", rec.Header())
	}
}
