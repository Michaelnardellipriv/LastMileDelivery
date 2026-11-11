package spedizioni

import (
	"context"
	"testing"
	"time"
)

func TestModificaDati(t *testing.T) {
	s := servizioTest()
	nuovaData := s.adesso().Add(48 * time.Hour)
	d := DatiSpedizione{IDIndirizzoSpedizione: p(int32(5)), DataoraRichiesta: &nuovaData}

	for _, stato := range []Stato{Creata, Assegnata} {
		m, err := decidiDati(inStato(stato), d)
		if err != nil || m.Dati == nil || m.NuovoStato != stato {
			t.Fatalf("modifica in %q: %+v %v", stato, m, err)
		}
	}
	for _, stato := range []Stato{Ritirata, InConsegna, Consegnata, Annullata} {
		if _, err := decidiDati(inStato(stato), d); tipoErrore(err) != "409" {
			t.Errorf("modifica in %q: atteso 409, ottenuto %v", stato, err)
		}
	}
	if _, err := decidiDati(inStato(Creata), DatiSpedizione{IDIndirizzoSpedizione: p(int32(1))}); tipoErrore(err) != "400" {
		t.Fatalf("indirizzo di spedizione uguale a quello di ritiro: %v", err)
	}
	if m, _ := decidiDati(inStato(Creata), DatiSpedizione{IDIndirizzoRitiro: p(int32(1))}); !m.NessunaModifica {
		t.Fatal("stessi valori: nessuna modifica")
	}

	passata := s.adesso().Add(-time.Hour)
	for nome, dati := range map[string]DatiSpedizione{
		"nessun campo":  {},
		"id non valido": {IDClienteMittente: p(int32(0))},
		"data passata":  {DataoraRichiesta: &passata},
	} {
		if tipoErrore(s.validaDati(dati)) != "400" {
			t.Errorf("%s: atteso 400", nome)
		}
	}
	if _, err := s.ModificaDati(context.Background(), corriere1, 10, d); tipoErrore(err) != "403" {
		t.Fatalf("un corriere non modifica i dati: %v", err)
	}
}

func TestResoSoloNegliStatiAmmessi(t *testing.T) {
	reso := DatiSpedizione{IsReso: p(true)}
	for _, stato := range []Stato{Annullata, Persa, NonConsegnata} {
		m, err := decidiDati(inStato(stato), reso)
		if err != nil || m.Dati == nil || *m.Dati.IsReso != true || m.NuovoStato != stato {
			t.Fatalf("reso in %q: %+v %v", stato, m, err)
		}
	}
	for _, stato := range []Stato{Creata, Assegnata, Ritirata, InConsegna, Consegnata} {
		if _, err := decidiDati(inStato(stato), reso); tipoErrore(err) != "409" {
			t.Errorf("reso in %q: atteso 409, ottenuto %v", stato, err)
		}
	}
	if _, err := decidiDati(inStato(Persa), DatiSpedizione{IsReso: p(true), IDIndirizzoSpedizione: p(int32(5))}); tipoErrore(err) != "409" {
		t.Fatalf("su una spedizione persa si imposta solo is_reso: %v", err)
	}
	gia := inStato(Annullata)
	gia.IsReso = true
	if m, _ := decidiDati(gia, reso); !m.NessunaModifica {
		t.Fatal("is_reso già true: nessuna modifica")
	}
	if _, err := servizioTest().ModificaDati(context.Background(), corriere1, 10, reso); tipoErrore(err) != "403" {
		t.Fatalf("un corriere non imposta il reso: %v", err)
	}

	s := servizioTest()
	for _, m := range []Modifica{
		must(s.decidiStato(operatore, inStato(Assegnata), CambioStato{Stato: Annullata, Motivo: p("altro"), Nota: p("x")})),
		must(s.decidiStato(operatore, inStato(InConsegna), CambioStato{Stato: Persa, Motivo: p("furto"), Nota: p("x")})),
	} {
		if m.Dati != nil {
			t.Fatal("il cambio di stato non deve toccare is_reso")
		}
	}
}
