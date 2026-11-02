package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"lastmiledelivery/backend/internal/errori"
)

const MaxCorpoRichiesta = 1 << 20

type rispostaErrore struct {
	Errore   string               `json:"errore"`
	Dettagli []errori.ErroreCampo `json:"dettagli,omitempty"`
}

func ScriviJSON(w http.ResponseWriter, stato int, dati any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(stato)
	if err := json.NewEncoder(w).Encode(dati); err != nil {
		log.Println("scrittura della risposta fallita:", err)
	}
}

func ScriviErrore(w http.ResponseWriter, stato int, messaggio string, dettagli ...errori.ErroreCampo) {
	ScriviJSON(w, stato, rispostaErrore{Errore: messaggio, Dettagli: dettagli})
}

func LeggiCorpo(w http.ResponseWriter, r *http.Request, dest any) bool {
	tipo, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || tipo != "application/json" {
		ScriviErrore(w, http.StatusUnsupportedMediaType, "il corpo deve essere JSON (Content-Type: application/json)")
		return false
	}
	if messaggio, ok := leggiJSON(w, r, dest); !ok {
		ScriviErrore(w, http.StatusBadRequest, messaggio)
		return false
	}
	return true
}

func IDDalPercorso(w http.ResponseWriter, r *http.Request) (int32, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 32)
	if err != nil || id <= 0 {
		ScriviErrore(w, http.StatusBadRequest, "id non valido: deve essere un numero intero positivo")
		return 0, false
	}
	return int32(id), true
}

func leggiJSON(w http.ResponseWriter, r *http.Request, dest any) (string, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxCorpoRichiesta)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(dest)
	if err == nil {
		if dec.Decode(&struct{}{}) != io.EOF {
			return "il corpo deve contenere un solo oggetto JSON", false
		}
		return "", true
	}

	var errSintassi *json.SyntaxError
	var errTipo *json.UnmarshalTypeError
	var errDimensione *http.MaxBytesError
	switch {
	case errors.Is(err, io.EOF):
		return "corpo della richiesta vuoto", false
	case errors.As(err, &errSintassi), errors.Is(err, io.ErrUnexpectedEOF):
		return "JSON non valido", false
	case errors.As(err, &errTipo):
		return fmt.Sprintf("campo %s: tipo non valido, atteso %s", errTipo.Field, nomeTipo(errTipo.Type.String())), false
	case errors.As(err, &errDimensione):
		return "corpo della richiesta troppo grande", false
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		return "campo non previsto: " + strings.Trim(strings.TrimPrefix(err.Error(), "json: unknown field "), `"`), false
	default:
		return "JSON non valido", false
	}
}

func nomeTipo(t string) string {
	switch strings.TrimPrefix(t, "*") {
	case "int32", "int":
		return "numero intero"
	case "string":
		return "stringa"
	case "bool":
		return "booleano"
	default:
		return "oggetto"
	}
}
