package spedizioni

import (
	"context"
	"testing"

	"lastmiledelivery/backend/internal/configurazione"
	"lastmiledelivery/backend/internal/identita"
)

// I limiti di ritiro e di consegna sono indipendenti e vengono dalla configurazione.
func TestTentativiMassimiDistintiPerRitiroEConsegna(t *testing.T) {
	regole := regoleTest
	regole.MaxTentativiRitiro, regole.MaxTentativiConsegna = 1, 5

	ritiro := TentativoFallito{Tipo: TentativoRitiro, Nota: p("chiuso"), Motivo: p("mittente assente")}
	assegnata := inStato(Assegnata)
	assegnata.TentativiRitiroFalliti = 1
	if _, err := decidiTentativo(regole, corriere1, assegnata, ritiro); tipoErrore(err) != "409" {
		t.Fatalf("con 1 ritiro ammesso il secondo è rifiutato: %v", err)
	}
	if m, err := decidiStato(regole, operatore, assegnata, CambioStato{Stato: Annullata}); err != nil || !m.Annulla {
		t.Fatalf("ritiri esauriti: l'operatore annulla senza motivo: %+v %v", m, err)
	}

	consegna := TentativoFallito{Tipo: TentativoConsegna, Nota: p("assente"), Motivo: p("destinatario assente")}
	inConsegna := inStato(InConsegna)
	inConsegna.TentativiConsegnaFalliti = 3 // quarta uscita
	if m, err := decidiTentativo(regole, corriere1, inConsegna, consegna); err != nil || m.NuovoStato != InConsegna {
		t.Fatalf("con 5 consegne ammesse il 4° fallimento lascia la spedizione in consegna: %+v %v", m, err)
	}
	inConsegna.TentativiConsegnaFalliti = 4
	if m, err := decidiTentativo(regole, corriere1, inConsegna, consegna); err != nil || m.NuovoStato != NonConsegnata {
		t.Fatalf("il 5° fallimento porta in non consegnata: %+v %v", m, err)
	}
}

// Valgono solo i motivi attivi: uno disattivato è rifiutato, uno nuovo è accettato.
func TestMotiviDallaConfigurazione(t *testing.T) {
	regole := regoleTest
	regole.Motivi = []string{"cliente in ferie", "altro"}

	nuovo := TentativoFallito{Tipo: TentativoConsegna, Nota: p("x"), Motivo: p("cliente in ferie")}
	if err := validaTentativo(regole, nuovo); err != nil {
		t.Fatalf("un motivo aggiunto dall'amministratore è ammesso: %v", err)
	}
	tolto := TentativoFallito{Tipo: TentativoConsegna, Nota: p("x"), Motivo: p("destinatario assente")}
	if err := validaTentativo(regole, tolto); tipoErrore(err) != "400" {
		t.Fatalf("un motivo disattivato è rifiutato: %v", err)
	}
	if err := validaCambio(regole, CambioStato{Stato: Persa, Motivo: p("furto"), Nota: p("x")}); tipoErrore(err) != "400" {
		t.Fatalf("anche per annullata e persa valgono solo i motivi attivi: %v", err)
	}
}

// I giorni di visibilità del cliente sono quelli della sede di ogni spedizione: li
// applica la query, quindi l'elenco non legge la configurazione per nessun ruolo.
func TestElencoNonLeggeLaConfigurazione(t *testing.T) {
	a := &archivioElenco{}
	contate := 0
	s := NuovoServizio(a, regoleContate{&contate})
	for _, r := range []identita.Richiedente{operatore, cliente1, amministratore} {
		s.Elenca(context.Background(), r, filtroBase)
	}
	if contate != 0 {
		t.Fatalf("configurazione letta %d volte", contate)
	}
}

// Le regole sono quelle della sede di chi lavora sulla spedizione.
func TestRegoleDellaSedeDelRichiedente(t *testing.T) {
	a := &archivioElenco{attuale: inStato(InConsegna)}
	sede := int64(0)
	s := NuovoServizio(a, regoleDiSede{&sede})
	s.RegistraTentativo(context.Background(), corriere1, 10, TentativoFallito{Tipo: TentativoConsegna, Nota: p("x"), Motivo: p("altro")})
	if sede != sedeTrento {
		t.Fatalf("regole lette per la sede %d, attesa %d", sede, sedeTrento)
	}
}

type regoleContate struct{ n *int }

func (r regoleContate) Regole(context.Context, int64) (configurazione.Regole, error) {
	*r.n++
	return regoleTest, nil
}

type regoleDiSede struct{ sede *int64 }

func (r regoleDiSede) Regole(_ context.Context, idSede int64) (configurazione.Regole, error) {
	*r.sede = idSede
	return regoleTest, nil
}
