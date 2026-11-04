package posizioni

import (
	"context"
	"fmt"
	"time"

	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

const (
	IntervalloMinimoSecondi  = 5
	IntervalloMassimoSecondi = 3600
)

type Posizione struct {
	IDCorriere  int32
	Nome        string
	Cognome     string
	Latitudine  float64
	Longitudine float64
	RilevataIl  time.Time
	Spedizioni  []int32
}

type archivio interface {
	Registra(ctx context.Context, idCorriere int32, lat, lon float64) (int32, error)
	Posizioni(ctx context.Context) ([]Posizione, error)
	Intervallo(ctx context.Context) (int32, error)
	AggiornaIntervallo(ctx context.Context, secondi, idAmministratore int32) error
}

type Servizio struct {
	archivio archivio
}

func NuovoServizio(a archivio) *Servizio {
	return &Servizio{archivio: a}
}

func (s *Servizio) Invia(ctx context.Context, r identita.Richiedente, lat, lon *float64) (int32, error) {
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
	return s.archivio.Registra(ctx, r.ID, *lat, *lon)
}

func (s *Servizio) Elenca(ctx context.Context, r identita.Richiedente) (int32, []Posizione, error) {
	if r.Ruolo != identita.Operatore {
		return 0, nil, &errori.NonAutorizzato{Messaggio: "la posizione dei corrieri è visibile solo agli operatori logistici"}
	}
	intervallo, err := s.archivio.Intervallo(ctx)
	if err != nil {
		return 0, nil, err
	}
	posizioni, err := s.archivio.Posizioni(ctx)
	return intervallo, posizioni, err
}

func (s *Servizio) Intervallo(ctx context.Context, r identita.Richiedente) (int32, error) {
	if r.Ruolo == identita.Cliente {
		return 0, &errori.NonAutorizzato{Messaggio: "l'intervallo della posizione è riservato a corrieri, operatori e amministratori"}
	}
	return s.archivio.Intervallo(ctx)
}

func (s *Servizio) ModificaIntervallo(ctx context.Context, r identita.Richiedente, secondi *int32) (int32, error) {
	if r.Ruolo != identita.Amministratore {
		return 0, &errori.NonAutorizzato{Messaggio: "solo un amministratore può modificare l'intervallo della posizione"}
	}
	e := &errori.Validazione{}
	switch {
	case secondi == nil:
		e.Aggiungi("intervallo_secondi", "obbligatorio")
	case *secondi < IntervalloMinimoSecondi || *secondi > IntervalloMassimoSecondi:
		e.Aggiungi("intervallo_secondi", fmt.Sprintf("deve essere tra %d e %d", IntervalloMinimoSecondi, IntervalloMassimoSecondi))
	}
	if err := e.Err(); err != nil {
		return 0, err
	}
	return *secondi, s.archivio.AggiornaIntervallo(ctx, *secondi, r.ID)
}
