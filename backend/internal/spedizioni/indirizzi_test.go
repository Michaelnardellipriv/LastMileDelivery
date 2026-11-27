package spedizioni

import (
	"context"
	"errors"
	"testing"

	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
	"lastmiledelivery/backend/internal/indirizzo"
	"lastmiledelivery/backend/internal/interno"
)

// archivioIndirizziFinto: la sede di Trento (o quella in sede) e gli indirizzi salvati.
type archivioIndirizziFinto struct {
	sede     interno.Sede
	cercata  int64 // sede passata a CercaIndirizzi
	salvato  *indirizzo.Dati
	esistono bool // TrovaOCreaIndirizzo trova un indirizzo già presente
}

func (a *archivioIndirizziFinto) Sede(_ context.Context, id int64) (interno.Sede, bool, error) {
	if id != sedeTrento && id != sedeBolzano {
		return interno.Sede{}, false, nil
	}
	return a.sede, true, nil
}

func (a *archivioIndirizziFinto) CercaIndirizzi(_ context.Context, idSede int64, _, _ string, _, _ int) ([]IndirizzoSede, int, error) {
	a.cercata = idSede
	return []IndirizzoSede{{ID: 9, IDSede: idSede, Via: "Via Roma", Civico: "1"}}, 1, nil
}

func (a *archivioIndirizziFinto) TrovaOCreaIndirizzo(_ context.Context, idSede int64, sede interno.Sede, d indirizzo.Dati) (IndirizzoSede, bool, error) {
	a.salvato = &d
	return IndirizzoSede{ID: 5, IDSede: idSede, IDCitta: sede.IDCitta, Via: d.Via, Civico: d.Civico, CAP: d.CAP}, !a.esistono, nil
}

func trentoSede() interno.Sede {
	return interno.Sede{Nome: "Veloce24 Trento", Attiva: true, IDCitta: 1, Citta: "Trento", Regione: "Trentino-Alto Adige", Nazione: "Italia"}
}

func nuovoIndirizzo(via, civico, citta, cap string) indirizzo.Nuovo {
	return indirizzo.Nuovo{Via: p(via), Civico: p(civico), Citta: p(citta), CAP: p(cap),
		Regione: p("Trentino-Alto Adige"), Nazione: p("Italia")}
}

func campoInErrore(err error, campo string) bool {
	var v *errori.Validazione
	return errors.As(err, &v) && v.Segnalato(campo)
}

func TestIndirizziSoloOperatoreNellaCittaDellaSede(t *testing.T) {
	ctx := context.Background()
	a := &archivioIndirizziFinto{sede: trentoSede()}
	s := NuovoServizioIndirizzi(a)

	i, creato, err := s.TrovaOCrea(ctx, operatore, nuovoIndirizzo(" Via Roma ", "1", "trento", "38121"))
	if err != nil || !creato || i.ID != 5 || a.salvato.Via != "Via Roma" || i.Citta != "Trento" || i.Nazione != "Italia" || i.IDCitta != 1 {
		t.Fatalf("indirizzo nella città della sede (maiuscole a parte), campi ripuliti, città della sede: %+v %v", i, err)
	}
	a.esistono = true
	if _, creato, err := s.TrovaOCrea(ctx, operatore, nuovoIndirizzo("via roma", "1", "Trento", "38121")); err != nil || creato {
		t.Fatalf("lo stesso indirizzo si ritrova (200, non 201): %v %v", creato, err)
	}

	a.salvato = nil
	_, _, err = s.TrovaOCrea(ctx, operatore, nuovoIndirizzo("Via Museo", "1", "Bolzano", "39100"))
	if tipoErrore(err) != "400" || !campoInErrore(err, "citta") || a.salvato != nil {
		t.Fatalf("la sede è a Trento: un indirizzo a Bolzano non serve, atteso 400 su citta senza salvare, ottenuto %v", err)
	}
	if _, _, err := s.TrovaOCrea(ctx, operatore, indirizzo.Nuovo{Via: p("Via Roma")}); tipoErrore(err) != "400" {
		t.Fatalf("tutti i campi sono obbligatori: %v", err)
	}
	for _, r := range []identita.Richiedente{corriere1, cliente1, amministratore} {
		if _, _, err := s.TrovaOCrea(ctx, r, nuovoIndirizzo("Via Roma", "1", "Trento", "38121")); tipoErrore(err) != "403" {
			t.Errorf("%s non inserisce indirizzi: %v", r.Ruolo, err)
		}
	}
}

func TestFormatoIndirizzo(t *testing.T) {
	ctx := context.Background()
	sede := trentoSede()
	sede.Citta = "Sant'Orsola Terme"
	a := &archivioIndirizziFinto{sede: sede}
	s := NuovoServizioIndirizzi(a)
	valido := func() indirizzo.Nuovo { return nuovoIndirizzo("Via Roma", "12/B", "Sant'Orsola Terme", "38121") }
	if _, _, err := s.TrovaOCrea(ctx, operatore, valido()); err != nil {
		t.Fatalf("indirizzo valido rifiutato: %v", err)
	}
	estero := valido()
	estero.Nazione, estero.CAP, estero.Civico = p("Regno Unito"), p("SW1A 1AA"), p("SNC")
	a.sede.Nazione = "Regno Unito"
	if _, _, err := s.TrovaOCrea(ctx, operatore, estero); err != nil {
		t.Fatalf("CAP estero e civico SNC sono ammessi: %v", err)
	}
	a.sede.Nazione = "Italia"
	for nome, c := range map[string]struct {
		campo  string
		cambia func(*indirizzo.Nuovo)
	}{
		"via solo numeri":      {"via", func(n *indirizzo.Nuovo) { n.Via = p("123") }},
		"civico con parole":    {"civico", func(n *indirizzo.Nuovo) { n.Civico = p("dodici") }},
		"città con cifre":      {"citta", func(n *indirizzo.Nuovo) { n.Citta = p("Trento2") }},
		"regione con simboli":  {"regione", func(n *indirizzo.Nuovo) { n.Regione = p("Trentino#") }},
		"nazione con cifre":    {"nazione", func(n *indirizzo.Nuovo) { n.Nazione = p("1talia") }},
		"CAP italiano a 4":     {"cap", func(n *indirizzo.Nuovo) { n.CAP = p("3812") }},
		"CAP italiano lettere": {"cap", func(n *indirizzo.Nuovo) { n.CAP = p("38A21") }},
		"CAP estero con #":     {"cap", func(n *indirizzo.Nuovo) { n.Nazione, n.CAP = p("Francia"), p("75#01") }},
	} {
		n := valido()
		c.cambia(&n)
		if _, _, err := s.TrovaOCrea(ctx, operatore, n); !campoInErrore(err, c.campo) {
			t.Errorf("%s: atteso errore sul campo %s, ottenuto %v", nome, c.campo, err)
		}
	}
}

func TestCercaIndirizziPerSede(t *testing.T) {
	ctx := context.Background()
	a := &archivioIndirizziFinto{sede: trentoSede()}
	s := NuovoServizioIndirizzi(a)
	ric := RicercaIndirizzi{Pagina: 1, PerPagina: 20}

	trovati, totale, err := s.Cerca(ctx, operatore, ric)
	if err != nil || a.cercata != sedeTrento || totale != 1 || trovati[0].Citta != "Trento" {
		t.Fatalf("l'operatore cerca nella sua sede, con la città della sede: %+v %v", trovati, err)
	}
	altra := ric
	altra.IDSede = p(int64(sedeBolzano))
	if _, _, err := s.Cerca(ctx, operatore, altra); tipoErrore(err) != "403" {
		t.Fatalf("l'operatore non cerca in un'altra sede: %v", err)
	}

	// l'amministratore sceglie sempre la sede
	if _, _, err := s.Cerca(ctx, amministratore, ric); tipoErrore(err) != "400" || !campoInErrore(err, "id_sede") {
		t.Fatalf("amministratore senza sede: atteso 400 su id_sede, ottenuto %v", err)
	}
	if _, _, err := s.Cerca(ctx, amministratore, altra); err != nil || a.cercata != sedeBolzano {
		t.Fatalf("amministratore con la sede: %v", err)
	}
	inesistente := ric
	inesistente.IDSede = p(int64(999))
	var nonTrovata *errori.NonTrovato
	if _, _, err := s.Cerca(ctx, amministratore, inesistente); !errors.As(err, &nonTrovata) {
		t.Fatalf("sede inesistente: atteso 404, ottenuto %v", err)
	}

	for _, r := range []identita.Richiedente{corriere1, cliente1} {
		if _, _, err := s.Cerca(ctx, r, ric); tipoErrore(err) != "403" {
			t.Errorf("%s non consulta gli indirizzi: %v", r.Ruolo, err)
		}
	}
	for nome, sbagliata := range map[string]RicercaIndirizzi{
		"pagina 0":       {Pagina: 0, PerPagina: 20},
		"per_pagina 101": {Pagina: 1, PerPagina: 101},
	} {
		if _, _, err := s.Cerca(ctx, operatore, sbagliata); tipoErrore(err) != "400" {
			t.Errorf("%s: atteso 400, ottenuto %v", nome, err)
		}
	}
}
