package spedizioni

import (
	"context"
	"testing"
	"time"

	"lastmiledelivery/backend/internal/identita"
)

type archivioElenco struct {
	vis    Visibilita
	filtro FiltroElenco
	voci   []VoceElenco
}

func (a *archivioElenco) Crea(context.Context, int32, NuovaSpedizione) (Spedizione, error) {
	return Spedizione{}, nil
}

func (a *archivioElenco) Aggiorna(context.Context, int32, identita.Richiedente,
	func(Spedizione) (Modifica, error)) (Spedizione, bool, error) {
	return Spedizione{}, false, nil
}

func (a *archivioElenco) Storico(context.Context, int32) ([]VoceStorico, error) {
	return []VoceStorico{{ID: 1, Operazione: "creazione"}}, nil
}

func (a *archivioElenco) Elenca(_ context.Context, v Visibilita, f FiltroElenco) ([]VoceElenco, error) {
	a.vis, a.filtro = v, f
	return a.voci, nil
}

var (
	cliente1       = identita.Richiedente{Ruolo: identita.Cliente, ID: 7}
	amministratore = identita.Richiedente{Ruolo: identita.Amministratore, ID: 1}
	filtroBase     = FiltroElenco{Pagina: 1, PerPagina: 20}
)

func servizioElenco(voci ...VoceElenco) (*Servizio, *archivioElenco) {
	a := &archivioElenco{voci: voci}
	s := NuovoServizio(a, 3, 30)
	s.adesso = func() time.Time { return time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC) }
	return s, a
}

func TestVisibilitaPerRuolo(t *testing.T) {
	s, a := servizioElenco()

	for _, r := range []identita.Richiedente{operatore, amministratore} {
		if _, err := s.Elenca(context.Background(), r, filtroBase); err != nil {
			t.Fatal(err)
		}
		if a.vis.IDCorriere != nil || a.vis.IDDestinatario != nil || a.vis.ConcluseDal != nil || len(a.vis.Stati) != 0 || !a.vis.MostraCorriere {
			t.Fatalf("%s: deve vedere tutte le spedizioni in tutti gli stati: %+v", r.Ruolo, a.vis)
		}
	}

	s.Elenca(context.Background(), corriere1, filtroBase)
	if a.vis.IDCorriere == nil || *a.vis.IDCorriere != 1 || a.vis.MostraCorriere {
		t.Fatalf("corriere: solo le spedizioni assegnate a lui: %+v", a.vis)
	}
	for _, st := range a.vis.Stati {
		if st.Finale() || st == Creata {
			t.Fatalf("il corriere non deve vedere lo stato %q", st)
		}
	}
	if len(a.vis.Stati) != 3 {
		t.Fatalf("il corriere vede assegnata, ritirata, in consegna: %v", a.vis.Stati)
	}

	s.Elenca(context.Background(), cliente1, filtroBase)
	limite := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	if a.vis.IDDestinatario == nil || *a.vis.IDDestinatario != 7 || a.vis.ConcluseDal == nil ||
		!a.vis.ConcluseDal.Equal(limite) || !a.vis.SoloNotePubbliche || a.vis.MostraCorriere {
		t.Fatalf("cliente: solo le sue degli ultimi 30 giorni, senza note interne: %+v", a.vis)
	}
}

func TestGiorniVisibilitaClienteImpostabili(t *testing.T) {
	a := &archivioElenco{}
	s := NuovoServizio(a, 3, 7)
	adesso := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	s.adesso = func() time.Time { return adesso }
	s.Elenca(context.Background(), cliente1, filtroBase)
	if !a.vis.ConcluseDal.Equal(adesso.AddDate(0, 0, -7)) {
		t.Fatalf("con 7 giorni il limite deve essere 7 giorni fa: %v", a.vis.ConcluseDal)
	}
}

func TestElencoNascondeIlCorriere(t *testing.T) {
	id := int32(4)
	s, _ := servizioElenco(VoceElenco{ID: 1, IDCorriere: &id})

	voci, _ := s.Elenca(context.Background(), operatore, filtroBase)
	if voci[0].IDCorriere == nil {
		t.Fatal("l'operatore vede il corriere")
	}
	voci, _ = s.Elenca(context.Background(), cliente1, filtroBase)
	if voci[0].IDCorriere != nil {
		t.Fatal("il cliente non vede il corriere")
	}
}

func TestValidaFiltro(t *testing.T) {
	sconosciuto := Stato("smarrita")
	casi := map[string]FiltroElenco{
		"stato sconosciuto":    {Stato: &sconosciuto, Pagina: 1, PerPagina: 20},
		"pagina zero":          {Pagina: 0, PerPagina: 20},
		"per_pagina zero":      {Pagina: 1, PerPagina: 0},
		"per_pagina oltre 100": {Pagina: 1, PerPagina: 101},
	}
	s, _ := servizioElenco()
	for nome, f := range casi {
		t.Run(nome, func(t *testing.T) {
			if _, err := s.Elenca(context.Background(), operatore, f); tipoErrore(err) != "400" {
				t.Fatalf("atteso 400, ottenuto %v", err)
			}
		})
	}
}

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

func TestFiltriElenco(t *testing.T) {
	s, a := servizioElenco()
	oggi := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	ieri := oggi.AddDate(0, 0, -1)
	corriere, cod := int32(3), "ORD-2026"
	f := FiltroElenco{IDCorriere: &corriere, Dal: &ieri, Al: &oggi, CodOrdine: &cod, Pagina: 1, PerPagina: 20}
	if _, err := s.Elenca(context.Background(), operatore, f); err != nil {
		t.Fatalf("filtri validi: %v", err)
	}
	if *a.filtro.IDCorriere != 3 || !a.filtro.Dal.Equal(ieri) || !a.filtro.Al.Equal(oggi) || *a.filtro.CodOrdine != cod {
		t.Fatalf("i filtri arrivano al repository: %+v", a.filtro)
	}

	vuoto, lungo, zero := "", string(make([]rune, 51)), int32(0)
	for nome, f := range map[string]FiltroElenco{
		"al prima di dal":         {Dal: &oggi, Al: &ieri, Pagina: 1, PerPagina: 20},
		"corriere non valido":     {IDCorriere: &zero, Pagina: 1, PerPagina: 20},
		"cod_ordine vuoto":        {CodOrdine: &vuoto, Pagina: 1, PerPagina: 20},
		"cod_ordine troppo lungo": {CodOrdine: &lungo, Pagina: 1, PerPagina: 20},
	} {
		if _, err := s.Elenca(context.Background(), operatore, f); tipoErrore(err) != "400" {
			t.Errorf("%s: atteso 400, ottenuto %v", nome, err)
		}
	}

	for _, r := range []identita.Richiedente{corriere1, cliente1} {
		if _, err := s.Elenca(context.Background(), r, FiltroElenco{IDCorriere: &corriere, Pagina: 1, PerPagina: 20}); tipoErrore(err) != "403" {
			t.Errorf("%s non filtra per corriere: %v", r.Ruolo, err)
		}
	}
	if _, err := s.Elenca(context.Background(), amministratore, FiltroElenco{IDCorriere: &corriere, Pagina: 1, PerPagina: 20}); err != nil {
		t.Errorf("l'amministratore filtra per corriere: %v", err)
	}
}
