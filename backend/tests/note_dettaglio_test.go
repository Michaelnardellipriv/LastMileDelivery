//go:build integration

package apitest

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func (f *fixture) dettaglio(t *testing.T, id int, chi map[string]string, atteso int) risposta {
	t.Helper()
	return f.invia(t, http.MethodGet, id, "", chi, nil, atteso)
}

func (f *fixture) nota(t *testing.T, id int, chi map[string]string, corpo map[string]any, atteso int) risposta {
	t.Helper()
	return f.invia(t, http.MethodPost, id, "/note", chi, corpo, atteso)
}

func autoreNota(n map[string]any) (string, int32) {
	a, ok := n["autore"].(map[string]any)
	if !ok {
		return "", 0
	}
	return a["ruolo"].(string), int32(a["id"].(float64))
}

func TestNoteLibere(t *testing.T) {
	f := preparaFixture(t)
	op, c1, c2 := comeOperatore(f), comeCorriere(f.idCorriere1), comeCorriere(f.idCorriere2)

	id := f.nuovaSpedizione(t, "NOTE")
	f.assegna(t, id, f.idCorriere1, 201)
	f.nota(t, id, c1, map[string]any{"nota": "x"}, 409)
	f.stato(t, id, c1, map[string]any{"stato": "ritirata"}, 200)
	righeStorico := f.storico(t, id)

	t.Run("operatore_e_corriere_assegnato_201", func(t *testing.T) {
		r := f.nota(t, id, op, map[string]any{"nota": "  pacco fragile  "}, 201)
		if r.corpo["nota"] != "pacco fragile" || r.corpo["is_interna"] != true || r.corpo["motivo"] != nil {
			t.Errorf("nota interna dell'operatore: %s", r.grezzo)
		}
		if ruolo, idAutore := autoreNota(r.corpo); ruolo != "operatore" || idAutore != f.idOperatore {
			t.Errorf("autore operatore: %s", r.grezzo)
		}
		r = f.nota(t, id, c1, map[string]any{"nota": "citofono rotto"}, 201)
		if ruolo, idAutore := autoreNota(r.corpo); ruolo != "corriere" || idAutore != f.idCorriere1 || r.corpo["is_interna"] != true {
			t.Errorf("nota interna del corriere: %s", r.grezzo)
		}
		if n := f.conta(t, `SELECT count(*) FROM note WHERE id_spedizione = $1 AND is_interna`, id); n != 2 {
			t.Errorf("attese 2 note interne salvate, trovate %d", n)
		}
		if n := f.storico(t, id); n != righeStorico {
			t.Errorf("una nota non cambia la spedizione: storico da %d a %d righe", righeStorico, n)
		}
	})

	t.Run("validazione_400", func(t *testing.T) {
		for _, corpo := range []map[string]any{{}, {"nota": "   "}, {"nota": strings.Repeat("a", 2001)}} {
			if r := f.nota(t, id, op, corpo, 400); !contiene(campiInErrore(r), "nota") {
				t.Errorf("%v: errore atteso sul campo nota: %s", corpo, r.grezzo)
			}
		}
		for _, chi := range []map[string]string{op, c1} {
			f.nota(t, id, chi, map[string]any{"nota": "x", "is_interna": false}, 400)
		}
	})

	t.Run("non_autorizzati_403", func(t *testing.T) {
		for _, chi := range []map[string]string{c2, comeCliente(f.idDestinatario), comeAmministratore(f)} {
			f.nota(t, id, chi, map[string]any{"nota": "x"}, 403)
		}
	})

	t.Run("stati_ammessi_per_il_corriere", func(t *testing.T) {
		consegnata := f.nuovaSpedizione(t, "NOTE-CONS")
		f.consegnaCompleta(t, consegnata)
		f.nota(t, consegnata, c1, map[string]any{"nota": "lasciato al portiere"}, 409)
		f.nota(t, consegnata, op, map[string]any{"nota": "lasciato al portiere"}, 201)

		persa := f.nuovaSpedizione(t, "NOTE-PERSA")
		f.assegna(t, persa, f.idCorriere1, 201)
		f.stato(t, persa, c1, map[string]any{"stato": "ritirata"}, 200)
		f.stato(t, persa, op, map[string]any{"stato": "persa", "motivo": "furto", "nota": "x"}, 200)
		f.nota(t, persa, c1, map[string]any{"nota": "x"}, 409)
		f.nota(t, persa, op, map[string]any{"nota": "denuncia presentata"}, 201)

		creata := f.nuovaSpedizione(t, "NOTE-CREATA")
		f.nota(t, creata, op, map[string]any{"nota": "cliente ha chiamato"}, 201)
	})

	t.Run("note_interne_solo_da_post_note", func(t *testing.T) {
		altra := f.nuovaSpedizione(t, "NOTE-STATO")
		f.assegna(t, altra, f.idCorriere1, 201)
		r := f.stato(t, altra, c1, map[string]any{"stato": "ritirata", "nota": "ritirato"}, 400)
		if !contiene(campiInErrore(r), "nota") {
			t.Errorf("la nota con il cambio di stato è ammessa solo per annullata e persa: %s", r.grezzo)
		}
	})

	t.Run("spedizione_inesistente_e_id_non_valido", func(t *testing.T) {
		f.nota(t, 999999999, op, map[string]any{"nota": "x"}, 404)
		if r := f.chiama(t, http.MethodPost, "/api/spedizioni/abc/note", map[string]any{"nota": "x"}, op); r.stato != http.StatusBadRequest {
			t.Errorf("id non valido: atteso 400, ottenuto %d", r.stato)
		}
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

func TestAmministratoreSoloLettura(t *testing.T) {
	f := preparaFixture(t)
	admin := comeAmministratore(f)
	id := f.nuovaSpedizione(t, "ADMIN")
	f.assegna(t, id, f.idCorriere1, 201)
	percorso := "/api/spedizioni/" + strconv.Itoa(id)
	righeStorico := f.storico(t, id)

	for _, c := range []struct {
		metodo, percorso string
		corpo            any
	}{
		{http.MethodPost, "/api/spedizioni", f.corpoValido("ADMIN-NUOVA")},
		{http.MethodPatch, percorso, map[string]any{"is_reso": true}},
		{http.MethodPost, percorso + "/assegnazioni", map[string]any{"id_corriere": f.idCorriere2}},
		{http.MethodPost, percorso + "/tentativi", map[string]any{"tipo": "ritiro", "nota": "x", "motivo": "altro"}},
		{http.MethodPatch, percorso + "/stato", map[string]any{"stato": "annullata", "motivo": "altro", "nota": "x"}},
		{http.MethodPost, percorso + "/note", map[string]any{"nota": "x"}},
	} {
		if r := f.chiama(t, c.metodo, c.percorso, c.corpo, admin); r.stato != http.StatusForbidden {
			t.Errorf("%s %s: atteso 403, ottenuto %d: %s", c.metodo, c.percorso, r.stato, r.grezzo)
		}
	}
	for _, p := range []string{percorso, percorso + "/storico"} {
		if r := f.chiama(t, http.MethodDelete, p, nil, admin); r.stato != http.StatusMethodNotAllowed {
			t.Errorf("DELETE %s: nessuna eliminazione, atteso 405, ottenuto %d", p, r.stato)
		}
	}
	for _, p := range []string{"/api/spedizioni", percorso, percorso + "/storico"} {
		if r := f.chiama(t, http.MethodGet, p, nil, admin); r.stato != http.StatusOK {
			t.Errorf("GET %s: l'amministratore consulta, atteso 200, ottenuto %d", p, r.stato)
		}
	}
	if n := f.storico(t, id); n != righeStorico {
		t.Errorf("l'amministratore non cambia nulla: storico da %d a %d righe", righeStorico, n)
	}
}
