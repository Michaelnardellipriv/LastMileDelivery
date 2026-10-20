package main

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DatiNuovaSpedizione struct {
	CodOrdine                 string
	IsReso                    bool
	IDOperatoreHaCreato       int32
	IDClienteMittente         int32
	IDClienteDestinatario     int32
	DataOraRichiestaSpedizione time.Time
	IDIndirizzoRitiro         int32
	IDIndirizzoSpedizione     int32
}

func inserisciSpedizione(ctx context.Context, db *pgxpool.Pool, dati DatiNuovaSpedizione) (int32, error) {
	const query = `
		INSERT INTO spedizioni (
			cod_ordine,
			is_reso,
			id_operatore_ha_creato,
			id_cliente_mittente,
			id_cliente_destinatario,
			dataora_richiesta_spedizione,
			id_indirizzo_ritiro,
			id_indirizzo_spedizione
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`

	var idSpedizione int32

	err := db.QueryRow(
		ctx,
		query,
		dati.CodOrdine,
		dati.IsReso,
		dati.IDOperatoreHaCreato,
		dati.IDClienteMittente,
		dati.IDClienteDestinatario,
		dati.DataOraRichiestaSpedizione,
		dati.IDIndirizzoRitiro,
		dati.IDIndirizzoSpedizione,
	).Scan(&idSpedizione)

	if err != nil {
		return 0, err
	}

	return idSpedizione, nil
}