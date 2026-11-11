//go:build integration

package apitest

import (
	"net/http"
	"strconv"
	"testing"
)

func TestTentativi(t *testing.T) {
	f := preparaFixture(t)
	op, c1, c2 := comeOperatore(f), comeCorriere(f.idCorriere1), comeCorriere(f.idCorriere2)
	ritiroFallito := map[string]any{"tipo": "ritiro", "nota": "mittente assente", "motivo": "mittente assente"}
	consegnaFallita := map[string]any{"tipo": "consegna", "nota": "citofono rotto", "motivo": "destinatario assente"}
	consegnaSenzaMotivo := map[string]any{"tipo": "consegna", "nota": "traffico, ripasso domani"}

	t.Run("tre_ritiri_falliti_poi_annullata_dall_operatore", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "RITIRO")
		f.assegna(t, id, f.idCorriere1, 201)

		for tentativo := 1; tentativo <= 3; tentativo++ {
			r := f.tentativo(t, id, c1, ritiroFallito, 201)
			if r.corpo["stato"] != "assegnata" || int(r.corpo["tentativi_ritiro_falliti"].(float64)) != tentativo {
				t.Fatalf("ritiro fallito %d: %s", tentativo, r.grezzo)
			}
		}
		f.tentativo(t, id, c1, ritiroFallito, 409)
		f.stato(t, id, c1, map[string]any{"stato": "ritirata"}, 409)
		f.assegna(t, id, f.idCorriere2, 409)
		f.stato(t, id, c1, map[string]any{"stato": "annullata", "motivo": "altro", "nota": "x"}, 403)
		f.stato(t, id, op, map[string]any{"stato": "annullata", "motivo": "ritiro non riuscito"}, 400)
		note := f.conta(t, `SELECT count(*) FROM note WHERE id_spedizione = $1`, id)
		if r := f.stato(t, id, op, map[string]any{"stato": "annullata"}, 200); r.corpo["stato"] != "annullata" {
			t.Fatalf("dopo 3 ritiri falliti si annulla senza motivo né nota: %s", r.grezzo)
		}
		if n := f.conta(t, `SELECT count(*) FROM note WHERE id_spedizione = $1`, id); n != note {
			t.Fatalf("annullare dopo 3 ritiri falliti non aggiunge una quarta nota: da %d a %d", note, n)
		}

		meno := f.nuovaSpedizione(t, "RITIRO2")
		f.assegna(t, meno, f.idCorriere1, 201)
		f.tentativo(t, meno, c1, ritiroFallito, 201)
		f.stato(t, meno, op, map[string]any{"stato": "annullata"}, 400)
		f.stato(t, meno, op, map[string]any{"stato": "annullata", "motivo": "ritiro non riuscito", "nota": "il negozio ha chiuso"}, 200)

		if n := f.conta(t, `SELECT count(*) FROM tentativi t JOIN note n ON n.id = t.id_nota
			WHERE t.id_spedizione = $1 AND t.tipo = 'ritiro' AND n.id_corriere_creata = $2
			  AND n.nota = 'mittente assente' AND n.motivo = 'mittente assente' AND NOT n.is_interna`, id, f.idCorriere1); n != 3 {
			t.Fatalf("attesi 3 tentativi di ritiro con nota del corriere e motivo pubblico, trovati %d", n)
		}
	})

	t.Run("tentativi_di_consegna_fino_a_non_consegnata", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "TENT")
		f.assegna(t, id, f.idCorriere1, 201)
		f.stato(t, id, c1, map[string]any{"stato": "ritirata"}, 200)

		attesi := []string{"in consegna", "in consegna", "non consegnata"}
		for i, atteso := range attesi {
			f.stato(t, id, c1, map[string]any{"stato": "in consegna"}, 200)
			if r := f.tentativo(t, id, c1, consegnaSenzaMotivo, 400); !contiene(campiInErrore(r), "motivo") {
				t.Fatalf("il motivo è obbligatorio a ogni tentativo: %s", r.grezzo)
			}
			r := f.tentativo(t, id, c1, consegnaFallita, 201)
			if r.corpo["stato"] != atteso || int(r.corpo["tentativi_consegna_falliti"].(float64)) != i+1 || r.corpo["is_reso"] != false {
				t.Fatalf("tentativo %d: atteso %q, ottenuto %s", i+1, atteso, r.grezzo)
			}
			if atteso == "in consegna" {
				f.tentativo(t, id, c1, consegnaFallita, 409)
				f.stato(t, id, c1, map[string]any{"stato": "ritirata"}, 403)
				f.stato(t, id, op, map[string]any{"stato": "ritirata"}, 200)
			}
		}
		if n := f.conta(t, `SELECT count(*) FROM tentativi t JOIN note n ON n.id = t.id_nota
			WHERE t.id_spedizione = $1 AND t.tipo = 'consegna' AND n.id_corriere_creata = $2`, id, f.idCorriere1); n != 3 {
			t.Fatalf("attesi 3 tentativi di consegna con la nota del corriere, trovati %d", n)
		}
		if n := f.conta(t, `SELECT count(*) FROM tentativi t JOIN note n ON n.id = t.id_nota
			WHERE t.id_spedizione = $1 AND n.motivo = 'destinatario assente' AND NOT n.is_interna`, id); n != 3 {
			t.Fatalf("attesi 3 tentativi con motivo pubblico, trovati %d", n)
		}
		f.stato(t, id, c1, map[string]any{"stato": "in consegna"}, 409)
		if r := f.modifica(t, id, comeOperatore(f), map[string]any{"is_reso": true}, 200); r.corpo["is_reso"] != true {
			t.Fatalf("reso su spedizione non consegnata: %s", r.grezzo)
		}
	})

	t.Run("regole_e_dati_400_403_409", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "TENTERR")
		f.tentativo(t, id, c1, ritiroFallito, 403)
		f.assegna(t, id, f.idCorriere1, 201)
		f.tentativo(t, id, c2, ritiroFallito, 403)
		f.tentativo(t, id, op, ritiroFallito, 403)
		f.tentativo(t, id, c1, consegnaFallita, 409)

		r := f.tentativo(t, id, c1, map[string]any{"tipo": "ritiro"}, 400)
		if campi := campiInErrore(r); !contiene(campi, "nota") || !contiene(campi, "motivo") {
			t.Fatalf("attesi gli errori su nota e motivo: %v", campi)
		}
		f.tentativo(t, id, c1, map[string]any{"nota": "x"}, 400)
		f.tentativo(t, id, c1, map[string]any{"tipo": "ritiro", "nota": "x", "motivo": "destinatario assente"}, 400)
		r = f.tentativo(t, id, c1, map[string]any{"tipo": "ritiro", "nota": "x", "motivo": "pacco non pronto"}, 201)
		if int(r.corpo["tentativi_ritiro_falliti"].(float64)) != 1 {
			t.Fatalf("ritiro con motivo: %s", r.grezzo)
		}
	})

	t.Run("tentativo_ripetuto_con_idempotency_key", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "IDEMRIT")
		f.assegna(t, id, f.idCorriere1, 201)
		header := comeCorriere(f.idCorriere1)
		header["Idempotency-Key"] = "ritiro-" + f.suffisso + "-" + strconv.Itoa(id)
		percorso := "/api/spedizioni/" + strconv.Itoa(id) + "/tentativi"

		primo := f.chiama(t, http.MethodPost, percorso, ritiroFallito, header)
		secondo := f.chiama(t, http.MethodPost, percorso, ritiroFallito, header)
		if primo.stato != 201 || secondo.stato != 201 || secondo.header.Get("Idempotent-Replayed") != "true" {
			t.Fatalf("attesa ripetizione della stessa risposta: %d / %d %v", primo.stato, secondo.stato, secondo.header)
		}
		if n := f.conta(t, `SELECT count(*) FROM tentativi WHERE id_spedizione = $1`, id); n != 1 {
			t.Fatalf("con la stessa Idempotency-Key il tentativo va registrato una volta sola, trovati %d", n)
		}
	})
}
