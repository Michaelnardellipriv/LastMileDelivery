package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Identificazione TEMPORANEA di chi fa la richiesta.
//
// Il login (login-api-backend) non esiste ancora. Finché non c'è, chi chiama
// dichiara il proprio id nel ruolo con un header, come già fanno i test
// (X-Id-Operatore). ATTENZIONE: non è sicuro, chiunque può scrivere un id
// qualsiasi. Quando arriverà il login basterà cambiare idDaHeader in modo che
// legga l'id dalla sessione/token: gli handler non dovranno cambiare.

type ruolo struct {
	nome    string // per i messaggi di errore
	header  string // header HTTP con l'id
	tabella string // tabella del ruolo nello schema
}

var (
	ruoloCorriere       = ruolo{"corriere", "X-Id-Corriere", "corrieri"}
	ruoloOperatore      = ruolo{"operatore logistico", "X-Id-Operatore", "operatori_logistici"}
	ruoloAmministratore = ruolo{"amministratore", "X-Id-Amministratore", "amministratori"}
)

var ErrRuoloNonTrovato = errors.New("utente non trovato con questo ruolo")

// idDaHeader legge l'id dall'header del ruolo. ok=false se manca o non è valido.
func idDaHeader(r *http.Request, ru ruolo) (int32, bool) {
	valore := strings.TrimSpace(r.Header.Get(ru.header))
	id, err := strconv.ParseInt(valore, 10, 32)
	if err != nil || id <= 0 {
		return 0, false
	}
	return int32(id), true
}

// esisteRuolo controlla che l'id esista davvero nella tabella del ruolo.
func esisteRuolo(ctx context.Context, db *pgxpool.Pool, ru ruolo, id int32) error {
	// Il nome della tabella viene da una costante del codice, mai dall'utente:
	// concatenarlo non permette SQL injection.
	var trovato int32
	err := db.QueryRow(ctx, "SELECT id FROM "+ru.tabella+" WHERE id = $1", id).Scan(&trovato)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrRuoloNonTrovato
	}
	return err
}

// richiediRuolo restituisce l'id di chi chiama se ha il ruolo richiesto.
// Se non ce l'ha scrive già la risposta di errore e restituisce ok=false:
//
//	401 header mancante o non valido
//	403 l'id non corrisponde a nessun utente con quel ruolo
func richiediRuolo(ctx context.Context, w http.ResponseWriter, r *http.Request, db *pgxpool.Pool, ru ruolo) (int32, bool) {
	id, ok := idDaHeader(r, ru)
	if !ok {
		http.Error(w, fmt.Sprintf("operazione riservata a: %s (header %s mancante o non valido)", ru.nome, ru.header), http.StatusUnauthorized)
		return 0, false
	}

	err := esisteRuolo(ctx, db, ru, id)
	if errors.Is(err, ErrRuoloNonTrovato) {
		http.Error(w, fmt.Sprintf("nessun %s con id %d", ru.nome, id), http.StatusForbidden)
		return 0, false
	}
	if err != nil {
		rispondiErroreInterno(w, "verifica del ruolo", err)
		return 0, false
	}

	return id, true
}

// richiediUnoDeiRuoli: come richiediRuolo, ma basta averne uno tra quelli
// indicati. Si usa il primo header presente nella richiesta.
func richiediUnoDeiRuoli(ctx context.Context, w http.ResponseWriter, r *http.Request, db *pgxpool.Pool, ruoli ...ruolo) bool {
	for _, ru := range ruoli {
		if strings.TrimSpace(r.Header.Get(ru.header)) != "" {
			_, ok := richiediRuolo(ctx, w, r, db, ru)
			return ok
		}
	}

	nomi := make([]string, len(ruoli))
	for i, ru := range ruoli {
		nomi[i] = ru.header
	}
	http.Error(w, "serve uno di questi header: "+strings.Join(nomi, ", "), http.StatusUnauthorized)
	return false
}
