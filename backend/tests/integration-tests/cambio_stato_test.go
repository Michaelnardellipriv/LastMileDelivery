//go:build integration

package apitest

import (
	"net/http"
	"testing"
)

func TestCambioStato(t *testing.T) {
	f := preparaFixture(t)
	op, c1 := comeOperatore(f), comeCorriere(f.idCorriere1)

	t.Run("ciclo_completo_fino_a_consegnata", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "CICLO")
		f.assegna(t, id, f.idCorriere1, 201)
		f.stato(t, id, c1, map[string]any{"stato": "ritirata"}, 200)
		f.stato(t, id, c1, map[string]any{"stato": "in consegna"}, 200)
		r := f.stato(t, id, c1, map[string]any{"stato": "consegnata"}, 200)
		if r.corpo["stato"] != "consegnata" || r.corpo["is_reso"] != false {
			t.Fatalf("stato finale errato: %s", r.grezzo)
		}
		if n := f.storico(t, id); n != 5 {
			t.Fatalf("attese 5 righe di storico, trovate %d", n)
		}
		if n := f.conta(t, `SELECT count(*) FROM cambiamenti_spedizioni WHERE id_spedizione = $1 AND id_corriere = $2`, id, f.idCorriere1); n != 3 {
			t.Fatalf("attese 3 righe di storico del corriere, trovate %d", n)
		}
		f.stato(t, id, c1, map[string]any{"stato": "in consegna"}, 409)
		f.modifica(t, id, op, map[string]any{"is_reso": true}, 409)
	})

	t.Run("ripetere_lo_stesso_stato_non_cambia_niente", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "RIPETI")
		f.assegna(t, id, f.idCorriere1, 201)
		f.stato(t, id, c1, map[string]any{"stato": "ritirata"}, 200)
		prima := f.storico(t, id)
		f.stato(t, id, c1, map[string]any{"stato": "ritirata"}, 200)
		if dopo := f.storico(t, id); dopo != prima {
			t.Fatalf("la ripetizione non deve scrivere nello storico: da %d a %d", prima, dopo)
		}
	})

	t.Run("annullamento_solo_prima_del_ritiro", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "ANN")
		f.assegna(t, id, f.idCorriere1, 201)
		r := f.stato(t, id, op, map[string]any{"stato": "annullata", "motivo": "richiesta del mittente", "nota": "il negozio ha annullato l'ordine"}, 200)
		if r.corpo["annullata_il"] == nil {
			t.Fatalf("manca la data di annullamento: %s", r.grezzo)
		}
		if n := f.conta(t, `SELECT count(*) FROM cambiamenti_spedizioni WHERE id_spedizione = $1 AND operazione = 'annullamento' AND id_operatore = $2`, id, f.idOperatore); n != 1 {
			t.Fatalf("annullamento non registrato nello storico con l'operatore")
		}

		id2 := f.nuovaSpedizione(t, "ANN2")
		f.assegna(t, id2, f.idCorriere1, 201)
		f.stato(t, id2, c1, map[string]any{"stato": "ritirata"}, 200)
		f.stato(t, id2, op, map[string]any{"stato": "annullata", "motivo": "altro", "nota": "x"}, 409)
	})

	t.Run("persa_solo_operatore_con_nota", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "PERSA")
		f.assegna(t, id, f.idCorriere1, 201)
		f.stato(t, id, op, map[string]any{"stato": "persa", "motivo": "furto", "nota": "non ancora ritirata"}, 409)
		f.stato(t, id, c1, map[string]any{"stato": "ritirata"}, 200)
		f.stato(t, id, c1, map[string]any{"stato": "persa", "motivo": "furto", "nota": "pacco non trovato"}, 403)
		f.stato(t, id, op, map[string]any{"stato": "persa", "motivo": "furto"}, 400)
		r := f.stato(t, id, op, map[string]any{"stato": "persa", "nota": "non trovato nel furgone"}, 400)
		if !contiene(campiInErrore(r), "motivo") {
			t.Fatalf("atteso errore sul motivo: %s", r.grezzo)
		}
		f.stato(t, id, op, map[string]any{"stato": "persa", "motivo": "smarrita durante il trasporto", "nota": "non trovato nel furgone"}, 200)
		if n := f.conta(t, `SELECT count(*) FROM note WHERE id_spedizione = $1 AND motivo = 'smarrita durante il trasporto'
			AND nota = 'non trovato nel furgone' AND id_operatore_logistico_creata = $2`, id, f.idOperatore); n != 1 {
			t.Fatal("motivo e nota della perdita devono essere salvati con l'operatore")
		}
		if r := f.modifica(t, id, op, map[string]any{"is_reso": true}, 200); r.corpo["is_reso"] != true {
			t.Fatalf("reso su spedizione persa: %s", r.grezzo)
		}
		f.stato(t, id, c1, map[string]any{"stato": "in consegna"}, 409)
	})

	t.Run("stati_che_si_impostano_altrove_400", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "ALTROVE")
		for _, stato := range []string{"assegnata", "non consegnata", "creata", "volata"} {
			f.stato(t, id, op, map[string]any{"stato": stato}, 400)
		}
		f.stato(t, id, op, map[string]any{}, 400)
		f.stato(t, id, op, map[string]any{"stato": "annullata", "id_corriere": 1}, 400)
	})

	t.Run("transizioni_non_ammesse_409", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "SALTO")
		f.stato(t, id, c1, map[string]any{"stato": "consegnata"}, 409)
		f.assegna(t, id, f.idCorriere1, 201)
		f.stato(t, id, c1, map[string]any{"stato": "consegnata"}, 409)
	})

	t.Run("spedizione_inesistente_404_e_id_non_valido_400", func(t *testing.T) {
		if r := f.chiama(t, http.MethodPatch, "/api/spedizioni/999999999/stato", map[string]any{"stato": "annullata", "motivo": "altro", "nota": "x"}, op); r.stato != http.StatusNotFound {
			t.Fatalf("atteso 404, ottenuto %d", r.stato)
		}
		if r := f.chiama(t, http.MethodPatch, "/api/spedizioni/abc/stato", map[string]any{"stato": "annullata"}, op); r.stato != http.StatusBadRequest {
			t.Fatalf("atteso 400, ottenuto %d", r.stato)
		}
	})
}
