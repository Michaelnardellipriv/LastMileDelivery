package httpx

import (
	"net/http"
	"slices"
	"strings"
)

type Middleware func(http.Handler) http.Handler

type Rotte struct {
	mux    *http.ServeMux
	metodi map[string][]string
}

func NuoveRotte() *Rotte {
	return &Rotte{mux: http.NewServeMux(), metodi: map[string][]string{}}
}

func (r *Rotte) Gestisci(metodo, percorso string, h http.Handler, mw ...Middleware) {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	r.mux.Handle(metodo+" "+percorso, h)
	r.metodi[percorso] = append(r.metodi[percorso], metodo)
}

func (r *Rotte) Handler() http.Handler {
	for percorso, metodi := range r.metodi {
		slices.Sort(metodi)
		ammessi := strings.Join(metodi, ", ")
		r.mux.HandleFunc(percorso, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Allow", ammessi)
			ScriviErrore(w, http.StatusMethodNotAllowed, "metodo non ammesso: usare "+ammessi)
		})
	}
	r.mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		ScriviErrore(w, http.StatusNotFound, "risorsa non trovata")
	})
	return intestazioniSicurezza(r.mux)
}

func intestazioniSicurezza(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
