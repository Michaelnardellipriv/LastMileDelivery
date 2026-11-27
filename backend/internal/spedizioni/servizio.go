package spedizioni

import (
	"context"
	"fmt"
	"strings"
	"time"

	"lastmiledelivery/backend/internal/configurazione"
	"lastmiledelivery/backend/internal/database"
	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

// archivio legge e scrive le spedizioni. Ogni funzione riceve l'ambito delle sedi del
// richiedente (o la sua sede): è il repository che lo mette nelle query, chi chiama non
// può dimenticarlo. Il database lo controlla una seconda volta (Row-Level Security).
type archivio interface {
	Crea(ctx context.Context, idSede, idOperatore int64, n NuovaSpedizione) (Spedizione, error)
	Aggiorna(ctx context.Context, ambito database.Ambito, id int64, autore identita.Richiedente,
		decidi func(attuale Spedizione) (Modifica, error)) (Spedizione, bool, error)
	Elenca(ctx context.Context, ambito database.Ambito, v Visibilita, f FiltroElenco) ([]VoceElenco, error)
	Conta(ctx context.Context, ambito database.Ambito, v Visibilita, f FiltroElenco) (int, error)
	Storico(ctx context.Context, ambito database.Ambito, id int64) ([]VoceStorico, error)
	Dettaglio(ctx context.Context, ambito database.Ambito, v Visibilita, id int64) (DettaglioSpedizione, error)
	AggiungiNota(ctx context.Context, ambito database.Ambito, id int64, autore identita.Richiedente, testo string,
		verifica func(attuale Spedizione) error) (Nota, error)
	// SedeCorriere dice in quale sede lavora il corriere (trovato=false se non esiste)
	SedeCorriere(ctx context.Context, idCorriere int64) (idSede int64, trovato bool, err error)
}

// fonteRegole dà i parametri impostati dall'amministratore per la sede (tentativi,
// giorni di visibilità) e i motivi: si leggono a ogni operazione, così una modifica
// vale subito.
type fonteRegole interface {
	Regole(ctx context.Context, idSede int64) (configurazione.Regole, error)
}

type Servizio struct {
	archivio archivio
	regole   fonteRegole
	adesso   func() time.Time
}

func NuovoServizio(a archivio, r fonteRegole) *Servizio {
	return &Servizio{archivio: a, regole: r, adesso: time.Now}
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
	// la spedizione nasce nella sede dell'operatore: ritiro e consegna devono essere
	// nella sua città (lo garantisce il database) e il corriere deve essere della sede
	if n.IDCorriere != nil {
		if err := s.corriereDellaSede(ctx, r.IDSede, *n.IDCorriere); err != nil {
			return Spedizione{}, err
		}
	}
	return s.archivio.Crea(ctx, r.IDSede, r.ID, n)
}

// corriereDellaSede: una spedizione si affida solo a un corriere della sua sede, perché
// la consegna resta in città ed è gestita da operatori e corrieri di quella città.
func (s *Servizio) corriereDellaSede(ctx context.Context, idSede, idCorriere int64) error {
	sedeCorriere, trovato, err := s.archivio.SedeCorriere(ctx, idCorriere)
	if err != nil {
		return err
	}
	if !trovato {
		return &errori.RiferimentoNonTrovato{Campo: "id_corriere", Messaggio: "corriere non trovato"}
	}
	if sedeCorriere != idSede {
		return &errori.Conflitto{
			Messaggio: "il corriere lavora in un'altra sede: una spedizione si assegna solo a corrieri della sua sede",
			Dettagli:  []errori.ErroreCampo{{Campo: "id_corriere", Messaggio: "corriere di un'altra sede"}},
		}
	}
	return nil
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
	controllaDataRichiesta(e, n.DataoraRichiesta, s.adesso())
	if n.IDCorriere != nil {
		controllaID(e, "id_corriere", *n.IDCorriere)
	}
	return e
}

func (s *Servizio) ModificaDati(ctx context.Context, r identita.Richiedente, id int64, d DatiSpedizione) (Spedizione, error) {
	if err := soloOperatore(r, "modificare i dati di una spedizione"); err != nil {
		return Spedizione{}, err
	}
	if err := s.validaDati(d); err != nil {
		return Spedizione{}, err
	}
	sped, _, err := s.archivio.Aggiorna(ctx, r.Ambito(), id, r, func(attuale Spedizione) (Modifica, error) {
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
	for campo, v := range map[string]*int64{
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
		controllaDataRichiesta(e, *d.DataoraRichiesta, s.adesso())
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

func (s *Servizio) Assegna(ctx context.Context, r identita.Richiedente, id int64, idCorriere *int64) (Spedizione, bool, error) {
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
	if err := s.corriereDellaSede(ctx, r.IDSede, *idCorriere); err != nil {
		return Spedizione{}, false, err
	}
	reg, err := s.regole.Regole(ctx, r.IDSede)
	if err != nil {
		return Spedizione{}, false, err
	}
	return s.archivio.Aggiorna(ctx, r.Ambito(), id, r, func(attuale Spedizione) (Modifica, error) {
		return decidiAssegnazione(reg, attuale, idCorriere)
	})
}

func decidiAssegnazione(reg configurazione.Regole, attuale Spedizione, idCorriere *int64) (Modifica, error) {
	nuovo := attuale.Stato
	switch attuale.Stato {
	case Creata:
		nuovo = Assegnata
	case Assegnata:
		if attuale.TentativiRitiroFalliti >= reg.MaxTentativiRitiro {
			return Modifica{}, &errori.Conflitto{Messaggio: fmt.Sprintf(
				"falliti %d tentativi di ritiro su %d: la spedizione va annullata", attuale.TentativiRitiroFalliti, reg.MaxTentativiRitiro)}
		}
	case Ritirata:
		if attuale.TentativiConsegnaFalliti == 0 {
			return Modifica{}, &errori.Conflitto{Messaggio: "una spedizione ritirata si può riassegnare solo dopo almeno un tentativo di consegna fallito"}
		}
	default:
		return Modifica{}, &errori.Conflitto{Messaggio: fmt.Sprintf(
			"si può assegnare un corriere solo a una spedizione creata, assegnata o ritirata (stato attuale: %q)", attuale.Stato)}
	}
	if stessoCorriere(idCorriere, attuale.IDCorriere) {
		return Modifica{NessunaModifica: true}, nil
	}
	return Modifica{NuovoStato: nuovo, NuovoCorriere: idCorriere}, nil
}

func (s *Servizio) RegistraTentativo(ctx context.Context, r identita.Richiedente, id int64, t TentativoFallito) (Spedizione, error) {
	if r.Ruolo != identita.Corriere {
		return Spedizione{}, &errori.NonAutorizzato{Messaggio: "solo il corriere assegnato può registrare un tentativo"}
	}
	reg, err := s.regole.Regole(ctx, r.IDSede)
	if err != nil {
		return Spedizione{}, err
	}
	if err := validaTentativo(reg, t); err != nil {
		return Spedizione{}, err
	}
	sped, _, err := s.archivio.Aggiorna(ctx, r.Ambito(), id, r, func(attuale Spedizione) (Modifica, error) {
		return decidiTentativo(reg, r, attuale, t)
	})
	return sped, err
}

func validaTentativo(reg configurazione.Regole, t TentativoFallito) error {
	e := &errori.Validazione{}
	if !t.Tipo.Valido() {
		e.Aggiungi("tipo", "valore non previsto: ammessi ritiro, consegna")
	}
	controllaNota(e, t.Nota, "obbligatoria per ogni tentativo non riuscito")
	if t.Tipo.Valido() {
		controllaMotivo(e, reg.Motivi, t.Motivo, "obbligatorio per ogni tentativo non riuscito")
	}
	return e.Err()
}

func decidiTentativo(reg configurazione.Regole, r identita.Richiedente, attuale Spedizione, t TentativoFallito) (Modifica, error) {
	if !stessoCorriere(&r.ID, attuale.IDCorriere) {
		return Modifica{}, &errori.NonAutorizzato{Messaggio: "la spedizione non è assegnata a te"}
	}
	m := Modifica{Tentativo: t.Tipo, Nota: notaDaSalvare(t.Nota, t.Motivo)}
	m.Nota.Pubblica = true

	if t.Tipo == TentativoRitiro {
		if attuale.Stato != Assegnata {
			return Modifica{}, &errori.Conflitto{Messaggio: fmt.Sprintf(
				"un tentativo di ritiro si registra solo su una spedizione assegnata (stato attuale: %q)", attuale.Stato)}
		}
		if attuale.TentativiRitiroFalliti >= reg.MaxTentativiRitiro {
			return Modifica{}, &errori.Conflitto{Messaggio: fmt.Sprintf(
				"già falliti %d tentativi di ritiro su %d: l'operatore deve annullare la spedizione",
				attuale.TentativiRitiroFalliti, reg.MaxTentativiRitiro)}
		}
		m.NuovoStato = Assegnata
		return m, nil
	}

	if attuale.Stato != InConsegna {
		return Modifica{}, &errori.Conflitto{Messaggio: fmt.Sprintf(
			"un tentativo di consegna si registra solo su una spedizione in consegna (stato attuale: %q)", attuale.Stato)}
	}
	// un solo tentativo per uscita: per riprovare la spedizione torna in ritirata (operatore)
	// e il corriere la rimette in consegna
	if attuale.TentativoInUscita {
		return Modifica{}, &errori.Conflitto{Messaggio: messaggioUscitaFallita}
	}
	m.NuovoStato = InConsegna
	if attuale.TentativiConsegnaFalliti+1 >= reg.MaxTentativiConsegna {
		m.NuovoStato = NonConsegnata
	}
	return m, nil
}

func (s *Servizio) CambiaStato(ctx context.Context, r identita.Richiedente, id int64, c CambioStato) (Spedizione, error) {
	if r.Ruolo != identita.Operatore && r.Ruolo != identita.Corriere {
		return Spedizione{}, &errori.NonAutorizzato{Messaggio: "solo operatori logistici e corrieri possono cambiare lo stato"}
	}
	reg, err := s.regole.Regole(ctx, r.IDSede)
	if err != nil {
		return Spedizione{}, err
	}
	if err := validaCambio(reg, c); err != nil {
		return Spedizione{}, err
	}
	sped, _, err := s.archivio.Aggiorna(ctx, r.Ambito(), id, r, func(attuale Spedizione) (Modifica, error) {
		return decidiStato(reg, r, attuale, c)
	})
	return sped, err
}

func validaCambio(reg configurazione.Regole, c CambioStato) error {
	e := &errori.Validazione{}
	switch {
	case !c.Stato.Valido():
		e.Aggiungi("stato", "valore non previsto: ammessi "+elencoStati())
	case altroEndpoint[c.Stato] != "":
		e.Aggiungi("stato", "\""+c.Stato.String()+"\" "+altroEndpoint[c.Stato])
	}
	if _, conMotivo := azioniConMotivo[c.Stato]; conMotivo {
		controllaMotivo(e, reg.Motivi, c.Motivo, "")
		controllaNota(e, c.Nota, "")
	} else {
		if c.Nota != nil {
			e.Aggiungi("nota", "ammessa solo con gli stati annullata e persa: per una nota interna usare POST /api/spedizioni/{id}/note")
		}
		if c.Motivo != nil {
			e.Aggiungi("motivo", "ammesso solo con gli stati annullata e persa")
		}
	}
	return e.Err()
}

func decidiStato(reg configurazione.Regole, r identita.Richiedente, attuale Spedizione, c CambioStato) (Modifica, error) {
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
	if attuale.Stato == Assegnata && c.Stato == Ritirata && attuale.TentativiRitiroFalliti >= reg.MaxTentativiRitiro {
		return Modifica{}, &errori.Conflitto{Messaggio: fmt.Sprintf(
			"falliti %d tentativi di ritiro su %d: la spedizione deve essere annullata dall'operatore",
			attuale.TentativiRitiroFalliti, reg.MaxTentativiRitiro)}
	}
	if attuale.Stato == InConsegna && c.Stato == Consegnata && attuale.TentativoInUscita {
		return Modifica{}, &errori.Conflitto{Messaggio: messaggioUscitaFallita}
	}
	if attuale.Stato == InConsegna && c.Stato == Ritirata && !attuale.TentativoInUscita {
		return Modifica{}, &errori.Conflitto{Messaggio: messaggioRitiroSenzaTentativo}
	}
	if err := richiediMotivoENota(reg, attuale, c); err != nil {
		return Modifica{}, err
	}
	nota := notaDaSalvare(c.Nota, c.Motivo)
	if nota != nil {
		nota.Pubblica = true
	}
	return Modifica{NuovoStato: c.Stato, Annulla: c.Stato == Annullata, Nota: nota}, nil
}

func richiediMotivoENota(reg configurazione.Regole, attuale Spedizione, c CambioStato) error {
	azione, conMotivo := azioniConMotivo[c.Stato]
	if !conMotivo {
		return nil
	}
	e := &errori.Validazione{}
	ritiriEsauriti := c.Stato == Annullata && attuale.TentativiRitiroFalliti >= reg.MaxTentativiRitiro
	if ritiriEsauriti {
		if (c.Motivo == nil) != (c.Nota == nil) {
			e.Aggiungi("motivo", "motivo e nota vanno indicati insieme oppure omessi: i tentativi di ritiro falliti hanno già i loro motivi")
		}
		return e.Err()
	}
	if c.Motivo == nil {
		e.Aggiungi("motivo", "obbligatorio per "+azione)
	}
	if c.Nota == nil {
		e.Aggiungi("nota", "obbligatoria per "+azione)
	}
	return e.Err()
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
