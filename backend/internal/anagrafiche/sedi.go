package anagrafiche

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"lastmiledelivery/backend/internal/database"
	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

// Sede è la sede operativa di una città: le sue spedizioni hanno ritiro e consegna in
// quella città e sono gestite dai suoi operatori e corrieri. Una sede per città.
// Una sede disattivata non accetta nuove spedizioni né nuovi corrieri o operatori; chi
// ci lavora entra ancora e porta a termine le spedizioni già aperte.
type Sede struct {
	ID        int64
	Nome      string
	Attiva    bool
	Indirizzo Indirizzo // indirizzo della sede: dà anche città, regione e nazione
	// Gruppo: il database delle spedizioni della sede (mappa sede → database). Si sceglie
	// alla creazione e poi non cambia.
	Gruppo   int
	CreataIl time.Time
}

type NuovaSede struct {
	Nome      *string
	Indirizzo NuovoIndirizzo
	// Gruppo: nil = 0, il database principale delle spedizioni
	Gruppo *int
}

// ModificaSede contiene i campi da cambiare (nil = invariato).
type ModificaSede struct {
	Nome      *string
	Indirizzo *NuovoIndirizzo
	Attiva    *bool
}

// DatiSede sono i campi già validati da salvare.
type DatiSede struct {
	Nome      *string
	Indirizzo *Indirizzo
	Attiva    *bool
}

type RicercaSedi struct {
	Ricerca
	Attiva *bool
}

const maxNomeSede = 100

// CercaSedi: l'amministratore vede tutte le sedi; operatori e corrieri solo la propria.
func (s *Servizio) CercaSedi(ctx context.Context, r identita.Richiedente, ric RicercaSedi) ([]Sede, int, error) {
	if r.Ruolo != identita.Amministratore && !r.Ruolo.HaSede() {
		return nil, 0, &errori.NonAutorizzato{Messaggio: "le sedi sono consultabili da amministratori, operatori e corrieri"}
	}
	if err := validaRicerca(ric.Ricerca); err != nil {
		return nil, 0, err
	}
	limitata, err := limitaAllaSede(r, ric.Ricerca)
	if err != nil {
		return nil, 0, err
	}
	ric.Ricerca = limitata
	return s.archivio.CercaSedi(ctx, ric)
}

func (s *Servizio) Sede(ctx context.Context, r identita.Richiedente, id int64) (Sede, error) {
	if r.Ruolo != identita.Amministratore && !(r.Ruolo.HaSede() && r.IDSede == id) {
		return Sede{}, &errori.NonAutorizzato{Messaggio: "una sede la vedono l'amministratore e chi ci lavora"}
	}
	return s.archivio.Sede(ctx, id)
}

// CreaSede: l'amministratore apre una nuova sede in una città che non ne ha ancora una.
// Città, regione e nazione si ricavano dall'indirizzo; la sede nasce con le impostazioni
// predefinite, che l'amministratore cambia con PATCH /api/sedi/{id}/impostazioni.
func (s *Servizio) CreaSede(ctx context.Context, r identita.Richiedente, n NuovaSede) (Sede, error) {
	if err := soloAmministratore(r, "creare una sede"); err != nil {
		return Sede{}, err
	}
	e := &errori.Validazione{}
	nome := nomeSede(e, n.Nome)
	i := validaIndirizzo(e, "indirizzo.", n.Indirizzo)
	gruppo := 0
	if n.Gruppo != nil {
		gruppo = *n.Gruppo
		if gruppo < 0 || gruppo > database.MassimoGruppo {
			e.Aggiungi("gruppo_spedizioni", fmt.Sprintf("tra 0 e %d", database.MassimoGruppo))
		}
	}
	if err := e.Err(); err != nil {
		return Sede{}, err
	}
	return s.archivio.CreaSede(ctx, nome, i, gruppo)
}

// ModificaSede cambia nome, indirizzo o stato della sede: solo i campi presenti.
// L'indirizzo resta nella città della sede, perché le spedizioni della sede sono legate
// alla sua città. Con attiva=false la sede non accetta nuove spedizioni né nuovi corrieri
// o operatori; quelle già aperte si chiudono normalmente.
func (s *Servizio) ModificaSede(ctx context.Context, r identita.Richiedente, id int64, m ModificaSede) (Sede, error) {
	if err := soloAmministratore(r, "modificare una sede"); err != nil {
		return Sede{}, err
	}
	e := &errori.Validazione{}
	if m.Nome == nil && m.Indirizzo == nil && m.Attiva == nil {
		e.Aggiungi("corpo", "indicare almeno un campo da modificare: nome, indirizzo o attiva")
		return Sede{}, e
	}
	d := DatiSede{Attiva: m.Attiva}
	if m.Nome != nil {
		n := nomeSede(e, m.Nome)
		d.Nome = &n
	}
	if m.Indirizzo != nil {
		i := validaIndirizzo(e, "indirizzo.", *m.Indirizzo)
		d.Indirizzo = &i
	}
	if err := e.Err(); err != nil {
		return Sede{}, err
	}
	return s.archivio.ModificaSede(ctx, id, d)
}

func nomeSede(e *errori.Validazione, nome *string) string {
	if nome == nil || strings.TrimSpace(*nome) == "" {
		e.Aggiungi("nome", "obbligatorio")
		return ""
	}
	n := strings.Join(strings.Fields(*nome), " ")
	if utf8.RuneCountInString(n) > maxNomeSede {
		e.Aggiungi("nome", "al massimo 100 caratteri")
	}
	return n
}
