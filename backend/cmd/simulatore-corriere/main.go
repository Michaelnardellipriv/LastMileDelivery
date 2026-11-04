// Simulatore del dispositivo del corriere.
//
// Fa finta di essere il telefono del corriere: parte da una posizione,
// si sposta di poco a ogni giro e invia le coordinate al backend
// all'intervallo deciso dall'amministratore.
//
// Uso (dalla cartella backend/):
//
//	go run ./cmd/simulatore-corriere -id-corriere 1
//	go run ./cmd/simulatore-corriere -id-corriere 1 -url http://localhost:8080 -lat 46.0679 -lon 11.1211
//
// Si ferma con Ctrl+C.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"
)

// Spostamento massimo a ogni invio, in gradi: 0.0005° ≈ 50 metri.
const passoMassimoGradi = 0.0005

type simulatore struct {
	url        string
	idCorriere int
	client     *http.Client
}

func main() {
	url := flag.String("url", "http://localhost:8080", "indirizzo del backend")
	idCorriere := flag.Int("id-corriere", 0, "id del corriere (tabella corrieri)")
	lat := flag.Float64("lat", 46.0679, "latitudine di partenza (default: Trento)")
	lon := flag.Float64("lon", 11.1211, "longitudine di partenza (default: Trento)")
	flag.Parse()

	if *idCorriere <= 0 {
		fmt.Fprintln(os.Stderr, "serve -id-corriere con un id positivo")
		flag.Usage()
		os.Exit(2)
	}

	s := simulatore{
		url:        strings.TrimRight(*url, "/"),
		idCorriere: *idCorriere,
		client:     &http.Client{Timeout: 10 * time.Second},
	}

	// Ctrl+C ferma il simulatore in modo pulito.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	intervallo, err := s.leggiIntervallo(ctx)
	if err != nil {
		log.Fatal("lettura intervallo: ", err)
	}
	log.Printf("corriere %d: invio la posizione ogni %d secondi", s.idCorriere, intervallo)

	for {
		*lat, *lon = spostati(*lat, *lon)

		nuovoIntervallo, err := s.inviaPosizione(ctx, *lat, *lon)
		if err != nil {
			log.Println(err)
		}
		if nuovoIntervallo > 0 && nuovoIntervallo != intervallo {
			log.Printf("l'amministratore ha cambiato l'intervallo: %d → %d secondi", intervallo, nuovoIntervallo)
			intervallo = nuovoIntervallo
		}

		select {
		case <-ctx.Done():
			log.Println("simulatore fermato")
			return
		case <-time.After(time.Duration(intervallo) * time.Second):
		}
	}
}

// spostati muove il corriere di un piccolo passo casuale.
func spostati(lat, lon float64) (float64, float64) {
	lat += (rand.Float64()*2 - 1) * passoMassimoGradi
	lon += (rand.Float64()*2 - 1) * passoMassimoGradi
	return lat, lon
}

func (s simulatore) leggiIntervallo(ctx context.Context) (int, error) {
	stato, corpo, _, err := s.richiesta(ctx, http.MethodGet, "/api/impostazioni/intervallo-posizione", nil)
	if err != nil {
		return 0, err
	}
	if stato != http.StatusOK {
		return 0, fmt.Errorf("HTTP %d: %s", stato, strings.TrimSpace(string(corpo)))
	}

	var r struct {
		IntervalloSecondi int `json:"intervallo_secondi"`
	}
	if err := json.Unmarshal(corpo, &r); err != nil {
		return 0, err
	}
	return r.IntervalloSecondi, nil
}

// inviaPosizione restituisce l'intervallo comunicato dal server (0 se non c'è).
func (s simulatore) inviaPosizione(ctx context.Context, lat, lon float64) (int, error) {
	dati, _ := json.Marshal(map[string]float64{"latitudine": lat, "longitudine": lon})

	stato, corpo, intestazioni, err := s.richiesta(ctx, http.MethodPost, "/api/corrieri/posizione", dati)
	if err != nil {
		return 0, err
	}
	messaggio := strings.TrimSpace(string(corpo))

	switch stato {
	case http.StatusCreated:
		var r struct {
			IntervalloSecondi int `json:"intervallo_secondi"`
		}
		_ = json.Unmarshal(corpo, &r)
		log.Printf("posizione inviata: %.6f, %.6f", lat, lon)
		return r.IntervalloSecondi, nil

	case http.StatusConflict:
		// Nessuna spedizione in consegna: il simulatore resta acceso
		// e riprova, così riparte da solo quando una spedizione passa in consegna.
		return 0, fmt.Errorf("non inviata (%s), riprovo al prossimo giro", messaggio)

	case http.StatusTooManyRequests:
		secondi, _ := strconv.Atoi(intestazioni.Get("Retry-After"))
		return secondi, fmt.Errorf("non inviata: troppo presto")

	case http.StatusUnauthorized, http.StatusForbidden:
		log.Fatalf("HTTP %d: %s", stato, messaggio)
	}

	return 0, fmt.Errorf("HTTP %d: %s", stato, messaggio)
}

func (s simulatore) richiesta(ctx context.Context, metodo, percorso string, corpo []byte) (int, []byte, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, metodo, s.url+percorso, bytes.NewReader(corpo))
	if err != nil {
		return 0, nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Id-Corriere", strconv.Itoa(s.idCorriere))

	risposta, err := s.client.Do(req)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("backend non raggiungibile: %w", err)
	}
	defer risposta.Body.Close()

	dati, err := io.ReadAll(risposta.Body)
	return risposta.StatusCode, dati, risposta.Header, err
}
