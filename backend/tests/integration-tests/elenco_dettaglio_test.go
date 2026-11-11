//go:build integration

package apitest

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestElencoSpedizioni(t *testing.T) {
	f := preparaFixture(t)
	op, c1, c2 := comeOperatore(f), comeCorriere(f.idCorriere1), comeCorriere(f.idCorriere2)
	dest, mitt, admin := comeCliente(f.idDestinatario), comeCliente(f.idMittente), comeAmministratore(f)

	creata := f.nuovaSpedizione(t, "EL-CREATA")
	assegnata := f.nuovaSpedizione(t, "EL-ASSEGNATA")
	f.assegna(t, assegnata, f.idCorriere1, 201)
	inConsegna := f.nuovaSpedizione(t, "EL-INCONSEGNA")
	f.assegna(t, inConsegna, f.idCorriere1, 201)
	f.stato(t, inConsegna, c1, map[string]any{"stato": "ritirata"}, 200)
	f.stato(t, inConsegna, c1, map[string]any{"stato": "in consegna"}, 200)
	consegnata := f.nuovaSpedizione(t, "EL-CONSEGNATA")
	f.assegna(t, consegnata, f.idCorriere1, 201)
	f.stato(t, consegnata, c1, map[string]any{"stato": "ritirata"}, 200)
	f.stato(t, consegnata, c1, map[string]any{"stato": "in consegna"}, 200)
	f.stato(t, consegnata, c1, map[string]any{"stato": "consegnata"}, 200)
	annullata := f.nuovaSpedizione(t, "EL-ANNULLATA")
	f.assegna(t, annullata, f.idCorriere1, 201)
	f.stato(t, annullata, op, map[string]any{"stato": "annullata", "motivo": "richiesta del mittente", "nota": "x"}, 200)
	altroCorriere := f.nuovaSpedizione(t, "EL-ALTRO")
	f.assegna(t, altroCorriere, f.idCorriere2, 201)
	consegnataDa10 := f.nuovaSpedizione(t, "EL-CONS10")
	f.consegnaCompleta(t, consegnataDa10)
	f.retrodataStorico(t, consegnataDa10, 10)
	consegnataDa40 := f.nuovaSpedizione(t, "EL-CONS40")
	f.consegnaCompleta(t, consegnataDa40)
	f.retrodataStorico(t, consegnataDa40, 40)
	annullataDa40 := f.nuovaSpedizione(t, "EL-ANN40")
	f.stato(t, annullataDa40, op, map[string]any{"stato": "annullata", "motivo": "richiesta del mittente", "nota": "x"}, 200)
	f.retrodataStorico(t, annullataDa40, 40)
	inCorsoDa40 := f.nuovaSpedizione(t, "EL-INCORSO40")
	f.assegna(t, inCorsoDa40, f.idCorriere1, 201)
	f.retrodata(t, inCorsoDa40, 40)
	f.retrodataStorico(t, inCorsoDa40, 40)
	if _, err := f.db.Exec(context.Background(), `INSERT INTO note (nota, is_interna, id_operatore_logistico_creata, id_spedizione)
		VALUES ('nota interna', true, $1, $2), ('nota con motivo per il cliente', false, $1, $2)`, f.idOperatore, assegnata); err != nil {
		t.Fatal(err)
	}

	presenti := func(t *testing.T, voci map[int]map[string]any, ids ...int) {
		t.Helper()
		for _, id := range ids {
			if _, ok := voci[id]; !ok {
				t.Errorf("la spedizione %d deve essere visibile", id)
			}
		}
	}
	assenti := func(t *testing.T, voci map[int]map[string]any, ids ...int) {
		t.Helper()
		for _, id := range ids {
			if _, ok := voci[id]; ok {
				t.Errorf("la spedizione %d non deve essere visibile", id)
			}
		}
	}

	t.Run("operatore_e_amministratore_vedono_tutto", func(t *testing.T) {
		for _, chi := range []map[string]string{op, admin} {
			voci, _ := f.elenco(t, chi, "per_pagina=100", 200)
			presenti(t, voci, creata, assegnata, inConsegna, consegnata, annullata, altroCorriere, consegnataDa10, consegnataDa40)
			if voci[assegnata]["id_corriere"] == nil {
				t.Errorf("l'operatore vede il corriere assegnato")
			}
			if len(voci[assegnata]["note"].([]any)) != 2 {
				t.Errorf("l'operatore vede anche le note interne: %v", voci[assegnata]["note"])
			}
		}
	})

	t.Run("corriere_vede_solo_le_sue_non_concluse", func(t *testing.T) {
		voci, _ := f.elenco(t, c1, "per_pagina=100", 200)
		presenti(t, voci, assegnata, inConsegna)
		assenti(t, voci, creata, consegnata, annullata, altroCorriere, consegnataDa10)
		for id, v := range voci {
			if _, c := v["id_corriere"]; c {
				t.Errorf("il corriere non riceve id_corriere (spedizione %d)", id)
			}
		}
		voci, _ = f.elenco(t, c2, "per_pagina=100", 200)
		presenti(t, voci, altroCorriere)
		assenti(t, voci, assegnata, inConsegna)
	})

	t.Run("cliente_vede_le_sue_fino_a_30_giorni_dalla_chiusura", func(t *testing.T) {
		voci, _ := f.elenco(t, dest, "per_pagina=100", 200)
		presenti(t, voci, creata, assegnata, inConsegna, consegnata, annullata, consegnataDa10, inCorsoDa40)
		assenti(t, voci, consegnataDa40, annullataDa40)
		if voci[consegnata]["conclusa_il"] == nil || voci[assegnata]["conclusa_il"] != nil {
			t.Errorf("conclusa_il valorizzata solo per le spedizioni chiuse: %v / %v", voci[consegnata]["conclusa_il"], voci[assegnata]["conclusa_il"])
		}
		note := voci[assegnata]["note"].([]any)
		if len(note) != 1 || note[0].(map[string]any)["is_interna"] != false {
			t.Errorf("il cliente vede solo le note pubbliche: %v", note)
		} else if _, ok := note[0].(map[string]any)["nota"]; ok {
			t.Errorf("il cliente vede solo il motivo, non il testo della nota: %v", note)
		}
		if _, c := voci[assegnata]["id_corriere"]; c {
			t.Errorf("il cliente non riceve id_corriere")
		}
		voci, _ = f.elenco(t, mitt, "per_pagina=100", 200)
		assenti(t, voci, creata, assegnata, consegnata, consegnataDa10)
	})

	t.Run("filtro_per_stato_e_paginazione", func(t *testing.T) {
		voci, _ := f.elenco(t, op, "stato=consegnata&per_pagina=100", 200)
		presenti(t, voci, consegnata)
		for id, v := range voci {
			if v["stato"] != "consegnata" {
				t.Errorf("con il filtro arrivano solo spedizioni consegnate (spedizione %d: %v)", id, v["stato"])
			}
		}
		voci, r := f.elenco(t, op, "pagina=2&per_pagina=1", 200)
		if len(voci) != 1 || r.corpo["pagina"].(float64) != 2 || r.corpo["per_pagina"].(float64) != 1 {
			t.Errorf("paginazione: %s", r.grezzo)
		}
		voci, _ = f.elenco(t, c1, "stato=consegnata&per_pagina=100", 200)
		assenti(t, voci, consegnata)
	})

	t.Run("filtri_corriere_data_riferimento", func(t *testing.T) {
		voci, _ := f.elenco(t, op, "id_corriere="+strconv.Itoa(int(f.idCorriere2))+"&per_pagina=100", 200)
		presenti(t, voci, altroCorriere)
		assenti(t, voci, creata, assegnata)
		for id, v := range voci {
			if int32(v["id_corriere"].(float64)) != f.idCorriere2 {
				t.Errorf("filtro per corriere: la spedizione %d è di un altro corriere", id)
			}
		}

		voci, _ = f.elenco(t, op, "cod_ordine="+url.QueryEscape(strings.ToLower("EL-ASSEGNATA-"+f.suffisso)), 200)
		if len(voci) != 1 || voci[assegnata] == nil {
			t.Errorf("filtro per riferimento (senza distinguere maiuscole): attesa solo la spedizione %d, ottenute %v", assegnata, len(voci))
		}
		voci, _ = f.elenco(t, op, "cod_ordine="+f.suffisso+"&per_pagina=100", 200)
		presenti(t, voci, creata, assegnata, consegnata)

		oggi, domani := f.dataRoma(t, 0), f.dataRoma(t, 1)
		voci, _ = f.elenco(t, op, "dal="+oggi+"&al="+oggi+"&per_pagina=100", 200)
		presenti(t, voci, creata, assegnata)
		assenti(t, voci, inCorsoDa40)
		voci, _ = f.elenco(t, op, "dal="+domani+"&per_pagina=100", 200)
		assenti(t, voci, creata, assegnata)
		voci, _ = f.elenco(t, op, "al="+f.dataRoma(t, -30)+"&cod_ordine="+f.suffisso+"&per_pagina=100", 200)
		presenti(t, voci, inCorsoDa40)
		assenti(t, voci, creata)

		voci, _ = f.elenco(t, op, "stato=assegnata&id_corriere="+strconv.Itoa(int(f.idCorriere1))+"&cod_ordine="+f.suffisso+"&per_pagina=100", 200)
		presenti(t, voci, assegnata, inCorsoDa40)
		assenti(t, voci, inConsegna, consegnata, altroCorriere)

		f.elenco(t, dest, "id_corriere=1", 403)
		f.elenco(t, c1, "id_corriere=1", 403)
	})

	t.Run("parametri_non_validi_400", func(t *testing.T) {
		for _, q := range []string{"stato=volata", "pagina=0", "per_pagina=101", "per_pagina=abc",
			"id_corriere=abc", "id_corriere=0", "dal=01/11/2026", "al=ieri", "dal=2026-11-02&al=2026-11-01", "cod_ordine="} {
			f.elenco(t, op, q, 400)
		}
	})

	t.Run("cliente_e_amministratore_non_modificano", func(t *testing.T) {
		if r := f.chiama(t, http.MethodPost, "/api/spedizioni", f.corpoValido("EL-ADMIN"), admin); r.stato != http.StatusForbidden {
			t.Errorf("l'amministratore non crea spedizioni: %d", r.stato)
		}
		if r := f.chiama(t, http.MethodPost, "/api/spedizioni", f.corpoValido("EL-CLI"), dest); r.stato != http.StatusForbidden {
			t.Errorf("il cliente non crea spedizioni: %d", r.stato)
		}
		f.stato(t, assegnata, dest, map[string]any{"stato": "ritirata"}, 403)
		f.stato(t, assegnata, admin, map[string]any{"stato": "ritirata"}, 403)
	})
}

func TestDettaglioSpedizione(t *testing.T) {
	f := preparaFixture(t)
	op, c1, c2 := comeOperatore(f), comeCorriere(f.idCorriere1), comeCorriere(f.idCorriere2)
	dest, mitt, admin := comeCliente(f.idDestinatario), comeCliente(f.idMittente), comeAmministratore(f)

	id := f.nuovaSpedizione(t, "DETT")
	f.assegna(t, id, f.idCorriere1, 201)
	f.tentativo(t, id, c1, map[string]any{"tipo": "ritiro", "nota": "mittente assente", "motivo": "mittente assente"}, 201)
	f.stato(t, id, c1, map[string]any{"stato": "ritirata"}, 200)
	f.nota(t, id, op, map[string]any{"nota": "in arrivo"}, 201)

	t.Run("operatore_e_amministratore_vedono_tutto", func(t *testing.T) {
		for _, chi := range []map[string]string{op, admin} {
			r := f.dettaglio(t, id, chi, 200)
			c := r.corpo
			if int(c["id"].(float64)) != id || c["stato"] != "ritirata" || c["cod_ordine"] != "DETT-"+f.suffisso {
				t.Errorf("dati della spedizione: %s", r.grezzo)
			}
			if int32(c["id_corriere"].(float64)) != f.idCorriere1 || int(c["tentativi_ritiro_falliti"].(float64)) != 1 ||
				int(c["tentativi_consegna_falliti"].(float64)) != 0 || c["annullata_il"] != nil || c["conclusa_il"] != nil {
				t.Errorf("corriere, tentativi e date: %s", r.grezzo)
			}
			if _, ok := c["indirizzo_ritiro"].(map[string]any)["via"]; !ok {
				t.Errorf("indirizzo di ritiro: %s", r.grezzo)
			}
			note := c["note"].([]any)
			if len(note) != 2 {
				t.Fatalf("attese 2 note (tentativo e nota libera): %s", r.grezzo)
			}
			if ruolo, _ := autoreNota(note[0].(map[string]any)); ruolo != "corriere" || note[0].(map[string]any)["motivo"] != "mittente assente" {
				t.Errorf("nota del tentativo con autore e motivo: %v", note[0])
			}
		}
	})

	t.Run("corriere_solo_le_sue_aperte", func(t *testing.T) {
		f.dettaglio(t, id, c1, 200)
		f.dettaglio(t, id, c2, 404)
		chiusa := f.nuovaSpedizione(t, "DETT-CHIUSA")
		f.consegnaCompleta(t, chiusa)
		f.dettaglio(t, chiusa, c1, 404)
		r := f.dettaglio(t, chiusa, op, 200)
		if r.corpo["stato"] != "consegnata" || r.corpo["conclusa_il"] == nil {
			t.Errorf("spedizione chiusa con data di chiusura: %s", r.grezzo)
		}
	})

	t.Run("cliente_destinatario_senza_dati_interni", func(t *testing.T) {
		r := f.dettaglio(t, id, dest, 200)
		if _, ok := r.corpo["id_corriere"]; ok {
			t.Errorf("il cliente non vede il corriere: %s", r.grezzo)
		}
		note := r.corpo["note"].([]any)
		if len(note) != 1 {
			t.Fatalf("il cliente vede solo la nota del tentativo, non quella libera: %s", r.grezzo)
		}
		n := note[0].(map[string]any)
		_, conTesto := n["nota"]
		if n["motivo"] != "mittente assente" || n["is_interna"] != false || conTesto {
			t.Errorf("del tentativo il cliente vede solo il motivo: %v", n)
		}
		f.dettaglio(t, id, mitt, 404)
	})

	t.Run("cliente_vede_solo_il_motivo_delle_note_pubbliche", func(t *testing.T) {
		persa := f.nuovaSpedizione(t, "DETT-PERSA")
		f.assegna(t, persa, f.idCorriere1, 201)
		f.stato(t, persa, c1, map[string]any{"stato": "ritirata"}, 200)
		f.nota(t, persa, op, map[string]any{"nota": "controllare il deposito"}, 201)
		f.stato(t, persa, op, map[string]any{"stato": "persa", "motivo": "furto", "nota": "furgone forzato in via Verdi"}, 200)

		r := f.dettaglio(t, persa, dest, 200)
		note := r.corpo["note"].([]any)
		if len(note) != 1 {
			t.Fatalf("il cliente vede solo la nota della perdita: %s", r.grezzo)
		}
		n := note[0].(map[string]any)
		_, conTesto := n["nota"]
		_, conAutore := n["autore"]
		if n["motivo"] != "furto" || n["is_interna"] != false || conTesto || conAutore {
			t.Errorf("il cliente vede il motivo, non il testo né l'autore: %v", n)
		}

		r = f.dettaglio(t, persa, op, 200)
		note = r.corpo["note"].([]any)
		if len(note) != 2 || note[1].(map[string]any)["nota"] != "furgone forzato in via Verdi" {
			t.Errorf("l'operatore vede anche il testo della nota pubblica: %s", r.grezzo)
		}
	})

	t.Run("cliente_fino_a_30_giorni_dalla_chiusura", func(t *testing.T) {
		vecchia := f.nuovaSpedizione(t, "DETT-VECCHIA")
		f.consegnaCompleta(t, vecchia)
		f.dettaglio(t, vecchia, dest, 200)
		f.retrodataStorico(t, vecchia, 40)
		f.dettaglio(t, vecchia, dest, 404)
		f.dettaglio(t, vecchia, op, 200)
	})

	t.Run("inesistente_e_id_non_valido", func(t *testing.T) {
		f.dettaglio(t, 999999999, op, 404)
		if r := f.chiama(t, http.MethodGet, "/api/spedizioni/abc", nil, op); r.stato != http.StatusBadRequest {
			t.Errorf("id non valido: atteso 400, ottenuto %d", r.stato)
		}
		if r := f.chiama(t, http.MethodGet, "/api/spedizioni/"+strconv.Itoa(id), nil, nil); r.stato != http.StatusUnauthorized {
			t.Errorf("senza identità: atteso 401, ottenuto %d", r.stato)
		}
	})
}
