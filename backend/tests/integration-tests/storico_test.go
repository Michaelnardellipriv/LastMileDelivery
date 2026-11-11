//go:build integration

package apitest

import (
	"net/http"
	"strconv"
	"testing"
)

func TestStoricoSpedizione(t *testing.T) {
	f := preparaFixture(t)
	op, c1, admin := comeOperatore(f), comeCorriere(f.idCorriere1), comeAmministratore(f)

	id := f.nuovaSpedizione(t, "STORICO")
	f.assegna(t, id, f.idCorriere1, 201)
	f.stato(t, id, c1, map[string]any{"stato": "ritirata"}, 200)
	percorso := "/api/spedizioni/" + strconv.Itoa(id) + "/storico"

	for _, chi := range []map[string]string{admin, op} {
		r := f.chiama(t, http.MethodGet, percorso, nil, chi)
		if r.stato != http.StatusOK {
			t.Fatalf("storico: atteso 200, ottenuto %d: %s", r.stato, r.grezzo)
		}
		dati := r.corpo["dati"].([]any)
		if len(dati) != 3 {
			t.Fatalf("attese 3 righe (creazione, assegnazione, ritiro), trovate %d: %s", len(dati), r.grezzo)
		}
		primo, ultimo := dati[0].(map[string]any), dati[2].(map[string]any)
		autorePrimo, autoreUltimo := primo["autore"].(map[string]any), ultimo["autore"].(map[string]any)
		if primo["operazione"] != "creazione" || primo["valore_vecchio"] != nil ||
			autorePrimo["ruolo"] != "operatore" || int32(autorePrimo["id"].(float64)) != f.idOperatore {
			t.Errorf("prima riga: creazione da parte dell'operatore: %v", primo)
		}
		vecchio, nuovo := ultimo["valore_vecchio"].(map[string]any), ultimo["valore_nuovo"].(map[string]any)
		if vecchio["stato"] != "assegnata" || nuovo["stato"] != "ritirata" ||
			autoreUltimo["ruolo"] != "corriere" || int32(autoreUltimo["id"].(float64)) != f.idCorriere1 {
			t.Errorf("ultima riga: ritiro da parte del corriere: %v", ultimo)
		}
	}

	for _, chi := range []map[string]string{c1, comeCliente(f.idDestinatario)} {
		if r := f.chiama(t, http.MethodGet, percorso, nil, chi); r.stato != http.StatusForbidden {
			t.Errorf("solo amministratore e operatore vedono lo storico: %d", r.stato)
		}
	}
	if r := f.chiama(t, http.MethodGet, "/api/spedizioni/999999999/storico", nil, admin); r.stato != http.StatusNotFound {
		t.Errorf("spedizione inesistente: atteso 404, ottenuto %d", r.stato)
	}
	if r := f.chiama(t, http.MethodGet, "/api/spedizioni/abc/storico", nil, admin); r.stato != http.StatusBadRequest {
		t.Errorf("id non valido: atteso 400, ottenuto %d", r.stato)
	}
}
