package spedizioni

import (
	"context"
	"time"

	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

// OperazioneTentativo: voce dello storico per un tentativo fallito di ritiro o di consegna
// (le altre operazioni sono quelle di cambiamenti_spedizioni: creazione, modifica,
// annullamento).
const OperazioneTentativo = "tentativo"

type VoceStorico struct {
	ID            int64
	Operazione    string
	ValoreVecchio map[string]any
	ValoreNuovo   map[string]any
	ModificatoIl  time.Time
	IDOperatore   *int64
	IDCorriere    *int64
	NomeAutore    *string
	CognomeAutore *string
}

func (s *Servizio) Storico(ctx context.Context, r identita.Richiedente, id int64) ([]VoceStorico, error) {
	if r.Ruolo != identita.Amministratore && r.Ruolo != identita.Operatore {
		return nil, &errori.NonAutorizzato{Messaggio: "lo storico è visibile solo ad amministratori e operatori logistici"}
	}
	return s.archivio.Storico(ctx, r.Ambito(), id)
}
