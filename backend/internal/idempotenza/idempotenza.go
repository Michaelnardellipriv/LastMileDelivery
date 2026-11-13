package idempotenza

import (
	"bytes"
	"crypto/sha256"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"lastmiledelivery/backend/internal/httpx"
	"lastmiledelivery/backend/internal/identita"
)

const durataChiave = 24 * time.Hour

type Registro struct {
	mu   sync.Mutex
	voci map[string]*voce
}

type voce struct {
	hashCorpo [32]byte
	inCorso   bool
	creata    time.Time
	stato     int
	header    http.Header
	corpo     []byte
}

func NuovoRegistro() *Registro {
	reg := &Registro{voci: map[string]*voce{}}
	go func() {
		for range time.Tick(10 * time.Minute) {
			reg.mu.Lock()
			for k, v := range reg.voci {
				if !v.inCorso && time.Since(v.creata) > durataChiave {
					delete(reg.voci, k)
				}
			}
			reg.mu.Unlock()
		}
	}()
	return reg
}

func (reg *Registro) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chiave := r.Header.Get("Idempotency-Key")
		if chiave == "" {
			next.ServeHTTP(w, r)
			return
		}
		if !chiaveValida(chiave) {
			httpx.ScriviErrore(w, http.StatusBadRequest, "Idempotency-Key non valida: da 1 a 255 caratteri visibili")
			return
		}

		corpo, err := io.ReadAll(http.MaxBytesReader(w, r.Body, httpx.MaxCorpoRichiesta))
		if err != nil {
			httpx.ScriviErrore(w, http.StatusBadRequest, "corpo della richiesta troppo grande o illeggibile")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(corpo))
		hash := sha256.Sum256(corpo)
		rich := identita.DalContesto(r.Context())
		id := string(rich.Ruolo) + ":" + strconv.Itoa(int(rich.ID)) + ":" + r.Method + " " + r.URL.Path + ":" + chiave

		reg.mu.Lock()
		v, esiste := reg.voci[id]
		if esiste && !v.inCorso && time.Since(v.creata) > durataChiave {
			esiste = false
		}
		switch {
		case esiste && v.hashCorpo != hash:
			reg.mu.Unlock()
			httpx.ScriviErrore(w, http.StatusConflict, "Idempotency-Key già usata per una richiesta con dati diversi")
			return
		case esiste && v.inCorso:
			reg.mu.Unlock()
			httpx.ScriviErrore(w, http.StatusConflict, "una richiesta con questa Idempotency-Key è ancora in corso")
			return
		case esiste:
			reg.mu.Unlock()
			for k, valori := range v.header {
				w.Header()[k] = valori
			}
			w.Header().Set("Idempotent-Replayed", "true")
			w.WriteHeader(v.stato)
			w.Write(v.corpo)
			return
		}
		reg.voci[id] = &voce{hashCorpo: hash, inCorso: true, creata: time.Now()}
		reg.mu.Unlock()

		copia := &registratore{ResponseWriter: w, stato: http.StatusOK}
		next.ServeHTTP(copia, r)

		reg.mu.Lock()
		defer reg.mu.Unlock()
		// Si ricorda solo una richiesta riuscita (2xx): una richiesta rifiutata (dati non
		// validi, conflitto, permessi...) non ha cambiato nulla, quindi chi corregge i dati
		// può rimandarla con la stessa chiave senza ricevere 409.
		if copia.stato < 200 || copia.stato >= 300 {
			delete(reg.voci, id)
			return
		}
		reg.voci[id] = &voce{
			hashCorpo: hash, creata: time.Now(),
			stato: copia.stato, header: copia.Header().Clone(), corpo: copia.corpo.Bytes(),
		}
	})
}

func chiaveValida(k string) bool {
	if len(k) > 255 {
		return false
	}
	for i := 0; i < len(k); i++ {
		if k[i] < 0x21 || k[i] > 0x7e {
			return false
		}
	}
	return true
}

type registratore struct {
	http.ResponseWriter
	stato int
	corpo bytes.Buffer
}

func (r *registratore) WriteHeader(stato int) {
	r.stato = stato
	r.ResponseWriter.WriteHeader(stato)
}

func (r *registratore) Write(b []byte) (int, error) {
	r.corpo.Write(b)
	return r.ResponseWriter.Write(b)
}
