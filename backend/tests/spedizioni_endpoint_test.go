package apitest

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

type configurazioneAPI struct {
	url                    string
	idOperatore            int64
	idClienteMittente      int64
	idClienteDestinatario  int64
	idIndirizzoRitiro      int64
	idIndirizzoSpedizione  int64
}

func TestCreaSpedizione(t *testing.T) {
	config := leggiConfigurazione(t)

	t.Run("creazione_valida", func(t *testing.T) {
		corpo := richiestaValida(config, nuovoCodiceOrdine(t))

		verificaCreazione(t, config, corpo)
	})

	t.Run("codice_ordine_vuoto", func(t *testing.T) {
		// Tutti gli altri campi sono validi: cambiamo solo il codice ordine.
		corpo := richiestaValida(config, "")

		stato, _, risposta := inviaRichiesta(t, config, corpo)
		if stato != http.StatusBadRequest {
			t.Fatalf("atteso 400, ricevuto %d. Corpo: %s", stato, risposta)
		}
	})

	t.Run("codice_ordine_duplicato", func(t *testing.T) {
		corpo := richiestaValida(config, nuovoCodiceOrdine(t))

		// La prima richiesta deve creare la spedizione.
		verificaCreazione(t, config, corpo)

		// Ripetiamo esattamente la richiesta con lo stesso codice ordine.
		stato, _, risposta := inviaRichiesta(t, config, corpo)
		if stato != http.StatusConflict {
			t.Fatalf("atteso 409, ricevuto %d. Corpo: %s", stato, risposta)
		}
	})
}

// Controlla sia lo stato HTTP sia la struttura della risposta JSON.
func verificaCreazione(t *testing.T, config configurazioneAPI, corpo map[string]any) {
	t.Helper()

	stato, intestazioni, risposta := inviaRichiesta(t, config, corpo)
	if stato != http.StatusCreated {
		t.Fatalf("atteso 201, ricevuto %d. Corpo: %s", stato, risposta)
	}

	tipo, _, err := mime.ParseMediaType(intestazioni.Get("Content-Type"))
	if err != nil || tipo != "application/json" {
		t.Fatalf("atteso Content-Type application/json, ricevuto %q",
			intestazioni.Get("Content-Type"))
	}

	var creata struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(risposta, &creata); err != nil {
		t.Fatalf("risposta JSON non valida: %v. Corpo: %s", err, risposta)
	}
	if creata.ID <= 0 {
		t.Fatalf("atteso un id positivo. Corpo: %s", risposta)
	}

	t.Logf("spedizione creata: id=%d, cod_ordine=%v", creata.ID, corpo["cod_ordine"])
}

// I campi seguono il contratto HTTP: non importiamo i tipi interni del server.
func richiestaValida(config configurazioneAPI, codiceOrdine string) map[string]any {
	return map[string]any{
		"cod_ordine":                    codiceOrdine,
		"is_reso":                       false,
		"id_cliente_mittente":           config.idClienteMittente,
		"id_cliente_destinatario":       config.idClienteDestinatario,
		"dataora_richiesta_spedizione":   time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
		"id_indirizzo_ritiro":            config.idIndirizzoRitiro,
		"id_indirizzo_spedizione":        config.idIndirizzoSpedizione,
	}
}

// Invia una richiesta HTTP al server e legge la risposta prima di chiuderla.
func inviaRichiesta(
	t *testing.T,
	config configurazioneAPI,
	corpo map[string]any,
) (int, http.Header, []byte) {
	t.Helper()

	dati, err := json.Marshal(corpo)
	if err != nil {
		t.Fatalf("creazione del JSON: %v", err)
	}

	richiesta, err := http.NewRequest(
		http.MethodPost,
		config.url+"/api/spedizioni",
		bytes.NewReader(dati),
	)
	if err != nil {
		t.Fatalf("creazione della richiesta HTTP: %v", err)
	}
	richiesta.Header.Set("Content-Type", "application/json")
	richiesta.Header.Set("Accept", "application/json")
	richiesta.Header.Set("X-Id-Operatore", strconv.FormatInt(config.idOperatore, 10))

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

	return risposta.StatusCode, risposta.Header, datiRisposta
}

func leggiConfigurazione(t *testing.T) configurazioneAPI {
	t.Helper()

	url := strings.TrimSpace(os.Getenv("TEST_API_URL"))
	if url == "" {
		url = "http://localhost:8080"
	}

	return configurazioneAPI{
		url:                   strings.TrimRight(url, "/"),
		idOperatore:           leggiID(t, "TEST_ID_OPERATORE"),
		idClienteMittente:     leggiID(t, "TEST_ID_MITTENTE"),
		idClienteDestinatario: leggiID(t, "TEST_ID_DESTINATARIO"),
		idIndirizzoRitiro:     leggiID(t, "TEST_ID_RITIRO"),
		idIndirizzoSpedizione: leggiID(t, "TEST_ID_CONSEGNA"),
	}
}

// ParseInt limita gli ID all'intervallo int32 delle colonne INTEGER PostgreSQL.
func leggiID(t *testing.T, nome string) int64 {
	t.Helper()

	valore := strings.TrimSpace(os.Getenv(nome))
	id, err := strconv.ParseInt(valore, 10, 32)
	if err != nil || id <= 0 {
		t.Fatalf("imposta %s con un ID positivo esistente nel database; ricevuto %q", nome, valore)
	}

	return id
}

// Un codice diverso a ogni prova evita conflitti con le esecuzioni precedenti.
func nuovoCodiceOrdine(t *testing.T) string {
	t.Helper()

	var dati [12]byte
	if _, err := rand.Read(dati[:]); err != nil {
		t.Fatalf("generazione del codice ordine di prova: %v", err)
	}

	return fmt.Sprintf("TEST-%x", dati)
}
