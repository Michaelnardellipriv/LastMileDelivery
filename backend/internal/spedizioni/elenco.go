package spedizioni

import (
	"context"
	"fmt"
	"time"

	"lastmiledelivery/backend/internal/database"
	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

const (
	PerPaginaPredefinito = 10
	PerPaginaMassimo     = 100
	PaginaMassima        = 100_000
)

type FiltroElenco struct {
	Stato      *Stato
	IDCorriere *int64
	// IDSede filtra per sede: serve all'amministratore, che vede tutte le sedi
	IDSede    *int64
	Dal       *time.Time
	Al        *time.Time
	CodOrdine *string
	Pagina    int
	PerPagina int
}

type Indirizzo struct {
	Via     string
	Civico  string
	Citta   *string
	CAP     *string
	Regione *string
	Nazione *string
}

type Persona struct {
	ID       int64
	Nome     string
	Cognome  string
	Email    *string
	Telefono *string
}

type VoceElenco struct {
	ID                    int64
	IDSede                int64
	NomeSede              string
	CodOrdine             string
	Stato                 Stato
	IsReso                bool
	CreataIl              time.Time
	DataoraRichiesta      time.Time
	RitirataIl            *time.Time
	IDClienteMittente     int64
	IDClienteDestinatario int64
	IndirizzoRitiro       Indirizzo
	IndirizzoSpedizione   Indirizzo
	Mittente              *Persona
	Destinatario          Persona
	IDCorriere            *int64
	Corriere              *Persona
	AssegnataDa           *Persona // operatore che ha fatto l'assegnazione attuale
	ConclusaIl            *time.Time
	Note                  []Nota
}

type Visibilita struct {
	IDCorriere     *int64
	Stati          []Stato
	IDDestinatario *int64
	// ConcluseEntroGiorni: le spedizioni chiuse restano visibili per i giorni impostati
	// nella sede di ciascuna spedizione, contati da Adesso
	ConcluseEntroGiorni bool
	Adesso              time.Time
	SoloNotePubbliche   bool
	MostraCorriere      bool
	NascondiIDMittente  bool
	NascondiAssegnante  bool
}

var statiVisibiliAlCorriere = []Stato{Assegnata, Ritirata, InConsegna}

func (s *Servizio) visibilita(r identita.Richiedente) (Visibilita, error) {
	return visibilitaPer(r, s.adesso())
}

func visibilitaPer(r identita.Richiedente, adesso time.Time) (Visibilita, error) {
	switch r.Ruolo {
	case identita.Operatore, identita.Amministratore:
		return Visibilita{MostraCorriere: true}, nil
	case identita.Corriere:
		id := r.ID
		return Visibilita{IDCorriere: &id, Stati: statiVisibiliAlCorriere}, nil
	case identita.Cliente:
		id := r.ID
		return Visibilita{IDDestinatario: &id, ConcluseEntroGiorni: true, Adesso: adesso, SoloNotePubbliche: true,
			NascondiIDMittente: true, NascondiAssegnante: true}, nil
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
	if f.IDSede != nil && *f.IDSede <= 0 {
		e.Aggiungi("id_sede", "deve essere un id positivo")
	}
	if f.Dal != nil && f.Al != nil && f.Al.Before(*f.Dal) {
		e.Aggiungi("al", "non può essere prima di dal")
	}
	if f.CodOrdine != nil && (*f.CodOrdine == "" || len([]rune(*f.CodOrdine)) > maxCodOrdine) {
		e.Aggiungi("cod_ordine", "da 1 a 50 caratteri")
	}
	switch {
	case f.Pagina < 1:
		e.Aggiungi("pagina", "deve essere almeno 1")
	case f.Pagina > PaginaMassima:
		// oltre non serve a nessuno, e pagina × per_pagina resta un numero piccolo (anche
		// quando l'elenco di un cliente unisce i database di più gruppi)
		e.Aggiungi("pagina", fmt.Sprintf("al massimo %d", PaginaMassima))
	}
	if f.PerPagina < 1 || f.PerPagina > PerPaginaMassimo {
		e.Aggiungi("per_pagina", "deve essere tra 1 e 100")
	}
	return e.Err()
}

// controllaFiltroSede: l'amministratore sceglie sempre una sede (id_sede obbligatorio:
// ogni sede ha il suo database delle spedizioni, e non c'è un elenco di tutte le sedi
// insieme); operatori e corrieri vedono solo la propria, quindi possono indicare solo
// quella; i clienti non hanno sede.
func controllaFiltroSede(r identita.Richiedente, f FiltroElenco) error {
	if r.Ruolo == identita.Amministratore {
		if f.IDSede == nil {
			e := &errori.Validazione{}
			e.Aggiungi("id_sede", "obbligatorio per l'amministratore: scegliere una sede")
			return e
		}
		return nil
	}
	if f.IDSede == nil {
		return nil
	}
	if r.Ruolo.HaSede() && *f.IDSede == r.IDSede {
		return nil
	}
	return &errori.NonAutorizzato{Messaggio: "il filtro per sede è per gli amministratori: operatori e corrieri vedono solo la propria sede"}
}

func (s *Servizio) Elenca(ctx context.Context, r identita.Richiedente, f FiltroElenco) ([]VoceElenco, error) {
	vis, err := s.visibilita(r)
	if err != nil {
		return nil, err
	}
	if f.IDCorriere != nil && !vis.MostraCorriere {
		return nil, &errori.NonAutorizzato{Messaggio: "il filtro per corriere è disponibile solo per operatori e amministratori"}
	}
	if err := controllaFiltroSede(r, f); err != nil {
		return nil, err
	}
	if err := validaFiltro(f); err != nil {
		return nil, err
	}
	voci, err := s.archivio.Elenca(ctx, ambitoElenco(r, f), vis, f)
	if err != nil {
		return nil, err
	}
	for i := range voci {
		nascondi(vis, &voci[i])
	}
	return voci, nil
}

// ElencaPagina è Elenca più il numero totale di spedizioni che rispettano visibilità e
// filtri, per mostrare "elementi da–a di N" e il numero di pagine.
func (s *Servizio) ElencaPagina(ctx context.Context, r identita.Richiedente, f FiltroElenco) ([]VoceElenco, int, error) {
	voci, err := s.Elenca(ctx, r, f)
	if err != nil {
		return nil, 0, err
	}
	vis, err := s.visibilita(r)
	if err != nil {
		return nil, 0, err
	}
	totale, err := s.archivio.Conta(ctx, ambitoElenco(r, f), vis, f)
	return voci, totale, err
}

// ambitoElenco: l'amministratore vede la sede che ha scelto (controllaFiltroSede l'ha già
// resa obbligatoria), gli altri le sedi del loro ambito.
func ambitoElenco(r identita.Richiedente, f FiltroElenco) database.Ambito {
	if r.Ruolo == identita.Amministratore && f.IDSede != nil {
		return database.SoloSede(*f.IDSede)
	}
	return r.Ambito()
}
