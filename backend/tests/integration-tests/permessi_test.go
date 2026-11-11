//go:build integration

package apitest

import (
	"net/http"
	"strconv"
	"testing"
)

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
