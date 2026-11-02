package spedizioni

import (
	"context"
	"time"

	"lastmiledelivery/backend/internal/identita"
)

type DettaglioSpedizione struct {
	VoceElenco
	AnnullataIl              *time.Time
	TentativiRitiroFalliti   int
	TentativiConsegnaFalliti int
}

func (s *Servizio) Dettaglio(ctx context.Context, r identita.Richiedente, id int32) (DettaglioSpedizione, error) {
	vis, err := visibilitaPer(r, s.giorniCliente, s.adesso())
	if err != nil {
		return DettaglioSpedizione{}, err
	}
	d, err := s.archivio.Dettaglio(ctx, vis, id)
	if err != nil {
		return DettaglioSpedizione{}, err
	}
	nascondi(vis, &d.VoceElenco)
	return d, nil
}

func nascondi(vis Visibilita, v *VoceElenco) {
	if !vis.MostraCorriere {
		v.IDCorriere = nil
	}
	if vis.SoloNotePubbliche {
		for i := range v.Note {
			v.Note[i].Testo, v.Note[i].IDOperatore, v.Note[i].IDCorriere = "", nil, nil
		}
	}
}
