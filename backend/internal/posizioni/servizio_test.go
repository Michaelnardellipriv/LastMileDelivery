package posizioni

import (
	"context"
	"errors"
	"testing"

	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

const sedeTrento = 10

// archivioFinto ricorda la sede con cui è stato chiamato: il servizio deve passare
// sempre quella del richiedente.
type archivioFinto struct {
	registrata bool
	intervallo int64
	sede       int64
}

func (a *archivioFinto) Registra(_ context.Context, idSede, _ int64, _, _ float64) (int64, error) {
	a.registrata, a.sede = true, idSede
	return a.intervallo, nil
}

func (a *archivioFinto) Posizioni(_ context.Context, idSede int64) ([]Posizione, error) {
	a.sede = idSede
	return []Posizione{{IDCorriere: 1}}, nil
}

func (a *archivioFinto) Intervallo(_ context.Context, idSede int64) (int64, error) {
	a.sede = idSede
	return a.intervallo, nil
}

var (
	operatore      = identita.Richiedente{Ruolo: identita.Operatore, ID: 1, IDSede: sedeTrento}
	corriere       = identita.Richiedente{Ruolo: identita.Corriere, ID: 2, IDSede: sedeTrento}
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
	if a.sede != sedeTrento {
		t.Errorf("la posizione va nella sede del corriere: %d", a.sede)
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

func TestOperatoreVedeSoloICorrieriDellaSuaSede(t *testing.T) {
	a := &archivioFinto{intervallo: 30}
	s := NuovoServizio(a)
	if intervallo, posizioni, err := s.Elenca(context.Background(), operatore); err != nil || intervallo != 30 || len(posizioni) != 1 {
		t.Fatalf("l'operatore vede le posizioni: %d %v %v", intervallo, posizioni, err)
	}
	if a.sede != sedeTrento {
		t.Errorf("le posizioni sono quelle della sede dell'operatore: %d", a.sede)
	}
	for _, r := range []identita.Richiedente{corriere, cliente, amministratore} {
		if _, _, err := s.Elenca(context.Background(), r); tipoErrore(err) != "403" {
			t.Errorf("%s non vede le posizioni: %v", r.Ruolo, err)
		}
	}
}

func TestIntervalloDellaPropriaSede(t *testing.T) {
	ctx := context.Background()
	a := &archivioFinto{intervallo: 30}
	s := NuovoServizio(a)

	for _, r := range []identita.Richiedente{corriere, operatore} {
		if intervallo, err := s.Intervallo(ctx, r); err != nil || intervallo != 30 || a.sede != sedeTrento {
			t.Errorf("%s legge l'intervallo della sua sede: %d %v (sede %d)", r.Ruolo, intervallo, err, a.sede)
		}
	}
	// l'amministratore non ha una sede: usa GET /api/sedi/{id}/impostazioni
	for _, r := range []identita.Richiedente{cliente, amministratore} {
		if _, err := s.Intervallo(ctx, r); tipoErrore(err) != "403" {
			t.Errorf("%s non ha un intervallo di sede: %v", r.Ruolo, err)
		}
	}
}
