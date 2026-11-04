package spedizioni

import "time"

type Spedizione struct {
	ID                       int32
	CodOrdine                string
	Stato                    Stato
	IsReso                   bool
	CreataIl                 time.Time
	DataoraRichiesta         time.Time
	IDClienteMittente        int32
	IDClienteDestinatario    int32
	IDIndirizzoRitiro        int32
	IDIndirizzoSpedizione    int32
	IDCorriere               *int32
	AnnullataIl              *time.Time
	TentativiRitiroFalliti   int
	TentativiConsegnaFalliti int
	UsciteInConsegna         int
}

type NuovaSpedizione struct {
	CodOrdine             string
	IDClienteMittente     int32
	IDClienteDestinatario int32
	IDIndirizzoRitiro     int32
	IDIndirizzoSpedizione int32
	DataoraRichiesta      time.Time
}

type DatiSpedizione struct {
	IsReso                *bool
	IDClienteMittente     *int32
	IDClienteDestinatario *int32
	IDIndirizzoRitiro     *int32
	IDIndirizzoSpedizione *int32
	DataoraRichiesta      *time.Time
}

func (d DatiSpedizione) altriCampi() bool {
	return d.IDClienteMittente != nil || d.IDClienteDestinatario != nil ||
		d.IDIndirizzoRitiro != nil || d.IDIndirizzoSpedizione != nil || d.DataoraRichiesta != nil
}

func (d DatiSpedizione) vuoti() bool {
	return d.IsReso == nil && !d.altriCampi()
}

func (d DatiSpedizione) ugualiA(s Spedizione) bool {
	return (d.IsReso == nil || *d.IsReso == s.IsReso) &&
		(d.IDClienteMittente == nil || *d.IDClienteMittente == s.IDClienteMittente) &&
		(d.IDClienteDestinatario == nil || *d.IDClienteDestinatario == s.IDClienteDestinatario) &&
		(d.IDIndirizzoRitiro == nil || *d.IDIndirizzoRitiro == s.IDIndirizzoRitiro) &&
		(d.IDIndirizzoSpedizione == nil || *d.IDIndirizzoSpedizione == s.IDIndirizzoSpedizione) &&
		(d.DataoraRichiesta == nil || d.DataoraRichiesta.Equal(s.DataoraRichiesta))
}

type CambioStato struct {
	Stato  Stato
	Nota   *string
	Motivo *string
}

type TentativoFallito struct {
	Tipo   TipoTentativo
	Nota   *string
	Motivo *string
}

type Modifica struct {
	NessunaModifica bool
	NuovoStato      Stato
	NuovoCorriere   *int32
	Annulla         bool
	Nota            *NotaDaSalvare
	Tentativo       TipoTentativo
	Dati            *DatiSpedizione
}

type NotaDaSalvare struct {
	Testo    string
	Motivo   *string
	Pubblica bool
}
