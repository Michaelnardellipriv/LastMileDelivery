package main

import (
	"context"
	"crypto/rand"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"lastmiledelivery/backend/internal/autenticazione"
	"lastmiledelivery/backend/internal/database"
	"lastmiledelivery/backend/internal/server"
)

func main() {
	if err := esegui(); err != nil {
		log.Fatal("avvio fallito: ", err)
	}
}

func esegui() error {
	connessione := os.Getenv("DATABASE_URL")
	if connessione == "" {
		return errors.New("DATABASE_URL non impostata")
	}
	indirizzo := valore("LISTEN_ADDR", valore("HTTP_ADDR", "127.0.0.1:8080"))
	maxTentativi, err := strconv.Atoi(valore("MAX_TENTATIVI", "3"))
	if err != nil || maxTentativi < 1 {
		return errors.New("MAX_TENTATIVI deve essere un numero intero positivo")
	}
	giorniCliente, err := strconv.Atoi(valore("GIORNI_VISIBILITA_CLIENTE", "30"))
	if err != nil || giorniCliente < 1 {
		return errors.New("GIORNI_VISIBILITA_CLIENTE deve essere un numero intero positivo")
	}
	firmatario, err := firmatarioToken()
	if err != nil {
		return err
	}
	ammettiHeaderID := valore("IDENTITA_HEADER_SVILUPPO", "attiva") == "attiva"

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Collega(ctx, connessione)
	if err != nil {
		return err
	}
	defer db.Close()
	log.Println("collegamento a PostgreSQL riuscito")
	if ammettiHeaderID {
		log.Println("ATTENZIONE: accettati anche gli header X-Id-* senza login, solo per sviluppo (IDENTITA_HEADER_SVILUPPO=disattiva per spegnerli)")
	}

	cfg := server.Config{
		MaxTentativi:            maxTentativi,
		GiorniVisibilitaCliente: giorniCliente,
		Firmatario:              firmatario,
		Postino:                 postino(),
		LinkRecuperoPassword:    valore("LINK_RECUPERO_PASSWORD", "http://localhost:8080/reimposta-password"),
		AmmettiHeaderID:         ammettiHeaderID,
	}
	srv := &http.Server{
		Addr:              indirizzo,
		Handler:           server.Nuovo(db, cfg),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errServer := make(chan error, 1)
	go func() {
		log.Println("server avviato su", indirizzo)
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
	durata, err := time.ParseDuration(valore("DURATA_TOKEN", "8h"))
	if err != nil || durata <= 0 {
		return nil, errors.New("DURATA_TOKEN deve essere una durata positiva, es. 8h o 30m")
	}
	segreto := []byte(os.Getenv("JWT_SEGRETO"))
	switch {
	case len(segreto) == 0:
		segreto = make([]byte, 32)
		if _, err := rand.Read(segreto); err != nil {
			return nil, err
		}
		log.Println("ATTENZIONE: JWT_SEGRETO non impostata, uso un segreto casuale: i token non valgono dopo un riavvio né tra repliche diverse")
	case len(segreto) < 32:
		return nil, errors.New("JWT_SEGRETO deve essere lunga almeno 32 caratteri")
	}
	return autenticazione.NuovoFirmatario(segreto, durata), nil
}

func postino() autenticazione.Postino {
	host := os.Getenv("SMTP_HOST")
	if host == "" {
		log.Println("ATTENZIONE: SMTP_HOST non impostata, le mail di recupero password vengono scritte nel log")
		return autenticazione.PostinoLog{}
	}
	return autenticazione.PostinoSMTP{
		Host:     host,
		Porta:    valore("SMTP_PORT", "587"),
		Utente:   os.Getenv("SMTP_UTENTE"),
		Password: os.Getenv("SMTP_PASSWORD"),
		Mittente: valore("SMTP_MITTENTE", "noreply@veloce24.test"),
	}
}

func valore(chiave, predefinito string) string {
	if v := os.Getenv(chiave); v != "" {
		return v
	}
	return predefinito
}
