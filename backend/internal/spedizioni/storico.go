package spedizioni

import (
	"context"
	"time"

	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

type VoceStorico struct {
	ID            int32
	Operazione    string
	ValoreVecchio map[string]any
	ValoreNuovo   map[string]any
	ModificatoIl  time.Time
	IDOperatore   *int32
	IDCorriere    *int32
	NomeAutore    *string
	CognomeAutore *string
}

func (s *Servizio) Storico(ctx context.Context, r identita.Richiedente, id int32) ([]VoceStorico, error) {
	if r.Ruolo != identita.Amministratore && r.Ruolo != identita.Operatore {
		return nil, &errori.NonAutorizzato{Messaggio: "lo storico è visibile solo ad amministratori e operatori logistici"}
	}
	return s.archivio.Storico(ctx, id)
}
