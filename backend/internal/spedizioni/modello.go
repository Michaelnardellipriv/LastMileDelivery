package spedizioni

import "time"

type Spedizione struct {
	ID                       int64
	IDSede                   int64
	CodOrdine                string
	Stato                    Stato
	IsReso                   bool
	CreataIl                 time.Time
	DataoraRichiesta         time.Time
	RitirataIl               *time.Time
	IDClienteMittente        int64
	IDClienteDestinatario    int64
	IDIndirizzoRitiro        int64
	IDIndirizzoSpedizione    int64
	IDCorriere               *int64
	AnnullataIl              *time.Time
	TentativiRitiroFalliti   int
	TentativiConsegnaFalliti int
	// TentativoInUscita: nell'uscita in corso (dall'ultimo passaggio a "in consegna") il
	// corriere ha registrato almeno un tentativo di consegna fallito. Solo allora l'operatore
	// può riportare la spedizione in ritirata.
	TentativoInUscita bool
}

type NuovaSpedizione struct {
	CodOrdine             string
	IDClienteMittente     int64
	IDClienteDestinatario int64
	IDIndirizzoRitiro     int64
	IDIndirizzoSpedizione int64
	DataoraRichiesta      time.Time
	IDCorriere            *int64
}

type DatiSpedizione struct {
	IsReso                *bool
	IDClienteMittente     *int64
	IDClienteDestinatario *int64
	IDIndirizzoRitiro     *int64
	IDIndirizzoSpedizione *int64
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
	NuovoCorriere   *int64
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
