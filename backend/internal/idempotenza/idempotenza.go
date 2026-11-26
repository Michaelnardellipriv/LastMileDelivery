package idempotenza

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"lastmiledelivery/backend/internal/httpx"
	"lastmiledelivery/backend/internal/identita"
)

const (
	// per quanto tempo una risposta resta ripetibile con la stessa chiave
	durataChiave = 24 * time.Hour
	// una richiesta "in corso" più vecchia di così è stata abbandonata (replica caduta,
	// panic...): la chiave si può riusare. Il server chiude comunque le richieste dopo 15s.
	durataInCorso = time.Minute
	ogniPulizia   = 10 * time.Minute
)

// Esito della prenotazione di una chiave.
type Esito int

const (
	// Nuova: la chiave non era mai stata usata, la richiesta va eseguita
	Nuova Esito = iota
	// InCorso: un'altra richiesta con la stessa chiave non è ancora finita
	InCorso
	// CorpoDiverso: la chiave è già stata usata con dati diversi
	CorpoDiverso
	// Completata: la richiesta è già stata eseguita, c'è la sua risposta
	Completata
)

type Risposta struct {
	Stato  int
	Header http.Header
	Corpo  []byte
}

// archivio salva le chiavi nel database: così una richiesta ripetuta trova la risposta
// anche se il bilanciatore la manda a un'altra replica o dopo un riavvio.
type archivio interface {
	Prenota(ctx context.Context, chiave string, hashCorpo []byte, durata, durataInCorso time.Duration) (Esito, Risposta, error)
	Completa(ctx context.Context, chiave string, r Risposta) error
	Libera(ctx context.Context, chiave string) error
	Pulisci(ctx context.Context, durata time.Duration) (int64, error)
}

type Registro struct {
	archivio archivio
}

func NuovoRegistro(a archivio) *Registro {
	return &Registro{archivio: a}
}

// PuliziaPeriodica cancella le chiavi scadute finché ctx non viene chiuso. Ogni replica
// la esegue: la DELETE è la stessa, farla due volte non fa danni.
func (reg *Registro) PuliziaPeriodica(ctx context.Context) {
	t := time.NewTicker(ogniPulizia)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, err := reg.archivio.Pulisci(ctx, durataChiave); err != nil && ctx.Err() == nil {
				log.Println("pulizia delle chiavi di idempotenza fallita:", err)
			}
		}
	}
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
		id := string(rich.Ruolo) + ":" + strconv.FormatInt(rich.ID, 10) + ":" + r.Method + " " + r.URL.Path + ":" + chiave

		esito, salvata, err := reg.archivio.Prenota(r.Context(), id, hash[:], durataChiave, durataInCorso)
		if err != nil {
			httpx.RispondiErrore(w, r, err)
			return
		}
		switch esito {
		case CorpoDiverso:
			httpx.ScriviErrore(w, http.StatusConflict, "Idempotency-Key già usata per una richiesta con dati diversi")
			return
		case InCorso:
			httpx.ScriviErrore(w, http.StatusConflict, "una richiesta con questa Idempotency-Key è ancora in corso")
			return
		case Completata:
			for k, valori := range salvata.Header {
				w.Header()[k] = valori
			}
			w.Header().Set("Idempotent-Replayed", "true")
			w.WriteHeader(salvata.Stato)
			w.Write(salvata.Corpo)
			return
		}

		copia := &registratore{ResponseWriter: w, stato: http.StatusOK}
		next.ServeHTTP(copia, r)

		// anche se il client ha chiuso la connessione, l'esito va salvato
		ctx := context.WithoutCancel(r.Context())
		// Si ricorda solo una richiesta riuscita (2xx): una richiesta rifiutata (dati non
		// validi, conflitto, permessi...) non ha cambiato nulla, quindi chi corregge i dati
		// può rimandarla con la stessa chiave senza ricevere 409.
		if copia.stato < 200 || copia.stato >= 300 {
			if err := reg.archivio.Libera(ctx, id); err != nil {
				log.Printf("idempotenza: liberazione della chiave fallita: %v", err)
			}
			return
		}
		if err := reg.archivio.Completa(ctx, id, Risposta{Stato: copia.stato, Header: copia.Header().Clone(), Corpo: copia.corpo.Bytes()}); err != nil {
			log.Printf("idempotenza: salvataggio della risposta fallito: %v", err)
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
