package spedizioni

import (
	"context"
	"testing"
)

func voceConPersone() VoceElenco {
	m := Persona{ID: 3, Nome: "Marco", Cognome: "Mittente", Email: p("mittente@test.it"), Telefono: p("+39 3330000005")}
	return VoceElenco{
		ID: 10, IDClienteMittente: 3, IDClienteDestinatario: 7, Mittente: &m,
		Destinatario: Persona{ID: 7, Nome: "Dora", Cognome: "Destinataria", Email: p("destinatario@test.it"), Telefono: p("+39 3330000006")},
		IDCorriere:   p(int32(1)), Corriere: &Persona{ID: 1, Nome: "Carlo", Cognome: "Corriere", Telefono: p("+39 3330000003")},
		IndirizzoRitiro: Indirizzo{Via: "Via Roma", Civico: "1", Citta: p("Trento"), CAP: p("38121"),
			Regione: p("Trentino-Alto Adige"), Nazione: p("Italia")},
	}
}

func TestDatiPersonePerRuolo(t *testing.T) {
	ctx := context.Background()

	for nome, r := range map[string]struct{ elenca func() ([]VoceElenco, error) }{
		"operatore": {func() ([]VoceElenco, error) {
			s, _ := servizioElenco(voceConPersone())
			return s.Elenca(ctx, operatore, filtroBase)
		}},
		"amministratore": {func() ([]VoceElenco, error) {
			s, _ := servizioElenco(voceConPersone())
			return s.Elenca(ctx, amministratore, filtroBase)
		}},
	} {
		voci, err := r.elenca()
		if err != nil {
			t.Fatal(err)
		}
		v := voci[0]
		if v.Mittente == nil || v.Mittente.Email == nil || v.Destinatario.Email == nil || v.Corriere == nil || v.Corriere.Nome != "Carlo" {
			t.Errorf("%s vede mittente, destinatario e corriere: %+v", nome, v)
		}
	}

	for nome, chi := range map[string]struct{ elenca func() ([]VoceElenco, error) }{
		"corriere": {func() ([]VoceElenco, error) {
			s, _ := servizioElenco(voceConPersone())
			return s.Elenca(ctx, corriere1, filtroBase)
		}},
		"cliente": {func() ([]VoceElenco, error) {
			s, _ := servizioElenco(voceConPersone())
			return s.Elenca(ctx, cliente1, filtroBase)
		}},
	} {
		voci, _ := chi.elenca()
		v := voci[0]
		d := v.Destinatario
		if v.Corriere != nil || v.IDCorriere != nil {
			t.Errorf("%s non vede il corriere: %+v", nome, v)
		}
		if m := v.Mittente; m == nil || m.Nome != "Marco" || m.Cognome != "Mittente" || m.Email == nil || m.Telefono == nil {
			t.Errorf("%s vede nome, cognome, email e telefono del mittente: %+v", nome, m)
		} else if idAtteso := map[string]int32{"corriere": 3, "cliente": 0}[nome]; m.ID != idAtteso {
			t.Errorf("%s: id del mittente %d, atteso %d (il cliente non lo riceve)", nome, m.ID, idAtteso)
		}
		if d.ID != 7 || d.Nome != "Dora" || d.Cognome != "Destinataria" || d.Email == nil || d.Telefono == nil {
			t.Errorf("%s vede id, nome, cognome, email e telefono del destinatario: %+v", nome, d)
		}
		if i := v.IndirizzoRitiro; i.Citta == nil || i.Regione == nil || *i.Nazione != "Italia" {
			t.Errorf("%s vede città, regione e nazione degli indirizzi: %+v", nome, i)
		}
	}
}

func TestDettaglioPersonePerRuolo(t *testing.T) {
	ctx := context.Background()
	s, a := servizioElenco()
	a.dettaglio = DettaglioSpedizione{VoceElenco: voceConPersone()}
	d, err := s.Dettaglio(ctx, cliente1, 10)
	if err != nil || d.Corriere != nil || d.Mittente == nil || d.Mittente.ID != 0 || d.Mittente.Email == nil || d.Destinatario.Email == nil {
		t.Fatalf("il dettaglio segue le stesse regole dell'elenco: %+v %v", d, err)
	}
}
