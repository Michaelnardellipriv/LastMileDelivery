package spedizioni

import (
	"context"
	"testing"

	"lastmiledelivery/backend/internal/identita"
)

func TestClienteEAmministratoreNonModificano(t *testing.T) {
	s := servizioTest()
	for _, r := range []identita.Richiedente{cliente1, amministratore} {
		if _, err := s.CambiaStato(context.Background(), r, 10, CambioStato{Stato: Ritirata}); tipoErrore(err) != "403" {
			t.Errorf("%s non cambia lo stato: %v", r.Ruolo, err)
		}
		if _, err := s.Crea(context.Background(), r, NuovaSpedizione{}); tipoErrore(err) != "403" {
			t.Errorf("%s non crea spedizioni: %v", r.Ruolo, err)
		}
		if _, _, err := s.Assegna(context.Background(), r, 10, p(int32(1))); tipoErrore(err) != "403" {
			t.Errorf("%s non assegna: %v", r.Ruolo, err)
		}
		if _, err := s.RegistraTentativo(context.Background(), r, 10, TentativoFallito{}); tipoErrore(err) != "403" {
			t.Errorf("%s non registra tentativi: %v", r.Ruolo, err)
		}
		if _, err := s.ModificaDati(context.Background(), r, 10, DatiSpedizione{IsReso: p(true)}); tipoErrore(err) != "403" {
			t.Errorf("%s non modifica i dati: %v", r.Ruolo, err)
		}
	}
}
