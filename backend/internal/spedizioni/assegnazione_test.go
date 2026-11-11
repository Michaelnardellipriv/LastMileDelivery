package spedizioni

import (
	"context"
	"testing"
)

func TestAssegnazione(t *testing.T) {
	s := servizioTest()
	casi := []struct {
		da     Stato
		atteso Stato
	}{{Creata, Assegnata}, {Assegnata, Assegnata}, {Ritirata, Ritirata}}
	for _, c := range casi {
		attuale := inStato(c.da)
		if c.da == Ritirata {
			attuale.TentativiConsegnaFalliti, attuale.UsciteInConsegna = 1, 1
		}
		m, err := s.decidiAssegnazione(attuale, p(int32(2)))
		if err != nil || m.NuovoStato != c.atteso || *m.NuovoCorriere != 2 {
			t.Fatalf("assegnazione da %q: %+v %v", c.da, m, err)
		}
	}
	if m, _ := s.decidiAssegnazione(inStato(Assegnata), p(int32(1))); !m.NessunaModifica {
		t.Fatal("riassegnare allo stesso corriere deve essere senza effetto")
	}
	for _, stato := range []Stato{InConsegna, Consegnata, Annullata, Persa} {
		if _, err := s.decidiAssegnazione(inStato(stato), p(int32(2))); tipoErrore(err) != "409" {
			t.Errorf("assegnazione in %q: atteso 409, ottenuto %v", stato, err)
		}
	}
	esauriti := inStato(Assegnata)
	esauriti.TentativiRitiroFalliti = 3
	if _, err := s.decidiAssegnazione(esauriti, p(int32(2))); tipoErrore(err) != "409" {
		t.Fatalf("dopo 3 ritiri falliti non si riassegna: %v", err)
	}
	if _, err := s.decidiAssegnazione(inStato(Ritirata), p(int32(2))); tipoErrore(err) != "409" {
		t.Fatalf("ritirata senza tentativi di consegna falliti: non si riassegna: %v", err)
	}
	if _, _, err := s.Assegna(context.Background(), corriere1, 10, p(int32(2))); tipoErrore(err) != "403" {
		t.Fatalf("un corriere non può assegnare: %v", err)
	}
	if _, _, err := s.Assegna(context.Background(), operatore, 10, nil); tipoErrore(err) != "400" {
		t.Fatalf("id_corriere obbligatorio: %v", err)
	}
}
