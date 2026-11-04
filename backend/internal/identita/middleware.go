package identita

import (
	"context"
	"net/http"
	"strconv"

	"lastmiledelivery/backend/internal/httpx"
)

type verificaUtente interface {
	Esiste(ctx context.Context, ruolo Ruolo, id int32) (bool, error)
}

var intestazioni = []struct {
	nome  string
	ruolo Ruolo
}{
	{"X-Id-Operatore", Operatore},
	{"X-Id-Corriere", Corriere},
	{"X-Id-Cliente", Cliente},
	{"X-Id-Amministratore", Amministratore},
}

func Richiedi(utenti verificaUtente) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var rich Richiedente
			var valore string
			trovate := 0
			for _, h := range intestazioni {
				if v := r.Header.Get(h.nome); v != "" {
					trovate++
					rich.Ruolo, valore = h.ruolo, v
				}
			}
			switch {
			case trovate > 1:
				httpx.ScriviErrore(w, http.StatusBadRequest,
					"indicare una sola identità tra X-Id-Operatore, X-Id-Corriere, X-Id-Cliente e X-Id-Amministratore")
				return
			case trovate == 0:
				httpx.ScriviErrore(w, http.StatusUnauthorized, "utente non identificato")
				return
			}

			id, err := strconv.ParseInt(valore, 10, 32)
			if err != nil || id <= 0 {
				httpx.ScriviErrore(w, http.StatusUnauthorized, "utente non identificato")
				return
			}
			rich.ID = int32(id)

			ok, err := utenti.Esiste(r.Context(), rich.Ruolo, rich.ID)
			if err != nil {
				httpx.RispondiErrore(w, r, err)
				return
			}
			if !ok {
				httpx.ScriviErrore(w, http.StatusUnauthorized, "utente non riconosciuto")
				return
			}

			next.ServeHTTP(w, r.WithContext(NelContesto(r.Context(), rich)))
		})
	}
}
