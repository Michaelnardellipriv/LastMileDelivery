package spedizioni

import (
	"context"
	"fmt"
	"strings"
	"time"

	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

type archivio interface {
	Crea(ctx context.Context, idOperatore int32, n NuovaSpedizione) (Spedizione, error)
	Aggiorna(ctx context.Context, id int32, autore identita.Richiedente,
		decidi func(attuale Spedizione) (Modifica, error)) (Spedizione, bool, error)
	Elenca(ctx context.Context, v Visibilita, f FiltroElenco) ([]VoceElenco, error)
	Storico(ctx context.Context, id int32) ([]VoceStorico, error)
}

type Servizio struct {
	archivio      archivio
	maxTentativi  int
	giorniCliente int
	adesso        func() time.Time
}

func NuovoServizio(a archivio, maxTentativi, giorniCliente int) *Servizio {
	return &Servizio{archivio: a, maxTentativi: maxTentativi, giorniCliente: giorniCliente, adesso: time.Now}
}

func soloOperatore(r identita.Richiedente, azione string) error {
	if r.Ruolo != identita.Operatore {
		return &errori.NonAutorizzato{Messaggio: "solo un operatore logistico può " + azione}
	}
	return nil
}

func (s *Servizio) Crea(ctx context.Context, r identita.Richiedente, n NuovaSpedizione) (Spedizione, error) {
	if err := soloOperatore(r, "creare una spedizione"); err != nil {
		return Spedizione{}, err
	}
	n.CodOrdine = strings.TrimSpace(n.CodOrdine)
	if err := s.validaNuova(n).Err(); err != nil {
		return Spedizione{}, err
	}
	return s.archivio.Crea(ctx, r.ID, n)
}

func (s *Servizio) RegoleNuova(n NuovaSpedizione) []errori.ErroreCampo {
	n.CodOrdine = strings.TrimSpace(n.CodOrdine)
	return s.validaNuova(n).Dettagli
}

func (s *Servizio) validaNuova(n NuovaSpedizione) *errori.Validazione {
	e := &errori.Validazione{}
	controllaCodOrdine(e, n.CodOrdine)
	controllaID(e, "id_cliente_mittente", n.IDClienteMittente)
	controllaID(e, "id_cliente_destinatario", n.IDClienteDestinatario)
	controllaID(e, "id_indirizzo_ritiro", n.IDIndirizzoRitiro)
	controllaID(e, "id_indirizzo_spedizione", n.IDIndirizzoSpedizione)
	controllaIndirizzi(e, n.IDIndirizzoRitiro, n.IDIndirizzoSpedizione)
	controllaData(e, n.DataoraRichiesta, s.adesso())
	return e
}

func (s *Servizio) ModificaDati(ctx context.Context, r identita.Richiedente, id int32, d DatiSpedizione) (Spedizione, error) {
	if err := soloOperatore(r, "modificare i dati di una spedizione"); err != nil {
		return Spedizione{}, err
	}
	if err := s.validaDati(d); err != nil {
		return Spedizione{}, err
	}
	sped, _, err := s.archivio.Aggiorna(ctx, id, r, func(attuale Spedizione) (Modifica, error) {
		return decidiDati(attuale, d)
	})
	return sped, err
}

func (s *Servizio) validaDati(d DatiSpedizione) error {
	e := &errori.Validazione{}
	if d.vuoti() {
		e.Aggiungi("corpo", "indicare almeno un campo da modificare")
		return e
	}
	for campo, v := range map[string]*int32{
		"id_cliente_mittente":     d.IDClienteMittente,
		"id_cliente_destinatario": d.IDClienteDestinatario,
		"id_indirizzo_ritiro":     d.IDIndirizzoRitiro,
		"id_indirizzo_spedizione": d.IDIndirizzoSpedizione,
	} {
		if v != nil {
			controllaID(e, campo, *v)
		}
	}
	if d.DataoraRichiesta != nil {
		controllaData(e, *d.DataoraRichiesta, s.adesso())
	}
	return e.Err()
}

func decidiDati(attuale Spedizione, d DatiSpedizione) (Modifica, error) {
	if d.IsReso != nil && !attuale.Stato.AmmetteReso() {
		return Modifica{}, &errori.Conflitto{Messaggio: fmt.Sprintf(
			"is_reso si può impostare solo su una spedizione annullata, persa o non consegnata (stato attuale: %q)", attuale.Stato)}
	}
	if d.altriCampi() && !attuale.Stato.Modificabile() {
		return Modifica{}, &errori.Conflitto{Messaggio: fmt.Sprintf(
			"la spedizione si può modificare solo finché non è ritirata (stato attuale: %q)", attuale.Stato)}
	}
	ritiro, spedizione := attuale.IDIndirizzoRitiro, attuale.IDIndirizzoSpedizione
	if d.IDIndirizzoRitiro != nil {
		ritiro = *d.IDIndirizzoRitiro
	}
	if d.IDIndirizzoSpedizione != nil {
		spedizione = *d.IDIndirizzoSpedizione
	}
	e := &errori.Validazione{}
	controllaIndirizzi(e, ritiro, spedizione)
	if err := e.Err(); err != nil {
		return Modifica{}, err
	}
	if d.ugualiA(attuale) {
		return Modifica{NessunaModifica: true}, nil
	}
	return Modifica{NuovoStato: attuale.Stato, Dati: &d}, nil
}

func (s *Servizio) Assegna(ctx context.Context, r identita.Richiedente, id int32, idCorriere *int32) (Spedizione, bool, error) {
	if err := soloOperatore(r, "assegnare una spedizione"); err != nil {
		return Spedizione{}, false, err
	}
	e := &errori.Validazione{}
	if idCorriere == nil {
		e.Aggiungi("id_corriere", "obbligatorio")
	} else {
		controllaID(e, "id_corriere", *idCorriere)
	}
	if err := e.Err(); err != nil {
		return Spedizione{}, false, err
	}
	return s.archivio.Aggiorna(ctx, id, r, func(attuale Spedizione) (Modifica, error) {
		return s.decidiAssegnazione(attuale, idCorriere)
	})
}

func (s *Servizio) decidiAssegnazione(attuale Spedizione, idCorriere *int32) (Modifica, error) {
	nuovo := attuale.Stato
	switch attuale.Stato {
	case Creata:
		nuovo = Assegnata
	case Assegnata:
		if attuale.TentativiRitiroFalliti >= s.maxTentativi {
			return Modifica{}, &errori.Conflitto{Messaggio: fmt.Sprintf(
				"falliti %d tentativi di ritiro su %d: la spedizione va annullata", attuale.TentativiRitiroFalliti, s.maxTentativi)}
		}
	case Ritirata:
	default:
		return Modifica{}, &errori.Conflitto{Messaggio: fmt.Sprintf(
			"si può assegnare un corriere solo a una spedizione creata, assegnata o ritirata (stato attuale: %q)", attuale.Stato)}
	}
	if stessoCorriere(idCorriere, attuale.IDCorriere) {
		return Modifica{NessunaModifica: true}, nil
	}
	return Modifica{NuovoStato: nuovo, NuovoCorriere: idCorriere}, nil
}

func (s *Servizio) RegistraTentativo(ctx context.Context, r identita.Richiedente, id int32, t TentativoFallito) (Spedizione, error) {
	if r.Ruolo != identita.Corriere {
		return Spedizione{}, &errori.NonAutorizzato{Messaggio: "solo il corriere assegnato può registrare un tentativo"}
	}
	if err := validaTentativo(t); err != nil {
		return Spedizione{}, err
	}
	sped, _, err := s.archivio.Aggiorna(ctx, id, r, func(attuale Spedizione) (Modifica, error) {
		return s.decidiTentativo(r, attuale, t)
	})
	return sped, err
}

func validaTentativo(t TentativoFallito) error {
	e := &errori.Validazione{}
	if !t.Tipo.Valido() {
		e.Aggiungi("tipo", "valore non previsto: ammessi ritiro, consegna")
	}
	controllaNota(e, t.Nota, "obbligatoria per ogni tentativo non riuscito")
	if t.Tipo.Valido() {
		controllaMotivo(e, t.Motivo, motiviTentativo(t.Tipo), "")
	}
	return e.Err()
}

func (s *Servizio) decidiTentativo(r identita.Richiedente, attuale Spedizione, t TentativoFallito) (Modifica, error) {
	if !stessoCorriere(&r.ID, attuale.IDCorriere) {
		return Modifica{}, &errori.NonAutorizzato{Messaggio: "la spedizione non è assegnata a te"}
	}
	m := Modifica{Tentativo: t.Tipo, Nota: notaDaSalvare(t.Nota, t.Motivo)}

	if t.Tipo == TentativoRitiro {
		if attuale.Stato != Assegnata {
			return Modifica{}, &errori.Conflitto{Messaggio: fmt.Sprintf(
				"un tentativo di ritiro si registra solo su una spedizione assegnata (stato attuale: %q)", attuale.Stato)}
		}
		if attuale.TentativiRitiroFalliti >= s.maxTentativi {
			return Modifica{}, &errori.Conflitto{Messaggio: fmt.Sprintf(
				"già falliti %d tentativi di ritiro su %d: l'operatore deve annullare la spedizione",
				attuale.TentativiRitiroFalliti, s.maxTentativi)}
		}
		m.NuovoStato = Assegnata
		return m, nil
	}

	if attuale.Stato != InConsegna {
		return Modifica{}, &errori.Conflitto{Messaggio: fmt.Sprintf(
			"un tentativo di consegna si registra solo su una spedizione in consegna (stato attuale: %q)", attuale.Stato)}
	}
	if attuale.TentativiConsegnaFalliti >= attuale.UsciteInConsegna {
		return Modifica{}, &errori.Conflitto{Messaggio: "tentativo di consegna già registrato per questa uscita: " +
			"l'operatore deve riportare la spedizione in ritirata e il corriere rimetterla in consegna"}
	}
	m.NuovoStato = InConsegna
	if attuale.TentativiConsegnaFalliti+1 >= s.maxTentativi {
		if t.Motivo == nil {
			e := &errori.Validazione{}
			e.Aggiungi("motivo", fmt.Sprintf(
				"obbligatorio: è il tentativo %d su %d e la spedizione diventa non consegnata", attuale.TentativiConsegnaFalliti+1, s.maxTentativi))
			return Modifica{}, e
		}
		m.NuovoStato = NonConsegnata
	}
	return m, nil
}

func (s *Servizio) CambiaStato(ctx context.Context, r identita.Richiedente, id int32, c CambioStato) (Spedizione, error) {
	if r.Ruolo != identita.Operatore && r.Ruolo != identita.Corriere {
		return Spedizione{}, &errori.NonAutorizzato{Messaggio: "solo operatori logistici e corrieri possono cambiare lo stato"}
	}
	if err := validaCambio(c); err != nil {
		return Spedizione{}, err
	}
	sped, _, err := s.archivio.Aggiorna(ctx, id, r, func(attuale Spedizione) (Modifica, error) {
		return s.decidiStato(r, attuale, c)
	})
	return sped, err
}

func validaCambio(c CambioStato) error {
	e := &errori.Validazione{}
	switch {
	case !c.Stato.Valido():
		e.Aggiungi("stato", "valore non previsto: ammessi "+elencoStati())
	case altroEndpoint[c.Stato] != "":
		e.Aggiungi("stato", "\""+c.Stato.String()+"\" "+altroEndpoint[c.Stato])
	}
	if regola, chiusa := motiviStato[c.Stato]; chiusa {
		controllaMotivo(e, c.Motivo, regola.ammessi, "obbligatorio per "+regola.azione)
		controllaNota(e, c.Nota, "obbligatoria per "+regola.azione)
	} else {
		controllaNota(e, c.Nota, "")
		if c.Motivo != nil {
			e.Aggiungi("motivo", "ammesso solo con gli stati annullata e persa")
		}
	}
	return e.Err()
}

func (s *Servizio) decidiStato(r identita.Richiedente, attuale Spedizione, c CambioStato) (Modifica, error) {
	ruolo, ammessa := transizioni[passaggio{attuale.Stato, c.Stato}]
	if !ammessa {
		if err := rifiuta(r, attuale, c.Stato); err != nil {
			return Modifica{}, err
		}
		return Modifica{NessunaModifica: true}, nil
	}
	if ruolo != r.Ruolo {
		return Modifica{}, &errori.NonAutorizzato{Messaggio: "solo un " + ruolo.Nome() + " può portare la spedizione in \"" + c.Stato.String() + "\""}
	}
	if r.Ruolo == identita.Corriere && !stessoCorriere(&r.ID, attuale.IDCorriere) {
		return Modifica{}, &errori.NonAutorizzato{Messaggio: "la spedizione non è assegnata a te"}
	}
	if attuale.Stato == Assegnata && c.Stato == Ritirata && attuale.TentativiRitiroFalliti >= s.maxTentativi {
		return Modifica{}, &errori.Conflitto{Messaggio: fmt.Sprintf(
			"falliti %d tentativi di ritiro su %d: la spedizione deve essere annullata dall'operatore",
			attuale.TentativiRitiroFalliti, s.maxTentativi)}
	}
	return Modifica{NuovoStato: c.Stato, Annulla: c.Stato == Annullata, Nota: notaDaSalvare(c.Nota, c.Motivo)}, nil
}

func rifiuta(r identita.Richiedente, attuale Spedizione, stato Stato) error {
	if stato == attuale.Stato && raggiungibile(stato, r.Ruolo) {
		if r.Ruolo == identita.Corriere && !stessoCorriere(&r.ID, attuale.IDCorriere) {
			return &errori.NonAutorizzato{Messaggio: "la spedizione non è assegnata a te"}
		}
		return nil
	}
	altro := identita.Operatore
	if r.Ruolo == identita.Operatore {
		altro = identita.Corriere
	}
	if stato == attuale.Stato && raggiungibile(stato, altro) {
		return &errori.NonAutorizzato{Messaggio: "solo un " + altro.Nome() + " può portare la spedizione in \"" + stato.String() + "\""}
	}
	if attuale.Stato.Finale() {
		return &errori.Conflitto{Messaggio: "la spedizione è chiusa (stato \"" + attuale.Stato.String() + "\"): non può più cambiare stato"}
	}
	if stato == Annullata {
		return &errori.Conflitto{Messaggio: "la spedizione si può annullare solo prima del ritiro"}
	}
	return &errori.Conflitto{Messaggio: "da \"" + attuale.Stato.String() + "\" non si può passare a \"" + stato.String() + "\""}
}
