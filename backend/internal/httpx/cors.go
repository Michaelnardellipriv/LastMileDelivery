package httpx

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	metodiCORS       = "GET, POST, PATCH, PUT, OPTIONS"
	intestazioniCORS = "Authorization, Content-Type, Idempotency-Key, X-Id-Operatore, X-Id-Corriere, X-Id-Cliente, X-Id-Amministratore"
	esposteCORS      = "Retry-After, Idempotent-Replayed, WWW-Authenticate, Allow"
)

type CORS struct {
	tutte   bool
	origini map[string]bool
}

func NuovoCORS(valore string) (*CORS, error) {
	if strings.TrimSpace(valore) == "" {
		return nil, fmt.Errorf("CORS_ORIGIN non impostata: indicare l'origine del frontend, es. http://localhost:5173")
	}
	c := &CORS{origini: map[string]bool{}}
	for _, o := range strings.Split(valore, ",") {
		o = strings.TrimSpace(o)
		if o == "*" {
			c.tutte = true
			continue
		}
		u, err := url.Parse(o)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, fmt.Errorf("CORS_ORIGIN: %q non è un'origine valida, es. http://localhost:5173", o)
		}
		if u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return nil, fmt.Errorf("CORS_ORIGIN: %q non deve avere percorso né barra finale, usare %s://%s", o, u.Scheme, u.Host)
		}
		c.origini[u.Scheme+"://"+u.Host] = true
	}
	return c, nil
}

func (c *CORS) ammessa(origine string) bool {
	return c.tutte || c.origini[origine]
}

func (c *CORS) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origine := r.Header.Get("Origin")
		w.Header().Add("Vary", "Origin")
		if origine == "" {
			next.ServeHTTP(w, r)
			return
		}
		preflight := r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
		if !c.ammessa(origine) {
			if preflight {
				ScriviErrore(w, http.StatusForbidden, "origine non ammessa: "+origine)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origine)
		if preflight {
			w.Header().Add("Vary", "Access-Control-Request-Method")
			w.Header().Add("Vary", "Access-Control-Request-Headers")
			w.Header().Set("Access-Control-Allow-Methods", metodiCORS)
			w.Header().Set("Access-Control-Allow-Headers", intestazioniCORS)
			w.Header().Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Access-Control-Expose-Headers", esposteCORS)
		next.ServeHTTP(w, r)
	})
}
