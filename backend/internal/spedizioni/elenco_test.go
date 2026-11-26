package spedizioni

import (
	"context"
	"testing"
	"time"

	"lastmiledelivery/backend/internal/database"
	"lastmiledelivery/backend/internal/identita"
)

// archivioElenco ricorda visibilità, filtro e ambito delle sedi con cui è stato chiamato.
type archivioElenco struct {
	vis       Visibilita
	filtro    FiltroElenco
	ambito    database.Ambito
	voci      []VoceElenco
	dettaglio DettaglioSpedizione
	attuale   Spedizione
	nota      Nota
	// sedeCorriere: in quale sede lavora ogni corriere (assente = corriere inesistente)
	sedeCorriere map[int64]int64
	creata       *NuovaSpedizione
	sedeCreata   int64
	aggiornata   bool
}

func (a *archivioElenco) Dettaglio(_ context.Context, ambito database.Ambito, v Visibilita, _ int64) (DettaglioSpedizione, error) {
	a.vis, a.ambito = v, ambito
	return a.dettaglio, nil
}

func (a *archivioElenco) AggiungiNota(_ context.Context, ambito database.Ambito, _ int64, autore identita.Richiedente, testo string,
	verifica func(Spedizione) error) (Nota, error) {
	a.ambito = ambito
	if err := verifica(a.attuale); err != nil {
		return Nota{}, err
	}
	a.nota = Nota{Testo: testo, IsInterna: true}
	a.nota.IDOperatore, a.nota.IDCorriere = autoreNota(autore)
	return a.nota, nil
}

func (a *archivioElenco) Crea(_ context.Context, idSede, _ int64, n NuovaSpedizione) (Spedizione, error) {
	a.creata, a.sedeCreata = &n, idSede
	return Spedizione{IDSede: idSede}, nil
}

func (a *archivioElenco) Aggiorna(_ context.Context, ambito database.Ambito, _ int64, _ identita.Richiedente,
	decidi func(Spedizione) (Modifica, error)) (Spedizione, bool, error) {
	a.ambito, a.aggiornata = ambito, true
	_, err := decidi(a.attuale)
	return a.attuale, err == nil, err
}

func (a *archivioElenco) SedeCorriere(_ context.Context, id int64) (int64, bool, error) {
	sede, ok := a.sedeCorriere[id]
	return sede, ok, nil
}

func (a *archivioElenco) Storico(_ context.Context, ambito database.Ambito, _ int64) ([]VoceStorico, error) {
	a.ambito = ambito
	return []VoceStorico{{ID: 1, Operazione: "creazione"}}, nil
}

func (a *archivioElenco) Elenca(_ context.Context, ambito database.Ambito, v Visibilita, f FiltroElenco) ([]VoceElenco, error) {
	a.vis, a.filtro, a.ambito = v, f, ambito
	return a.voci, nil
}

func (a *archivioElenco) Conta(context.Context, database.Ambito, Visibilita, FiltroElenco) (int, error) {
	return len(a.voci), nil
}

var (
	cliente1       = identita.Richiedente{Ruolo: identita.Cliente, ID: 7}
	amministratore = identita.Richiedente{Ruolo: identita.Amministratore, ID: 1}
	filtroBase     = FiltroElenco{Pagina: 1, PerPagina: 20}
)

func servizioElenco(voci ...VoceElenco) (*Servizio, *archivioElenco) {
	a := &archivioElenco{voci: voci}
	s := NuovoServizio(a, regoleFisse(regoleTest))
	s.adesso = func() time.Time { return time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC) }
	return s, a
}

// filtroAmministratore: l'amministratore sceglie sempre una sede (id_sede obbligatorio).
func filtroAmministratore() FiltroElenco {
	f := filtroBase
	f.IDSede = p(int64(sedeTrento))
	return f
}

// filtroPer: il filtro di base, con la sede per l'amministratore.
func filtroPer(r identita.Richiedente) FiltroElenco {
	if r.Ruolo == identita.Amministratore {
		return filtroAmministratore()
	}
	return filtroBase
}

func TestVisibilitaPerRuolo(t *testing.T) {
	s, a := servizioElenco()

	for _, r := range []identita.Richiedente{operatore, amministratore} {
		if _, err := s.Elenca(context.Background(), r, filtroPer(r)); err != nil {
			t.Fatal(err)
		}
		if a.vis.IDCorriere != nil || a.vis.IDDestinatario != nil || a.vis.ConcluseEntroGiorni || len(a.vis.Stati) != 0 || !a.vis.MostraCorriere {
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
	adesso := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	if a.vis.IDDestinatario == nil || *a.vis.IDDestinatario != 7 || !a.vis.ConcluseEntroGiorni ||
		!a.vis.Adesso.Equal(adesso) || !a.vis.SoloNotePubbliche || a.vis.MostraCorriere {
		t.Fatalf("cliente: solo le sue, chiuse da pochi giorni (quelli della sede), senza note interne: %+v", a.vis)
	}
}

func TestOgniRuoloVedeLeSueSedi(t *testing.T) {
	s, a := servizioElenco()
	casi := []struct {
		r      identita.Richiedente
		atteso database.Ambito
	}{
		{operatore, database.SoloSede(sedeTrento)},
		{corriere1, database.SoloSede(sedeTrento)},
		// l'amministratore vede la sede che sceglie (id_sede obbligatorio)
		{amministratore, database.SoloSede(sedeTrento)},
		// il cliente non ha una sede: vede le spedizioni destinate a lui in qualunque sede
		{cliente1, database.TutteLeSedi()},
	}
	for _, c := range casi {
		if _, err := s.Elenca(context.Background(), c.r, filtroPer(c.r)); err != nil {
			t.Fatal(err)
		}
		if a.ambito != c.atteso {
			t.Errorf("%s: ambito %+v, atteso %+v", c.r.Ruolo, a.ambito, c.atteso)
		}
	}
	// dettaglio: si cerca per id (nel database del suo gruppo); l'amministratore vede
	// qualunque sede, come il cliente le spedizioni destinate a lui
	for _, c := range []struct {
		r      identita.Richiedente
		atteso database.Ambito
	}{
		{operatore, database.SoloSede(sedeTrento)},
		{corriere1, database.SoloSede(sedeTrento)},
		{amministratore, database.TutteLeSedi()},
		{cliente1, database.TutteLeSedi()},
	} {
		s.Dettaglio(context.Background(), c.r, 1)
		if a.ambito != c.atteso {
			t.Errorf("%s, dettaglio: ambito %+v, atteso %+v", c.r.Ruolo, a.ambito, c.atteso)
		}
	}
}

func TestFiltroPerSede(t *testing.T) {
	s, a := servizioElenco()
	bolzano := filtroBase
	bolzano.IDSede = p(int64(sedeBolzano))

	if _, err := s.Elenca(context.Background(), amministratore, bolzano); err != nil || *a.filtro.IDSede != sedeBolzano ||
		a.ambito != database.SoloSede(sedeBolzano) {
		t.Fatalf("l'amministratore vede la sede scelta: %+v %+v %v", a.filtro, a.ambito, err)
	}
	a.filtro = FiltroElenco{}
	if _, err := s.Elenca(context.Background(), amministratore, filtroBase); tipoErrore(err) != "400" || a.filtro.PerPagina != 0 {
		t.Fatalf("senza sede l'amministratore riceve 400 (niente elenco di tutte le sedi): %v", err)
	}
	if _, _, err := s.ElencaPagina(context.Background(), amministratore, filtroBase); tipoErrore(err) != "400" {
		t.Fatalf("anche la pagina con il totale: atteso 400, ottenuto %v", err)
	}
	for _, r := range []identita.Richiedente{operatore, corriere1, cliente1} {
		if _, err := s.Elenca(context.Background(), r, bolzano); tipoErrore(err) != "403" {
			t.Errorf("%s non filtra su un'altra sede: %v", r.Ruolo, err)
		}
	}
	trento := filtroBase
	trento.IDSede = p(int64(sedeTrento))
	if _, err := s.Elenca(context.Background(), operatore, trento); err != nil {
		t.Errorf("l'operatore può indicare la sua sede: %v", err)
	}
	trento.IDSede = p(int64(0))
	if _, err := s.Elenca(context.Background(), amministratore, trento); tipoErrore(err) != "400" {
		t.Errorf("sede 0: atteso 400, ottenuto %v", err)
	}
}

func TestElencoNascondeIlCorriere(t *testing.T) {
	id := int64(4)
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
		"stato sconosciuto":       {Stato: &sconosciuto, Pagina: 1, PerPagina: 20},
		"pagina zero":             {Pagina: 0, PerPagina: 20},
		"pagina oltre il massimo": {Pagina: PaginaMassima + 1, PerPagina: 20},
		"per_pagina zero":         {Pagina: 1, PerPagina: 0},
		"per_pagina oltre 100":    {Pagina: 1, PerPagina: 101},
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
		if _, _, err := s.Assegna(context.Background(), r, 10, p(int64(1))); tipoErrore(err) != "403" {
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
	corriere, cod := int64(3), "ORD-2026"
	f := FiltroElenco{IDCorriere: &corriere, Dal: &ieri, Al: &oggi, CodOrdine: &cod, Pagina: 1, PerPagina: 20}
	if _, err := s.Elenca(context.Background(), operatore, f); err != nil {
		t.Fatalf("filtri validi: %v", err)
	}
	if *a.filtro.IDCorriere != 3 || !a.filtro.Dal.Equal(ieri) || !a.filtro.Al.Equal(oggi) || *a.filtro.CodOrdine != cod {
		t.Fatalf("i filtri arrivano al repository: %+v", a.filtro)
	}

	vuoto, lungo, zero := "", string(make([]rune, 51)), int64(0)
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
	if _, err := s.Elenca(context.Background(), amministratore, FiltroElenco{IDCorriere: &corriere, IDSede: p(int64(sedeTrento)), Pagina: 1, PerPagina: 20}); err != nil {
		t.Errorf("l'amministratore filtra per corriere: %v", err)
	}
}
