package spedizioni

import (
	"context"
	"testing"
	"time"
)

func TestCreaSoloOperatore(t *testing.T) {
	if _, err := servizioTest().Crea(context.Background(), corriere1, NuovaSpedizione{}); tipoErrore(err) != "403" {
		t.Fatalf("un corriere non può creare spedizioni: %v", err)
	}
}

func TestRegoleNuova(t *testing.T) {
	s := servizioTest()
	n := NuovaSpedizione{
		CodOrdine: "ORD 1", IDClienteMittente: 1, IDClienteDestinatario: 2,
		IDIndirizzoRitiro: 1, IDIndirizzoSpedizione: 1,
		DataoraRichiesta: s.adesso().Add(-time.Hour),
	}
	campi := map[string]bool{}
	for _, e := range s.RegoleNuova(n) {
		campi[e.Campo] = true
	}
	for _, c := range []string{"cod_ordine", "id_indirizzo_spedizione", "dataora_richiesta_spedizione"} {
		if !campi[c] {
			t.Errorf("manca l'errore su %s", c)
		}
	}
}
