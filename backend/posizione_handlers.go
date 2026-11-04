package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	intervalloMinimoSecondi  = 5
	intervalloMassimoSecondi = 3600
)

type RichiestaPosizione struct {
	// Puntatori: così distinguiamo "campo mancante" da "vale 0"
	// (0 è una coordinata valida).
	Latitudine  *float64 `json:"latitudine"`
	Longitudine *float64 `json:"longitudine"`
}

type RispostaIntervallo struct {
	IntervalloSecondi int32 `json:"intervallo_secondi"`
}

type RispostaPosizioniCorrieri struct {
	IntervalloSecondi int32               `json:"intervallo_secondi"`
	Corrieri          []PosizioneCorriere `json:"corrieri"`
}

// POST /api/corrieri/posizione — solo corrieri.
// Il corriere invia la propria posizione (simulata); viene salvata solo se ha
// spedizioni "in consegna".
func inviaPosizioneCorriere(w http.ResponseWriter, r *http.Request, db *pgxpool.Pool) {
	ctx, cancel := contestoRichiesta(r)
	defer cancel()

	idCorriere, ok := richiediRuolo(ctx, w, r, db, ruoloCorriere)
	if !ok {
		return
	}

	var richiesta RichiestaPosizione
	if !leggiJSON(w, r, &richiesta) {
		return
	}

	if richiesta.Latitudine == nil || richiesta.Longitudine == nil {
		http.Error(w, "latitudine e longitudine sono obbligatorie", http.StatusBadRequest)
		return
	}
	lat, lon := *richiesta.Latitudine, *richiesta.Longitudine
	if lat < -90 || lat > 90 {
		http.Error(w, "latitudine deve essere tra -90 e 90", http.StatusBadRequest)
		return
	}
	if lon < -180 || lon > 180 {
		http.Error(w, "longitudine deve essere tra -180 e 180", http.StatusBadRequest)
		return
	}

	intervallo, err := registraPosizioneCorriere(ctx, db, idCorriere, lat, lon)
	switch {
	case err == nil:
		scriviJSON(w, http.StatusCreated, RispostaIntervallo{IntervalloSecondi: intervallo})
	case errors.Is(err, ErrNessunaSpedizioneAttiva):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, ErrInvioTroppoFrequente):
		w.Header().Set("Retry-After", strconv.Itoa(int(intervallo)))
		http.Error(w, err.Error(), http.StatusTooManyRequests)
	default:
		rispondiErroreInterno(w, "salvataggio posizione", err)
	}
}

// GET /api/corrieri/posizioni — solo operatori logistici.
// Ultima posizione di ogni corriere con spedizioni in consegna.
func elencaPosizioniCorrieri(w http.ResponseWriter, r *http.Request, db *pgxpool.Pool) {
	ctx, cancel := contestoRichiesta(r)
	defer cancel()

	if _, ok := richiediRuolo(ctx, w, r, db, ruoloOperatore); !ok {
		return
	}

	intervallo, err := leggiIntervalloPosizione(ctx, db)
	if err != nil {
		rispondiErroreInterno(w, "lettura intervallo", err)
		return
	}

	posizioni, err := posizioniCorrieriAttivi(ctx, db)
	if err != nil {
		rispondiErroreInterno(w, "lettura posizioni", err)
		return
	}

	scriviJSON(w, http.StatusOK, RispostaPosizioniCorrieri{
		IntervalloSecondi: intervallo,
		Corrieri:          posizioni,
	})
}

// GET /api/impostazioni/intervallo-posizione — corrieri, operatori, amministratori.
// Il corriere lo legge per sapere ogni quanto inviare la posizione.
func leggiIntervalloHandler(w http.ResponseWriter, r *http.Request, db *pgxpool.Pool) {
	ctx, cancel := contestoRichiesta(r)
	defer cancel()

	if !richiediUnoDeiRuoli(ctx, w, r, db, ruoloCorriere, ruoloOperatore, ruoloAmministratore) {
		return
	}

	intervallo, err := leggiIntervalloPosizione(ctx, db)
	if err != nil {
		rispondiErroreInterno(w, "lettura intervallo", err)
		return
	}

	scriviJSON(w, http.StatusOK, RispostaIntervallo{IntervalloSecondi: intervallo})
}

// PUT /api/impostazioni/intervallo-posizione — solo amministratori.
func modificaIntervalloHandler(w http.ResponseWriter, r *http.Request, db *pgxpool.Pool) {
	ctx, cancel := contestoRichiesta(r)
	defer cancel()

	idAmministratore, ok := richiediRuolo(ctx, w, r, db, ruoloAmministratore)
	if !ok {
		return
	}

	var richiesta struct {
		IntervalloSecondi *int32 `json:"intervallo_secondi"`
	}
	if !leggiJSON(w, r, &richiesta) {
		return
	}

	if richiesta.IntervalloSecondi == nil {
		http.Error(w, "intervallo_secondi è obbligatorio", http.StatusBadRequest)
		return
	}
	secondi := *richiesta.IntervalloSecondi
	if secondi < intervalloMinimoSecondi || secondi > intervalloMassimoSecondi {
		http.Error(w, "intervallo_secondi deve essere tra 5 e 3600", http.StatusBadRequest)
		return
	}

	if err := aggiornaIntervalloPosizione(ctx, db, secondi, idAmministratore); err != nil {
		rispondiErroreInterno(w, "aggiornamento intervallo", err)
		return
	}

	scriviJSON(w, http.StatusOK, RispostaIntervallo{IntervalloSecondi: secondi})
}

// ---------- funzioni di supporto ----------

func contestoRichiesta(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), 5*time.Second)
}

func leggiJSON(w http.ResponseWriter, r *http.Request, destinazione any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destinazione); err != nil {
		http.Error(w, "Corpo JSON non valido", http.StatusBadRequest)
		return false
	}
	return true
}

func scriviJSON(w http.ResponseWriter, stato int, dati any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(stato)
	if err := json.NewEncoder(w).Encode(dati); err != nil {
		log.Println("Errore durante l'invio della risposta:", err)
	}
}

func rispondiErroreInterno(w http.ResponseWriter, operazione string, err error) {
	log.Printf("%s fallita: %v", operazione, err)
	http.Error(w, "Errore interno del server", http.StatusInternalServerError)
}
