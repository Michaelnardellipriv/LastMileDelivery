package main

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Ruolo string

const (
	RuoloCliente        Ruolo = "cliente"
	RuoloCorriere       Ruolo = "corriere"
	RuoloOperatore      Ruolo = "operatore"
	RuoloAmministratore Ruolo = "amministratore"
)

// Utente autenticato: il ruolo e l'id nella tabella del ruolo
// (clienti.id, corrieri.id, operatori_logistici.id, amministratori.id).
type UtenteAutenticato struct {
	Ruolo   Ruolo
	IDRuolo int32
}

func trovaUtentePerEmail(ctx context.Context, db *pgxpool.Pool, email string) (UtenteAutenticato, error) {
	const query = `
		SELECT
			(SELECT id FROM amministratori     WHERE id_utente = u.id),
			(SELECT id FROM operatori_logistici WHERE id_utente = u.id),
			(SELECT id FROM corrieri           WHERE id_utente = u.id),
			(SELECT id FROM clienti            WHERE id_utente = u.id)
		FROM utenti AS u
		WHERE u.email = $1
	`

	var idAdmin, idOperatore, idCorriere, idCliente *int32

	err := db.QueryRow(ctx, query, email).Scan(&idAdmin, &idOperatore, &idCorriere, &idCliente)

	if err != nil {
		return UtenteAutenticato{}, err
	}

	switch {
	case idAdmin != nil:
		return UtenteAutenticato{RuoloAmministratore, *idAdmin}, nil
	case idOperatore != nil:
		return UtenteAutenticato{RuoloOperatore, *idOperatore}, nil
	case idCorriere != nil:
		return UtenteAutenticato{RuoloCorriere, *idCorriere}, nil
	case idCliente != nil:
		return UtenteAutenticato{RuoloCliente, *idCliente}, nil
	}

	return UtenteAutenticato{}, errUtenteSenzaRuolo
}
