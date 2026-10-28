package main

import (
	"strings"
	"testing"
	"time"
)

// richiestaValida restituisce una richiesta che passa la validazione;
// ogni test ne modifica un solo campo per volta.
func richiestaValida() RichiestaCreazioneSpedizione {
	return RichiestaCreazioneSpedizione{
		CodOrdine:                  "ORD-001",
		IDClienteMittente:          1,
		IDClienteDestinatario:      2,
		IDIndirizzoRitiro:          1,
		IDIndirizzoSpedizione:      2,
		DataOraRichiestaSpedizione: time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC),
	}
}

func TestValidaRichiestaCreazioneSpedizione(t *testing.T) {
	casi := []struct {
		nome      string
		modifica  func(*RichiestaCreazioneSpedizione)
		attesoErr bool
		contiene  string // parte del messaggio d'errore atteso
	}{
		{"richiesta valida", func(r *RichiestaCreazioneSpedizione) {}, false, ""},
		{"reso valido", func(r *RichiestaCreazioneSpedizione) { r.IsReso = true }, false, ""},
		{"cod_ordine vuoto", func(r *RichiestaCreazioneSpedizione) { r.CodOrdine = "" }, true, "cod_ordine"},
		{"cod_ordine solo spazi", func(r *RichiestaCreazioneSpedizione) { r.CodOrdine = "   " }, true, "cod_ordine"},
		{"cod_ordine di 50 caratteri", func(r *RichiestaCreazioneSpedizione) { r.CodOrdine = strings.Repeat("a", 50) }, false, ""},
		{"cod_ordine di 51 caratteri", func(r *RichiestaCreazioneSpedizione) { r.CodOrdine = strings.Repeat("a", 51) }, true, "50"},
		{"cod_ordine con 50 caratteri multibyte", func(r *RichiestaCreazioneSpedizione) { r.CodOrdine = strings.Repeat("è", 50) }, false, ""},
		{"mittente zero", func(r *RichiestaCreazioneSpedizione) { r.IDClienteMittente = 0 }, true, "id_cliente_mittente"},
		{"mittente negativo", func(r *RichiestaCreazioneSpedizione) { r.IDClienteMittente = -1 }, true, "id_cliente_mittente"},
		{"destinatario zero", func(r *RichiestaCreazioneSpedizione) { r.IDClienteDestinatario = 0 }, true, "id_cliente_destinatario"},
		{"destinatario negativo", func(r *RichiestaCreazioneSpedizione) { r.IDClienteDestinatario = -5 }, true, "id_cliente_destinatario"},
		{"indirizzo ritiro zero", func(r *RichiestaCreazioneSpedizione) { r.IDIndirizzoRitiro = 0 }, true, "id_indirizzo_ritiro"},
		{"indirizzo spedizione zero", func(r *RichiestaCreazioneSpedizione) { r.IDIndirizzoSpedizione = 0 }, true, "id_indirizzo_spedizione"},
		{"data assente", func(r *RichiestaCreazioneSpedizione) { r.DataOraRichiestaSpedizione = time.Time{} }, true, "dataora_richiesta_spedizione"},
	}

	for _, c := range casi {
		t.Run(c.nome, func(t *testing.T) {
			richiesta := richiestaValida()
			c.modifica(&richiesta)

			err := validaRichiestaCreazioneSpedizione(richiesta)

			if c.attesoErr && err == nil {
				t.Fatalf("atteso un errore, ricevuto nil")
			}
			if !c.attesoErr && err != nil {
				t.Fatalf("nessun errore atteso, ricevuto: %v", err)
			}
			if c.attesoErr && !strings.Contains(err.Error(), c.contiene) {
				t.Fatalf("errore %q non contiene %q", err.Error(), c.contiene)
			}
		})
	}
}
