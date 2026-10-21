package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RichiestaCreazioneSpedizione struct {
	CodOrdine                 string    `json:"cod_ordine"`
	IsReso                    bool      `json:"is_reso"`
	IDClienteMittente         int32     `json:"id_cliente_mittente"`
	IDClienteDestinatario     int32     `json:"id_cliente_destinatario"`
	IDIndirizzoRitiro         int32     `json:"id_indirizzo_ritiro"`
	IDIndirizzoSpedizione     int32     `json:"id_indirizzo_spedizione"`
	DataOraRichiestaSpedizione time.Time `json:"dataora_richiesta_spedizione"`
}

type RispostaCreazioneSpedizione struct {
	ID int32 `json:"id"`
}

func main() {

	db, err := collegaDatabase()

	if err != nil {
		log.Fatal("Collegamento a PostgreSQL fallito:", err)
	}

	defer db.Close()

	log.Println("Collegamento a PostgreSQL riuscito")


	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	idOperatoreTest, err := trovaIDOperatorePerEmail(
		ctx,
		db,
		"operatore@veloce24.test",
	)

	cancel()

	if err != nil {
		log.Fatal("Recupero dell'operatore di test fallito:", err)
	}

	log.Println("ID operatore di test:", idOperatoreTest)
	router := http.NewServeMux()


	//saluto senza parentesi seno la eseguirei subito la funzione va chiamata in futuro
	router.HandleFunc("POST /api/spedizioni", func(w http.ResponseWriter, r *http.Request) {
		creaSpedizione(w, r, db, idOperatoreTest)
	})
	
	//passo al server il router 
	err = http.ListenAndServe(":8080", router)

	if err != nil {
		log.Fatal(err)
	}
}

func collegaDatabase() (*pgxpool.Pool, error) {
	connessione := os.Getenv("DATABASE_URL")

	if connessione == "" {
		return nil, errors.New("DATABASE_URL non impostata")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := pgxpool.New(ctx, connessione)

	if err != nil {
		return nil, err
	}

	err = db.Ping(ctx)

	if err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func creaSpedizione(w http.ResponseWriter, r *http.Request, db *pgxpool.Pool, idOperatore int32) {
	var richiesta RichiestaCreazioneSpedizione

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	err := decoder.Decode(&richiesta)

	if err != nil {
		http.Error(w, "Corpo JSON non valido", http.StatusBadRequest)
		return
	}

	err = validaRichiestaCreazioneSpedizione(richiesta)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	idSpedizione, err := salvaNuovaSpedizione(
		ctx,
		db,
		richiesta,
		idOperatore,
	)

	if err != nil {
		if errors.Is(err, ErrCodiceOrdineDuplicato) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}

		if errors.Is(err, ErrRiferimentoNonValido) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		log.Println("Creazione spedizione fallita:", err)
		http.Error(w, "Impossibile creare la spedizione", http.StatusInternalServerError)
		return
	}

	risposta := RispostaCreazioneSpedizione{
		ID: idSpedizione,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	err = json.NewEncoder(w).Encode(risposta)

	if err != nil {
		log.Println("Errore durante l'invio della risposta:", err)
	}
}


func validaRichiestaCreazioneSpedizione(richiesta RichiestaCreazioneSpedizione) error {
	if strings.TrimSpace(richiesta.CodOrdine) == "" {
		return errors.New("cod_ordine è obbligatorio")
	}

	if richiesta.IDClienteMittente <= 0 {
		return errors.New("id_cliente_mittente deve essere positivo")
	}

	if richiesta.IDClienteDestinatario <= 0 {
		return errors.New("id_cliente_destinatario deve essere positivo")
	}

	if richiesta.IDIndirizzoRitiro <= 0 {
		return errors.New("id_indirizzo_ritiro deve essere positivo")
	}

	if richiesta.IDIndirizzoSpedizione <= 0 {
		return errors.New("id_indirizzo_spedizione deve essere positivo")
	}

	if richiesta.DataOraRichiestaSpedizione.IsZero() {
		return errors.New("dataora_richiesta_spedizione è obbligatoria")
	}

	if utf8.RuneCountInString(richiesta.CodOrdine) > 50 {
		return errors.New("cod_ordine può contenere al massimo 50 caratteri")
	}

	return nil
}
