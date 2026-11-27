package posizioni

import (
	"context"
	"fmt"
	"time"

	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

// Limiti dell'intervallo di invio della posizione. L'intervallo lo modifica
// l'amministratore con PATCH /api/impostazioni (pacchetto configurazione).
const (
	IntervalloMinimoSecondi  = 5
	IntervalloMassimoSecondi = 3600
)

type Posizione struct {
	IDCorriere  int64
	Nome        string
	Cognome     string
	Latitudine  float64
	Longitudine float64
	RilevataIl  time.Time
	Spedizioni  []int64
}

// archivio tiene solo l'ultima posizione di ogni corriere. Ogni funzione riceve la
// sede: un corriere scrive nella sua, un operatore vede solo i corrieri della sua.
type archivio interface {
	Registra(ctx context.Context, idSede, idCorriere int64, lat, lon float64) (int64, error)
	Posizioni(ctx context.Context, idSede int64) ([]Posizione, error)
	Intervallo(ctx context.Context, idSede int64) (int64, error)
}

type Servizio struct {
	archivio archivio
}

func NuovoServizio(a archivio) *Servizio {
	return &Servizio{archivio: a}
}

func (s *Servizio) Invia(ctx context.Context, r identita.Richiedente, lat, lon *float64) (int64, error) {
	if r.Ruolo != identita.Corriere {
		return 0, &errori.NonAutorizzato{Messaggio: "solo un corriere può inviare la propria posizione"}
	}
	e := &errori.Validazione{}
	coordinata := func(campo string, v *float64, limite float64) {
		switch {
		case v == nil:
			e.Aggiungi(campo, "obbligatoria")
		case *v < -limite || *v > limite:
			e.Aggiungi(campo, fmt.Sprintf("deve essere tra %g e %g", -limite, limite))
		}
	}
	coordinata("latitudine", lat, 90)
	coordinata("longitudine", lon, 180)
	if err := e.Err(); err != nil {
		return 0, err
	}
	return s.archivio.Registra(ctx, r.IDSede, r.ID, *lat, *lon)
}

// Elenca l'ultima posizione dei corrieri in consegna della sede dell'operatore.
func (s *Servizio) Elenca(ctx context.Context, r identita.Richiedente) (int64, []Posizione, error) {
	if r.Ruolo != identita.Operatore {
		return 0, nil, &errori.NonAutorizzato{Messaggio: "la posizione dei corrieri è visibile solo agli operatori logistici"}
	}
	intervallo, err := s.archivio.Intervallo(ctx, r.IDSede)
	if err != nil {
		return 0, nil, err
	}
	posizioni, err := s.archivio.Posizioni(ctx, r.IDSede)
	return intervallo, posizioni, err
}

// Intervallo con cui i corrieri della sede inviano la posizione. L'amministratore lo
// legge e lo cambia per ogni sede con /api/sedi/{id}/impostazioni.
func (s *Servizio) Intervallo(ctx context.Context, r identita.Richiedente) (int64, error) {
	if !r.Ruolo.HaSede() {
		return 0, &errori.NonAutorizzato{Messaggio: "l'intervallo della posizione è quello della propria sede: " +
			"riservato a corrieri e operatori (l'amministratore usa GET /api/sedi/{id}/impostazioni)"}
	}
	return s.archivio.Intervallo(ctx, r.IDSede)
}
