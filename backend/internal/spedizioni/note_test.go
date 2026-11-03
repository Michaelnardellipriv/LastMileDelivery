package spedizioni

import (
	"context"
	"strings"
	"testing"

	"lastmiledelivery/backend/internal/identita"
)

func TestAggiungiNota(t *testing.T) {
	ctx := context.Background()
	assegnataA1 := Spedizione{ID: 10, Stato: InConsegna, IDCorriere: p(int32(1))}

	t.Run("operatore_sempre_interna", func(t *testing.T) {
		s, a := servizioElenco()
		a.attuale = Spedizione{ID: 10, Stato: Consegnata}
		n, err := s.AggiungiNota(ctx, operatore, 10, p("  pacco fragile  "))
		if err != nil {
			t.Fatal(err)
		}
		if n.Testo != "pacco fragile" || !n.IsInterna || n.IDOperatore == nil || n.IDCorriere != nil {
			t.Fatalf("nota interna dell'operatore, testo senza spazi: %+v", n)
		}
	})

	t.Run("corriere_su_ritirata_e_in_consegna", func(t *testing.T) {
		s, a := servizioElenco()
		a.attuale = assegnataA1
		n, err := s.AggiungiNota(ctx, corriere1, 10, p("citofono rotto"))
		if err != nil || !n.IsInterna || n.IDCorriere == nil || *n.IDCorriere != 1 {
			t.Fatalf("nota interna del corriere: %+v %v", n, err)
		}
		if _, err := s.AggiungiNota(ctx, corriere2, 10, p("x")); tipoErrore(err) != "403" {
			t.Errorf("spedizione di un altro corriere: atteso 403, ottenuto %v", err)
		}
		for _, stato := range Stati {
			a.attuale = Spedizione{ID: 10, Stato: stato, IDCorriere: p(int32(1))}
			_, err := s.AggiungiNota(ctx, corriere1, 10, p("x"))
			ammesso := stato == Ritirata || stato == InConsegna
			if ammesso && err != nil || !ammesso && tipoErrore(err) != "409" {
				t.Errorf("stato %q: ammesso=%v, ottenuto %v", stato, ammesso, err)
			}
		}
	})

	t.Run("operatore_in_ogni_stato", func(t *testing.T) {
		s, a := servizioElenco()
		for _, stato := range Stati {
			a.attuale = Spedizione{ID: 10, Stato: stato}
			if _, err := s.AggiungiNota(ctx, operatore, 10, p("x")); err != nil {
				t.Errorf("stato %q: %v", stato, err)
			}
		}
	})

	t.Run("testo_obbligatorio", func(t *testing.T) {
		s, _ := servizioElenco()
		for _, testo := range []*string{nil, p("   "), p(strings.Repeat("a", 2001))} {
			if _, err := s.AggiungiNota(ctx, operatore, 10, testo); tipoErrore(err) != "400" {
				t.Errorf("testo %v: atteso 400, ottenuto %v", testo, err)
			}
		}
	})

	t.Run("clienti_e_amministratori_no", func(t *testing.T) {
		s, _ := servizioElenco()
		for _, r := range []identita.Richiedente{cliente1, amministratore} {
			if _, err := s.AggiungiNota(ctx, r, 10, p("x")); tipoErrore(err) != "403" {
				t.Errorf("%s: atteso 403, ottenuto %v", r.Ruolo, err)
			}
		}
	})
}

func TestNotePubbliche(t *testing.T) {
	s := servizioTest()
	conMotivo := func(stato Stato, motivo string) CambioStato {
		return CambioStato{Stato: stato, Motivo: p(motivo), Nota: p("dettaglio interno")}
	}

	for nome, c := range map[string]struct {
		da     Stato
		cambio CambioStato
	}{
		"annullata": {Assegnata, conMotivo(Annullata, "richiesta del mittente")},
		"persa":     {Ritirata, conMotivo(Persa, "furto")},
	} {
		m, err := s.decidiStato(operatore, inStato(c.da), c.cambio)
		if err != nil || m.Nota == nil || !m.Nota.Pubblica {
			t.Errorf("%s: la nota col motivo è pubblica: %+v %v", nome, m.Nota, err)
		}
	}

	for _, stato := range []Stato{Ritirata, InConsegna, Consegnata} {
		if err := validaCambio(CambioStato{Stato: stato, Nota: p("partito")}); tipoErrore(err) != "400" {
			t.Errorf("%q: la nota si scrive solo con annullata e persa, le interne con POST /note: %v", stato, err)
		}
	}

	ritiro := TentativoFallito{Tipo: TentativoRitiro, Nota: p("cancello chiuso"), Motivo: p("mittente assente")}
	if m, err := s.decidiTentativo(corriere1, inStato(Assegnata), ritiro); err != nil || !m.Nota.Pubblica {
		t.Errorf("ogni tentativo di ritiro ha la nota pubblica: %+v %v", m, err)
	}
	consegna := TentativoFallito{Tipo: TentativoConsegna, Nota: p("nessuno in casa"), Motivo: p("destinatario assente")}
	inConsegna := inStato(InConsegna)
	if m, err := s.decidiTentativo(corriere1, inConsegna, consegna); err != nil || m.NuovoStato != InConsegna || !m.Nota.Pubblica {
		t.Errorf("il primo tentativo di consegna ha la nota pubblica: %+v %v", m, err)
	}
	inConsegna.TentativiConsegnaFalliti, inConsegna.UsciteInConsegna = 2, 3
	if m, err := s.decidiTentativo(corriere1, inConsegna, consegna); err != nil || m.NuovoStato != NonConsegnata || !m.Nota.Pubblica {
		t.Errorf("l'ultimo tentativo (non consegnata) ha la nota pubblica: %+v %v", m, err)
	}
}

func TestDettaglioVisibilita(t *testing.T) {
	ctx := context.Background()
	voce := func() VoceElenco {
		return VoceElenco{ID: 10, IDCorriere: p(int32(4)),
			Note: []Nota{{ID: 1, Testo: "dettaglio interno", Motivo: p("furto"), IDOperatore: p(int32(2))}}}
	}

	s, a := servizioElenco()
	a.dettaglio = DettaglioSpedizione{VoceElenco: voce()}
	d, err := s.Dettaglio(ctx, operatore, 10)
	if err != nil || d.IDCorriere == nil || d.Note[0].IDOperatore == nil || d.Note[0].Testo == "" || !a.vis.MostraCorriere {
		t.Fatalf("l'operatore vede corriere, testo e autori delle note: %+v %v", d, err)
	}

	a.dettaglio = DettaglioSpedizione{VoceElenco: voce()}
	d, _ = s.Dettaglio(ctx, cliente1, 10)
	n := d.Note[0]
	if d.IDCorriere != nil || n.IDOperatore != nil || n.Testo != "" || n.Motivo == nil || !a.vis.SoloNotePubbliche || *a.vis.IDDestinatario != 7 {
		t.Fatalf("il cliente vede solo il motivo delle note pubbliche: %+v %+v", d, a.vis)
	}

	s.Dettaglio(ctx, corriere1, 10)
	if a.vis.IDCorriere == nil || *a.vis.IDCorriere != 1 || len(a.vis.Stati) != 3 {
		t.Fatalf("il corriere vede solo le sue spedizioni aperte: %+v", a.vis)
	}
}
