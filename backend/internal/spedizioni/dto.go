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
	// decisa dal sistema: se arriva viene rifiutata con un messaggio chiaro
	RitirataIl any    `json:"ritirata_il"`
	IDCorriere *int32 `json:"id_corriere"`
}

type richiestaModifica struct {
	IsReso                *bool   `json:"is_reso"`
	IDClienteMittente     *int32  `json:"id_cliente_mittente"`
	IDClienteDestinatario *int32  `json:"id_cliente_destinatario"`
	IDIndirizzoRitiro     *int32  `json:"id_indirizzo_ritiro"`
	IDIndirizzoSpedizione *int32  `json:"id_indirizzo_spedizione"`
	DataoraRichiesta      *string `json:"dataora_richiesta_spedizione"`
	RitirataIl            any     `json:"ritirata_il"`
}

type richiestaAssegnazione struct {
	IDCorriere *int32 `json:"id_corriere"`
}

type richiestaTentativo struct {
	Tipo   *string `json:"tipo"`
	Nota   *string `json:"nota"`
	Motivo *string `json:"motivo"`
}

type richiestaNota struct {
	Nota *string `json:"nota"`
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
	RitirataIl               *time.Time `json:"ritirata_il"`
	IDClienteMittente        int32      `json:"id_cliente_mittente"`
	IDClienteDestinatario    int32      `json:"id_cliente_destinatario"`
	IDIndirizzoRitiro        int32      `json:"id_indirizzo_ritiro"`
	IDIndirizzoSpedizione    int32      `json:"id_indirizzo_spedizione"`
	IDCorriere               *int32     `json:"id_corriere"`
	AnnullataIl              *time.Time `json:"annullata_il"`
	TentativiRitiroFalliti   int        `json:"tentativi_ritiro_falliti"`
	TentativiConsegnaFalliti int        `json:"tentativi_consegna_falliti"`
}

const (
	messaggioData       = "formato non valido: usare ISO 8601 con fuso orario, es. 2026-11-02T09:00:00+01:00"
	messaggioDataRitiro = "non si imposta: data e ora del ritiro le registra il sistema quando il corriere porta la spedizione in ritirata"
)

// dataRitiroDiSistema rifiuta ritirata_il: la decide il sistema, non il client.
func dataRitiroDiSistema(e *errori.Validazione, ritiro any) {
	if ritiro != nil {
		e.Aggiungi("ritirata_il", messaggioDataRitiro)
	}
}

// leggiData interpreta una data ISO 8601 con fuso orario.
func leggiData(e *errori.Validazione, campo, v string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		e.Aggiungi(campo, messaggioData)
		return time.Time{}, false
	}
	return t, true
}

func versoDTO(s Spedizione) spedizioneDTO {
	d := spedizioneDTO{
		ID: s.ID, CodOrdine: s.CodOrdine, Stato: s.Stato.String(), IsReso: s.IsReso,
		CreataIl: s.CreataIl.UTC(), DataoraRichiesta: s.DataoraRichiesta.UTC(),
		IDClienteMittente: s.IDClienteMittente, IDClienteDestinatario: s.IDClienteDestinatario,
		IDIndirizzoRitiro: s.IDIndirizzoRitiro, IDIndirizzoSpedizione: s.IDIndirizzoSpedizione,
		IDCorriere:             s.IDCorriere,
		TentativiRitiroFalliti: s.TentativiRitiroFalliti, TentativiConsegnaFalliti: s.TentativiConsegnaFalliti,
	}
	d.AnnullataIl = inUTC(s.AnnullataIl)
	d.RitirataIl = inUTC(s.RitirataIl)
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
	} else if t, ok := leggiData(e, "dataora_richiesta_spedizione", *req.DataoraRichiesta); ok {
		n.DataoraRichiesta = t
	}
	dataRitiroDiSistema(e, req.RitirataIl)
	n.IDCorriere = req.IDCorriere
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
	e := &errori.Validazione{}
	if req.DataoraRichiesta != nil {
		if t, ok := leggiData(e, "dataora_richiesta_spedizione", *req.DataoraRichiesta); ok {
			d.DataoraRichiesta = &t
		}
	}
	dataRitiroDiSistema(e, req.RitirataIl)
	return d, e.Err()
}

type indirizzoDTO struct {
	Via     string  `json:"via"`
	Civico  string  `json:"civico"`
	Citta   *string `json:"citta"`
	CAP     *string `json:"cap"`
	Regione *string `json:"regione"`
	Nazione *string `json:"nazione"`
}

type personaDTO struct {
	ID       *int32  `json:"id,omitempty"`
	Nome     string  `json:"nome"`
	Cognome  string  `json:"cognome"`
	Email    *string `json:"email,omitempty"`
	Telefono *string `json:"telefono,omitempty"`
}

func versoPersonaDTO(p Persona) personaDTO {
	d := personaDTO{Nome: p.Nome, Cognome: p.Cognome, Email: p.Email, Telefono: p.Telefono}
	if p.ID != 0 {
		id := p.ID
		d.ID = &id
	}
	return d
}

func versoIndirizzoDTO(i Indirizzo) indirizzoDTO {
	return indirizzoDTO{Via: i.Via, Civico: i.Civico, Citta: i.Citta, CAP: i.CAP, Regione: i.Regione, Nazione: i.Nazione}
}

type notaDTO struct {
	ID        int32      `json:"id"`
	Nota      string     `json:"nota,omitempty"`
	IsInterna bool       `json:"is_interna"`
	Motivo    *string    `json:"motivo"`
	CreataIl  time.Time  `json:"creata_il"`
	Autore    *autoreDTO `json:"autore,omitempty"`
}

func versoNotaDTO(n Nota) notaDTO {
	d := notaDTO{ID: n.ID, Nota: n.Testo, IsInterna: n.IsInterna, Motivo: n.Motivo, CreataIl: n.CreataIl.UTC(),
		Autore: autoreDa(n.IDOperatore, n.IDCorriere)}
	if d.Autore != nil {
		d.Autore.Nome, d.Autore.Cognome = n.NomeAutore, n.CognomeAutore
	}
	return d
}

type voceElencoDTO struct {
	ID                    int32        `json:"id"`
	CodOrdine             string       `json:"cod_ordine"`
	Stato                 string       `json:"stato"`
	IsReso                bool         `json:"is_reso"`
	CreataIl              time.Time    `json:"creata_il"`
	DataoraRichiesta      time.Time    `json:"dataora_richiesta_spedizione"`
	RitirataIl            *time.Time   `json:"ritirata_il"`
	IDClienteMittente     *int32       `json:"id_cliente_mittente,omitempty"`
	IDClienteDestinatario int32        `json:"id_cliente_destinatario"`
	IndirizzoRitiro       indirizzoDTO `json:"indirizzo_ritiro"`
	IndirizzoSpedizione   indirizzoDTO `json:"indirizzo_spedizione"`
	Mittente              *personaDTO  `json:"mittente,omitempty"`
	Destinatario          personaDTO   `json:"destinatario"`
	IDCorriere            *int32       `json:"id_corriere,omitempty"`
	Corriere              *personaDTO  `json:"corriere,omitempty"`
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
		r.Dati[i] = versoVoceDTO(v)
	}
	return r
}

func versoVoceDTO(v VoceElenco) voceElencoDTO {
	note := make([]notaDTO, len(v.Note))
	for j, n := range v.Note {
		note[j] = versoNotaDTO(n)
	}
	var corriere, mittente *personaDTO
	var idMittente *int32
	if v.Corriere != nil {
		c := versoPersonaDTO(*v.Corriere)
		corriere = &c
	}
	if v.Mittente != nil {
		m := versoPersonaDTO(*v.Mittente)
		mittente, idMittente = &m, m.ID
	}
	return voceElencoDTO{
		ID: v.ID, CodOrdine: v.CodOrdine, Stato: v.Stato.String(), IsReso: v.IsReso,
		CreataIl: v.CreataIl.UTC(), DataoraRichiesta: v.DataoraRichiesta.UTC(), RitirataIl: inUTC(v.RitirataIl),
		IDClienteMittente: idMittente, IDClienteDestinatario: v.IDClienteDestinatario,
		IndirizzoRitiro:     versoIndirizzoDTO(v.IndirizzoRitiro),
		IndirizzoSpedizione: versoIndirizzoDTO(v.IndirizzoSpedizione),
		Mittente:            mittente,
		Destinatario:        versoPersonaDTO(v.Destinatario),
		IDCorriere:          v.IDCorriere,
		Corriere:            corriere,
		ConclusaIl:          inUTC(v.ConclusaIl),
		Note:                note,
	}
}

type dettaglioDTO struct {
	voceElencoDTO
	AnnullataIl              *time.Time `json:"annullata_il"`
	TentativiRitiroFalliti   int        `json:"tentativi_ritiro_falliti"`
	TentativiConsegnaFalliti int        `json:"tentativi_consegna_falliti"`
}

func versoDettaglioDTO(d DettaglioSpedizione) dettaglioDTO {
	return dettaglioDTO{
		voceElencoDTO:            versoVoceDTO(d.VoceElenco),
		AnnullataIl:              inUTC(d.AnnullataIl),
		TentativiRitiroFalliti:   d.TentativiRitiroFalliti,
		TentativiConsegnaFalliti: d.TentativiConsegnaFalliti,
	}
}

func inUTC(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

type autoreDTO struct {
	Ruolo   string  `json:"ruolo"`
	ID      int32   `json:"id"`
	Nome    *string `json:"nome,omitempty"`
	Cognome *string `json:"cognome,omitempty"`
}

type voceStoricoDTO struct {
	ID            int32          `json:"id"`
	Operazione    string         `json:"operazione"`
	ValoreVecchio map[string]any `json:"valore_vecchio"`
	ValoreNuovo   map[string]any `json:"valore_nuovo"`
	ModificatoIl  time.Time      `json:"modificato_il"`
	Autore        autoreDTO      `json:"autore"`
}

type rispostaMotivi struct {
	Dati []Motivo `json:"dati"`
}

type rispostaStorico struct {
	Dati []voceStoricoDTO `json:"dati"`
}

func autoreDa(idOperatore, idCorriere *int32) *autoreDTO {
	switch {
	case idOperatore != nil:
		return &autoreDTO{Ruolo: string(identita.Operatore), ID: *idOperatore}
	case idCorriere != nil:
		return &autoreDTO{Ruolo: string(identita.Corriere), ID: *idCorriere}
	}
	return nil
}

func versoStoricoDTO(voci []VoceStorico) rispostaStorico {
	r := rispostaStorico{Dati: make([]voceStoricoDTO, len(voci))}
	for i, v := range voci {
		autore := autoreDTO{}
		if a := autoreDa(v.IDOperatore, v.IDCorriere); a != nil {
			autore = *a
			autore.Nome, autore.Cognome = v.NomeAutore, v.CognomeAutore
		}
		r.Dati[i] = voceStoricoDTO{
			ID: v.ID, Operazione: v.Operazione,
			ValoreVecchio: v.ValoreVecchio, ValoreNuovo: v.ValoreNuovo,
			ModificatoIl: v.ModificatoIl.UTC(), Autore: autore,
		}
	}
	return r
}
