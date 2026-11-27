package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"lastmiledelivery/backend/internal/errori"
)

// pool che non si collega mai (nessuna query nei test): serve solo come identità
func poolFinto(t *testing.T) *pgxpool.Pool {
	t.Helper()
	p, err := pgxpool.New(context.Background(), "postgres://x:x@127.0.0.1:1/x")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

type mappaFinta struct {
	gruppi   map[int64]int
	chiamate int
	errore   error
}

func (m *mappaFinta) GruppoDellaSede(_ context.Context, id int64) (int, bool, error) {
	m.chiamate++
	g, ok := m.gruppi[id]
	return g, ok, m.errore
}

func TestGruppoDellID(t *testing.T) {
	for id, atteso := range map[int64]int{
		1: 0, PassoIDGruppo - 1: 0, PassoIDGruppo + 1: 1, 2*PassoIDGruppo + 42: 2,
		int64(MassimoGruppo)*PassoIDGruppo + 1: MassimoGruppo, 0: -1, -5: -1,
	} {
		if g := GruppoDellID(id); g != atteso {
			t.Errorf("id %d: gruppo %d, atteso %d", id, g, atteso)
		}
	}
	// l'id più alto resta un numero esatto in JavaScript (2^53)
	if massimo := int64(MassimoGruppo+1)*PassoIDGruppo - 1; massimo >= 1<<53 {
		t.Errorf("l'id massimo %d supera 2^53", massimo)
	}
}

func TestMappaSedeDatabase(t *testing.T) {
	ctx := context.Background()
	zero, uno := poolFinto(t), poolFinto(t)
	mappa := &mappaFinta{gruppi: map[int64]int{10: 0, 20: 1, 30: 7}}
	g, err := NuoviGruppi(map[int]*pgxpool.Pool{1: uno, 0: zero}, mappa)
	if err != nil {
		t.Fatal(err)
	}
	// i pool finti non rispondono: si segnano come già controllati (all'avvio lo fa avvio)
	g.Segna(0)
	g.Segna(1)
	tutti, err := g.Tutti(ctx)
	if c := g.Codici(); err != nil || len(c) != 2 || c[0] != 0 || c[1] != 1 || g.Principale() != zero || tutti[1] != uno {
		t.Fatalf("gruppi in ordine, principale = gruppo 0: %v %v", c, err)
	}

	if db, err := g.PerSede(ctx, 20); err != nil || db != uno {
		t.Fatalf("la sede 20 è nel gruppo 1: %v", err)
	}
	if db, err := g.PerSede(ctx, 20); err != nil || db != uno || mappa.chiamate != 1 {
		t.Fatalf("la mappa di una sede si chiede una volta sola (chiamate %d): %v", mappa.chiamate, err)
	}
	if db, err := g.PerSede(ctx, 10); err != nil || db != zero {
		t.Fatalf("la sede 10 è nel principale: %v", err)
	}
	var nonTrovato *errori.NonTrovato
	if _, err := g.PerSede(ctx, 99); !errors.As(err, &nonTrovato) {
		t.Fatalf("sede inesistente: atteso 404, ottenuto %v", err)
	}
	if _, err := g.PerSede(ctx, 0); !errors.As(err, &nonTrovato) {
		t.Fatalf("sede 0: atteso 404, ottenuto %v", err)
	}
	if _, err := g.PerSede(ctx, 30); err == nil || errors.As(err, &nonTrovato) {
		t.Fatalf("sede in un gruppo senza database: errore di configurazione (500), ottenuto %v", err)
	}

	if db, ok, err := g.PerID(ctx, PassoIDGruppo+5); !ok || err != nil || db != uno {
		t.Fatal("un id del gruppo 1 si cerca nel database del gruppo 1")
	}
	if _, ok, err := g.PerID(ctx, 3*PassoIDGruppo+5); ok || err != nil {
		t.Fatal("un id di un gruppo che non c'è non ha database")
	}

	// operatore della sede 10 (gruppo 0): un id del gruppo 1 per lui non esiste
	operatore := ConAmbito(ctx, SoloSede(10))
	if _, ok, err := g.PerID(operatore, PassoIDGruppo+5); ok || err != nil {
		t.Fatalf("chi ha una sede non apre il database di un altro gruppo: %v %v", ok, err)
	}
	if db, ok, err := g.PerID(operatore, 7); !ok || err != nil || db != zero {
		t.Fatalf("un id del suo gruppo sì: %v %v", ok, err)
	}
	if db, ok, err := g.PerID(ConAmbito(ctx, TutteLeSedi()), PassoIDGruppo+5); !ok || err != nil || db != uno {
		t.Fatalf("amministratore e cliente cercano per id in ogni gruppo: %v %v", ok, err)
	}

	// ambito della richiesta: la sede di operatori e corrieri, altrimenti il principale
	if db, err := g.PerAmbito(ConAmbito(ctx, SoloSede(20))); err != nil || db != uno {
		t.Fatalf("ambito della sede 20: %v", err)
	}
	if db, err := g.PerAmbito(ConAmbito(ctx, TutteLeSedi())); err != nil || db != zero {
		t.Fatalf("senza sede: il principale, %v", err)
	}

	// il servizio utenti non risponde: errore, e la sede non entra nella cache
	giu := &mappaFinta{errore: errors.New("giù")}
	g2, _ := NuoviGruppi(map[int]*pgxpool.Pool{0: zero}, giu)
	g2.Segna(0)
	if _, err := g2.PerSede(ctx, 10); err == nil {
		t.Fatal("servizio utenti giù: errore")
	}
	giu.errore, giu.gruppi = nil, map[int64]int{10: 0}
	if db, err := g2.PerSede(ctx, 10); err != nil || db != zero {
		t.Fatalf("al tentativo dopo si richiede: %v", err)
	}
}

func TestNuoviGruppiSenzaPrincipale(t *testing.T) {
	if _, err := NuoviGruppi(map[int]*pgxpool.Pool{1: poolFinto(t)}, nil); err == nil {
		t.Fatal("senza gruppo 0: errore")
	}
	if _, err := NuoviGruppi(map[int]*pgxpool.Pool{0: poolFinto(t), MassimoGruppo + 1: poolFinto(t)}, nil); err == nil {
		t.Fatal("gruppo oltre il massimo: errore")
	}
}

// Un gruppo che all'avvio non rispondeva si controlla al primo uso: finché non risponde le
// sue sedi ricevono 503 (database non disponibile), gli altri gruppi lavorano.
func TestGruppoNonControllatoNonRisponde(t *testing.T) {
	ctx, annulla := context.WithTimeout(context.Background(), 10*time.Second)
	defer annulla()
	zero, uno := poolFinto(t), poolFinto(t)
	g, _ := NuoviGruppi(map[int]*pgxpool.Pool{0: zero, 1: uno}, &mappaFinta{gruppi: map[int64]int{10: 0, 20: 1}})
	g.Segna(0)
	if db, err := g.PerSede(ctx, 10); err != nil || db != zero {
		t.Fatalf("il gruppo 0 lavora: %v", err)
	}
	_, err := g.PerSede(ctx, 20)
	if err == nil || !NonDisponibile(err) {
		t.Fatalf("gruppo 1 giù: atteso database non disponibile (503), ottenuto %v", err)
	}
	if _, err := g.Tutti(ctx); err == nil {
		t.Fatal("chi legge tutti i gruppi riceve l'errore")
	}
	if v := g.Verificati(); len(v) != 1 || v[0] != zero {
		t.Fatalf("solo il gruppo 0 è controllato: %v", v)
	}
}
