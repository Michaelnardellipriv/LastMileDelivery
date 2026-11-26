package configurazione

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
	"lastmiledelivery/backend/internal/posizioni"
)

// Limiti dei parametri: gli stessi dei CHECK sulla tabella impostazioni.
const (
	GiorniMinimi     = 1
	GiorniMassimi    = 365
	TentativiMinimi  = 1
	TentativiMassimi = 10
	maxDescrizione   = 100
)

// Impostazioni sono i parametri della gestione delle spedizioni che l'amministratore
// sceglie per ogni sede.
type Impostazioni struct {
	IDSede                   int64
	GiorniVisibilitaCliente  int
	MaxTentativiRitiro       int
	MaxTentativiConsegna     int
	IntervalloPosizioneSec   int
	ModificateIl             time.Time
	IDAmministratoreModifica *int64
}

// Regole sono le impostazioni della sede più i motivi attivi (uguali per tutte le sedi):
// quello che serve alle spedizioni.
type Regole struct {
	Impostazioni
	Motivi []string
}

// ModificaImpostazioni contiene solo i campi da cambiare (nil = invariato).
type ModificaImpostazioni struct {
	GiorniVisibilitaCliente *int
	MaxTentativiRitiro      *int
	MaxTentativiConsegna    *int
	IntervalloPosizioneSec  *int
}

func (m ModificaImpostazioni) vuota() bool {
	return m.GiorniVisibilitaCliente == nil && m.MaxTentativiRitiro == nil &&
		m.MaxTentativiConsegna == nil && m.IntervalloPosizioneSec == nil
}

type Motivo struct {
	ID          int64
	Descrizione string
	Attivo      bool
	CreatoIl    time.Time
}

type archivio interface {
	Regole(ctx context.Context, idSede int64) (Regole, error)
	AggiornaImpostazioni(ctx context.Context, idSede int64, m ModificaImpostazioni, idAmministratore int64) (Impostazioni, error)
	Motivi(ctx context.Context) ([]Motivo, error)
	MotiviAttivi(ctx context.Context) ([]string, error)
	CreaMotivo(ctx context.Context, descrizione string) (Motivo, error)
	ImpostaMotivoAttivo(ctx context.Context, id int64, attivo bool) (Motivo, error)
}

type Servizio struct {
	archivio archivio
}

func NuovoServizio(a archivio) *Servizio {
	return &Servizio{archivio: a}
}

func soloAmministratore(r identita.Richiedente, azione string) error {
	if r.Ruolo != identita.Amministratore {
		return &errori.NonAutorizzato{Messaggio: "solo un amministratore può " + azione}
	}
	return nil
}

// Impostazioni di una sede: l'amministratore le vede per tutte le sedi, operatori e
// corrieri solo per la propria.
func (s *Servizio) Impostazioni(ctx context.Context, r identita.Richiedente, idSede int64) (Impostazioni, error) {
	if r.Ruolo != identita.Amministratore && !(r.Ruolo.HaSede() && r.IDSede == idSede) {
		return Impostazioni{}, &errori.NonAutorizzato{Messaggio: "le impostazioni di una sede le vedono l'amministratore e chi lavora in quella sede"}
	}
	regole, err := s.archivio.Regole(ctx, idSede)
	return regole.Impostazioni, err
}

// ModificaImpostazioni cambia le impostazioni della sede scelta dall'amministratore.
func (s *Servizio) ModificaImpostazioni(ctx context.Context, r identita.Richiedente, idSede int64, m ModificaImpostazioni) (Impostazioni, error) {
	if err := soloAmministratore(r, "modificare le impostazioni"); err != nil {
		return Impostazioni{}, err
	}
	e := &errori.Validazione{}
	if m.vuota() {
		e.Aggiungi("corpo", "indicare almeno un parametro da modificare")
		return Impostazioni{}, e
	}
	intervallo := func(campo string, v *int, minimo, massimo int) {
		if v != nil && (*v < minimo || *v > massimo) {
			e.Aggiungi(campo, fmt.Sprintf("deve essere tra %d e %d", minimo, massimo))
		}
	}
	intervallo("giorni_visibilita_cliente", m.GiorniVisibilitaCliente, GiorniMinimi, GiorniMassimi)
	intervallo("max_tentativi_ritiro", m.MaxTentativiRitiro, TentativiMinimi, TentativiMassimi)
	intervallo("max_tentativi_consegna", m.MaxTentativiConsegna, TentativiMinimi, TentativiMassimi)
	intervallo("intervallo_posizione_sec", m.IntervalloPosizioneSec,
		posizioni.IntervalloMinimoSecondi, posizioni.IntervalloMassimoSecondi)
	if err := e.Err(); err != nil {
		return Impostazioni{}, err
	}
	return s.archivio.AggiornaImpostazioni(ctx, idSede, m, r.ID)
}

// MotiviAttivi è l'elenco per i menu a tendina: lo leggono tutti gli utenti autenticati.
func (s *Servizio) MotiviAttivi(ctx context.Context) ([]string, error) {
	return s.archivio.MotiviAttivi(ctx)
}

// Motivi restituisce tutti i motivi, anche quelli disattivati.
func (s *Servizio) Motivi(ctx context.Context, r identita.Richiedente) ([]Motivo, error) {
	if err := soloAmministratore(r, "gestire i motivi"); err != nil {
		return nil, err
	}
	return s.archivio.Motivi(ctx)
}

func (s *Servizio) CreaMotivo(ctx context.Context, r identita.Richiedente, descrizione *string) (Motivo, error) {
	if err := soloAmministratore(r, "gestire i motivi"); err != nil {
		return Motivo{}, err
	}
	e := &errori.Validazione{}
	var testo string
	if descrizione != nil {
		testo = strings.Join(strings.Fields(*descrizione), " ")
	}
	switch {
	case testo == "":
		e.Aggiungi("descrizione", "obbligatoria")
	case utf8.RuneCountInString(testo) > maxDescrizione:
		e.Aggiungi("descrizione", fmt.Sprintf("al massimo %d caratteri", maxDescrizione))
	}
	if err := e.Err(); err != nil {
		return Motivo{}, err
	}
	return s.archivio.CreaMotivo(ctx, testo)
}

func (s *Servizio) ImpostaMotivoAttivo(ctx context.Context, r identita.Richiedente, id int64, attivo *bool) (Motivo, error) {
	if err := soloAmministratore(r, "gestire i motivi"); err != nil {
		return Motivo{}, err
	}
	if attivo == nil {
		e := &errori.Validazione{}
		e.Aggiungi("attivo", "obbligatorio")
		return Motivo{}, e
	}
	return s.archivio.ImpostaMotivoAttivo(ctx, id, *attivo)
}
