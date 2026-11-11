package spedizioni

import (
	"context"
	"testing"
)

func TestTentativiDiRitiro(t *testing.T) {
	s := servizioTest()
	tentativo := TentativoFallito{Tipo: TentativoRitiro, Nota: p("cancello chiuso"), Motivo: p("accesso impossibile")}
	attuale := inStato(Assegnata)

	for falliti := 0; falliti < 3; falliti++ {
		attuale.TentativiRitiroFalliti = falliti
		m, err := s.decidiTentativo(corriere1, attuale, tentativo)
		if err != nil || m.Tentativo != TentativoRitiro || m.NuovoStato != Assegnata || m.Nota == nil {
			t.Fatalf("tentativo di ritiro %d: %+v %v", falliti+1, m, err)
		}
	}

	attuale.TentativiRitiroFalliti = 3
	if _, err := s.decidiTentativo(corriere1, attuale, tentativo); tipoErrore(err) != "409" {
		t.Fatalf("dopo 3 ritiri falliti il corriere non può registrarne altri: %v", err)
	}
	if _, err := s.decidiStato(corriere1, attuale, CambioStato{Stato: Ritirata}); tipoErrore(err) != "409" {
		t.Fatalf("dopo 3 ritiri falliti il corriere non può più ritirare: %v", err)
	}
	if m, err := s.decidiStato(operatore, attuale, CambioStato{Stato: Annullata}); err != nil || !m.Annulla || m.Nota != nil {
		t.Fatalf("dopo 3 ritiri falliti l'operatore annulla senza motivo né nota: %+v %v", m, err)
	}
	conMotivo := CambioStato{Stato: Annullata, Motivo: p("ritiro non riuscito"), Nota: p("negozio chiuso")}
	if m, err := s.decidiStato(operatore, attuale, conMotivo); err != nil || m.Nota == nil || !m.Nota.Pubblica {
		t.Fatalf("dopo 3 ritiri falliti motivo e nota restano facoltativi: %+v %v", m, err)
	}
	for _, c := range []CambioStato{
		{Stato: Annullata, Motivo: p("ritiro non riuscito")},
		{Stato: Annullata, Nota: p("negozio chiuso")},
	} {
		if _, err := s.decidiStato(operatore, attuale, c); tipoErrore(err) != "400" {
			t.Errorf("motivo e nota vanno insieme: %+v %v", c, err)
		}
	}
	attuale.TentativiRitiroFalliti = 2
	if _, err := s.decidiStato(operatore, attuale, CambioStato{Stato: Annullata}); tipoErrore(err) != "400" {
		t.Fatalf("prima del 3° ritiro fallito motivo e nota sono obbligatori: %v", err)
	}
	if _, err := s.decidiTentativo(corriere1, inStato(Ritirata), tentativo); tipoErrore(err) != "409" {
		t.Fatalf("ritiro fallito su una spedizione già ritirata: %v", err)
	}
	if _, err := s.decidiTentativo(corriere2, inStato(Assegnata), tentativo); tipoErrore(err) != "403" {
		t.Fatalf("corriere non assegnato: %v", err)
	}
}

func TestTentativiDiConsegna(t *testing.T) {
	s := servizioTest()
	tentativo := TentativoFallito{Tipo: TentativoConsegna, Nota: p("nessuno in casa"), Motivo: p("destinatario assente")}
	attuale := inStato(InConsegna)

	attese := []Stato{InConsegna, InConsegna, NonConsegnata}
	for falliti, atteso := range attese {
		attuale.TentativiConsegnaFalliti = falliti
		attuale.UsciteInConsegna = falliti + 1
		m, err := s.decidiTentativo(corriere1, attuale, tentativo)
		if err != nil || m.NuovoStato != atteso || m.Tentativo != TentativoConsegna || *m.Nota.Motivo != "destinatario assente" {
			t.Fatalf("tentativo di consegna %d: atteso %q, ottenuto %+v %v", falliti+1, atteso, m, err)
		}
	}

	stessaUscita := inStato(InConsegna)
	stessaUscita.TentativiConsegnaFalliti = 1
	if _, err := s.decidiTentativo(corriere1, stessaUscita, tentativo); tipoErrore(err) != "409" {
		t.Fatalf("un secondo tentativo nella stessa uscita va rifiutato: %v", err)
	}

	if _, err := s.decidiTentativo(corriere1, inStato(Ritirata), tentativo); tipoErrore(err) != "409" {
		t.Fatalf("consegna fallita su una spedizione non in consegna: %v", err)
	}
}

func TestValidaTentativo(t *testing.T) {
	casi := map[string]TentativoFallito{
		"tipo sconosciuto":             {Tipo: "volo", Nota: p("x")},
		"senza nota":                   {Tipo: TentativoRitiro},
		"consegna senza nota":          {Tipo: TentativoConsegna, Motivo: p("x")},
		"nota vuota":                   {Tipo: TentativoRitiro, Nota: p("  ")},
		"motivo fuori elenco":          {Tipo: TentativoConsegna, Nota: p("x"), Motivo: p("pioveva")},
		"motivo di ritiro su consegna": {Tipo: TentativoConsegna, Nota: p("x"), Motivo: p("mittente assente")},
		"motivo di consegna su ritiro": {Tipo: TentativoRitiro, Nota: p("x"), Motivo: p("destinatario assente")},
		"ritiro senza motivo":          {Tipo: TentativoRitiro, Nota: p("x")},
		"consegna senza motivo":        {Tipo: TentativoConsegna, Nota: p("x")},
	}
	for nome, c := range casi {
		t.Run(nome, func(t *testing.T) {
			if tipoErrore(validaTentativo(c)) != "400" {
				t.Fatal("atteso errore di validazione")
			}
		})
	}
	for nome, c := range map[string]TentativoFallito{
		"ritiro con motivo":   {Tipo: TentativoRitiro, Nota: p("x"), Motivo: p("pacco non pronto")},
		"consegna con motivo": {Tipo: TentativoConsegna, Nota: p("x"), Motivo: p("indirizzo errato")},
		"altro":               {Tipo: TentativoConsegna, Nota: p("x"), Motivo: p("altro")},
	} {
		if err := validaTentativo(c); err != nil {
			t.Errorf("%s: deve essere valido, ottenuto %v", nome, err)
		}
	}
	if _, err := servizioTest().RegistraTentativo(context.Background(), operatore, 10, TentativoFallito{}); tipoErrore(err) != "403" {
		t.Fatalf("un operatore non registra tentativi: %v", err)
	}
}
