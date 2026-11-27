package idempotenza

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"lastmiledelivery/backend/internal/identita"
)

// archivioFinto simula la tabella chiavi_idempotenza. Più middleware che condividono lo
// stesso archivio sono come più repliche del backend con lo stesso database.
type archivioFinto struct {
	mu     sync.Mutex
	voci   map[string]*voceFinta
	errore error
}

type voceFinta struct {
	hash     []byte
	risposta *Risposta
}

func nuovoArchivio() *archivioFinto { return &archivioFinto{voci: map[string]*voceFinta{}} }

func (a *archivioFinto) Prenota(_ context.Context, chiave string, hash []byte, _, _ time.Duration) (Esito, Risposta, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.errore != nil {
		return 0, Risposta{}, a.errore
	}
	v, ok := a.voci[chiave]
	switch {
	case !ok:
		a.voci[chiave] = &voceFinta{hash: hash}
		return Nuova, Risposta{}, nil
	case !bytes.Equal(v.hash, hash):
		return CorpoDiverso, Risposta{}, nil
	case v.risposta == nil:
		return InCorso, Risposta{}, nil
	}
	return Completata, *v.risposta, nil
}

func (a *archivioFinto) Completa(_ context.Context, chiave string, r Risposta) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.voci[chiave].risposta = &r
	return nil
}

func (a *archivioFinto) Libera(_ context.Context, chiave string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.voci, chiave)
	return nil
}

func (a *archivioFinto) Pulisci(context.Context, time.Duration) (int64, error) { return 0, nil }

// gestore conta quante volte la richiesta viene davvero eseguita.
type gestore struct {
	eseguite int
	stato    int
}

func (g *gestore) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	g.eseguite++
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(g.stato)
	w.Write([]byte(`{"id":1}`))
}

func richiesta(chiave, corpo string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/spedizioni", strings.NewReader(corpo))
	r.Header.Set("Idempotency-Key", chiave)
	return r.WithContext(identita.NelContesto(r.Context(), identita.Richiedente{Ruolo: identita.Operatore, ID: 1, IDSede: 10}))
}

func esegui(h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestRichiestaRipetutaSuUnAltraReplicaNonSiRipete(t *testing.T) {
	db := nuovoArchivio()
	g := &gestore{stato: http.StatusCreated}
	replicaA := NuovoRegistro(db).Middleware(g)
	replicaB := NuovoRegistro(db).Middleware(g)

	prima := esegui(replicaA, richiesta("chiave-1", `{"cod":"A"}`))
	seconda := esegui(replicaB, richiesta("chiave-1", `{"cod":"A"}`))

	if g.eseguite != 1 {
		t.Fatalf("la spedizione va creata una volta sola, eseguite %d", g.eseguite)
	}
	if seconda.Code != http.StatusCreated || seconda.Body.String() != prima.Body.String() ||
		seconda.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("la seconda replica restituisce la risposta salvata: %d %q %v", seconda.Code, seconda.Body.String(), seconda.Header())
	}
}

func TestStessaChiaveConCorpoDiversoRestituisce409(t *testing.T) {
	db := nuovoArchivio()
	h := NuovoRegistro(db).Middleware(&gestore{stato: http.StatusCreated})
	esegui(h, richiesta("chiave-2", `{"cod":"A"}`))
	if w := esegui(h, richiesta("chiave-2", `{"cod":"B"}`)); w.Code != http.StatusConflict {
		t.Fatalf("atteso 409, ottenuto %d", w.Code)
	}
}

func TestRichiestaInCorsoRestituisce409(t *testing.T) {
	db := nuovoArchivio()
	db.voci["operatore:1:POST /api/spedizioni:chiave-3"] = &voceFinta{hash: hashDi(`{}`)}
	h := NuovoRegistro(db).Middleware(&gestore{stato: http.StatusCreated})
	if w := esegui(h, richiesta("chiave-3", `{}`)); w.Code != http.StatusConflict {
		t.Fatalf("atteso 409 mentre l'altra richiesta è in corso, ottenuto %d", w.Code)
	}
}

func TestRichiestaRifiutataSiPuoRimandare(t *testing.T) {
	db := nuovoArchivio()
	g := &gestore{stato: http.StatusBadRequest}
	h := NuovoRegistro(db).Middleware(g)
	esegui(h, richiesta("chiave-4", `{}`))
	if len(db.voci) != 0 {
		t.Fatal("una richiesta rifiutata libera la chiave")
	}
	g.stato = http.StatusCreated
	if w := esegui(h, richiesta("chiave-4", `{}`)); w.Code != http.StatusCreated || g.eseguite != 2 {
		t.Fatalf("corretti i dati, la stessa chiave si riusa: %d %d", w.Code, g.eseguite)
	}
}

func TestDatabaseGiuRestituisce500SenzaEseguire(t *testing.T) {
	db := nuovoArchivio()
	db.errore = errors.New("connessione persa")
	g := &gestore{stato: http.StatusCreated}
	if w := esegui(NuovoRegistro(db).Middleware(g), richiesta("chiave-5", `{}`)); w.Code != http.StatusInternalServerError || g.eseguite != 0 {
		t.Fatalf("senza database la richiesta non si esegue: %d %d", w.Code, g.eseguite)
	}
}

func TestSenzaChiaveNessunControllo(t *testing.T) {
	g := &gestore{stato: http.StatusCreated}
	h := NuovoRegistro(nuovoArchivio()).Middleware(g)
	r := richiesta("", `{}`)
	r.Header.Del("Idempotency-Key")
	esegui(h, r)
	esegui(h, r)
	if g.eseguite != 2 {
		t.Fatalf("senza Idempotency-Key ogni richiesta si esegue: %d", g.eseguite)
	}
}

func TestChiaveNonValida(t *testing.T) {
	h := NuovoRegistro(nuovoArchivio()).Middleware(&gestore{stato: http.StatusCreated})
	if w := esegui(h, richiesta(strings.Repeat("x", 256), `{}`)); w.Code != http.StatusBadRequest {
		t.Fatalf("chiave troppo lunga: atteso 400, ottenuto %d", w.Code)
	}
}

func hashDi(corpo string) []byte {
	h := sha256.Sum256([]byte(corpo))
	return h[:]
}
