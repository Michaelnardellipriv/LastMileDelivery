package spedizioni

import (
	"context"
	"errors"
	"testing"
	"time"

	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

var (
	operatore = identita.Richiedente{Ruolo: identita.Operatore, ID: 1}
	corriere1 = identita.Richiedente{Ruolo: identita.Corriere, ID: 1}
	corriere2 = identita.Richiedente{Ruolo: identita.Corriere, ID: 2}
)

func p[T any](v T) *T { return &v }

func servizioTest() *Servizio {
	s := NuovoServizio(nil, 3, 30)
	s.adesso = func() time.Time { return time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC) }
	return s
}

func inStato(stato Stato) Spedizione {
	s := Spedizione{ID: 10, Stato: stato, IDIndirizzoRitiro: 1, IDIndirizzoSpedizione: 2}
	if stato != Creata && stato != Annullata {
		s.IDCorriere = p(int32(1))
	}
	if stato == InConsegna {
		s.UsciteInConsegna = 1
	}
	return s
}

func tipoErrore(err error) string {
	var v *errori.Validazione
	var na *errori.NonAutorizzato
	var c *errori.Conflitto
	switch {
	case err == nil:
		return "nessuno"
	case errors.As(err, &v):
		return "400"
	case errors.As(err, &na):
		return "403"
	case errors.As(err, &c):
		return "409"
	}
	return err.Error()
}

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

func must(m Modifica, err error) Modifica {
	if err != nil {
		panic(err)
	}
	return m
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
