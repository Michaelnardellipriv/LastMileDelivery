//go:build integration

package apitest

import (
	"net/http"
	"slices"
	"strconv"
	"testing"
)

func TestElencoMotivi(t *testing.T) {
	f := preparaFixture(t)

	t.Run("elenco_unico_200_per_ogni_ruolo", func(t *testing.T) {
		for _, chi := range []map[string]string{comeOperatore(f), comeCorriere(f.idCorriere1), comeCliente(f.idDestinatario), comeAmministratore(f)} {
			r := f.chiama(t, http.MethodGet, "/api/motivi", nil, chi)
			if r.stato != http.StatusOK {
				t.Fatalf("%v: atteso 200, ottenuto %d %s", chi, r.stato, r.grezzo)
			}
			voci, _ := r.corpo["dati"].([]any)
			// un solo elenco con i motivi di ritiro, consegna, annullamento e perdita insieme
			for _, atteso := range []string{"mittente assente", "destinatario assente", "richiesta del mittente", "furto", "altro"} {
				if !slices.Contains(voci, any(atteso)) {
					t.Errorf("l'elenco deve contenere %q: %v", atteso, voci)
				}
			}
		}
	})

	t.Run("senza_identita_401", func(t *testing.T) {
		if r := f.chiama(t, http.MethodGet, "/api/motivi", nil, nil); r.stato != http.StatusUnauthorized {
			t.Fatalf("atteso 401, ottenuto %d", r.stato)
		}
	})

	t.Run("motivo_di_altro_tipo_accettato_perche_non_si_filtra", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "MOTIVOLIBERO")
		f.assegna(t, id, f.idCorriere1, 201)
		// "furto" è nato come motivo di perdita: con l'elenco unico vale anche per annullare
		f.stato(t, id, comeOperatore(f), map[string]any{"stato": "annullata", "motivo": "furto", "nota": "prova"}, 200)
	})

	t.Run("motivo_fuori_elenco_400", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "MOTIVOERR")
		r := f.stato(t, id, comeOperatore(f), map[string]any{"stato": "annullata", "motivo": "ci ho ripensato", "nota": "x"}, 400)
		if !contiene(campiInErrore(r), "motivo") {
			t.Fatalf("atteso errore sul campo motivo: %s", r.grezzo)
		}
	})
}

func TestNomeAutoreInStoricoENote(t *testing.T) {
	f := preparaFixture(t)
	id := f.nuovaSpedizione(t, "AUTORI")
	f.assegna(t, id, f.idCorriere1, 201)
	c1 := comeCorriere(f.idCorriere1)
	f.stato(t, id, c1, map[string]any{"stato": "ritirata"}, 200)
	f.invia(t, http.MethodPost, id, "/note", c1, map[string]any{"nota": "pacco caricato"}, 201)

	t.Run("storico_con_nome_cognome_e_id", func(t *testing.T) {
		r := f.chiama(t, http.MethodGet, "/api/spedizioni/"+strconv.Itoa(id)+"/storico", nil, comeOperatore(f))
		if r.stato != http.StatusOK {
			t.Fatalf("storico: atteso 200, ottenuto %d %s", r.stato, r.grezzo)
		}
		nomi := map[string]bool{}
		for _, v := range r.corpo["dati"].([]any) {
			a := v.(map[string]any)["autore"].(map[string]any)
			if a["nome"] == nil || a["cognome"] != "Test" || a["id"] == nil {
				t.Errorf("ogni voce ha nome, cognome e id dell'autore: %v", a)
				continue
			}
			nomi[a["ruolo"].(string)+" "+a["nome"].(string)] = true
		}
		if !nomi["operatore operatore"] || !nomi["corriere corriere1"] {
			t.Fatalf("attesi l'operatore (creazione, assegnazione) e il corriere (ritiro): %v", nomi)
		}
	})

	t.Run("note_con_nome_cognome_e_id_ma_non_per_il_cliente", func(t *testing.T) {
		d := f.dettaglio(t, id, comeOperatore(f), 200)
		note := d.corpo["note"].([]any)
		a := note[len(note)-1].(map[string]any)["autore"].(map[string]any)
		if a["nome"] != "corriere1" || a["cognome"] != "Test" || int32(a["id"].(float64)) != f.idCorriere1 {
			t.Fatalf("la nota mostra nome, cognome e id del corriere: %v", a)
		}
		r := f.invia(t, http.MethodPost, id, "/note", c1, map[string]any{"nota": "seconda nota"}, 201)
		if a := r.corpo["autore"].(map[string]any); a["nome"] != "corriere1" {
			t.Fatalf("anche la nota appena creata ha il nome dell'autore: %s", r.grezzo)
		}
	})
}
