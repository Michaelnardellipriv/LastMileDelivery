//go:build integration

package apitest

import (
	"mime"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestCreaSpedizione(t *testing.T) {
	f := preparaFixture(t)

	t.Run("creazione_valida_201", func(t *testing.T) {
		r := f.chiama(t, http.MethodPost, "/api/spedizioni", f.corpoValido("OK"), comeOperatore(f))
		if r.stato != http.StatusCreated {
			t.Fatalf("atteso 201, ottenuto %d: %s", r.stato, r.grezzo)
		}
		if tipo, _, _ := mime.ParseMediaType(r.header.Get("Content-Type")); tipo != "application/json" {
			t.Fatalf("Content-Type inatteso: %q", r.header.Get("Content-Type"))
		}
		if r.corpo["stato"] != "creata" || r.corpo["id_corriere"] != nil || r.corpo["id"].(float64) <= 0 || r.corpo["is_reso"] != false {
			t.Fatalf("spedizione creata non corretta: %s", r.grezzo)
		}
		conReso := f.corpoValido("RESO")
		conReso["is_reso"] = true
		if r := f.chiama(t, http.MethodPost, "/api/spedizioni", conReso, comeOperatore(f)); r.stato != http.StatusBadRequest {
			t.Fatalf("is_reso non si imposta alla creazione: atteso 400, ottenuto %d", r.stato)
		}
	})

	t.Run("campi_mancanti_400", func(t *testing.T) {
		r := f.chiama(t, http.MethodPost, "/api/spedizioni", map[string]any{}, comeOperatore(f))
		campi := campiInErrore(r)
		if r.stato != http.StatusBadRequest || len(campi) != 6 {
			t.Fatalf("attesi 400 e 6 campi obbligatori, ottenuto %d %v", r.stato, campi)
		}
	})

	t.Run("valori_non_validi_tutti_insieme_400", func(t *testing.T) {
		corpo := f.corpoValido("X")
		corpo["cod_ordine"] = "ORD 1"
		corpo["dataora_richiesta_spedizione"] = time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)
		corpo["id_indirizzo_spedizione"] = f.idRitiro
		delete(corpo, "id_cliente_destinatario")
		r := f.chiama(t, http.MethodPost, "/api/spedizioni", corpo, comeOperatore(f))
		campi := campiInErrore(r)
		for _, c := range []string{"id_cliente_destinatario", "cod_ordine", "dataora_richiesta_spedizione", "id_indirizzo_spedizione"} {
			if r.stato != http.StatusBadRequest || !contiene(campi, c) {
				t.Fatalf("atteso 400 con errore su %s, ottenuto %d %v", c, r.stato, campi)
			}
		}
	})

	t.Run("campo_non_previsto_400", func(t *testing.T) {
		corpo := f.corpoValido("Y")
		corpo["stato"] = "consegnata"
		r := f.chiama(t, http.MethodPost, "/api/spedizioni", corpo, comeOperatore(f))
		if r.stato != http.StatusBadRequest || !strings.Contains(string(r.grezzo), "campo non previsto: stato") {
			t.Fatalf("atteso 400 'campo non previsto', ottenuto %d %s", r.stato, r.grezzo)
		}
	})

	t.Run("riferimenti_inesistenti_400", func(t *testing.T) {
		for _, campo := range []string{"id_cliente_mittente", "id_indirizzo_spedizione"} {
			corpo := f.corpoValido("RIF-" + campo)
			corpo[campo] = 999999999
			r := f.chiama(t, http.MethodPost, "/api/spedizioni", corpo, comeOperatore(f))
			if r.stato != http.StatusBadRequest || !contiene(campiInErrore(r), campo) {
				t.Fatalf("%s inesistente: atteso 400 sul campo, ottenuto %d %s", campo, r.stato, r.grezzo)
			}
		}
	})

	t.Run("codice_ordine_duplicato_409", func(t *testing.T) {
		f.nuovaSpedizione(t, "DUP")
		r := f.chiama(t, http.MethodPost, "/api/spedizioni", f.corpoValido("DUP"), comeOperatore(f))
		if r.stato != http.StatusConflict {
			t.Fatalf("atteso 409, ottenuto %d %s", r.stato, r.grezzo)
		}
	})

	t.Run("idempotency_key_una_sola_spedizione", func(t *testing.T) {
		h := comeOperatore(f)
		h["Idempotency-Key"] = "chiave-" + f.suffisso
		corpo := f.corpoValido("IDEM")
		prima := f.chiama(t, http.MethodPost, "/api/spedizioni", corpo, h)
		seconda := f.chiama(t, http.MethodPost, "/api/spedizioni", corpo, h)
		if prima.stato != 201 || seconda.stato != 201 || prima.corpo["id"] != seconda.corpo["id"] ||
			seconda.header.Get("Idempotent-Replayed") != "true" {
			t.Fatalf("la ripetizione deve restituire la stessa spedizione: %d %s / %d %s",
				prima.stato, prima.grezzo, seconda.stato, seconda.grezzo)
		}
	})

	t.Run("corriere_non_puo_creare_403", func(t *testing.T) {
		r := f.chiama(t, http.MethodPost, "/api/spedizioni", f.corpoValido("COR"), comeCorriere(f.idCorriere1))
		if r.stato != http.StatusForbidden {
			t.Fatalf("atteso 403, ottenuto %d", r.stato)
		}
	})

	t.Run("senza_identita_401_e_non_json_415", func(t *testing.T) {
		if r := f.chiama(t, http.MethodPost, "/api/spedizioni", f.corpoValido("A"), nil); r.stato != http.StatusUnauthorized {
			t.Fatalf("senza identità: atteso 401, ottenuto %d", r.stato)
		}
		h := comeOperatore(f)
		h["Content-Type"] = "text/plain"
		if r := f.chiama(t, http.MethodPost, "/api/spedizioni", "ciao", h); r.stato != http.StatusUnsupportedMediaType {
			t.Fatalf("corpo non JSON: atteso 415, ottenuto %d", r.stato)
		}
	})
}
