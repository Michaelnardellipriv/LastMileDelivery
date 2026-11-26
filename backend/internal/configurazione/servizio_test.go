package configurazione

import (
	"context"
	"errors"
	"testing"

	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

type archivioFinto struct {
	regole           Regole
	modifica         ModificaImpostazioni
	idSede           int64
	idAmministratore int64
	creato           string
	attivo           *bool
}

func (a *archivioFinto) Regole(_ context.Context, idSede int64) (Regole, error) {
	a.idSede = idSede
	return a.regole, nil
}

func (a *archivioFinto) MotiviAttivi(context.Context) ([]string, error) {
	return a.regole.Motivi, nil
}

func (a *archivioFinto) AggiornaImpostazioni(_ context.Context, idSede int64, m ModificaImpostazioni, idAmministratore int64) (Impostazioni, error) {
	a.idSede, a.modifica, a.idAmministratore = idSede, m, idAmministratore
	return a.regole.Impostazioni, nil
}

func (a *archivioFinto) Motivi(context.Context) ([]Motivo, error) {
	return []Motivo{{ID: 1, Descrizione: "furto", Attivo: false}}, nil
}

func (a *archivioFinto) CreaMotivo(_ context.Context, descrizione string) (Motivo, error) {
	a.creato = descrizione
	return Motivo{ID: 2, Descrizione: descrizione, Attivo: true}, nil
}

func (a *archivioFinto) ImpostaMotivoAttivo(_ context.Context, id int64, attivo bool) (Motivo, error) {
	a.attivo = &attivo
	return Motivo{ID: id, Attivo: attivo}, nil
}

const sedeTrento, sedeBolzano = 10, 20

var (
	amministratore    = identita.Richiedente{Ruolo: identita.Amministratore, ID: 4}
	nonAmministratori = []identita.Richiedente{
		{Ruolo: identita.Operatore, ID: 1, IDSede: sedeTrento}, {Ruolo: identita.Corriere, ID: 2, IDSede: sedeTrento},
		{Ruolo: identita.Cliente, ID: 3},
	}
)

func p[T any](v T) *T { return &v }

func tipoErrore(err error) string {
	var validazione *errori.Validazione
	var nonAutorizzato *errori.NonAutorizzato
	switch {
	case err == nil:
		return "nessuno"
	case errors.As(err, &validazione):
		return "400"
	case errors.As(err, &nonAutorizzato):
		return "403"
	}
	return err.Error()
}

func TestAmministratoreLeggeLeImpostazioniDiOgniSede(t *testing.T) {
	ctx := context.Background()
	a := &archivioFinto{regole: Regole{Impostazioni: Impostazioni{GiorniVisibilitaCliente: 30}}}
	s := NuovoServizio(a)

	for _, sede := range []int64{sedeTrento, sedeBolzano} {
		if imp, err := s.Impostazioni(ctx, amministratore, sede); err != nil || imp.GiorniVisibilitaCliente != 30 || a.idSede != sede {
			t.Fatalf("l'amministratore legge le impostazioni della sede %d: %+v %v (letta %d)", sede, imp, err, a.idSede)
		}
	}
}

func TestOperatoreECorriereLeggonoSoloLaPropriaSede(t *testing.T) {
	ctx := context.Background()
	s := NuovoServizio(&archivioFinto{})

	for _, r := range nonAmministratori[:2] {
		if _, err := s.Impostazioni(ctx, r, sedeTrento); err != nil {
			t.Errorf("%s legge le impostazioni della sua sede: %v", r.Ruolo, err)
		}
		if _, err := s.Impostazioni(ctx, r, sedeBolzano); tipoErrore(err) != "403" {
			t.Errorf("%s non legge le impostazioni di un'altra sede: %v", r.Ruolo, err)
		}
	}
	if _, err := s.Impostazioni(ctx, nonAmministratori[2], sedeTrento); tipoErrore(err) != "403" {
		t.Errorf("il cliente non legge le impostazioni: %v", err)
	}
}

func TestSoloAmministratoreModificaLeImpostazioni(t *testing.T) {
	ctx := context.Background()
	s := NuovoServizio(&archivioFinto{})
	for _, r := range nonAmministratori {
		if _, err := s.ModificaImpostazioni(ctx, r, sedeTrento, ModificaImpostazioni{MaxTentativiRitiro: p(2)}); tipoErrore(err) != "403" {
			t.Errorf("%s non modifica le impostazioni: %v", r.Ruolo, err)
		}
	}
}

func TestModificaImpostazioni(t *testing.T) {
	ctx := context.Background()
	a := &archivioFinto{}
	s := NuovoServizio(a)

	m := ModificaImpostazioni{GiorniVisibilitaCliente: p(60), MaxTentativiConsegna: p(5)}
	if _, err := s.ModificaImpostazioni(ctx, amministratore, sedeBolzano, m); err != nil || a.idAmministratore != 4 || a.idSede != sedeBolzano ||
		*a.modifica.GiorniVisibilitaCliente != 60 || *a.modifica.MaxTentativiConsegna != 5 || a.modifica.MaxTentativiRitiro != nil {
		t.Fatalf("si modificano solo i campi indicati: %+v %v", a.modifica, err)
	}

	for nome, m := range map[string]ModificaImpostazioni{
		"nessun campo":            {},
		"giorni a zero":           {GiorniVisibilitaCliente: p(0)},
		"giorni oltre l'anno":     {GiorniVisibilitaCliente: p(366)},
		"ritiri a zero":           {MaxTentativiRitiro: p(0)},
		"consegne oltre 10":       {MaxTentativiConsegna: p(11)},
		"intervallo troppo breve": {IntervalloPosizioneSec: p(4)},
		"intervallo oltre l'ora":  {IntervalloPosizioneSec: p(3601)},
	} {
		if _, err := s.ModificaImpostazioni(ctx, amministratore, sedeTrento, m); tipoErrore(err) != "400" {
			t.Errorf("%s: atteso 400, ottenuto %v", nome, err)
		}
	}
	for nome, m := range map[string]ModificaImpostazioni{
		"estremi bassi": {GiorniVisibilitaCliente: p(1), MaxTentativiRitiro: p(1), MaxTentativiConsegna: p(1), IntervalloPosizioneSec: p(5)},
		"estremi alti":  {GiorniVisibilitaCliente: p(365), MaxTentativiRitiro: p(10), MaxTentativiConsegna: p(10), IntervalloPosizioneSec: p(3600)},
	} {
		if _, err := s.ModificaImpostazioni(ctx, amministratore, sedeTrento, m); err != nil {
			t.Errorf("%s: valori ammessi, ottenuto %v", nome, err)
		}
	}
}

func TestMotivi(t *testing.T) {
	ctx := context.Background()
	a := &archivioFinto{regole: Regole{Motivi: []string{"altro"}}}
	s := NuovoServizio(a)

	if motivi, err := s.MotiviAttivi(ctx); err != nil || len(motivi) != 1 || motivi[0] != "altro" {
		t.Fatalf("l'elenco pubblico ha solo i motivi attivi: %v %v", motivi, err)
	}
	if motivi, err := s.Motivi(ctx, amministratore); err != nil || len(motivi) != 1 || motivi[0].Attivo {
		t.Fatalf("l'amministratore vede anche i disattivati: %v %v", motivi, err)
	}
	if m, err := s.CreaMotivo(ctx, amministratore, p("  cliente   in ferie ")); err != nil || a.creato != "cliente in ferie" || !m.Attivo {
		t.Fatalf("nuovo motivo, spazi normalizzati: %q %+v %v", a.creato, m, err)
	}
	for nome, d := range map[string]*string{"assente": nil, "vuoto": p("   "), "troppo lungo": p(string(make([]byte, 101)) + "x")} {
		if _, err := s.CreaMotivo(ctx, amministratore, d); tipoErrore(err) != "400" {
			t.Errorf("descrizione %s: atteso 400, ottenuto %v", nome, err)
		}
	}
	if m, err := s.ImpostaMotivoAttivo(ctx, amministratore, 3, p(false)); err != nil || m.Attivo || *a.attivo {
		t.Fatalf("disattivazione: %+v %v", m, err)
	}
	if _, err := s.ImpostaMotivoAttivo(ctx, amministratore, 3, nil); tipoErrore(err) != "400" {
		t.Errorf("attivo obbligatorio: %v", err)
	}
	for _, r := range nonAmministratori {
		if _, err := s.Motivi(ctx, r); tipoErrore(err) != "403" {
			t.Errorf("%s non gestisce i motivi: %v", r.Ruolo, err)
		}
		if _, err := s.CreaMotivo(ctx, r, p("nuovo")); tipoErrore(err) != "403" {
			t.Errorf("%s non crea motivi: %v", r.Ruolo, err)
		}
		if _, err := s.ImpostaMotivoAttivo(ctx, r, 1, p(false)); tipoErrore(err) != "403" {
			t.Errorf("%s non disattiva motivi: %v", r.Ruolo, err)
		}
	}
}
