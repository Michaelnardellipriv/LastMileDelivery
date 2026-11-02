package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Collega(ctx, connessione)
	if err != nil {
		return err
	}
	defer db.Close()
	log.Println("collegamento a PostgreSQL riuscito")
	log.Println("ATTENZIONE: utente letto dagli header X-Id-*, solo per sviluppo")

	srv := &http.Server{
		Addr:              indirizzo,
		Handler:           server.Nuovo(db, server.Config{MaxTentativi: maxTentativi, GiorniVisibilitaCliente: giorniCliente}),
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

func valore(chiave, predefinito string) string {
	if v := os.Getenv(chiave); v != "" {
		return v
	}
	return predefinito
}
