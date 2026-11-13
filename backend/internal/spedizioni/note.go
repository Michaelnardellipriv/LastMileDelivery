package spedizioni

import (
	"context"
	"slices"
	"strings"
	"time"

	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

type Nota struct {
	ID          int32
	Testo       string
	IsInterna   bool
	Motivo      *string
	CreataIl    time.Time
	IDOperatore *int32
	IDCorriere  *int32
	// nome e cognome di chi ha scritto la nota
	NomeAutore    *string
	CognomeAutore *string
}

var statiNoteCorriere = []Stato{Ritirata, InConsegna}

func (s *Servizio) AggiungiNota(ctx context.Context, r identita.Richiedente, id int32, testo *string) (Nota, error) {
	if r.Ruolo != identita.Operatore && r.Ruolo != identita.Corriere {
		return Nota{}, &errori.NonAutorizzato{Messaggio: "solo operatori logistici e corrieri possono scrivere note"}
	}
	e := &errori.Validazione{}
	controllaNota(e, testo, "obbligatoria")
	if err := e.Err(); err != nil {
		return Nota{}, err
	}
	return s.archivio.AggiungiNota(ctx, id, r, strings.TrimSpace(*testo), func(attuale Spedizione) error {
		if r.Ruolo != identita.Corriere {
			return nil
		}
		if !stessoCorriere(&r.ID, attuale.IDCorriere) {
			return &errori.NonAutorizzato{Messaggio: "la spedizione non è assegnata a te"}
		}
		if !slices.Contains(statiNoteCorriere, attuale.Stato) {
			return &errori.Conflitto{Messaggio: "il corriere può scrivere note solo su spedizioni ritirate o in consegna (stato attuale: \"" + attuale.Stato.String() + "\")"}
		}
		return nil
	})
}
