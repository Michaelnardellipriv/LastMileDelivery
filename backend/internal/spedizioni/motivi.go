package spedizioni

import (
	"slices"
	"strings"

	"lastmiledelivery/backend/internal/errori"
)

type Motivo string

const (
	MotivoMittenteAssente       Motivo = "mittente assente"
	MotivoPaccoNonPronto        Motivo = "pacco non pronto"
	MotivoIndirizzoRitiroErrato Motivo = "indirizzo di ritiro errato"
	MotivoDestinatarioAssente   Motivo = "destinatario assente"
	MotivoIndirizzoErrato       Motivo = "indirizzo errato"
	MotivoRifiutata             Motivo = "rifiutata dal destinatario"
	MotivoAccessoImpossibile    Motivo = "accesso impossibile"
	MotivoRichiestaMittente     Motivo = "richiesta del mittente"
	MotivoRichiestaDestinatario Motivo = "richiesta del destinatario"
	MotivoDatiErrati            Motivo = "dati della spedizione errati"
	MotivoRitiroNonRiuscito     Motivo = "ritiro non riuscito"
	MotivoSmarritaInDeposito    Motivo = "smarrita in deposito"
	MotivoSmarritaInTrasporto   Motivo = "smarrita durante il trasporto"
	MotivoFurto                 Motivo = "furto"
	MotivoAltro                 Motivo = "altro"
)

var (
	MotiviRitiro = []Motivo{
		MotivoMittenteAssente, MotivoPaccoNonPronto, MotivoIndirizzoRitiroErrato,
		MotivoAccessoImpossibile, MotivoAltro,
	}
	MotiviConsegna = []Motivo{
		MotivoDestinatarioAssente, MotivoIndirizzoErrato, MotivoRifiutata,
		MotivoAccessoImpossibile, MotivoAltro,
	}
	MotiviAnnullamento = []Motivo{
		MotivoRichiestaMittente, MotivoRichiestaDestinatario, MotivoDatiErrati,
		MotivoRitiroNonRiuscito, MotivoAltro,
	}
	MotiviPerdita = []Motivo{
		MotivoSmarritaInDeposito, MotivoSmarritaInTrasporto, MotivoFurto, MotivoAltro,
	}
)

func motiviTentativo(t TipoTentativo) []Motivo {
	if t == TentativoRitiro {
		return MotiviRitiro
	}
	return MotiviConsegna
}

var motiviStato = map[Stato]struct {
	azione  string
	ammessi []Motivo
}{
	Annullata: {"annullare una spedizione", MotiviAnnullamento},
	Persa:     {"segnalare una spedizione persa", MotiviPerdita},
}

func controllaMotivo(e *errori.Validazione, motivo *string, ammessi []Motivo, obbligatorio string) {
	switch {
	case motivo == nil:
		if obbligatorio != "" {
			e.Aggiungi("motivo", obbligatorio)
		}
	case !slices.Contains(ammessi, Motivo(*motivo)):
		e.Aggiungi("motivo", "valore non previsto: ammessi "+elencoMotivi(ammessi))
	}
}

func elencoMotivi(m []Motivo) string {
	nomi := make([]string, len(m))
	for i, v := range m {
		nomi[i] = string(v)
	}
	return strings.Join(nomi, ", ")
}
