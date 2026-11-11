//go:build integration

package apitest

import (
	"testing"
	"time"
)

func TestModificaDati(t *testing.T) {
	f := preparaFixture(t)
	op, c1 := comeOperatore(f), comeCorriere(f.idCorriere1)

	t.Run("modifica_prima_del_ritiro_200_e_storico", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "MOD")
		data := time.Now().Add(72 * time.Hour).UTC().Truncate(time.Second)
		corpo := map[string]any{"dataora_richiesta_spedizione": data.Format(time.RFC3339)}
		r := f.modifica(t, id, op, corpo, 200)
		if r.corpo["dataora_richiesta_spedizione"] != data.Format(time.RFC3339) {
			t.Fatalf("dati non aggiornati: %s", r.grezzo)
		}
		if n := f.conta(t, `SELECT count(*) FROM cambiamenti_spedizioni WHERE id_spedizione = $1
			AND operazione = 'modifica' AND id_operatore = $2 AND valore_nuovo ? 'dataora_richiesta_spedizione'`, id, f.idOperatore); n != 1 {
			t.Fatalf("modifica non registrata nello storico con l'operatore")
		}

		f.assegna(t, id, f.idCorriere1, 201)
		prima := f.storico(t, id)
		f.modifica(t, id, op, corpo, 200)
		if f.storico(t, id) != prima {
			t.Fatal("una modifica con gli stessi valori non deve scrivere nello storico")
		}
	})

	t.Run("dopo_il_ritiro_409", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "MODRIT")
		f.assegna(t, id, f.idCorriere1, 201)
		f.stato(t, id, c1, map[string]any{"stato": "ritirata"}, 200)
		f.modifica(t, id, op, map[string]any{"dataora_richiesta_spedizione": time.Now().Add(96 * time.Hour).UTC().Format(time.RFC3339)}, 409)
	})

	t.Run("reso_solo_se_annullata_persa_o_non_consegnata", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "RESOANN")
		f.modifica(t, id, op, map[string]any{"is_reso": true}, 409)
		f.assegna(t, id, f.idCorriere1, 201)
		f.modifica(t, id, op, map[string]any{"is_reso": true}, 409)
		r := f.stato(t, id, op, map[string]any{"stato": "annullata", "motivo": "richiesta del destinatario", "nota": "il cliente ha chiamato"}, 200)
		if r.corpo["is_reso"] != false {
			t.Fatalf("l'annullamento non deve impostare is_reso: %s", r.grezzo)
		}
		f.modifica(t, id, c1, map[string]any{"is_reso": true}, 403)
		f.modifica(t, id, op, map[string]any{"is_reso": true, "dataora_richiesta_spedizione": time.Now().Add(96 * time.Hour).UTC().Format(time.RFC3339)}, 409)
		r = f.modifica(t, id, op, map[string]any{"is_reso": true}, 200)
		if r.corpo["is_reso"] != true || r.corpo["stato"] != "annullata" {
			t.Fatalf("reso su spedizione annullata: %s", r.grezzo)
		}
		if n := f.conta(t, `SELECT count(*) FROM cambiamenti_spedizioni WHERE id_spedizione = $1
			AND operazione = 'modifica' AND id_operatore = $2 AND valore_nuovo = '{"is_reso": true}'`, id, f.idOperatore); n != 1 {
			t.Fatal("il reso deve comparire nello storico con l'operatore che l'ha impostato")
		}
	})

	t.Run("errori_400_403", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "MODERR")
		f.modifica(t, id, op, map[string]any{}, 400)
		f.modifica(t, id, op, map[string]any{"cod_ordine": "NUOVO"}, 400)
		f.modifica(t, id, op, map[string]any{"stato": "consegnata"}, 400)
		f.modifica(t, id, op, map[string]any{"id_indirizzo_spedizione": f.idRitiro}, 400)
		f.modifica(t, id, op, map[string]any{"dataora_richiesta_spedizione": "domani"}, 400)
		r := f.modifica(t, id, op, map[string]any{"id_cliente_destinatario": 999999999}, 400)
		if !contiene(campiInErrore(r), "id_cliente_destinatario") {
			t.Fatalf("atteso errore su id_cliente_destinatario: %s", r.grezzo)
		}
		f.modifica(t, id, c1, map[string]any{"is_reso": true}, 403)
		f.modifica(t, 999999999, op, map[string]any{"is_reso": true}, 404)
	})
}
