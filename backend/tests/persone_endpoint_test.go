//go:build integration

package apitest

import (
	"net/http"
	"strings"
	"testing"
)

func TestDatiClientiECorriere(t *testing.T) {
	f := preparaFixture(t)
	op, admin, c1, dest := comeOperatore(f), comeAmministratore(f), comeCorriere(f.idCorriere1), comeCliente(f.idDestinatario)

	id := f.nuovaSpedizione(t, "PERSONE")
	f.assegna(t, id, f.idCorriere1, 201)

	voceDallElenco := func(t *testing.T, chi map[string]string) map[string]any {
		t.Helper()
		voci, _ := f.elenco(t, chi, "cod_ordine=PERSONE-"+f.suffisso, 200)
		if voci[id] == nil {
			t.Fatalf("la spedizione %d deve essere nell'elenco", id)
		}
		return voci[id]
	}
	viste := func(t *testing.T, chi map[string]string) []map[string]any {
		t.Helper()
		return []map[string]any{voceDallElenco(t, chi), f.dettaglio(t, id, chi, 200).corpo}
	}
	persona := func(v map[string]any, chiave string) map[string]any {
		p, _ := v[chiave].(map[string]any)
		return p
	}
	destinatarioCompleto := func(t *testing.T, d map[string]any) {
		t.Helper()
		if d == nil || int32(d["id"].(float64)) != f.idDestinatario || d["nome"] != "destinatario" || d["cognome"] != "Test" ||
			d["email"] != f.emailDi("destinatario") || !strings.HasPrefix(d["telefono"].(string), "+39 ") {
			t.Errorf("destinatario con id, nome, cognome, email e telefono: %v", d)
		}
	}

	t.Run("indirizzi_con_citta_cap_regione_nazione_per_tutti", func(t *testing.T) {
		for _, chi := range []map[string]string{op, c1, dest} {
			for _, v := range viste(t, chi) {
				for _, chiave := range []string{"indirizzo_ritiro", "indirizzo_spedizione"} {
					ind := v[chiave].(map[string]any)
					if ind["citta"] != "Trento" || ind["cap"] != "38121" || ind["regione"] != "Regione" ||
						ind["nazione"] != "Nazione "+f.suffisso || ind["via"] == nil {
						t.Errorf("%s: via, civico, città, CAP, regione e nazione: %v", chiave, ind)
					}
				}
			}
		}
	})

	t.Run("operatore_e_amministratore_vedono_tutto", func(t *testing.T) {
		for _, chi := range []map[string]string{op, admin} {
			for _, v := range viste(t, chi) {
				m, c := persona(v, "mittente"), persona(v, "corriere")
				if m == nil || int32(m["id"].(float64)) != f.idMittente || m["nome"] != "mittente" ||
					m["email"] != f.emailDi("mittente") || m["telefono"] == nil || v["id_cliente_mittente"] == nil {
					t.Errorf("mittente completo: %v", m)
				}
				destinatarioCompleto(t, persona(v, "destinatario"))
				if c == nil || int32(c["id"].(float64)) != f.idCorriere1 || c["nome"] != "corriere1" || c["telefono"] == nil {
					t.Errorf("corriere con nome e telefono: %v", c)
				}
			}
		}
	})

	t.Run("corriere_e_cliente_vedono_mittente_e_destinatario", func(t *testing.T) {
		for _, chi := range []map[string]string{c1, dest} {
			for _, v := range viste(t, chi) {
				destinatarioCompleto(t, persona(v, "destinatario"))
				m := persona(v, "mittente")
				if m == nil || m["nome"] != "mittente" || m["cognome"] != "Test" ||
					m["email"] != f.emailDi("mittente") || !strings.HasPrefix(m["telefono"].(string), "+39 ") {
					t.Errorf("%v vede nome, cognome, email e telefono del mittente: %v", chi, m)
				}
				for _, chiave := range []string{"corriere", "id_corriere"} {
					if _, ok := v[chiave]; ok {
						t.Errorf("%v non riceve %s: %v", chi, chiave, v[chiave])
					}
				}
			}
		}
		for _, v := range viste(t, c1) {
			if m := persona(v, "mittente"); m["id"] == nil || v["id_cliente_mittente"] == nil {
				t.Errorf("il corriere riceve anche l'id del mittente: %v", v)
			}
		}
		for _, v := range viste(t, dest) {
			_, idPersona := persona(v, "mittente")["id"]
			_, idCampo := v["id_cliente_mittente"]
			if idPersona || idCampo {
				t.Errorf("il cliente non riceve l'id del mittente: %v", v)
			}
		}
	})

	t.Run("spedizione_senza_corriere", func(t *testing.T) {
		libera := f.nuovaSpedizione(t, "PERSONE-LIBERA")
		r := f.dettaglio(t, libera, op, 200)
		if _, ok := r.corpo["corriere"]; ok {
			t.Errorf("senza assegnazione non c'è il corriere: %s", r.grezzo)
		}
		if r.corpo["mittente"] == nil {
			t.Errorf("l'operatore vede sempre il mittente: %s", r.grezzo)
		}
	})

	t.Run("cors_dal_browser", func(t *testing.T) {
		r := f.chiama(t, http.MethodOptions, "/api/spedizioni", nil, map[string]string{
			"Origin": "http://localhost:5173", "Access-Control-Request-Method": "GET",
			"Access-Control-Request-Headers": "authorization",
		})
		if r.stato != http.StatusNoContent || r.header.Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
			t.Errorf("preflight dal frontend: atteso 204 con Access-Control-Allow-Origin, ottenuto %d %v", r.stato, r.header)
		}
	})
}
