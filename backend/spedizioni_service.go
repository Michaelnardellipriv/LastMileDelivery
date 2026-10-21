package main

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrCodiceOrdineDuplicato = errors.New("cod_ordine già presente")
var ErrRiferimentoNonValido = errors.New("cliente o indirizzo non esistente")

func salvaNuovaSpedizione(ctx context.Context, db *pgxpool.Pool, richiesta RichiestaCreazioneSpedizione, idOperatore int32) (int32, error) {
	dati := DatiNuovaSpedizione{
		CodOrdine:                 richiesta.CodOrdine,
		IsReso:                    richiesta.IsReso,
		IDOperatoreHaCreato:       idOperatore,
		IDClienteMittente:         richiesta.IDClienteMittente,
		IDClienteDestinatario:     richiesta.IDClienteDestinatario,
		DataOraRichiestaSpedizione: richiesta.DataOraRichiestaSpedizione,
		IDIndirizzoRitiro:         richiesta.IDIndirizzoRitiro,
		IDIndirizzoSpedizione:     richiesta.IDIndirizzoSpedizione,
	}

	id, err := inserisciSpedizione(ctx, db, dati)

	if err == nil {
		return id, nil
	}

	var errorePG *pgconn.PgError

	if errors.As(err, &errorePG) {
		if errorePG.Code == "23505" &&
			errorePG.ConstraintName == "uq_spedizioni_cod_ordine" {
			return 0, ErrCodiceOrdineDuplicato
		}

		if errorePG.Code == "23503" {
			switch errorePG.ConstraintName {
			case "spedizioni_id_cliente_mittente_fkey",
				"spedizioni_id_cliente_destinatario_fkey",
				"spedizioni_id_indirizzo_ritiro_fkey",
				"spedizioni_id_indirizzo_spedizione_fkey":
				return 0, ErrRiferimentoNonValido
			}
		}
	}

	return 0, err
}