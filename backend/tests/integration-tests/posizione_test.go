//go:build integration

package apitest

import (
	"net/http"
	"strconv"
	"testing"
)

const (
	percorsoPosizione  = "/api/corrieri/posizione"
	percorsoPosizioni  = "/api/corrieri/posizioni"
	percorsoIntervallo = "/api/impostazioni/intervallo-posizione"
)

func (f *fixture) posizioneCorriere(t *testing.T, idCorriere int32) map[string]any {
	t.Helper()
	r := f.chiama(t, http.MethodGet, percorsoPosizioni, nil, comeOperatore(f))
	if r.stato != http.StatusOK {
		t.Fatalf("lettura posizioni: atteso 200, ottenuto %d: %s", r.stato, r.grezzo)
	}
	for _, c := range r.corpo["corrieri"].([]any) {
		voce := c.(map[string]any)
		if int32(voce["id_corriere"].(float64)) == idCorriere {
			return voce
		}
	}
	return nil
}

func TestPosizioneCorriere(t *testing.T) {
	f := preparaFixture(t)
	op, admin := comeOperatore(f), comeAmministratore(f)
	attivo, fermo := comeCorriere(f.idCorrierePos), comeCorriere(f.idCorriereFermo)
	coordinate := map[string]any{"latitudine": 46.0679, "longitudine": 11.1211}

	r := f.chiama(t, http.MethodGet, percorsoIntervallo, nil, admin)
	if r.stato != http.StatusOK {
		t.Fatalf("lettura intervallo: atteso 200, ottenuto %d: %s", r.stato, r.grezzo)
	}
	precedente := r.corpo["intervallo_secondi"]
	t.Cleanup(func() {
		f.chiama(t, http.MethodPut, percorsoIntervallo, map[string]any{"intervallo_secondi": precedente}, admin)
	})
	if r := f.chiama(t, http.MethodPut, percorsoIntervallo, map[string]any{"intervallo_secondi": 60}, admin); r.stato != http.StatusOK {
		t.Fatalf("modifica intervallo: atteso 200, ottenuto %d: %s", r.stato, r.grezzo)
	}

	inConsegna := f.nuovaSpedizione(t, "POS")
	f.assegna(t, inConsegna, f.idCorrierePos, 201)
	f.stato(t, inConsegna, attivo, map[string]any{"stato": "ritirata"}, 200)

	soloRitirata := f.nuovaSpedizione(t, "POS-RIT")
	f.assegna(t, soloRitirata, f.idCorriereFermo, 201)
	f.stato(t, soloRitirata, fermo, map[string]any{"stato": "ritirata"}, 200)

	t.Run("permessi", func(t *testing.T) {
		for _, c := range []struct {
			nome             string
			metodo, percorso string
			chi              map[string]string
			corpo            any
			atteso           int
		}{
			{"invio senza identità", http.MethodPost, percorsoPosizione, nil, coordinate, http.StatusUnauthorized},
			{"invio da corriere inesistente", http.MethodPost, percorsoPosizione, comeCorriere(999999999), coordinate, http.StatusUnauthorized},
			{"invio da operatore", http.MethodPost, percorsoPosizione, op, coordinate, http.StatusForbidden},
			{"lettura da corriere", http.MethodGet, percorsoPosizioni, attivo, nil, http.StatusForbidden},
			{"lettura da amministratore", http.MethodGet, percorsoPosizioni, admin, nil, http.StatusForbidden},
			{"lettura da cliente", http.MethodGet, percorsoPosizioni, comeCliente(f.idDestinatario), nil, http.StatusForbidden},
			{"intervallo letto da cliente", http.MethodGet, percorsoIntervallo, comeCliente(f.idDestinatario), nil, http.StatusForbidden},
			{"intervallo modificato da operatore", http.MethodPut, percorsoIntervallo, op, map[string]any{"intervallo_secondi": 10}, http.StatusForbidden},
		} {
			if r := f.chiama(t, c.metodo, c.percorso, c.corpo, c.chi); r.stato != c.atteso {
				t.Errorf("%s: atteso %d, ottenuto %d: %s", c.nome, c.atteso, r.stato, r.grezzo)
			}
		}
		for _, chi := range []map[string]string{attivo, op, admin} {
			if r := f.chiama(t, http.MethodGet, percorsoIntervallo, nil, chi); r.stato != http.StatusOK || r.corpo["intervallo_secondi"] != float64(60) {
				t.Errorf("lettura intervallo: atteso 200 con 60 secondi, ottenuto %d: %s", r.stato, r.grezzo)
			}
		}
	})

	t.Run("validazione_400", func(t *testing.T) {
		for _, c := range []struct {
			nome     string
			metodo   string
			percorso string
			chi      map[string]string
			corpo    map[string]any
			campo    string
		}{
			{"longitudine mancante", http.MethodPost, percorsoPosizione, attivo, map[string]any{"latitudine": 46.0}, "longitudine"},
			{"latitudine fuori range", http.MethodPost, percorsoPosizione, attivo, map[string]any{"latitudine": 100.0, "longitudine": 11.0}, "latitudine"},
			{"intervallo troppo basso", http.MethodPut, percorsoIntervallo, admin, map[string]any{"intervallo_secondi": 1}, "intervallo_secondi"},
			{"intervallo troppo alto", http.MethodPut, percorsoIntervallo, admin, map[string]any{"intervallo_secondi": 3601}, "intervallo_secondi"},
		} {
			r := f.chiama(t, c.metodo, c.percorso, c.corpo, c.chi)
			if r.stato != http.StatusBadRequest || !contiene(campiInErrore(r), c.campo) {
				t.Errorf("%s: atteso 400 sul campo %s, ottenuto %d: %s", c.nome, c.campo, r.stato, r.grezzo)
			}
		}
	})

	t.Run("solo_con_spedizioni_in_consegna", func(t *testing.T) {
		if r := f.chiama(t, http.MethodPost, percorsoPosizione, coordinate, fermo); r.stato != http.StatusConflict {
			t.Fatalf("corriere con spedizione solo ritirata: atteso 409, ottenuto %d: %s", r.stato, r.grezzo)
		}
		if r := f.chiama(t, http.MethodPost, percorsoPosizione, coordinate, attivo); r.stato != http.StatusConflict {
			t.Fatalf("prima dell'uscita in consegna: atteso 409, ottenuto %d: %s", r.stato, r.grezzo)
		}
	})

	t.Run("invio_lettura_e_intervallo", func(t *testing.T) {
		f.stato(t, inConsegna, attivo, map[string]any{"stato": "in consegna"}, 200)

		r := f.chiama(t, http.MethodPost, percorsoPosizione, coordinate, attivo)
		if r.stato != http.StatusCreated || r.corpo["intervallo_secondi"] != float64(60) {
			t.Fatalf("primo invio: atteso 201 con l'intervallo, ottenuto %d: %s", r.stato, r.grezzo)
		}
		r = f.chiama(t, http.MethodPost, percorsoPosizione, map[string]any{"latitudine": 46.07, "longitudine": 11.13}, attivo)
		if r.stato != http.StatusTooManyRequests || r.header.Get("Retry-After") != "60" {
			t.Fatalf("secondo invio immediato: atteso 429 con Retry-After 60, ottenuto %d %q: %s", r.stato, r.header.Get("Retry-After"), r.grezzo)
		}

		voce := f.posizioneCorriere(t, f.idCorrierePos)
		if voce == nil {
			t.Fatal("il corriere in consegna deve comparire nelle posizioni")
		}
		spedizioni := voce["spedizioni_attive"].([]any)
		if voce["latitudine"] != 46.0679 || voce["longitudine"] != 11.1211 || voce["nome"] != "corrierepos" ||
			len(spedizioni) != 1 || int(spedizioni[0].(float64)) != inConsegna || voce["rilevata_il"] == nil {
			t.Errorf("ultima posizione del corriere: %v", voce)
		}
		if f.posizioneCorriere(t, f.idCorriereFermo) != nil {
			t.Error("il corriere senza spedizioni in consegna non compare")
		}
		if n := f.conta(t, `SELECT count(*) FROM posizioni_corrieri WHERE id_corriere = $1`, f.idCorrierePos); n != 1 {
			t.Errorf("salvata una sola posizione, trovate %d", n)
		}
	})

	t.Run("sparisce_dopo_la_consegna", func(t *testing.T) {
		f.stato(t, inConsegna, attivo, map[string]any{"stato": "consegnata"}, 200)
		if f.posizioneCorriere(t, f.idCorrierePos) != nil {
			t.Error("dopo la consegna il corriere non compare più nelle posizioni")
		}
		r := f.chiama(t, http.MethodPost, percorsoPosizione, coordinate, attivo)
		if r.stato != http.StatusConflict {
			t.Errorf("dopo la consegna l'invio è rifiutato: atteso 409, ottenuto %d: %s", r.stato, r.grezzo)
		}
	})

	t.Run("intervallo_registra_l_amministratore", func(t *testing.T) {
		var id *int32
		if err := f.db.QueryRow(t.Context(), `SELECT id_amministratore_modifica FROM impostazioni`).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if id == nil || *id != f.idAmministratore {
			t.Errorf("l'ultima modifica deve riportare l'amministratore %s, trovato %v", strconv.Itoa(int(f.idAmministratore)), id)
		}
	})
}
