package posizioni

import (
	"context"
	"errors"
	"testing"

	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

type archivioFinto struct {
	registrata       bool
	intervallo       int32
	idAmministratore int32
}

func (a *archivioFinto) Registra(context.Context, int32, float64, float64) (int32, error) {
	a.registrata = true
	return a.intervallo, nil
}

func (a *archivioFinto) Posizioni(context.Context) ([]Posizione, error) {
	return []Posizione{{IDCorriere: 1}}, nil
}

func (a *archivioFinto) Intervallo(context.Context) (int32, error) {
	return a.intervallo, nil
}

func (a *archivioFinto) AggiornaIntervallo(_ context.Context, secondi, idAmministratore int32) error {
	a.intervallo, a.idAmministratore = secondi, idAmministratore
	return nil
}

var (
	operatore      = identita.Richiedente{Ruolo: identita.Operatore, ID: 1}
	corriere       = identita.Richiedente{Ruolo: identita.Corriere, ID: 2}
	cliente        = identita.Richiedente{Ruolo: identita.Cliente, ID: 3}
	amministratore = identita.Richiedente{Ruolo: identita.Amministratore, ID: 4}
)

func p[T any](v T) *T { return &v }

func tipoErrore(err error) string {
	var validazione *errori.Validazione
	var nonAutorizzato *errori.NonAutorizzato
	switch {
	case err == nil:
		return "nessuno"
	case errors.As(err, &validazione):
		return "400"
	case errors.As(err, &nonAutorizzato):
		return "403"
	}
	return err.Error()
}

func TestInviaPosizione(t *testing.T) {
	ctx := context.Background()
	a := &archivioFinto{intervallo: 30}
	s := NuovoServizio(a)

	if intervallo, err := s.Invia(ctx, corriere, p(46.07), p(11.12)); err != nil || intervallo != 30 || !a.registrata {
		t.Fatalf("il corriere invia la posizione: %d %v", intervallo, err)
	}
	for _, r := range []identita.Richiedente{operatore, cliente, amministratore} {
		if _, err := s.Invia(ctx, r, p(46.07), p(11.12)); tipoErrore(err) != "403" {
			t.Errorf("%s non invia posizioni: %v", r.Ruolo, err)
		}
	}
	for nome, c := range map[string][2]*float64{
		"latitudine mancante":     {nil, p(11.0)},
		"longitudine mancante":    {p(46.0), nil},
		"latitudine fuori range":  {p(90.5), p(11.0)},
		"longitudine fuori range": {p(46.0), p(-180.5)},
	} {
		if _, err := s.Invia(ctx, corriere, c[0], c[1]); tipoErrore(err) != "400" {
			t.Errorf("%s: atteso 400, ottenuto %v", nome, err)
		}
	}
	if _, err := s.Invia(ctx, corriere, p(-90.0), p(180.0)); err != nil {
		t.Errorf("gli estremi sono coordinate valide: %v", err)
	}
}

func TestElencoPosizioniSoloOperatori(t *testing.T) {
	s := NuovoServizio(&archivioFinto{intervallo: 30})
	if intervallo, posizioni, err := s.Elenca(context.Background(), operatore); err != nil || intervallo != 30 || len(posizioni) != 1 {
		t.Fatalf("l'operatore vede le posizioni: %d %v %v", intervallo, posizioni, err)
	}
	for _, r := range []identita.Richiedente{corriere, cliente, amministratore} {
		if _, _, err := s.Elenca(context.Background(), r); tipoErrore(err) != "403" {
			t.Errorf("%s non vede le posizioni: %v", r.Ruolo, err)
		}
	}
}

func TestIntervallo(t *testing.T) {
	ctx := context.Background()
	a := &archivioFinto{intervallo: 30}
	s := NuovoServizio(a)

	for _, r := range []identita.Richiedente{corriere, operatore, amministratore} {
		if intervallo, err := s.Intervallo(ctx, r); err != nil || intervallo != 30 {
			t.Errorf("%s legge l'intervallo: %d %v", r.Ruolo, intervallo, err)
		}
	}
	if _, err := s.Intervallo(ctx, cliente); tipoErrore(err) != "403" {
		t.Errorf("il cliente non legge l'intervallo: %v", err)
	}

	if secondi, err := s.ModificaIntervallo(ctx, amministratore, p(int32(60))); err != nil || secondi != 60 || a.intervallo != 60 || a.idAmministratore != 4 {
		t.Fatalf("l'amministratore modifica l'intervallo: %d %v %+v", secondi, err, a)
	}
	for _, r := range []identita.Richiedente{corriere, operatore, cliente} {
		if _, err := s.ModificaIntervallo(ctx, r, p(int32(60))); tipoErrore(err) != "403" {
			t.Errorf("%s non modifica l'intervallo: %v", r.Ruolo, err)
		}
	}
	for _, secondi := range []*int32{nil, p(int32(4)), p(int32(3601))} {
		if _, err := s.ModificaIntervallo(ctx, amministratore, secondi); tipoErrore(err) != "400" {
			t.Errorf("intervallo %v: atteso 400, ottenuto %v", secondi, err)
		}
	}
}
