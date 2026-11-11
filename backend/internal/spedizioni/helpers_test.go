package spedizioni

import (
	"context"
	"errors"
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

func must(m Modifica, err error) Modifica {
	if err != nil {
		panic(err)
	}
	return m
}

type archivioElenco struct {
	vis       Visibilita
	filtro    FiltroElenco
	voci      []VoceElenco
	dettaglio DettaglioSpedizione
	attuale   Spedizione
	nota      Nota
}

func (a *archivioElenco) Dettaglio(_ context.Context, v Visibilita, _ int32) (DettaglioSpedizione, error) {
	a.vis = v
	return a.dettaglio, nil
}

func (a *archivioElenco) AggiungiNota(_ context.Context, _ int32, autore identita.Richiedente, testo string,
	verifica func(Spedizione) error) (Nota, error) {
	if err := verifica(a.attuale); err != nil {
		return Nota{}, err
	}
	a.nota = Nota{Testo: testo, IsInterna: true}
	a.nota.IDOperatore, a.nota.IDCorriere = autoreNota(autore)
	return a.nota, nil
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
