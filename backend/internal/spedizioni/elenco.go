package spedizioni

import (
	"context"
	"time"

	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

const (
	PerPaginaPredefinito = 20
	PerPaginaMassimo     = 100
)

type FiltroElenco struct {
	Stato      *Stato
	IDCorriere *int32
	Dal        *time.Time
	Al         *time.Time
	CodOrdine  *string
	Pagina     int
	PerPagina  int
}

type Indirizzo struct {
	Via    string
	Civico string
}

type VoceElenco struct {
	ID                    int32
	CodOrdine             string
	Stato                 Stato
	IsReso                bool
	CreataIl              time.Time
	DataoraRichiesta      time.Time
	IDClienteMittente     int32
	IDClienteDestinatario int32
	IndirizzoRitiro       Indirizzo
	IndirizzoSpedizione   Indirizzo
	IDCorriere            *int32
	ConclusaIl            *time.Time
	Note                  []Nota
}

type Visibilita struct {
	IDCorriere        *int32
	Stati             []Stato
	IDDestinatario    *int32
	ConcluseDal       *time.Time
	SoloNotePubbliche bool
	MostraCorriere    bool
}

var statiVisibiliAlCorriere = []Stato{Assegnata, Ritirata, InConsegna}

func visibilitaPer(r identita.Richiedente, giorniCliente int, adesso time.Time) (Visibilita, error) {
	switch r.Ruolo {
	case identita.Operatore, identita.Amministratore:
		return Visibilita{MostraCorriere: true}, nil
	case identita.Corriere:
		id := r.ID
		return Visibilita{IDCorriere: &id, Stati: statiVisibiliAlCorriere}, nil
	case identita.Cliente:
		id := r.ID
		dal := adesso.AddDate(0, 0, -giorniCliente)
		return Visibilita{IDDestinatario: &id, ConcluseDal: &dal, SoloNotePubbliche: true}, nil
	}
	return Visibilita{}, &errori.NonAutorizzato{Messaggio: "ruolo non autorizzato a vedere le spedizioni"}
}

func validaFiltro(f FiltroElenco) error {
	e := &errori.Validazione{}
	if f.Stato != nil && !f.Stato.Valido() {
		e.Aggiungi("stato", "valore non previsto: ammessi "+elencoStati())
	}
	if f.IDCorriere != nil && *f.IDCorriere <= 0 {
		e.Aggiungi("id_corriere", "deve essere un id positivo")
	}
	if f.Dal != nil && f.Al != nil && f.Al.Before(*f.Dal) {
		e.Aggiungi("al", "non può essere prima di dal")
	}
	if f.CodOrdine != nil && (*f.CodOrdine == "" || len([]rune(*f.CodOrdine)) > maxCodOrdine) {
		e.Aggiungi("cod_ordine", "da 1 a 50 caratteri")
	}
	if f.Pagina < 1 {
		e.Aggiungi("pagina", "deve essere almeno 1")
	}
	if f.PerPagina < 1 || f.PerPagina > PerPaginaMassimo {
		e.Aggiungi("per_pagina", "deve essere tra 1 e 100")
	}
	return e.Err()
}

func (s *Servizio) Elenca(ctx context.Context, r identita.Richiedente, f FiltroElenco) ([]VoceElenco, error) {
	vis, err := visibilitaPer(r, s.giorniCliente, s.adesso())
	if err != nil {
		return nil, err
	}
	if f.IDCorriere != nil && !vis.MostraCorriere {
		return nil, &errori.NonAutorizzato{Messaggio: "il filtro per corriere è disponibile solo per operatori e amministratori"}
	}
	if err := validaFiltro(f); err != nil {
		return nil, err
	}
	voci, err := s.archivio.Elenca(ctx, vis, f)
	if err != nil {
		return nil, err
	}
	for i := range voci {
		nascondi(vis, &voci[i])
	}
	return voci, nil
}
