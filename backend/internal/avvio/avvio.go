// Package avvio raccoglie ciò che serve ad avviare i servizi (cmd/utenti,
// cmd/spedizioni e cmd/server): lettura della configurazione dalle variabili
// d'ambiente, collegamento al database e spegnimento ordinato.
package avvio

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"lastmiledelivery/backend/internal/autenticazione"
	"lastmiledelivery/backend/internal/database"
	"lastmiledelivery/backend/internal/httpx"
	"lastmiledelivery/backend/internal/server"
)

func Valore(chiave, predefinito string) string {
	if v := os.Getenv(chiave); v != "" {
		return v
	}
	return predefinito
}

// Obbligatoria legge una variabile che deve esserci.
func Obbligatoria(chiave string) (string, error) {
	v := os.Getenv(chiave)
	if v == "" {
		return "", fmt.Errorf("%s non impostata", chiave)
	}
	return v, nil
}

// Contesto si chiude con Ctrl+C o con il SIGTERM che manda Kubernetes.
func Contesto() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

// Config legge la configurazione comune ai due servizi. Con casuale=true (cmd/server,
// sviluppo) una CHIAVE_INTERNA mancante si sostituisce con una casuale, perché le
// chiamate interne restano nel processo.
func Config(ctx context.Context, chiaveCasuale bool) (server.Config, error) {
	cors, err := httpx.NuovoCORS(os.Getenv("CORS_ORIGIN"))
	if err != nil {
		return server.Config{}, err
	}
	for _, vecchia := range []string{"MAX_TENTATIVI", "GIORNI_VISIBILITA_CLIENTE"} {
		if os.Getenv(vecchia) != "" {
			log.Printf("ATTENZIONE: %s non si usa più: il valore lo imposta l'amministratore per ogni sede con PATCH /api/sedi/{id}/impostazioni", vecchia)
		}
	}
	firmatario, err := firmatarioToken()
	if err != nil {
		return server.Config{}, err
	}
	chiave, err := chiaveInterna(chiaveCasuale)
	if err != nil {
		return server.Config{}, err
	}
	ammettiHeaderID := Valore("IDENTITA_HEADER_SVILUPPO", "attiva") == "attiva"
	if ammettiHeaderID {
		log.Println("ATTENZIONE: accettati anche gli header X-Id-* senza login, solo per sviluppo (IDENTITA_HEADER_SVILUPPO=disattiva per spegnerli)")
	}
	return server.Config{
		Firmatario: firmatario,
		Postino:    postino(ammettiHeaderID),
		// pagina del FRONTEND a cui punta il link della mail: il servizio aggiunge "?token=<token>",
		// che l'app legge per chiedere la nuova password (auth/recupero_link.dart)
		LinkRecuperoPassword: Valore("LINK_RECUPERO_PASSWORD", "http://localhost:5173/"),
		AmmettiHeaderID:      ammettiHeaderID,
		CORS:                 cors,
		ChiaveInterna:        chiave,
		Contesto:             ctx,
	}, nil
}

// Database si collega al database indicato dalla variabile (es. DATABASE_URL).
func Database(ctx context.Context, variabile string) (*pgxpool.Pool, error) {
	connessione, err := Obbligatoria(variabile)
	if err != nil {
		return nil, err
	}
	db, err := database.Collega(ctx, connessione)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", variabile, err)
	}
	log.Printf("collegamento a PostgreSQL riuscito (%s)", variabile)
	return db, nil
}

// GruppiSpedizioni si collega ai database delle spedizioni, uno per gruppo di sedi (mappa
// sede → database): variabile è il gruppo 0, variabile_<n> il gruppo n (es.
// DATABASE_URL_1). Senza variabili _<n> c'è il solo gruppo 0, come prima.
//
// Il gruppo 0 deve rispondere e dichiarare il gruppo giusto (tabella gruppo_database),
// altrimenti il servizio non parte. Un altro gruppo che non risponde non ferma l'avvio:
// verificati dice quali gruppi sono già stati controllati, gli altri si controllano al
// primo uso (database.Gruppi) e intanto le loro sedi ricevono 503. Un database che risponde
// ma dichiara un altro gruppo ferma l'avvio: è un errore di configurazione.
func GruppiSpedizioni(ctx context.Context, variabile string) (pool map[int]*pgxpool.Pool, verificati []int, err error) {
	nomi := map[int]string{0: variabile}
	for _, voce := range os.Environ() {
		nome, valore, _ := strings.Cut(voce, "=")
		resto, ok := strings.CutPrefix(nome, variabile+"_")
		if !ok || strings.TrimSpace(valore) == "" {
			continue
		}
		codice, err := strconv.Atoi(resto)
		if err != nil || codice < 1 || codice > database.MassimoGruppo || strconv.Itoa(codice) != resto {
			return nil, nil, fmt.Errorf("%s: il gruppo dopo %s_ deve essere un numero tra 1 e %d", nome, variabile, database.MassimoGruppo)
		}
		nomi[codice] = nome
	}
	pool = map[int]*pgxpool.Pool{}
	chiudi := func() {
		for _, p := range pool {
			p.Close()
		}
	}
	for codice, nome := range nomi {
		var db *pgxpool.Pool
		if codice == 0 {
			db, err = Database(ctx, nome)
		} else {
			db, err = database.Prepara(ctx, Valore(nome, ""))
		}
		if err != nil {
			chiudi()
			return nil, nil, fmt.Errorf("%s: %w", nome, err)
		}
		pool[codice] = db
		controllo, annulla := context.WithTimeout(ctx, 5*time.Second)
		err = database.VerificaGruppo(controllo, db, codice)
		annulla()
		switch {
		case err == nil:
			verificati = append(verificati, codice)
		case codice != 0 && !errors.Is(err, database.ErrGruppoSbagliato):
			log.Printf("ATTENZIONE: %s non risponde (%v): le sedi del gruppo %d avranno 503 finché non torna", nome, err, codice)
		default:
			chiudi()
			return nil, nil, fmt.Errorf("%s: %w", nome, err)
		}
	}
	if len(pool) > 1 {
		log.Printf("database delle spedizioni: %d gruppi (%d già controllati)", len(pool), len(verificati))
	}
	return pool, verificati, nil
}

// Servi avvia il server HTTP e lo spegne in ordine quando ctx si chiude.
func Servi(ctx context.Context, nome string, handler http.Handler) error {
	indirizzo := Valore("LISTEN_ADDR", Valore("HTTP_ADDR", "127.0.0.1:8080"))
	srv := &http.Server{
		Addr:              indirizzo,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	errServer := make(chan error, 1)
	go func() {
		log.Printf("servizio %s avviato su %s", nome, indirizzo)
		errServer <- srv.ListenAndServe()
	}()
	select {
	case err := <-errServer:
		return err
	case <-ctx.Done():
		log.Println("arresto in corso")
		spegni, annulla := context.WithTimeout(context.Background(), 10*time.Second)
		defer annulla()
		return srv.Shutdown(spegni)
	}
}

func firmatarioToken() (*autenticazione.Firmatario, error) {
	// breve: limita quanto vale un token copiato. La revoca (logout, disattivazione, cambio
	// di sede, password nuova) è immediata comunque, perché ogni richiesta controlla la
	// sessione nel database. L'app lo rinnova da sola (POST /api/auth/rinnova) finché la
	// sessione è aperta.
	durata, err := time.ParseDuration(Valore("DURATA_TOKEN", "15m"))
	if err != nil || durata < time.Minute {
		return nil, errors.New("DURATA_TOKEN deve essere una durata di almeno un minuto, es. 15m")
	}
	segreto := []byte(os.Getenv("JWT_SEGRETO"))
	switch {
	case len(segreto) == 0:
		segreto = make([]byte, 32)
		if _, err := rand.Read(segreto); err != nil {
			return nil, err
		}
		log.Println("ATTENZIONE: JWT_SEGRETO non impostata, uso un segreto casuale: i token non valgono dopo un riavvio, " +
			"né tra repliche o servizi diversi")
	case len(segreto) < 32:
		return nil, errors.New("JWT_SEGRETO deve essere lunga almeno 32 caratteri")
	}
	sessione, err := time.ParseDuration(Valore("DURATA_SESSIONE", "12h"))
	if err != nil || sessione < durata {
		return nil, errors.New("DURATA_SESSIONE deve essere una durata non minore di DURATA_TOKEN, es. 12h")
	}
	return autenticazione.NuovoFirmatario(segreto, durata).ConDurataSessione(sessione), nil
}

// chiaveInterna: la stessa per i due servizi (CHIAVE_INTERNA), almeno 32 caratteri.
func chiaveInterna(casuale bool) (string, error) {
	chiave := os.Getenv("CHIAVE_INTERNA")
	switch {
	case chiave == "" && casuale:
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		return hex.EncodeToString(b), nil
	case chiave == "":
		return "", errors.New("CHIAVE_INTERNA non impostata: serve ai due servizi per chiamarsi (la stessa per tutti e due)")
	case len(chiave) < 32:
		return "", errors.New("CHIAVE_INTERNA deve essere lunga almeno 32 caratteri")
	}
	return chiave, nil
}

func postino(sviluppo bool) autenticazione.Postino {
	host := os.Getenv("SMTP_HOST")
	if host == "" {
		if sviluppo {
			log.Println("ATTENZIONE: SMTP_HOST non impostata, le mail (con password e link di recupero) vengono scritte nel log: solo sviluppo")
		} else {
			log.Println("ATTENZIONE: SMTP_HOST non impostata, le mail non partono (e non finiscono nel log)")
		}
		return autenticazione.PostinoLog{MostraTesto: sviluppo}
	}
	return autenticazione.PostinoSMTP{
		Host:     host,
		Porta:    Valore("SMTP_PORT", "587"),
		Utente:   os.Getenv("SMTP_UTENTE"),
		Password: os.Getenv("SMTP_PASSWORD"),
		Mittente: Valore("SMTP_MITTENTE", "noreply@veloce24.test"),
	}
}
