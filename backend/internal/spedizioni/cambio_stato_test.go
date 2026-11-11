package spedizioni

import (
	"testing"

	"lastmiledelivery/backend/internal/identita"
)

func TestCambioStato(t *testing.T) {
	nota := p("pacco non trovato")
	annulla := CambioStato{Stato: Annullata, Motivo: p("richiesta del mittente"), Nota: nota}
	perdi := CambioStato{Stato: Persa, Motivo: p("furto"), Nota: nota}
	casi := []struct {
		nome   string
		chi    identita.Richiedente
		da     Stato
		cambio CambioStato
		esito  string
	}{
		{"annulla da creata", operatore, Creata, annulla, "nessuno"},
		{"annulla da assegnata", operatore, Assegnata, annulla, "nessuno"},
		{"ritira", corriere1, Assegnata, CambioStato{Stato: Ritirata}, "nessuno"},
		{"in consegna", corriere1, Ritirata, CambioStato{Stato: InConsegna}, "nessuno"},
		{"consegna", corriere1, InConsegna, CambioStato{Stato: Consegnata}, "nessuno"},
		{"persa dopo il ritiro", operatore, Ritirata, perdi, "nessuno"},
		{"persa in consegna", operatore, InConsegna, perdi, "nessuno"},
		{"annulla senza motivo né nota", operatore, Assegnata, CambioStato{Stato: Annullata}, "400"},
		{"persa senza motivo", operatore, Ritirata, CambioStato{Stato: Persa, Nota: nota}, "400"},

		{"salta stati", corriere1, Assegnata, CambioStato{Stato: Consegnata}, "409"},
		{"annulla dopo il ritiro", operatore, Ritirata, annulla, "409"},
		{"persa prima del ritiro", operatore, Assegnata, perdi, "409"},
		{"stato finale", corriere1, Consegnata, CambioStato{Stato: InConsegna}, "409"},

		{"operatore ritira", operatore, Assegnata, CambioStato{Stato: Ritirata}, "403"},
		{"operatore consegna", operatore, InConsegna, CambioStato{Stato: Consegnata}, "403"},
		{"corriere segna persa", corriere1, InConsegna, perdi, "403"},
		{"corriere annulla", corriere1, Assegnata, annulla, "403"},
		{"corriere non assegnato", corriere2, Assegnata, CambioStato{Stato: Ritirata}, "403"},

		{"operatore riporta in ritirata", operatore, InConsegna, CambioStato{Stato: Ritirata}, "nessuno"},
		{"corriere riporta in ritirata", corriere1, InConsegna, CambioStato{Stato: Ritirata}, "403"},
	}
	for _, c := range casi {
		t.Run(c.nome, func(t *testing.T) {
			m, err := servizioTest().decidiStato(c.chi, inStato(c.da), c.cambio)
			if got := tipoErrore(err); got != c.esito {
				t.Fatalf("atteso %s, ottenuto %s (%v)", c.esito, got, err)
			}
			if err == nil && m.NuovoStato != c.cambio.Stato {
				t.Fatalf("stato atteso %q, ottenuto %q", c.cambio.Stato, m.NuovoStato)
			}
		})
	}
}

func TestStatiDaAltriEndpoint(t *testing.T) {
	for _, stato := range []Stato{Creata, Assegnata, NonConsegnata, "smarrita"} {
		if tipoErrore(validaCambio(CambioStato{Stato: stato})) != "400" {
			t.Errorf("%q non deve essere accettato da /stato", stato)
		}
	}
}

func TestRipetereLoStessoStato(t *testing.T) {
	s := servizioTest()
	m, err := s.decidiStato(corriere1, inStato(Ritirata), CambioStato{Stato: Ritirata})
	if err != nil || !m.NessunaModifica {
		t.Fatalf("ripetere 'ritirata' deve essere senza effetto: %+v %v", m, err)
	}
	m, err = s.decidiStato(operatore, inStato(Annullata), CambioStato{Stato: Annullata})
	if err != nil || !m.NessunaModifica {
		t.Fatalf("ripetere 'annullata' deve essere senza effetto: %+v %v", m, err)
	}
	if _, err := s.decidiStato(corriere2, inStato(Ritirata), CambioStato{Stato: Ritirata}); tipoErrore(err) != "403" {
		t.Fatalf("un altro corriere non può ripetere la richiesta: %v", err)
	}
	if _, err := s.decidiStato(operatore, inStato(Consegnata), CambioStato{Stato: Consegnata}); tipoErrore(err) != "403" {
		t.Fatalf("l'operatore non può impostare 'consegnata': %v", err)
	}
}

func TestMotivoObbligatorio(t *testing.T) {
	casi := []struct {
		nome   string
		cambio CambioStato
		esito  string
	}{
		{"annullata nota vuota", CambioStato{Stato: Annullata, Motivo: p("richiesta del mittente"), Nota: p("  ")}, "400"},
		{"annullata con motivo e nota", CambioStato{Stato: Annullata, Motivo: p("richiesta del destinatario"), Nota: p("x")}, "nessuno"},
		{"persa con motivo e nota", CambioStato{Stato: Persa, Motivo: p("smarrita in deposito"), Nota: p("x")}, "nessuno"},
		{"motivo fuori elenco", CambioStato{Stato: Annullata, Motivo: p("ci ho ripensato"), Nota: p("x")}, "400"},
		{"motivo di perdita su annullata", CambioStato{Stato: Annullata, Motivo: p("furto"), Nota: p("x")}, "400"},
		{"motivo su un altro stato", CambioStato{Stato: Ritirata, Motivo: p("altro")}, "400"},
		{"ritirata senza nota né motivo", CambioStato{Stato: Ritirata}, "nessuno"},
	}
	for _, c := range casi {
		t.Run(c.nome, func(t *testing.T) {
			if got := tipoErrore(validaCambio(c.cambio)); got != c.esito {
				t.Fatalf("atteso %s, ottenuto %s", c.esito, got)
			}
		})
	}

	m, _ := servizioTest().decidiStato(operatore, inStato(Ritirata), CambioStato{Stato: Persa, Motivo: p("smarrita durante il trasporto"), Nota: p("non è nel furgone")})
	if m.Nota.Testo != "non è nel furgone" || *m.Nota.Motivo != "smarrita durante il trasporto" {
		t.Fatalf("nota e motivo vanno salvati insieme: %+v", m.Nota)
	}
}
