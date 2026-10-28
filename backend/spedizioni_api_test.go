package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// Test di integrazione: chiamano l'API già in esecuzione (cluster, port-forward
// o docker compose). Se TEST_BASE_URL non è impostata vengono saltati, così
// "go test ./..." resta sempre eseguibile anche senza cluster.
//
//	TEST_BASE_URL=http://lastmile.test go test -run Integrazione -v ./...
//	TEST_BASE_URL=http://localhost:8080 go test -run Integrazione -v ./...

func urlSpedizioni(t *testing.T) string {
	t.Helper()

	base := strings.TrimRight(os.Getenv("TEST_BASE_URL"), "/")
	if base == "" {
		t.Skip("TEST_BASE_URL non impostata: test di integrazione saltati")
	}

	return base + "/api/spedizioni"
}

// invia esegue una richiesta e restituisce codice HTTP e corpo della risposta.
func invia(t *testing.T, metodo, url, corpo string) (int, string) {
	t.Helper()

	var reader io.Reader
	if corpo != "" {
		reader = strings.NewReader(corpo)
	}

	req, err := http.NewRequest(metodo, url, reader)
	if err != nil {
		t.Fatalf("richiesta non creata: %v", err)
	}
	if corpo != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("chiamata a %s fallita: %v", url, err)
	}
	defer resp.Body.Close()

	dati, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("lettura risposta fallita: %v", err)
	}

	return resp.StatusCode, strings.TrimSpace(string(dati))
}

func corpoSpedizione(t *testing.T, codOrdine string, mittente, destinatario, ritiro, spedizione int) string {
	t.Helper()

	richiesta := map[string]any{
		"cod_ordine":                   codOrdine,
		"id_cliente_mittente":          mittente,
		"id_cliente_destinatario":      destinatario,
		"dataora_richiesta_spedizione": time.Now().Format(time.RFC3339),
		"id_indirizzo_ritiro":          ritiro,
		"id_indirizzo_spedizione":      spedizione,
	}

	dati, err := json.Marshal(richiesta)
	if err != nil {
		t.Fatalf("JSON non creato: %v", err)
	}

	return string(dati)
}

// codiceUnico evita collisioni tra esecuzioni successive (cod_ordine è univoco).
func codiceUnico(prefisso string) string {
	return fmt.Sprintf("%s-%d", prefisso, time.Now().UnixNano())
}

func TestIntegrazioneCreaSpedizione(t *testing.T) {
	url := urlSpedizioni(t)
	codice := codiceUnico("TEST")

	t.Run("creazione valida", func(t *testing.T) {
		stato, corpo := invia(t, http.MethodPost, url, corpoSpedizione(t, codice, 1, 2, 1, 2))
		if stato != http.StatusCreated {
			t.Fatalf("atteso %d, ricevuto %d (%s)", http.StatusCreated, stato, corpo)
		}

		var risposta RispostaCreazioneSpedizione
		if err := json.Unmarshal([]byte(corpo), &risposta); err != nil {
			t.Fatalf("risposta non è JSON valido: %v (%s)", err, corpo)
		}
		if risposta.ID <= 0 {
			t.Fatalf("id spedizione non valido: %d", risposta.ID)
		}
	})

	t.Run("cod_ordine duplicato", func(t *testing.T) {
		stato, corpo := invia(t, http.MethodPost, url, corpoSpedizione(t, codice, 1, 2, 1, 2))
		if stato != http.StatusConflict {
			t.Fatalf("atteso %d, ricevuto %d (%s)", http.StatusConflict, stato, corpo)
		}
	})
}

func TestIntegrazioneRichiesteNonValide(t *testing.T) {
	url := urlSpedizioni(t)

	casi := []struct {
		nome  string
		corpo string
		stato int
	}{
		{"cod_ordine mancante", corpoSpedizione(t, "", 1, 2, 1, 2), http.StatusBadRequest},
		{"mittente zero", corpoSpedizione(t, codiceUnico("T-MITT"), 0, 2, 1, 2), http.StatusBadRequest},
		{"destinatario negativo", corpoSpedizione(t, codiceUnico("T-DEST"), 1, -3, 1, 2), http.StatusBadRequest},
		{"cliente inesistente", corpoSpedizione(t, codiceUnico("T-CLI"), 9999, 2, 1, 2), http.StatusBadRequest},
		{"indirizzo inesistente", corpoSpedizione(t, codiceUnico("T-IND"), 1, 2, 9999, 2), http.StatusBadRequest},
		{"JSON non valido", "{non json", http.StatusBadRequest},
		{"campo sconosciuto", `{"cod_ordine":"A","campo_inventato":1}`, http.StatusBadRequest},
	}

	for _, c := range casi {
		t.Run(c.nome, func(t *testing.T) {
			stato, corpo := invia(t, http.MethodPost, url, c.corpo)
			if stato != c.stato {
				t.Fatalf("atteso %d, ricevuto %d (%s)", c.stato, stato, corpo)
			}
		})
	}
}

func TestIntegrazioneMetodoNonAmmesso(t *testing.T) {
	url := urlSpedizioni(t)

	stato, corpo := invia(t, http.MethodGet, url, "")
	if stato != http.StatusMethodNotAllowed {
		t.Fatalf("atteso %d, ricevuto %d (%s)", http.StatusMethodNotAllowed, stato, corpo)
	}
}
