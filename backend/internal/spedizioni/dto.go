package spedizioni

import (
	"time"

	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

type richiestaCrea struct {
	CodOrdine             *string `json:"cod_ordine"`
	IDClienteMittente     *int32  `json:"id_cliente_mittente"`
	IDClienteDestinatario *int32  `json:"id_cliente_destinatario"`
	IDIndirizzoRitiro     *int32  `json:"id_indirizzo_ritiro"`
	IDIndirizzoSpedizione *int32  `json:"id_indirizzo_spedizione"`
	DataoraRichiesta      *string `json:"dataora_richiesta_spedizione"`
}

type richiestaModifica struct {
	IsReso                *bool   `json:"is_reso"`
	IDClienteMittente     *int32  `json:"id_cliente_mittente"`
	IDClienteDestinatario *int32  `json:"id_cliente_destinatario"`
	IDIndirizzoRitiro     *int32  `json:"id_indirizzo_ritiro"`
	IDIndirizzoSpedizione *int32  `json:"id_indirizzo_spedizione"`
	DataoraRichiesta      *string `json:"dataora_richiesta_spedizione"`
}

type richiestaAssegnazione struct {
	IDCorriere *int32 `json:"id_corriere"`
}

type richiestaTentativo struct {
	Tipo   *string `json:"tipo"`
	Nota   *string `json:"nota"`
	Motivo *string `json:"motivo"`
}

type richiestaStato struct {
	Stato  *string `json:"stato"`
	Nota   *string `json:"nota"`
	Motivo *string `json:"motivo"`
}

type spedizioneDTO struct {
	ID                       int32      `json:"id"`
	CodOrdine                string     `json:"cod_ordine"`
	Stato                    string     `json:"stato"`
	IsReso                   bool       `json:"is_reso"`
	CreataIl                 time.Time  `json:"creata_il"`
	DataoraRichiesta         time.Time  `json:"dataora_richiesta_spedizione"`
	IDClienteMittente        int32      `json:"id_cliente_mittente"`
	IDClienteDestinatario    int32      `json:"id_cliente_destinatario"`
	IDIndirizzoRitiro        int32      `json:"id_indirizzo_ritiro"`
	IDIndirizzoSpedizione    int32      `json:"id_indirizzo_spedizione"`
	IDCorriere               *int32     `json:"id_corriere"`
	AnnullataIl              *time.Time `json:"annullata_il"`
	TentativiRitiroFalliti   int        `json:"tentativi_ritiro_falliti"`
	TentativiConsegnaFalliti int        `json:"tentativi_consegna_falliti"`
}

const messaggioData = "formato non valido: usare ISO 8601 con fuso orario, es. 2026-11-02T09:00:00+01:00"

func versoDTO(s Spedizione) spedizioneDTO {
	d := spedizioneDTO{
		ID: s.ID, CodOrdine: s.CodOrdine, Stato: s.Stato.String(), IsReso: s.IsReso,
		CreataIl: s.CreataIl.UTC(), DataoraRichiesta: s.DataoraRichiesta.UTC(),
		IDClienteMittente: s.IDClienteMittente, IDClienteDestinatario: s.IDClienteDestinatario,
		IDIndirizzoRitiro: s.IDIndirizzoRitiro, IDIndirizzoSpedizione: s.IDIndirizzoSpedizione,
		IDCorriere:             s.IDCorriere,
		TentativiRitiroFalliti: s.TentativiRitiroFalliti, TentativiConsegnaFalliti: s.TentativiConsegnaFalliti,
	}
	if s.AnnullataIl != nil {
		t := s.AnnullataIl.UTC()
		d.AnnullataIl = &t
	}
	return d
}

func (req richiestaCrea) versoDominio() (NuovaSpedizione, *errori.Validazione) {
	e := &errori.Validazione{}
	var n NuovaSpedizione

	if req.CodOrdine == nil {
		e.Aggiungi("cod_ordine", "obbligatorio")
	} else {
		n.CodOrdine = *req.CodOrdine
	}
	intero := func(campo string, p *int32, dest *int32) {
		if p == nil {
			e.Aggiungi(campo, "obbligatorio")
			return
		}
		*dest = *p
	}
	intero("id_cliente_mittente", req.IDClienteMittente, &n.IDClienteMittente)
	intero("id_cliente_destinatario", req.IDClienteDestinatario, &n.IDClienteDestinatario)
	intero("id_indirizzo_ritiro", req.IDIndirizzoRitiro, &n.IDIndirizzoRitiro)
	intero("id_indirizzo_spedizione", req.IDIndirizzoSpedizione, &n.IDIndirizzoSpedizione)

	if req.DataoraRichiesta == nil {
		e.Aggiungi("dataora_richiesta_spedizione", "obbligatoria")
	} else if t, err := time.Parse(time.RFC3339, *req.DataoraRichiesta); err != nil {
		e.Aggiungi("dataora_richiesta_spedizione", messaggioData)
	} else {
		n.DataoraRichiesta = t
	}
	return n, e
}

func (req richiestaModifica) versoDominio() (DatiSpedizione, error) {
	d := DatiSpedizione{
		IsReso:                req.IsReso,
		IDClienteMittente:     req.IDClienteMittente,
		IDClienteDestinatario: req.IDClienteDestinatario,
		IDIndirizzoRitiro:     req.IDIndirizzoRitiro,
		IDIndirizzoSpedizione: req.IDIndirizzoSpedizione,
	}
	if req.DataoraRichiesta != nil {
		t, err := time.Parse(time.RFC3339, *req.DataoraRichiesta)
		if err != nil {
			e := &errori.Validazione{}
			e.Aggiungi("dataora_richiesta_spedizione", messaggioData)
			return d, e
		}
		d.DataoraRichiesta = &t
	}
	return d, nil
}

type indirizzoDTO struct {
	Via    string `json:"via"`
	Civico string `json:"civico"`
}

type notaDTO struct {
	ID        int32     `json:"id"`
	Nota      string    `json:"nota"`
	IsInterna bool      `json:"is_interna"`
	Motivo    *string   `json:"motivo"`
	CreataIl  time.Time `json:"creata_il"`
}

type voceElencoDTO struct {
	ID                    int32        `json:"id"`
	CodOrdine             string       `json:"cod_ordine"`
	Stato                 string       `json:"stato"`
	IsReso                bool         `json:"is_reso"`
	CreataIl              time.Time    `json:"creata_il"`
	DataoraRichiesta      time.Time    `json:"dataora_richiesta_spedizione"`
	IDClienteMittente     int32        `json:"id_cliente_mittente"`
	IDClienteDestinatario int32        `json:"id_cliente_destinatario"`
	IndirizzoRitiro       indirizzoDTO `json:"indirizzo_ritiro"`
	IndirizzoSpedizione   indirizzoDTO `json:"indirizzo_spedizione"`
	IDCorriere            *int32       `json:"id_corriere,omitempty"`
	ConclusaIl            *time.Time   `json:"conclusa_il"`
	Note                  []notaDTO    `json:"note"`
}

type rispostaElenco struct {
	Dati      []voceElencoDTO `json:"dati"`
	Pagina    int             `json:"pagina"`
	PerPagina int             `json:"per_pagina"`
}

func versoElencoDTO(voci []VoceElenco, f FiltroElenco) rispostaElenco {
	r := rispostaElenco{Dati: make([]voceElencoDTO, len(voci)), Pagina: f.Pagina, PerPagina: f.PerPagina}
	for i, v := range voci {
		var conclusa *time.Time
		if v.ConclusaIl != nil {
			t := v.ConclusaIl.UTC()
			conclusa = &t
		}
		note := make([]notaDTO, len(v.Note))
		for j, n := range v.Note {
			note[j] = notaDTO{ID: n.ID, Nota: n.Testo, IsInterna: n.IsInterna, Motivo: n.Motivo, CreataIl: n.CreataIl.UTC()}
		}
		r.Dati[i] = voceElencoDTO{
			ID: v.ID, CodOrdine: v.CodOrdine, Stato: v.Stato.String(), IsReso: v.IsReso,
			CreataIl: v.CreataIl.UTC(), DataoraRichiesta: v.DataoraRichiesta.UTC(),
			IDClienteMittente: v.IDClienteMittente, IDClienteDestinatario: v.IDClienteDestinatario,
			IndirizzoRitiro:     indirizzoDTO{v.IndirizzoRitiro.Via, v.IndirizzoRitiro.Civico},
			IndirizzoSpedizione: indirizzoDTO{v.IndirizzoSpedizione.Via, v.IndirizzoSpedizione.Civico},
			IDCorriere:          v.IDCorriere,
			ConclusaIl:          conclusa,
			Note:                note,
		}
	}
	return r
}

type autoreDTO struct {
	Ruolo string `json:"ruolo"`
	ID    int32  `json:"id"`
}

type voceStoricoDTO struct {
	ID            int32          `json:"id"`
	Operazione    string         `json:"operazione"`
	ValoreVecchio map[string]any `json:"valore_vecchio"`
	ValoreNuovo   map[string]any `json:"valore_nuovo"`
	ModificatoIl  time.Time      `json:"modificato_il"`
	Autore        autoreDTO      `json:"autore"`
}

type rispostaStorico struct {
	Dati []voceStoricoDTO `json:"dati"`
}

func versoStoricoDTO(voci []VoceStorico) rispostaStorico {
	r := rispostaStorico{Dati: make([]voceStoricoDTO, len(voci))}
	for i, v := range voci {
		autore := autoreDTO{}
		switch {
		case v.IDOperatore != nil:
			autore = autoreDTO{Ruolo: string(identita.Operatore), ID: *v.IDOperatore}
		case v.IDCorriere != nil:
			autore = autoreDTO{Ruolo: string(identita.Corriere), ID: *v.IDCorriere}
		}
		r.Dati[i] = voceStoricoDTO{
			ID: v.ID, Operazione: v.Operazione,
			ValoreVecchio: v.ValoreVecchio, ValoreNuovo: v.ValoreNuovo,
			ModificatoIl: v.ModificatoIl.UTC(), Autore: autore,
		}
	}
	return r
}
