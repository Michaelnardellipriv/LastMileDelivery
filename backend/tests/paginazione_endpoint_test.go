//go:build integration

package apitest

import (
	"net/http"
	"strconv"
	"testing"
)

func TestPaginazioneConTotali(t *testing.T) {
	f := preparaFixture(t)
	op := comeOperatore(f)
	// codici diversi, tutti con lo stesso prefisso unico per questa esecuzione
	prefisso := "PAG" + f.suffisso + "N"
	for i := range 5 {
		f.nuovaSpedizione(t, prefisso+strconv.Itoa(i))
	}
	filtro := "cod_ordine=" + prefisso

	t.Run("totali_e_ultima_pagina", func(t *testing.T) {
		_, r := f.elenco(t, op, filtro+"&per_pagina=2&pagina=3", 200)
		dati, _ := r.corpo["dati"].([]any)
		if r.corpo["totale_elementi"] != float64(5) || r.corpo["totale_pagine"] != float64(3) || len(dati) != 1 {
			t.Fatalf("5 spedizioni a 2 per pagina: attese 3 pagine e 1 elemento nell'ultima, ottenuto %s", r.grezzo)
		}
	})

	t.Run("pagina_oltre_la_fine_vuota_con_i_totali", func(t *testing.T) {
		_, r := f.elenco(t, op, filtro+"&per_pagina=2&pagina=9", 200)
		if dati, _ := r.corpo["dati"].([]any); len(dati) != 0 || r.corpo["totale_pagine"] != float64(3) {
			t.Fatalf("oltre l'ultima pagina: nessun dato ma i totali per tornare indietro, ottenuto %s", r.grezzo)
		}
	})

	t.Run("nessun_risultato_zero_pagine", func(t *testing.T) {
		_, r := f.elenco(t, op, "cod_ordine=NESSUNO-"+f.suffisso, 200)
		if r.corpo["totale_elementi"] != float64(0) || r.corpo["totale_pagine"] != float64(0) {
			t.Fatalf("nessun risultato: totali a zero, ottenuto %s", r.grezzo)
		}
	})

	t.Run("anagrafiche_con_totali", func(t *testing.T) {
		r := f.chiama(t, http.MethodGet, "/api/corrieri?per_pagina=1&q=."+f.suffisso, nil, op)
		dati, _ := r.corpo["dati"].([]any)
		totale, _ := r.corpo["totale_elementi"].(float64)
		if r.stato != 200 || len(dati) != 1 || totale < 2 || r.corpo["totale_pagine"] != totale {
			t.Fatalf("corrieri a 1 per pagina: totale_pagine = totale_elementi, ottenuto %d %s", r.stato, r.grezzo)
		}
	})
}

func TestAssegnataDa(t *testing.T) {
	f := preparaFixture(t)
	id := f.nuovaSpedizione(t, "ASSDA")
	f.assegna(t, id, f.idCorriere1, 201)

	for nome, chi := range map[string]map[string]string{"corriere": comeCorriere(f.idCorriere1), "operatore": comeOperatore(f)} {
		a, _ := f.dettaglio(t, id, chi, 200).corpo["assegnata_da"].(map[string]any)
		if a == nil || int32(a["id"].(float64)) != f.idOperatore || a["nome"] != "operatore" || a["cognome"] != "Test" ||
			a["email"] != f.emailDi("operatore") || a["telefono"] == nil {
			t.Errorf("%s vede chi ha assegnato la spedizione (id, nome, cognome, email, telefono): %v", nome, a)
		}
	}
	if d := f.dettaglio(t, id, comeCliente(f.idDestinatario), 200); d.corpo["assegnata_da"] != nil {
		t.Errorf("il cliente non vede l'operatore: %s", d.grezzo)
	}
}
