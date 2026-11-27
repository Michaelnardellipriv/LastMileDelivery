package identita

import (
	"context"
	"strconv"

	"lastmiledelivery/backend/internal/audit"
	"lastmiledelivery/backend/internal/database"
)

type Ruolo string

const (
	Operatore      Ruolo = "operatore"
	Corriere       Ruolo = "corriere"
	Cliente        Ruolo = "cliente"
	Amministratore Ruolo = "amministratore"
)

func (r Ruolo) Nome() string {
	switch r {
	case Operatore:
		return "operatore logistico"
	case Corriere:
		return "corriere"
	case Cliente:
		return "cliente"
	}
	return "amministratore"
}

// HaSede dice se il ruolo lavora per una sede: operatori e corrieri sì,
// clienti e amministratori no.
func (r Ruolo) HaSede() bool {
	return r == Operatore || r == Corriere
}

type Richiedente struct {
	Ruolo Ruolo
	ID    int64
	// IDSede è la sede di operatori e corrieri (0 per clienti e amministratori). Sul
	// servizio utenti il middleware la rilegge dal database a ogni richiesta; sul servizio
	// spedizioni è quella del token, e un cambio di sede chiude le sessioni dell'utente
	// (sessioni.Archivio.RevocaUtente), quindi vale subito anche lì.
	IDSede int64
	// Sessione è l'id della sessione del login (claim "sid" del token): ogni richiesta
	// vale solo se la sessione è ancora aperta nel database. Vuota con gli header X-Id-*
	// di sviluppo.
	Sessione string
}

// String: "ruolo:id", come compare nel log di audit.
func (r Richiedente) String() string {
	return string(r.Ruolo) + ":" + strconv.FormatInt(r.ID, 10)
}

// Ambito sono le sedi che il richiedente può vedere: tutte per amministratori e
// clienti (i clienti vedono solo le proprie spedizioni, filtrate dal servizio), solo
// la propria per operatori e corrieri.
func (r Richiedente) Ambito() database.Ambito {
	if r.Ruolo.HaSede() {
		return database.SoloSede(r.IDSede)
	}
	if r.Ruolo == Amministratore || r.Ruolo == Cliente {
		return database.TutteLeSedi()
	}
	return database.Ambito{}
}

type chiaveContesto struct{}

// NelContesto mette nel contesto il richiedente e l'ambito delle sue sedi, che il
// database usa per la Row-Level Security.
func NelContesto(ctx context.Context, r Richiedente) context.Context {
	ctx = audit.ConUtente(ctx, r.String())
	return database.ConAmbito(context.WithValue(ctx, chiaveContesto{}, r), r.Ambito())
}

func DalContesto(ctx context.Context) Richiedente {
	r, _ := ctx.Value(chiaveContesto{}).(Richiedente)
	return r
}
