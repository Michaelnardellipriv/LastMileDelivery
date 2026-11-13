//go:build integration

package apitest

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func telefonoUnico() string {
	return fmt.Sprintf("5%d", time.Now().UnixNano()%1e9)
}

func TestCreazioneSpedizioneDaGUI(t *testing.T) {
	f := preparaFixture(t)
	op, admin, c1, dest := comeOperatore(f), comeAmministratore(f), comeCorriere(f.idCorriere1), comeCliente(f.idDestinatario)
	// nazione unica per ogni esecuzione, ma solo lettere (il backend rifiuta le cifre nei nomi dei luoghi)
	nazione := "Italia " + strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return 'g' + (r - '0')
		}
		return r
	}, f.suffisso)

	nuovoCliente := func(email string, password any) map[string]any {
		c := map[string]any{"nome": "Giulia", "cognome": "Nuova", "prefisso": "+39", "nr_telefono": telefonoUnico(), "email": email}
		if password != nil {
			c["password"] = password
		}
		return c
	}
	nuovoIndirizzo := func(via, civico, citta, cap string) map[string]any {
		return map[string]any{"via": via, "civico": civico, "citta": citta, "cap": cap, "regione": "Lombardia", "nazione": nazione}
	}

	var idMittente, idDestinatario, idRitiro, idConsegna int

	t.Run("cliente_nuovo_con_password_inviata_per_mail", func(t *testing.T) {
		email := "giulia." + f.suffisso + "@test.it"
		r := f.chiama(t, http.MethodPost, "/api/clienti", nuovoCliente(email, "PasswordGiulia1"), op)
		if r.stato != http.StatusCreated || r.corpo["mail_inviata"] != true {
			t.Fatalf("nuovo cliente: atteso 201 con mail_inviata, ottenuto %d %s", r.stato, r.grezzo)
		}
		cliente := r.corpo["cliente"].(map[string]any)
		idDestinatario = int(cliente["id"].(float64))
		if cliente["email"] != email || cliente["nome"] != "Giulia" {
			t.Fatalf("dati del cliente: %s", r.grezzo)
		}
		f.login(t, map[string]any{"email": email, "password": "PasswordGiulia1"}, 200)

		r = f.chiama(t, http.MethodPost, "/api/clienti", nuovoCliente(strings.ToUpper(email), "PasswordGiulia1"), op)
		if r.stato != http.StatusConflict || !contiene(campiInErrore(r), "email") {
			t.Fatalf("cliente già registrato: atteso 409 sul campo email, ottenuto %d %s", r.stato, r.grezzo)
		}

		r = f.chiama(t, http.MethodPost, "/api/clienti", nuovoCliente("marco."+f.suffisso+"@test.it", "PasswordMarco1"), op)
		if r.stato != http.StatusCreated {
			t.Fatalf("secondo cliente: %d %s", r.stato, r.grezzo)
		}
		idMittente = int(r.corpo["cliente"].(map[string]any)["id"].(float64))
	})

	t.Run("email_unica_tra_i_ruoli", func(t *testing.T) {
		email := "corriere.gui." + f.suffisso + "@test.it"
		var idUtente int32
		if err := f.db.QueryRow(t.Context(), `INSERT INTO utenti (nome, cognome, prefisso, nr_telefono, email, pwd_hash)
			VALUES ('Corriere', 'Gui', '+39', $1, $2, '!') RETURNING id`, telefonoUnico(), email).Scan(&idUtente); err != nil {
			t.Fatal(err)
		}
		if _, err := f.db.Exec(t.Context(), `INSERT INTO corrieri (id_utente) VALUES ($1)`, idUtente); err != nil {
			t.Fatal(err)
		}
		f.impostaPassword(t, email, "PasswordCorriere2")
		for _, e := range []string{email, strings.ToUpper(email), f.emailDi("operatore")} {
			r := f.chiama(t, http.MethodPost, "/api/clienti", nuovoCliente(e, "PasswordProva1"), op)
			if r.stato != http.StatusConflict || !contiene(campiInErrore(r), "email") {
				t.Errorf("email %s già usata con un altro ruolo: atteso 409, ottenuto %d %s", e, r.stato, r.grezzo)
			}
		}
		f.login(t, map[string]any{"email": email, "password": "PasswordCorriere2"}, 200)
	})

	t.Run("validazione_e_permessi", func(t *testing.T) {
		email := "senzapwd." + f.suffisso + "@test.it"
		if r := f.chiama(t, http.MethodPost, "/api/clienti", nuovoCliente(email, nil), op); r.stato != 400 || !contiene(campiInErrore(r), "password") {
			t.Errorf("nuovo utente senza password: atteso 400 sul campo password, ottenuto %d %s", r.stato, r.grezzo)
		}
		duplicato := nuovoCliente("telefono."+f.suffisso+"@test.it", "PasswordProva1")
		var telefono string
		if err := f.db.QueryRow(t.Context(), `SELECT nr_telefono FROM utenti WHERE email = $1`, f.emailDi("operatore")).Scan(&telefono); err != nil {
			t.Fatal(err)
		}
		duplicato["nr_telefono"] = telefono
		if r := f.chiama(t, http.MethodPost, "/api/clienti", duplicato, op); r.stato != 409 || !contiene(campiInErrore(r), "nr_telefono") {
			t.Errorf("telefono già usato: atteso 409 sul campo nr_telefono, ottenuto %d %s", r.stato, r.grezzo)
		}
		for _, chi := range []map[string]string{admin, c1, dest} {
			if r := f.chiama(t, http.MethodPost, "/api/clienti", nuovoCliente("x."+f.suffisso+"@test.it", "PasswordProva1"), chi); r.stato != 403 {
				t.Errorf("solo l'operatore crea clienti: ottenuto %d", r.stato)
			}
			if r := f.chiama(t, http.MethodPost, "/api/indirizzi", nuovoIndirizzo("Via X", "1", "Milano", "20100"), chi); r.stato != 403 {
				t.Errorf("solo l'operatore inserisce indirizzi: ottenuto %d", r.stato)
			}
		}
		for _, chi := range []map[string]string{c1, dest} {
			if r := f.chiama(t, http.MethodGet, "/api/clienti", nil, chi); r.stato != 403 {
				t.Errorf("corriere e cliente non cercano clienti: ottenuto %d", r.stato)
			}
		}
		if r := f.chiama(t, http.MethodPost, "/api/indirizzi", map[string]any{"via": "Via X"}, op); r.stato != 400 {
			t.Errorf("indirizzo incompleto: atteso 400, ottenuto %d", r.stato)
		}
	})

	t.Run("indirizzi_trova_o_crea", func(t *testing.T) {
		r := f.chiama(t, http.MethodPost, "/api/indirizzi", nuovoIndirizzo("Via Dante", "7", "Milano", "20121"), op)
		if r.stato != http.StatusCreated {
			t.Fatalf("nuovo indirizzo: atteso 201, ottenuto %d %s", r.stato, r.grezzo)
		}
		idRitiro = int(r.corpo["id"].(float64))
		if r.corpo["citta"] != "Milano" || r.corpo["regione"] != "Lombardia" || r.corpo["nazione"] != nazione {
			t.Fatalf("indirizzo completo: %s", r.grezzo)
		}
		r = f.chiama(t, http.MethodPost, "/api/indirizzi", nuovoIndirizzo(" via dante ", "7", "milano", "20121"), op)
		if r.stato != http.StatusOK || int(r.corpo["id"].(float64)) != idRitiro || r.corpo["via"] != "Via Dante" {
			t.Fatalf("stesso indirizzo scritto diversamente: atteso 200 con lo stesso id, ottenuto %d %s", r.stato, r.grezzo)
		}
		r = f.chiama(t, http.MethodPost, "/api/indirizzi", nuovoIndirizzo("Corso Como", "2", "Milano", "20121"), op)
		if r.stato != http.StatusCreated {
			t.Fatalf("secondo indirizzo nella stessa città: %d %s", r.stato, r.grezzo)
		}
		idConsegna = int(r.corpo["id"].(float64))
		if n := f.conta(t, `SELECT count(*) FROM citta c JOIN regioni r ON r.id = c.id_regione JOIN nazioni n ON n.id = r.id_nazione
			WHERE n.nome = $1`, nazione); n != 1 {
			t.Errorf("città, regione e nazione non vanno duplicate: trovate %d città", n)
		}
	})

	t.Run("ricerche", func(t *testing.T) {
		for _, chi := range []map[string]string{op, admin} {
			r := f.chiama(t, http.MethodGet, "/api/clienti?q="+url.QueryEscape("giulia."+f.suffisso), nil, chi)
			dati := r.corpo["dati"].([]any)
			if r.stato != 200 || len(dati) != 1 || int(dati[0].(map[string]any)["id"].(float64)) != idDestinatario {
				t.Errorf("ricerca cliente per email: %d %s", r.stato, r.grezzo)
			}
			r = f.chiama(t, http.MethodGet, "/api/indirizzi?q="+url.QueryEscape("dante"), nil, chi)
			trovato := false
			for _, d := range r.corpo["dati"].([]any) {
				if int(d.(map[string]any)["id"].(float64)) == idRitiro {
					trovato = true
				}
			}
			if r.stato != 200 || !trovato {
				t.Errorf("ricerca indirizzo per via: %d %s", r.stato, r.grezzo)
			}
		}
	})

	t.Run("elenco_dei_corrieri", func(t *testing.T) {
		for _, chi := range []map[string]string{op, admin} {
			r := f.chiama(t, http.MethodGet, "/api/corrieri?q="+url.QueryEscape("corriere1."+f.suffisso), nil, chi)
			dati, _ := r.corpo["dati"].([]any)
			if r.stato != 200 || len(dati) != 1 {
				t.Fatalf("ricerca corriere per email: %d %s", r.stato, r.grezzo)
			}
			c := dati[0].(map[string]any)
			if int32(c["id"].(float64)) != f.idCorriere1 || c["nome"] != "corriere1" || c["email"] != f.emailDi("corriere1") || c["nr_telefono"] == nil {
				t.Errorf("dati del corriere: %v", c)
			}
			r = f.chiama(t, http.MethodGet, "/api/corrieri?per_pagina=100", nil, chi)
			if r.stato != 200 || len(r.corpo["dati"].([]any)) < 2 {
				t.Errorf("elenco di tutti i corrieri, anche senza spedizioni: %d %s", r.stato, r.grezzo)
			}
		}
		for _, chi := range []map[string]string{c1, dest} {
			if r := f.chiama(t, http.MethodGet, "/api/corrieri", nil, chi); r.stato != 403 {
				t.Errorf("corriere e cliente non vedono l'elenco dei corrieri: %d", r.stato)
			}
		}
	})

	t.Run("spedizione_con_i_dati_appena_creati", func(t *testing.T) {
		corpo := f.corpoValido("GUI")
		corpo["id_cliente_mittente"], corpo["id_cliente_destinatario"] = idMittente, idDestinatario
		corpo["id_indirizzo_ritiro"], corpo["id_indirizzo_spedizione"] = idRitiro, idConsegna
		r := f.chiama(t, http.MethodPost, "/api/spedizioni", corpo, op)
		if r.stato != http.StatusCreated {
			t.Fatalf("creazione spedizione: %d %s", r.stato, r.grezzo)
		}
		if r.corpo["stato"] != "creata" {
			t.Fatalf("senza corriere la spedizione nasce creata: %s", r.grezzo)
		}
		d := f.dettaglio(t, int(r.corpo["id"].(float64)), op, 200).corpo
		ritiro := d["indirizzo_ritiro"].(map[string]any)
		if ritiro["via"] != "Via Dante" || ritiro["citta"] != "Milano" || ritiro["nazione"] != nazione ||
			int(d["destinatario"].(map[string]any)["id"].(float64)) != idDestinatario ||
			int(d["mittente"].(map[string]any)["id"].(float64)) != idMittente {
			t.Fatalf("il dettaglio mostra i dati inseriti: %v", d)
		}
	})

	t.Run("spedizione_con_corriere_gia_assegnato", func(t *testing.T) {
		corpo := f.corpoValido("GUI-ASS")
		corpo["id_cliente_mittente"], corpo["id_cliente_destinatario"] = idMittente, idDestinatario
		corpo["id_indirizzo_ritiro"], corpo["id_indirizzo_spedizione"] = idRitiro, idConsegna
		corpo["id_corriere"] = f.idCorriere2
		r := f.chiama(t, http.MethodPost, "/api/spedizioni", corpo, op)
		if r.stato != http.StatusCreated || r.corpo["stato"] != "assegnata" || int32(r.corpo["id_corriere"].(float64)) != f.idCorriere2 {
			t.Fatalf("con id_corriere la spedizione nasce assegnata: %d %s", r.stato, r.grezzo)
		}
		id := int(r.corpo["id"].(float64))
		if n := f.storico(t, id); n != 2 {
			t.Errorf("storico: creazione e assegnazione, trovate %d righe", n)
		}
		if n := f.conta(t, `SELECT count(*) FROM cambiamenti_spedizioni WHERE id_spedizione = $1 AND id_operatore = $2`, id, f.idOperatore); n != 2 {
			t.Errorf("entrambe le righe di storico hanno l'operatore come autore, trovate %d", n)
		}
		voci, _ := f.elenco(t, comeCorriere(f.idCorriere2), "cod_ordine=GUI-ASS-"+f.suffisso, 200)
		if voci[id] == nil {
			t.Error("la spedizione compare subito nell'elenco del corriere")
		}

		corpo = f.corpoValido("GUI-ERR")
		corpo["id_corriere"] = 999999999
		if r := f.chiama(t, http.MethodPost, "/api/spedizioni", corpo, op); r.stato != 400 || !contiene(campiInErrore(r), "id_corriere") {
			t.Errorf("corriere inesistente: atteso 400 sul campo id_corriere, ottenuto %d %s", r.stato, r.grezzo)
		}
		if n := f.conta(t, `SELECT count(*) FROM spedizioni WHERE cod_ordine = $1`, "GUI-ERR-"+f.suffisso); n != 0 {
			t.Errorf("con un corriere inesistente la spedizione non viene creata, trovate %d", n)
		}
	})
}
