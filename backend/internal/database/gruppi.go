package database

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lastmiledelivery/backend/internal/errori"
)

// Mappa sede → database delle spedizioni.
//
// Le sedi sono divise in gruppi: ogni gruppo ha il suo database delle spedizioni (le città
// grandi da sole, le piccole insieme). Il gruppo di una sede è la colonna
// sedi.gruppo_spedizioni del database globale; non cambia dopo la creazione della sede,
// perché le sue spedizioni stanno nel database di quel gruppo.
//
// Gli id di ogni tabella del gruppo g partono da g × PassoIDGruppo + 1
// (fn_imposta_gruppo in db/spedizioni/schema.sql): dall'id di una spedizione si sa in
// quale database cercarla, e gli id non si ripetono tra gruppi diversi.
const (
	PassoIDGruppo int64 = 1_000_000_000_000 // 10^12 id per gruppo
	// MassimoGruppo: (MassimoGruppo+1) × 10^12 resta sotto 2^53, quindi ogni id è un
	// numero esatto anche in JavaScript (l'app web).
	MassimoGruppo = 9006
)

// GruppoDellID è il gruppo che ha creato la riga con quell'id.
func GruppoDellID(id int64) int {
	if id <= 0 {
		return -1
	}
	return int(id / PassoIDGruppo)
}

// MappaSedi dice il gruppo di una sede (servizio utenti, database globale); ok è falso se
// la sede non esiste.
type MappaSedi interface {
	GruppoDellaSede(ctx context.Context, idSede int64) (gruppo int, ok bool, err error)
}

// Gruppi sono i database delle spedizioni, uno per gruppo di sedi. Il gruppo 0 c'è sempre
// (il database "principale": in un'installazione con un solo database è l'unico).
//
// Prima di usare il database di un gruppo si controlla, una volta, che dichiari quel gruppo
// (VerificaGruppo). Così un gruppo che all'avvio non rispondeva non ferma il servizio: le
// sue sedi ricevono 503 finché il suo database non torna, le altre lavorano.
type Gruppi struct {
	pool       map[int]*pgxpool.Pool
	codici     []int
	mappa      MappaSedi
	sedi       sync.Map // id sede → gruppo: la mappa non cambia, quindi si tiene per sempre
	verificati sync.Map // codice → true quando il database ha dichiarato il gruppo giusto
}

var errSedeNonTrovata = &errori.NonTrovato{Messaggio: "sede non trovata"}

// NuoviGruppi: pool ha un database per gruppo e deve contenere il gruppo 0.
func NuoviGruppi(pool map[int]*pgxpool.Pool, mappa MappaSedi) (*Gruppi, error) {
	if pool[0] == nil {
		return nil, errors.New("manca il database delle spedizioni del gruppo 0")
	}
	g := &Gruppi{pool: pool, mappa: mappa}
	for c := range pool {
		if c < 0 || c > MassimoGruppo {
			return nil, fmt.Errorf("gruppo %d: deve essere tra 0 e %d", c, MassimoGruppo)
		}
		g.codici = append(g.codici, c)
	}
	sort.Ints(g.codici)
	return g, nil
}

// Principale è il database del gruppo 0: tiene i dati di chi non ha una sede
// (chiavi di idempotenza di amministratori e clienti) ed è la copia di riferimento dei
// motivi.
func (g *Gruppi) Principale() *pgxpool.Pool { return g.pool[0] }

// Codici sono i gruppi configurati, in ordine.
func (g *Gruppi) Codici() []int { return append([]int(nil), g.codici...) }

// Tutti sono i database di tutti i gruppi, nell'ordine dei codici. Se uno non risponde
// (o dichiara un altro gruppo) restituisce l'errore: chi legge tutti i gruppi ha bisogno di
// tutti.
func (g *Gruppi) Tutti(ctx context.Context) ([]*pgxpool.Pool, error) {
	tutti := make([]*pgxpool.Pool, len(g.codici))
	for i, c := range g.codici {
		p, _, err := g.Gruppo(ctx, c)
		if err != nil {
			return nil, err
		}
		tutti[i] = p
	}
	return tutti, nil
}

// Verificati sono i database dei gruppi già controllati (per le pulizie periodiche, che
// non devono toccare un database di cui non si è sicuri).
func (g *Gruppi) Verificati() []*pgxpool.Pool {
	var pronti []*pgxpool.Pool
	for _, c := range g.codici {
		if _, ok := g.verificati.Load(c); ok {
			pronti = append(pronti, g.pool[c])
		}
	}
	return pronti
}

// Gruppo è il database di un gruppo; ok è falso se il gruppo non è configurato. Al primo
// uso controlla che il database dichiari quel gruppo: se non risponde l'errore diventa
// 503 (solo per chi ha bisogno di quel gruppo), se dichiara un altro gruppo è un errore di
// configurazione (500, nel log).
func (g *Gruppi) Gruppo(ctx context.Context, codice int) (*pgxpool.Pool, bool, error) {
	p, ok := g.pool[codice]
	if !ok {
		return nil, false, nil
	}
	if _, fatto := g.verificati.Load(codice); fatto {
		return p, true, nil
	}
	if err := VerificaGruppo(ctx, p, codice); err != nil {
		return nil, true, err
	}
	g.verificati.Store(codice, true)
	return p, true, nil
}

// PerID è il database che contiene la riga con quell'id (spedizione, nota, ...); ok è
// falso se l'id è di un gruppo che qui non c'è: per chi chiede, la riga non esiste.
//
// Per chi ha una sede (operatori e corrieri) un id di un altro gruppo non esiste: non si
// apre nemmeno quel database, così ognuno lavora solo nel database della sua sede.
func (g *Gruppi) PerID(ctx context.Context, id int64) (*pgxpool.Pool, bool, error) {
	codice := GruppoDellID(id)
	if a := AmbitoDalContesto(ctx); !a.TutteLeSedi && a.IDSede > 0 {
		mio, err := g.GruppoDellaSede(ctx, a.IDSede)
		if err != nil {
			return nil, false, err
		}
		if mio != codice {
			return nil, false, nil
		}
	}
	return g.Gruppo(ctx, codice)
}

// Segna registra un gruppo già controllato (all'avvio, avvio.GruppiSpedizioni).
func (g *Gruppi) Segna(codice int) { g.verificati.Store(codice, true) }

// GruppoDellaSede: il gruppo della sede, chiesto al servizio utenti la prima volta.
// Una sede che non esiste dà 404.
func (g *Gruppi) GruppoDellaSede(ctx context.Context, idSede int64) (int, error) {
	if c, ok := g.sedi.Load(idSede); ok {
		return c.(int), nil
	}
	if idSede <= 0 {
		return 0, errSedeNonTrovata
	}
	if len(g.codici) == 1 && g.mappa == nil {
		return 0, nil
	}
	c, ok, err := g.mappa.GruppoDellaSede(ctx, idSede)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, errSedeNonTrovata
	}
	g.sedi.Store(idSede, c)
	return c, nil
}

// PerSede è il database del gruppo della sede.
func (g *Gruppi) PerSede(ctx context.Context, idSede int64) (*pgxpool.Pool, error) {
	c, err := g.GruppoDellaSede(ctx, idSede)
	if err != nil {
		return nil, err
	}
	p, ok, err := g.Gruppo(ctx, c)
	switch {
	case err != nil:
		return nil, err
	case !ok:
		// configurazione incompleta: la sede è in un gruppo senza database (500, nel log)
		return nil, fmt.Errorf("la sede %d è nel gruppo %d, che non ha un database: manca DATABASE_URL_%d", idSede, c, c)
	}
	return p, nil
}

// PerAmbito: il database della sede dell'ambito della richiesta (operatori e corrieri,
// amministratore con la sede scelta); il principale per chi non ha una sede.
func (g *Gruppi) PerAmbito(ctx context.Context) (*pgxpool.Pool, error) {
	if a := AmbitoDalContesto(ctx); !a.TutteLeSedi && a.IDSede > 0 {
		return g.PerSede(ctx, a.IDSede)
	}
	return g.Principale(), nil
}

// Ping: il database principale risponde. Gli altri gruppi non si controllano qui: se il
// database di un gruppo è giù rispondono 503 solo le sue sedi, mentre un /ready fallito
// toglierebbe dal traffico il servizio per tutte.
func (g *Gruppi) Ping(ctx context.Context) error { return g.Principale().Ping(ctx) }

// Chiudi chiude tutti i database.
func (g *Gruppi) Chiudi() {
	for _, p := range g.pool {
		p.Close()
	}
}

// ErrGruppoSbagliato: il database risponde ma non è quello del gruppo (URL sbagliato, o
// database non migrato). Non si risolve aspettando: è un errore di configurazione.
var ErrGruppoSbagliato = errors.New("database di un altro gruppo")

// VerificaGruppo controlla che il database dichiari davvero quel gruppo (tabella
// gruppo_database): un URL sbagliato metterebbe le spedizioni di un gruppo nel database di
// un altro, con id che si ripetono. Se il database non risponde restituisce l'errore della
// connessione (NonDisponibile: 503).
func VerificaGruppo(ctx context.Context, db *pgxpool.Pool, codice int) error {
	var dichiarato int
	err := db.QueryRow(ctx, `SELECT codice FROM gruppo_database`).Scan(&dichiarato)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return fmt.Errorf("%w: il database del gruppo %d non dichiara il suo gruppo (tabella gruppo_database vuota)", ErrGruppoSbagliato, codice)
	case err != nil && NonDisponibile(err):
		return err
	case err != nil:
		return fmt.Errorf("%w: gruppo %d, lettura di gruppo_database: %v (database da migrare con db/spedizioni/migrazione_gruppi.sql?)", ErrGruppoSbagliato, codice, err)
	case dichiarato != codice:
		return fmt.Errorf("%w: l'URL del gruppo %d porta al database del gruppo %d", ErrGruppoSbagliato, codice, dichiarato)
	}
	return nil
}
