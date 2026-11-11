//go:build integration

package apitest

import (
	"net/http"
	"strings"
	"testing"
)

func TestAssegnazioni(t *testing.T) {
	f := preparaFixture(t)
	op := comeOperatore(f)
	c1, c2 := comeCorriere(f.idCorriere1), comeCorriere(f.idCorriere2)

	t.Run("assegna_e_riassegna_prima_e_dopo_il_ritiro", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "ASS")
		r := f.assegna(t, id, f.idCorriere1, 201)
		if r.corpo["stato"] != "assegnata" || int32(r.corpo["id_corriere"].(float64)) != f.idCorriere1 {
			t.Fatalf("assegnazione: %s", r.grezzo)
		}
		f.assegna(t, id, f.idCorriere1, 200)
		r = f.assegna(t, id, f.idCorriere2, 201)
		if r.corpo["stato"] != "assegnata" || int32(r.corpo["id_corriere"].(float64)) != f.idCorriere2 {
			t.Fatalf("riassegnazione prima del ritiro: %s", r.grezzo)
		}
		f.stato(t, id, c1, map[string]any{"stato": "ritirata"}, 403)
		f.stato(t, id, c2, map[string]any{"stato": "ritirata"}, 200)

		if r := f.assegna(t, id, f.idCorriere1, 409); !strings.Contains(string(r.grezzo), "tentativo di consegna") {
			t.Fatalf("ritirata senza tentativi di consegna: non si riassegna: %s", r.grezzo)
		}
		f.stato(t, id, c2, map[string]any{"stato": "in consegna"}, 200)
		f.tentativo(t, id, c2, map[string]any{"tipo": "consegna", "motivo": "destinatario assente", "nota": "nessuno in casa"}, 201)
		f.stato(t, id, op, map[string]any{"stato": "ritirata"}, 200)

		r = f.assegna(t, id, f.idCorriere1, 201)
		if r.corpo["stato"] != "ritirata" || int32(r.corpo["id_corriere"].(float64)) != f.idCorriere1 {
			t.Fatalf("riassegnazione dopo un tentativo di consegna fallito: %s", r.grezzo)
		}
		f.stato(t, id, c2, map[string]any{"stato": "in consegna"}, 403)
		f.stato(t, id, c1, map[string]any{"stato": "in consegna"}, 200)
		f.assegna(t, id, f.idCorriere2, 409)

		if n := f.conta(t, `SELECT count(*) FROM assegnazioni a JOIN spedizioni s ON s.id_assegnazione_corrente = a.id
			WHERE s.id = $1 AND a.id_operatore = $2`, id, f.idOperatore); n != 1 {
			t.Fatal("l'assegnazione corrente deve riportare l'operatore che l'ha fatta")
		}
	})

	t.Run("errori", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "ASSERR")
		f.invia(t, http.MethodPost, id, "/assegnazioni", op, map[string]any{}, 400)
		r := f.assegna(t, id, 999999999, 400)
		if !contiene(campiInErrore(r), "id_corriere") {
			t.Fatalf("atteso errore su id_corriere inesistente: %s", r.grezzo)
		}
		f.invia(t, http.MethodPost, id, "/assegnazioni", c1, map[string]any{"id_corriere": f.idCorriere1}, 403)
		f.assegna(t, 999999999, f.idCorriere1, 404)
	})
}
