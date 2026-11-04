//go:build integration

package apitest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Test di integrazione della posizione dei corrieri: il backend deve essere
// avviato. Con i dati di db/seed_posizione_corriere.sql:
//
//	TEST_ID_CORRIERE            corriere con una spedizione "in consegna"
//	TEST_ID_CORRIERE_INATTIVO   corriere senza spedizioni in consegna
//	                            (nel seed: ha una spedizione solo "ritirata")
//	TEST_ID_OPERATORE           operatore logistico
//	TEST_ID_AMMINISTRATORE      amministratore
func TestPosizioneCorriere(t *testing.T) {
	if strings.TrimSpace(os.Getenv("TEST_ID_CORRIERE")) == "" {
		t.Skip("TEST_ID_CORRIERE non impostata: test della posizione saltato")
	}

	url := strings.TrimSpace(os.Getenv("TEST_API_URL"))
	if url == "" {
		url = "http://localhost:8080"
	}
	url = strings.TrimRight(url, "/")

	corriere := leggiID(t, "TEST_ID_CORRIERE")
	corriereInattivo := leggiID(t, "TEST_ID_CORRIERE_INATTIVO")
	operatore := leggiID(t, "TEST_ID_OPERATORE")
	admin := leggiID(t, "TEST_ID_AMMINISTRATORE")

	hCorriere := map[string]int64{"X-Id-Corriere": corriere}
	hOperatore := map[string]int64{"X-Id-Operatore": operatore}
	hAdmin := map[string]int64{"X-Id-Amministratore": admin}

	const intervallo = "/api/impostazioni/intervallo-posizione"
	const posizione = "/api/corrieri/posizione"
	const posizioni = "/api/corrieri/posizioni"

	// L'intervallo minimo rende il test veloce; alla fine rimettiamo quello di prima.
	stato, corpo, _ := chiamaAPI(t, url, http.MethodGet, intervallo, hAdmin, nil)
	if stato != http.StatusOK {
		t.Fatalf("lettura intervallo: atteso 200, ricevuto %d. Corpo: %s", stato, corpo)
	}
	var precedente struct {
		IntervalloSecondi int `json:"intervallo_secondi"`
	}
	_ = json.Unmarshal(corpo, &precedente)
	t.Cleanup(func() {
		chiamaAPI(t, url, http.MethodPut, intervallo, hAdmin,
			map[string]any{"intervallo_secondi": precedente.IntervalloSecondi})
	})
	attendiStato(t, http.StatusOK)(chiamaAPI(t, url, http.MethodPut, intervallo, hAdmin,
		map[string]any{"intervallo_secondi": 5}))

	coordinate := map[string]any{"latitudine": 46.0679, "longitudine": 11.1211}

	t.Run("permessi", func(t *testing.T) {
		casi := []struct {
			nome   string
			metodo string
			path   string
			header map[string]int64
			corpo  map[string]any
			atteso int
		}{
			{"invio senza header", http.MethodPost, posizione, nil, coordinate, http.StatusUnauthorized},
			{"invio da operatore", http.MethodPost, posizione, hOperatore, coordinate, http.StatusUnauthorized},
			{"invio da corriere inesistente", http.MethodPost, posizione, map[string]int64{"X-Id-Corriere": 999999}, coordinate, http.StatusForbidden},
			{"lettura da corriere", http.MethodGet, posizioni, hCorriere, nil, http.StatusUnauthorized},
			{"lettura da amministratore", http.MethodGet, posizioni, hAdmin, nil, http.StatusUnauthorized},
			{"intervallo modificato da operatore", http.MethodPut, intervallo, hOperatore, map[string]any{"intervallo_secondi": 10}, http.StatusUnauthorized},
		}
		for _, c := range casi {
			stato, corpo, _ := chiamaAPI(t, url, c.metodo, c.path, c.header, c.corpo)
			if stato != c.atteso {
				t.Errorf("%s: atteso %d, ricevuto %d. Corpo: %s", c.nome, c.atteso, stato, corpo)
			}
		}
	})

	t.Run("validazione", func(t *testing.T) {
		casi := []struct {
			nome   string
			path   string
			metodo string
			header map[string]int64
			corpo  map[string]any
		}{
			{"longitudine mancante", posizione, http.MethodPost, hCorriere, map[string]any{"latitudine": 46.0}},
			{"latitudine fuori range", posizione, http.MethodPost, hCorriere, map[string]any{"latitudine": 100.0, "longitudine": 11.0}},
			{"intervallo troppo basso", intervallo, http.MethodPut, hAdmin, map[string]any{"intervallo_secondi": 1}},
		}
		for _, c := range casi {
			stato, corpo, _ := chiamaAPI(t, url, c.metodo, c.path, c.header, c.corpo)
			if stato != http.StatusBadRequest {
				t.Errorf("%s: atteso 400, ricevuto %d. Corpo: %s", c.nome, stato, corpo)
			}
		}
	})

	t.Run("corriere_senza_spedizioni_attive", func(t *testing.T) {
		stato, corpo, _ := chiamaAPI(t, url, http.MethodPost, posizione,
			map[string]int64{"X-Id-Corriere": corriereInattivo}, coordinate)
		if stato != http.StatusConflict {
			t.Fatalf("atteso 409, ricevuto %d. Corpo: %s", stato, corpo)
		}
	})

	t.Run("invio_e_lettura", func(t *testing.T) {
		lat, lon := 46.07+float64(time.Now().UnixNano()%1000)/1e6, 11.12
		mia := map[string]any{"latitudine": lat, "longitudine": lon}

		stato, corpo, intestazioni := chiamaAPI(t, url, http.MethodPost, posizione, hCorriere, mia)
		if stato == http.StatusTooManyRequests {
			// un'esecuzione precedente ha appena inviato: aspettiamo l'intervallo
			secondi, _ := strconv.Atoi(intestazioni.Get("Retry-After"))
			time.Sleep(time.Duration(secondi) * time.Second)
			stato, corpo, _ = chiamaAPI(t, url, http.MethodPost, posizione, hCorriere, mia)
		}
		if stato != http.StatusCreated {
			t.Fatalf("primo invio: atteso 201, ricevuto %d. Corpo: %s", stato, corpo)
		}

		stato, corpo, _ = chiamaAPI(t, url, http.MethodPost, posizione, hCorriere, mia)
		if stato != http.StatusTooManyRequests {
			t.Errorf("secondo invio immediato: atteso 429, ricevuto %d. Corpo: %s", stato, corpo)
		}

		stato, corpo, _ = chiamaAPI(t, url, http.MethodGet, posizioni, hOperatore, nil)
		if stato != http.StatusOK {
			t.Fatalf("lettura posizioni: atteso 200, ricevuto %d. Corpo: %s", stato, corpo)
		}
		var risposta struct {
			Corrieri []struct {
				IDCorriere  int64   `json:"id_corriere"`
				Latitudine  float64 `json:"latitudine"`
				Longitudine float64 `json:"longitudine"`
			} `json:"corrieri"`
		}
		if err := json.Unmarshal(corpo, &risposta); err != nil {
			t.Fatalf("JSON non valido: %v. Corpo: %s", err, corpo)
		}
		for _, c := range risposta.Corrieri {
			if c.IDCorriere == corriere {
				if c.Latitudine != lat || c.Longitudine != lon {
					t.Errorf("posizione letta %.6f,%.6f, attesa %.6f,%.6f", c.Latitudine, c.Longitudine, lat, lon)
				}
				return
			}
		}
		t.Errorf("il corriere %d non compare nelle posizioni. Corpo: %s", corriere, corpo)
	})
}

func attendiStato(t *testing.T, atteso int) func(int, []byte, http.Header) {
	t.Helper()
	return func(stato int, corpo []byte, _ http.Header) {
		t.Helper()
		if stato != atteso {
			t.Fatalf("atteso %d, ricevuto %d. Corpo: %s", atteso, stato, corpo)
		}
	}
}

func chiamaAPI(t *testing.T, url, metodo, percorso string, header map[string]int64, corpo map[string]any) (int, []byte, http.Header) {
	t.Helper()

	var dati []byte
	if corpo != nil {
		var err error
		if dati, err = json.Marshal(corpo); err != nil {
			t.Fatalf("creazione del JSON: %v", err)
		}
	}

	richiesta, err := http.NewRequest(metodo, url+percorso, bytes.NewReader(dati))
	if err != nil {
		t.Fatalf("creazione della richiesta HTTP: %v", err)
	}
	richiesta.Header.Set("Content-Type", "application/json")
	for nome, id := range header {
		richiesta.Header.Set(nome, strconv.FormatInt(id, 10))
	}

	client := &http.Client{Timeout: 10 * time.Second}
	risposta, err := client.Do(richiesta)
	if err != nil {
		t.Fatalf("chiamata al server fallita: %v. Controlla che il backend sia avviato", err)
	}
	defer risposta.Body.Close()

	datiRisposta, err := io.ReadAll(risposta.Body)
	if err != nil {
		t.Fatalf("lettura della risposta HTTP: %v", err)
	}
	return risposta.StatusCode, datiRisposta, risposta.Header
}
