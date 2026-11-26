// Package server mette insieme i due microservizi:
//
//   - utenti (NuovoUtenti, database globale): login, utenti, clienti, sedi (con il loro
//     indirizzo e la geografia);
//   - spedizioni (NuovoSpedizioni, database delle spedizioni, uno per gruppo di sedi):
//     spedizioni, indirizzi di ritiro e consegna, note, storico, impostazioni delle sedi,
//     motivi, posizione dei corrieri.
//
// In Kubernetes ognuno è un Deployment con la sua immagine e il suo database, e
// l'Ingress manda ogni percorso /api al servizio giusto (vedi PerSpedizioni). In
// sviluppo NuovoTutto li fa girare nello stesso processo, con lo stesso smistamento.
package server

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"

	"lastmiledelivery/backend/internal/anagrafiche"
	"lastmiledelivery/backend/internal/autenticazione"
	"lastmiledelivery/backend/internal/configurazione"
	"lastmiledelivery/backend/internal/database"
	"lastmiledelivery/backend/internal/httpx"
	"lastmiledelivery/backend/internal/idempotenza"
	"lastmiledelivery/backend/internal/identita"
	"lastmiledelivery/backend/internal/interno"
	"lastmiledelivery/backend/internal/posizioni"
	"lastmiledelivery/backend/internal/sessioni"
	"lastmiledelivery/backend/internal/spedizioni"
)

type Config struct {
	Firmatario           *autenticazione.Firmatario
	Postino              autenticazione.Postino
	LinkRecuperoPassword string
	AmmettiHeaderID      bool
	CORS                 *httpx.CORS
	// ChiaveInterna protegge le rotte /interno tra i due servizi (CHIAVE_INTERNA).
	ChiaveInterna string
	// Contesto del processo: quando si chiude si ferma la pulizia periodica delle chiavi
	// di idempotenza. Se è nil la pulizia non parte (es. nei test).
	Contesto context.Context
}

// idempotenza: le Idempotency-Key stanno nel database del servizio, quindi valgono per
// tutte le sue repliche.
func idempotente(repo *idempotenza.Repository, cfg Config) httpx.Middleware {
	registro := idempotenza.NuovoRegistro(repo)
	if cfg.Contesto != nil {
		go registro.PuliziaPeriodica(cfg.Contesto)
	}
	return registro.Middleware
}

func conCORS(h http.Handler, cfg Config) http.Handler {
	if cfg.CORS == nil {
		return h
	}
	return cfg.CORS.Middleware(h)
}

// NuovoUtenti è il microservizio con il database globale: fa il login (emette i token)
// e gestisce utenti, clienti, sedi e indirizzi. spedizioni è il client verso l'altro
// servizio (serve per spostare un corriere di sede).
func NuovoUtenti(db *pgxpool.Pool, cfg Config, spedizioni *interno.Client) http.Handler {
	// ogni token vale solo se la sua sessione è aperta nel database globale; chiuderla
	// (logout, disattivazione, cambio di sede, nuova password) lo invalida subito, anche
	// sul servizio spedizioni, a cui le sessioni chiuse si comunicano prima del commit
	utenti := identita.NuovoRepository(db)
	registro := sessioni.Nuovo(db, spedizioni, cfg.Firmatario.DurataSessione())
	richiediUtente := identita.Richiedi(utenti, utenti, cfg.Firmatario, cfg.AmmettiHeaderID)
	idem := idempotente(idempotenza.NuovoRepository(db), cfg)

	rotte := httpx.NuoveRotte()
	registraSalute(rotte, db)

	servizioAutenticazione := autenticazione.NuovoServizio(autenticazione.NuovoRepository(db, registro), registro,
		cfg.Firmatario, cfg.Postino, cfg.LinkRecuperoPassword)
	autenticazione.NuovoHandler(servizioAutenticazione).Registra(rotte, richiediUtente)

	repo := anagrafiche.NuovoRepository(db, spedizioni, registro)
	anagrafiche.NuovoHandler(anagrafiche.NuovoServizio(repo, cfg.Postino)).Registra(rotte, richiediUtente, idem)
	anagrafiche.NuovoHandlerInterno(repo).Registra(rotte, interno.Protetto(cfg.ChiaveInterna))

	return conCORS(rotte.Handler(), cfg)
}

// NuovoSpedizioni è il microservizio con i database delle spedizioni, uno per gruppo di
// sedi (mappa sede → database, database.Gruppi). Non emette token: verifica quelli del
// servizio utenti (stesso JWT_SEGRETO) e chiede sedi, persone e indirizzi al servizio
// utenti (anagrafe).
func NuovoSpedizioni(gruppi *database.Gruppi, cfg Config, anagrafe *interno.Client) http.Handler {
	richiediUtente := identita.Richiedi(identita.NuovoControlloRevoche(gruppi), anagrafe, cfg.Firmatario, cfg.AmmettiHeaderID)
	idem := idempotente(idempotenza.NuovoRepositoryPerGruppi(gruppi), cfg)

	rotte := httpx.NuoveRotte()
	// /ready guarda il database principale (database.Gruppi.Ping)
	registraSalute(rotte, gruppi)

	// tentativi, giorni di visibilità e motivi li sceglie l'amministratore (tabelle impostazioni e motivi)
	repositoryConfigurazione := configurazione.NuovoRepository(gruppi, anagrafe)
	if cfg.Contesto != nil {
		// i motivi si cambiano nel database principale e si copiano negli altri gruppi
		go repositoryConfigurazione.SincronizzazionePeriodica(cfg.Contesto)
	}
	configurazione.NuovoHandler(configurazione.NuovoServizio(repositoryConfigurazione)).Registra(rotte, richiediUtente, idem)

	repo := spedizioni.NuovoRepository(gruppi, anagrafe)
	spedizioni.NuovoHandler(spedizioni.NuovoServizio(repo, repositoryConfigurazione)).Registra(rotte, richiediUtente, idem)
	spedizioni.NuovoHandlerInterno(repo).Registra(rotte, interno.Protetto(cfg.ChiaveInterna))

	posizioni.NuovoHandler(posizioni.NuovoServizio(posizioni.NuovoRepository(gruppi, anagrafe))).Registra(rotte, richiediUtente)

	// indirizzi di ritiro e consegna: nel database del gruppo della sede
	spedizioni.NuovoHandlerIndirizzi(spedizioni.NuovoServizioIndirizzi(repo)).Registra(rotte, richiediUtente, idem)

	return conCORS(rotte.Handler(), cfg)
}

// NuovoTutto fa girare i due servizi nello stesso processo (sviluppo e test di
// integrazione): ognuno con i suoi database (dbSpedizioni: uno per gruppo, il gruppo 0
// obbligatorio), le chiamate /interno passano in memoria invece che in rete e i percorsi
// /api si smistano come fa l'Ingress.
func NuovoTutto(dbGlobale *pgxpool.Pool, dbSpedizioni map[int]*pgxpool.Pool, cfg Config) http.Handler {
	var utenti, sped rinvio
	verso := func(h *rinvio) http.RoundTripper { return interno.TrasportoLocale(h) }
	clientSpedizioni := interno.NuovoClient("spedizioni", "http://spedizioni", cfg.ChiaveInterna, verso(&sped))
	clientUtenti := interno.NuovoClient("utenti", "http://utenti", cfg.ChiaveInterna, verso(&utenti))
	gruppi, err := database.NuoviGruppi(dbSpedizioni, clientUtenti)
	if err != nil {
		panic(err) // chi chiama (avvio.GruppiSpedizioni) garantisce il gruppo 0
	}
	utenti.imposta(NuovoUtenti(dbGlobale, cfg, clientSpedizioni))
	sped.imposta(NuovoSpedizioni(gruppi, cfg, clientUtenti))
	return Instrada(&utenti, &sped)
}

// rinvio è un handler che si imposta dopo: i due servizi si chiamano a vicenda, quindi
// il client di uno si crea prima che esista l'handler dell'altro.
type rinvio struct {
	once sync.Once
	h    http.Handler
}

func (r *rinvio) imposta(h http.Handler) { r.once.Do(func() { r.h = h }) }

func (r *rinvio) ServeHTTP(w http.ResponseWriter, req *http.Request) { r.h.ServeHTTP(w, req) }

var impostazioniSede = regexp.MustCompile(`^/api/sedi/[^/]+/impostazioni$`)

// PerSpedizioni dice se un percorso è del servizio spedizioni: sono le stesse regole
// dell'Ingress (k8s/base/ingress.yaml). Tutto il resto va al servizio utenti.
func PerSpedizioni(percorso string) bool {
	for _, prefisso := range []string{"/api/spedizioni", "/api/corrieri/posizion", "/api/impostazioni", "/api/motivi", "/api/indirizzi"} {
		if strings.HasPrefix(percorso, prefisso) {
			return true
		}
	}
	return impostazioniSede.MatchString(percorso)
}

// Instrada smista le richieste tra i due servizi come l'Ingress.
func Instrada(utenti, spedizioni http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if PerSpedizioni(r.URL.Path) {
			spedizioni.ServeHTTP(w, r)
			return
		}
		utenti.ServeHTTP(w, r)
	})
}
