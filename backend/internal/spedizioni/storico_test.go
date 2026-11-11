package spedizioni

import (
	"context"
	"testing"

	"lastmiledelivery/backend/internal/identita"
)

func TestStoricoSoloAmministratoreEOperatore(t *testing.T) {
	s, _ := servizioElenco()
	for _, r := range []identita.Richiedente{amministratore, operatore} {
		voci, err := s.Storico(context.Background(), r, 10)
		if err != nil || len(voci) != 1 {
			t.Errorf("%s vede lo storico: %v %v", r.Ruolo, voci, err)
		}
	}
	for _, r := range []identita.Richiedente{corriere1, cliente1} {
		if _, err := s.Storico(context.Background(), r, 10); tipoErrore(err) != "403" {
			t.Errorf("%s non vede lo storico: %v", r.Ruolo, err)
		}
	}
}
