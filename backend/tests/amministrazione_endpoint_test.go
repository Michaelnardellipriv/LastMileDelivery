//go:build integration

package apitest

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const (
	percorsoImpostazioni = "/api/impostazioni"
	percorsoMotiviAdmin  = "/api/impostazioni/motivi"
	percorsoUtenti       = "/api/utenti"
)

// impostazioni modifica i parametri globali e li ripristina a fine test: gli altri
// test si aspettano i valori predefiniti (3 tentativi, 30 giorni).
func (f *fixture) impostazioni(t *testing.T, modifica map[string]any) map[string]any {
	t.Helper()
	admin := comeAmministratore(f)
	prima := f.chiama(t, http.MethodGet, percorsoImpostazioni, nil, admin)
	if prima.stato != http.StatusOK {
		t.Fatalf("lettura impostazioni: %d %s", prima.stato, prima.grezzo)
	}
	t.Cleanup(func() {
		ripristino := map[string]any{}
		for _, k := range []string{"giorni_visibilita_cliente", "max_tentativi_ritiro", "max_tentativi_consegna", "intervallo_posizione_sec"} {
			ripristino[k] = prima.corpo[k]
		}
		if r := f.chiama(t, http.MethodPatch, percorsoImpostazioni, ripristino, admin); r.stato != http.StatusOK {
			t.Errorf("ripristino impostazioni: %d %s", r.stato, r.grezzo)
		}
	})
	r := f.chiama(t, http.MethodPatch, percorsoImpostazioni, modifica, admin)
	if r.stato != http.StatusOK {
		t.Fatalf("modifica impostazioni %v: %d %s", modifica, r.stato, r.grezzo)
	}
	return r.corpo
}

func TestImpostazioniAmministratore(t *testing.T) {
	f := preparaFixture(t)
	admin, op, c1, dest := comeAmministratore(f), comeOperatore(f), comeCorriere(f.idCorriere1), comeCliente(f.idDestinatario)

	t.Run("solo_amministratore", func(t *testing.T) {
		for _, chi := range []map[string]string{op, c1, dest} {
			if r := f.chiama(t, http.MethodGet, percorsoImpostazioni, nil, chi); r.stato != http.StatusForbidden {
				t.Errorf("%v legge le impostazioni: atteso 403, ottenuto %d", chi, r.stato)
			}
			if r := f.chiama(t, http.MethodPatch, percorsoImpostazioni, map[string]any{"max_tentativi_ritiro": 2}, chi); r.stato != http.StatusForbidden {
				t.Errorf("%v modifica le impostazioni: atteso 403, ottenuto %d", chi, r.stato)
			}
		}
		r := f.chiama(t, http.MethodGet, percorsoImpostazioni, nil, admin)
		if r.stato != http.StatusOK || r.corpo["max_tentativi_ritiro"] == nil || r.corpo["giorni_visibilita_cliente"] == nil {
			t.Fatalf("l'amministratore legge le impostazioni: %d %s", r.stato, r.grezzo)
		}
	})

	t.Run("valori_fuori_limite_400", func(t *testing.T) {
		for _, corpo := range []map[string]any{
			{}, {"giorni_visibilita_cliente": 0}, {"max_tentativi_ritiro": 11}, {"max_tentativi_consegna": 0},
			{"intervallo_posizione_sec": 4}, {"campo_sconosciuto": 1},
		} {
			if r := f.chiama(t, http.MethodPatch, percorsoImpostazioni, corpo, admin); r.stato != http.StatusBadRequest {
				t.Errorf("%v: atteso 400, ottenuto %d %s", corpo, r.stato, r.grezzo)
			}
		}
	})

	t.Run("modifica_parziale_e_intervallo_condiviso", func(t *testing.T) {
		r := f.impostazioni(t, map[string]any{"intervallo_posizione_sec": 45})
		if r["intervallo_posizione_sec"] != float64(45) || r["id_amministratore_modifica"] != float64(f.idAmministratore) {
			t.Fatalf("modifica: %v", r)
		}
		// lo stesso valore dell'endpoint dedicato all'intervallo
		if g := f.chiama(t, http.MethodGet, percorsoIntervallo, nil, c1); g.corpo["intervallo_secondi"] != float64(45) {
			t.Fatalf("l'intervallo è uno solo: %s", g.grezzo)
		}
	})

	t.Run("tentativi_di_ritiro_dalla_configurazione", func(t *testing.T) {
		f.impostazioni(t, map[string]any{"max_tentativi_ritiro": 1})
		id := f.nuovaSpedizione(t, "CFG-RITIRO")
		f.assegna(t, id, f.idCorriere1, 201)
		f.tentativo(t, id, c1, map[string]any{"tipo": "ritiro", "nota": "chiuso", "motivo": "mittente assente"}, 201)
		f.tentativo(t, id, c1, map[string]any{"tipo": "ritiro", "nota": "chiuso", "motivo": "mittente assente"}, 409)
		// ritiri esauriti: l'operatore annulla senza motivo
		f.stato(t, id, op, map[string]any{"stato": "annullata"}, 200)
	})

	t.Run("tentativi_di_consegna_dalla_configurazione", func(t *testing.T) {
		f.impostazioni(t, map[string]any{"max_tentativi_consegna": 1})
		id := f.nuovaSpedizione(t, "CFG-CONS")
		f.assegna(t, id, f.idCorriere1, 201)
		f.stato(t, id, c1, map[string]any{"stato": "ritirata"}, 200)
		f.stato(t, id, c1, map[string]any{"stato": "in consegna"}, 200)
		r := f.tentativo(t, id, c1, map[string]any{"tipo": "consegna", "nota": "assente", "motivo": "destinatario assente"}, 201)
		if r.corpo["stato"] != "non consegnata" {
			t.Fatalf("con un solo tentativo di consegna il primo fallimento chiude la spedizione: %s", r.grezzo)
		}
	})

	t.Run("giorni_di_visibilita_dalla_configurazione", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "CFG-GIORNI")
		f.consegnaCompleta(t, id)
		f.retrodataStorico(t, id, 10)
		f.dettaglio(t, id, dest, 200)
		f.impostazioni(t, map[string]any{"giorni_visibilita_cliente": 5})
		f.dettaglio(t, id, dest, 404)
	})
}

func TestMotiviAmministratore(t *testing.T) {
	f := preparaFixture(t)
	admin, op, c1 := comeAmministratore(f), comeOperatore(f), comeCorriere(f.idCorriere1)
	descrizione := "cliente in ferie " + f.suffisso

	for _, chi := range []map[string]string{op, c1, comeCliente(f.idDestinatario)} {
		if r := f.chiama(t, http.MethodGet, percorsoMotiviAdmin, nil, chi); r.stato != http.StatusForbidden {
			t.Errorf("%v elenca i motivi dell'amministratore: atteso 403, ottenuto %d", chi, r.stato)
		}
		if r := f.chiama(t, http.MethodPost, percorsoMotiviAdmin, map[string]any{"descrizione": "x"}, chi); r.stato != http.StatusForbidden {
			t.Errorf("%v crea motivi: atteso 403, ottenuto %d", chi, r.stato)
		}
	}

	r := f.chiama(t, http.MethodPost, percorsoMotiviAdmin, map[string]any{"descrizione": descrizione}, admin)
	if r.stato != http.StatusCreated || r.corpo["attivo"] != true {
		t.Fatalf("nuovo motivo: %d %s", r.stato, r.grezzo)
	}
	idMotivo := strconv.Itoa(int(r.corpo["id"].(float64)))
	t.Cleanup(func() {
		f.chiama(t, http.MethodPatch, percorsoMotiviAdmin+"/"+idMotivo, map[string]any{"attivo": false}, admin)
	})
	if r := f.chiama(t, http.MethodPost, percorsoMotiviAdmin, map[string]any{"descrizione": "Cliente In Ferie " + f.suffisso}, admin); r.stato != http.StatusConflict {
		t.Fatalf("descrizione già presente (maiuscole a parte): atteso 409, ottenuto %d %s", r.stato, r.grezzo)
	}

	attivi := func() []any {
		r := f.chiama(t, http.MethodGet, "/api/motivi", nil, c1)
		voci, _ := r.corpo["dati"].([]any)
		return voci
	}
	if !slices.Contains(attivi(), any(descrizione)) {
		t.Fatal("il nuovo motivo compare subito nell'elenco pubblico")
	}

	id := f.nuovaSpedizione(t, "CFG-MOTIVO")
	f.assegna(t, id, f.idCorriere1, 201)
	f.tentativo(t, id, c1, map[string]any{"tipo": "ritiro", "nota": "chiuso", "motivo": descrizione}, 201)

	if r := f.chiama(t, http.MethodPatch, percorsoMotiviAdmin+"/"+idMotivo, map[string]any{"attivo": false}, admin); r.stato != http.StatusOK || r.corpo["attivo"] != false {
		t.Fatalf("disattivazione: %d %s", r.stato, r.grezzo)
	}
	if slices.Contains(attivi(), any(descrizione)) {
		t.Fatal("il motivo disattivato sparisce dall'elenco pubblico")
	}
	if r := f.tentativo(t, id, c1, map[string]any{"tipo": "ritiro", "nota": "chiuso", "motivo": descrizione}, 400); !contiene(campiInErrore(r), "motivo") {
		t.Fatalf("un motivo disattivato non si usa più: %s", r.grezzo)
	}
	// la nota già scritta col motivo resta com'era
	if n := f.conta(t, `SELECT count(*) FROM note WHERE id_spedizione = $1 AND motivo = $2`, id, descrizione); n != 1 {
		t.Fatalf("la nota col motivo disattivato resta: %d", n)
	}

	elenco := f.chiama(t, http.MethodGet, percorsoMotiviAdmin, nil, admin)
	trovato := false
	for _, v := range elenco.corpo["dati"].([]any) {
		m := v.(map[string]any)
		if m["descrizione"] == descrizione {
			trovato = m["attivo"] == false
		}
	}
	if !trovato {
		t.Fatalf("l'amministratore vede anche i motivi disattivati: %s", elenco.grezzo)
	}
	if r := f.chiama(t, http.MethodPatch, percorsoMotiviAdmin+"/999999", map[string]any{"attivo": true}, admin); r.stato != http.StatusNotFound {
		t.Fatalf("motivo inesistente: atteso 404, ottenuto %d", r.stato)
	}
}

func TestUtentiAmministratore(t *testing.T) {
	f := preparaFixture(t)
	admin, op := comeAmministratore(f), comeOperatore(f)
	nuovo := func(ruolo, nome string) map[string]any {
		return map[string]any{"ruolo": ruolo, "nome": nome, "cognome": "Nuovo", "prefisso": "+39",
			"nr_telefono": telefonoUnico(), "email": f.emailDi(nome), "password": "PasswordNuova1"}
	}

	t.Run("solo_amministratore", func(t *testing.T) {
		for _, chi := range []map[string]string{op, comeCorriere(f.idCorriere1), comeCliente(f.idDestinatario)} {
			if r := f.chiama(t, http.MethodPost, percorsoUtenti, nuovo("corriere", "vietato"), chi); r.stato != http.StatusForbidden {
				t.Errorf("%v crea utenti: atteso 403, ottenuto %d", chi, r.stato)
			}
			if r := f.chiama(t, http.MethodGet, percorsoUtenti, nil, chi); r.stato != http.StatusForbidden {
				t.Errorf("%v elenca utenti: atteso 403, ottenuto %d", chi, r.stato)
			}
		}
	})

	t.Run("validazione", func(t *testing.T) {
		for _, ruolo := range []any{nil, "cliente", "amministratore"} {
			corpo := nuovo("x", "ruolo")
			corpo["ruolo"] = ruolo
			if r := f.chiama(t, http.MethodPost, percorsoUtenti, corpo, admin); r.stato != 400 || !contiene(campiInErrore(r), "ruolo") {
				t.Errorf("ruolo %v: atteso 400 sul campo ruolo, ottenuto %d %s", ruolo, r.stato, r.grezzo)
			}
		}
		doppio := nuovo("operatore", "doppio")
		doppio["email"] = f.emailDi("operatore")
		if r := f.chiama(t, http.MethodPost, percorsoUtenti, doppio, admin); r.stato != 409 {
			t.Errorf("email già usata: atteso 409, ottenuto %d", r.stato)
		}
	})

	t.Run("corriere_creato_disattivato_e_riabilitato", func(t *testing.T) {
		r := f.chiama(t, http.MethodPost, percorsoUtenti, nuovo("corriere", "corrierenuovo"), admin)
		if r.stato != http.StatusCreated {
			t.Fatalf("nuovo corriere: %d %s", r.stato, r.grezzo)
		}
		utente := r.corpo["utente"].(map[string]any)
		idUtente := strconv.Itoa(int(utente["id"].(float64)))
		idCorriere := int32(utente["id_ruolo"].(float64))
		if utente["ruolo"] != "corriere" || utente["attivo"] != true {
			t.Fatalf("dati del corriere: %s", r.grezzo)
		}

		sessione := f.login(t, map[string]any{"email": f.emailDi("corrierenuovo"), "password": "PasswordNuova1"}, 200)
		token := conToken(sessione)
		if r := f.chiama(t, http.MethodGet, "/api/spedizioni", nil, token); r.stato != http.StatusOK {
			t.Fatalf("il nuovo corriere usa l'API: %d", r.stato)
		}
		id := f.nuovaSpedizione(t, "UTENTE-ATTIVO")
		f.assegna(t, id, idCorriere, 201)

		disattiva := f.chiama(t, http.MethodPatch, percorsoUtenti+"/"+idUtente, map[string]any{"attivo": false}, admin)
		if disattiva.stato != http.StatusOK || disattiva.corpo["attivo"] != false {
			t.Fatalf("disattivazione: %d %s", disattiva.stato, disattiva.grezzo)
		}
		f.login(t, map[string]any{"email": f.emailDi("corrierenuovo"), "password": "PasswordNuova1"}, 403)
		if r := f.chiama(t, http.MethodGet, "/api/spedizioni", nil, token); r.stato != http.StatusForbidden {
			t.Fatalf("il token di un utente disattivato non vale più: %d", r.stato)
		}
		if r := f.chiama(t, http.MethodGet, "/api/spedizioni", nil, comeCorriere(idCorriere)); r.stato != http.StatusForbidden {
			t.Fatalf("neanche con l'header di sviluppo: %d", r.stato)
		}
		altra := f.nuovaSpedizione(t, "UTENTE-DISATTIVO")
		if r := f.assegna(t, altra, idCorriere, 409); !contiene(campiInErrore(r), "id_corriere") {
			t.Fatalf("a un corriere disattivato non si assegnano spedizioni: %s", r.grezzo)
		}
		corrieri := f.chiama(t, http.MethodGet, "/api/corrieri?q="+f.emailDi("corrierenuovo"), nil, op)
		if corrieri.corpo["totale_elementi"] != float64(0) {
			t.Fatalf("l'operatore non vede il corriere disattivato tra quelli assegnabili: %s", corrieri.grezzo)
		}
		disattivi := f.chiama(t, http.MethodGet, percorsoUtenti+"?ruolo=corriere&attivo=false&q="+f.emailDi("corrierenuovo"), nil, admin)
		if disattivi.corpo["totale_elementi"] != float64(1) {
			t.Fatalf("l'amministratore trova il corriere disattivato: %s", disattivi.grezzo)
		}

		if r := f.chiama(t, http.MethodPatch, percorsoUtenti+"/"+idUtente, map[string]any{"attivo": true}, admin); r.stato != http.StatusOK {
			t.Fatalf("riabilitazione: %d %s", r.stato, r.grezzo)
		}
		f.login(t, map[string]any{"email": f.emailDi("corrierenuovo"), "password": "PasswordNuova1"}, 200)
		f.assegna(t, altra, idCorriere, 201)
	})

	t.Run("operatore_creato_puo_creare_clienti", func(t *testing.T) {
		r := f.chiama(t, http.MethodPost, percorsoUtenti, nuovo("operatore", "operatorenuovo"), admin)
		if r.stato != http.StatusCreated || r.corpo["utente"].(map[string]any)["ruolo"] != "operatore" {
			t.Fatalf("nuovo operatore: %d %s", r.stato, r.grezzo)
		}
		token := conToken(f.login(t, map[string]any{"email": f.emailDi("operatorenuovo"), "password": "PasswordNuova1"}, 200))
		cliente := map[string]any{"nome": "Lia", "cognome": "Cliente", "prefisso": "+39", "nr_telefono": telefonoUnico(),
			"email": f.emailDi("clientedioperatore"), "password": "PasswordLia1"}
		if r := f.chiama(t, http.MethodPost, "/api/clienti", cliente, token); r.stato != http.StatusCreated {
			t.Fatalf("il nuovo operatore crea un cliente destinatario: %d %s", r.stato, r.grezzo)
		}
	})

	t.Run("modifica_dati_e_password", func(t *testing.T) {
		r := f.chiama(t, http.MethodPost, percorsoUtenti, nuovo("operatore", "damodificare"), admin)
		if r.stato != http.StatusCreated {
			t.Fatalf("nuovo operatore: %d %s", r.stato, r.grezzo)
		}
		percorso := percorsoUtenti + "/" + strconv.Itoa(int(r.corpo["utente"].(map[string]any)["id"].(float64)))
		telefono := telefonoUnico()

		m := f.chiama(t, http.MethodPatch, percorso, map[string]any{
			"cognome": "Modificato", "nr_telefono": telefono, "email": f.emailDi("modificato"),
		}, admin)
		if m.stato != http.StatusOK || m.corpo["cognome"] != "Modificato" || m.corpo["nr_telefono"] != telefono ||
			m.corpo["email"] != f.emailDi("modificato") || m.corpo["nome"] != "damodificare" || m.corpo["attivo"] != true {
			t.Fatalf("modifica: si cambiano solo i campi indicati: %d %s", m.stato, m.grezzo)
		}
		f.login(t, map[string]any{"email": f.emailDi("damodificare"), "password": "PasswordNuova1"}, 401)
		f.login(t, map[string]any{"email": f.emailDi("modificato"), "password": "PasswordNuova1"}, 200)

		if p := f.chiama(t, http.MethodPatch, percorso, map[string]any{"password": "PasswordCambiata2"}, admin); p.stato != http.StatusOK {
			t.Fatalf("nuova password: %d %s", p.stato, p.grezzo)
		}
		f.login(t, map[string]any{"email": f.emailDi("modificato"), "password": "PasswordNuova1"}, 401)
		f.login(t, map[string]any{"email": f.emailDi("modificato"), "password": "PasswordCambiata2"}, 200)

		// la propria email con maiuscole diverse va bene, quella di un altro no
		if r := f.chiama(t, http.MethodPatch, percorso, map[string]any{"email": strings.ToUpper(f.emailDi("modificato"))}, admin); r.stato != http.StatusOK {
			t.Errorf("la propria email: atteso 200, ottenuto %d %s", r.stato, r.grezzo)
		}
		if r := f.chiama(t, http.MethodPatch, percorso, map[string]any{"email": f.emailDi("operatore")}, admin); r.stato != 409 || !contiene(campiInErrore(r), "email") {
			t.Errorf("email di un altro utente: atteso 409, ottenuto %d %s", r.stato, r.grezzo)
		}
		var telefonoAltrui string
		if err := f.db.QueryRow(t.Context(), `SELECT nr_telefono FROM utenti WHERE email = $1`, f.emailDi("operatore")).Scan(&telefonoAltrui); err != nil {
			t.Fatal(err)
		}
		if r := f.chiama(t, http.MethodPatch, percorso, map[string]any{"nr_telefono": telefonoAltrui}, admin); r.stato != 409 || !contiene(campiInErrore(r), "nr_telefono") {
			t.Errorf("telefono di un altro utente: atteso 409, ottenuto %d %s", r.stato, r.grezzo)
		}
		for _, corpo := range []map[string]any{{}, {"nome": ""}, {"prefisso": "39"}, {"password": "corta"}, {"ruolo": "corriere"}} {
			if r := f.chiama(t, http.MethodPatch, percorso, corpo, admin); r.stato != http.StatusBadRequest {
				t.Errorf("%v: atteso 400, ottenuto %d %s", corpo, r.stato, r.grezzo)
			}
		}
		if r := f.chiama(t, http.MethodPatch, percorso, map[string]any{"nome": "Vietato"}, op); r.stato != http.StatusForbidden {
			t.Errorf("l'operatore non modifica gli utenti: %d", r.stato)
		}
	})

	t.Run("clienti", func(t *testing.T) {
		// due clienti nuovi, creati dall'operatore: uno diventa destinatario, l'altro resta solo mittente
		creaCliente := func(nome string) (idCliente int, percorso string) {
			corpo := map[string]any{"nome": nome, "cognome": "Cliente", "prefisso": "+39", "nr_telefono": telefonoUnico(),
				"email": f.emailDi(nome), "password": "PasswordCliente1"}
			r := f.chiama(t, http.MethodPost, "/api/clienti", corpo, op)
			if r.stato != http.StatusCreated {
				t.Fatalf("creazione cliente %s: %d %s", nome, r.stato, r.grezzo)
			}
			c := r.corpo["cliente"].(map[string]any)
			if c["attivo"] != true || c["destinatario"] != false {
				t.Errorf("un cliente appena creato è attivo e mittente: %v", c)
			}
			return int(c["id"].(float64)), percorsoUtenti + "/" + strconv.Itoa(int(c["id_utente"].(float64)))
		}
		idDest, percorsoDest := creaCliente("clientedest")
		_, percorsoMitt := creaCliente("clientemitt")
		spedizione := func(codice string, idDestinatario int) risposta {
			corpo := f.corpoValido(codice)
			corpo["id_cliente_destinatario"] = idDestinatario
			return f.chiama(t, http.MethodPost, "/api/spedizioni", corpo, op)
		}
		if r := spedizione("CLI-DEST", idDest); r.stato != http.StatusCreated {
			t.Fatalf("spedizione al nuovo destinatario: %d %s", r.stato, r.grezzo)
		}

		// l'operatore vede chi è destinatario e chi mittente
		r := f.chiama(t, http.MethodGet, "/api/clienti?q="+f.suffisso+"&per_pagina=100", nil, op)
		if r.stato != http.StatusOK {
			t.Fatalf("elenco clienti dell'operatore: %d %s", r.stato, r.grezzo)
		}
		perEmail := map[string]any{}
		for _, v := range r.corpo["dati"].([]any) {
			c := v.(map[string]any)
			perEmail[c["email"].(string)] = c["destinatario"]
		}
		if perEmail[f.emailDi("clientedest")] != true || perEmail[f.emailDi("clientemitt")] != false {
			t.Errorf("GET /api/clienti: destinatario true per chi ha ricevuto una spedizione, false per il mittente; ottenuto %v", perEmail)
		}

		// l'amministratore vede anche i clienti, con destinatario o solo mittente
		r = f.chiama(t, http.MethodGet, percorsoUtenti+"?ruolo=cliente&q="+f.suffisso, nil, admin)
		if r.stato != http.StatusOK {
			t.Fatalf("elenco clienti: %d %s", r.stato, r.grezzo)
		}
		tipi := map[string]any{}
		for _, v := range r.corpo["dati"].([]any) {
			u := v.(map[string]any)
			if u["ruolo"] != "cliente" {
				t.Errorf("filtro ruolo=cliente: trovato %v", u)
			}
			tipi[u["email"].(string)] = u["destinatario"]
		}
		if tipi[f.emailDi("clientedest")] != true || tipi[f.emailDi("clientemitt")] != false {
			t.Errorf("destinatario: atteso true per il destinatario e false per il solo mittente, ottenuto %v", tipi)
		}
		r = f.chiama(t, http.MethodGet, percorsoUtenti+"?q="+f.suffisso+"&per_pagina=100", nil, admin)
		ruoli := map[any]bool{}
		for _, v := range r.corpo["dati"].([]any) {
			ruoli[v.(map[string]any)["ruolo"]] = true
		}
		if !ruoli["cliente"] || !ruoli["corriere"] || !ruoli["operatore"] {
			t.Errorf("senza filtro compaiono tutti i ruoli: %v", ruoli)
		}

		// l'operatore cambia solo i dati dei clienti
		for _, percorso := range []string{percorsoDest, percorsoMitt} {
			if r := f.chiama(t, http.MethodPatch, percorso, map[string]any{"cognome": "Modificato"}, op); r.stato != http.StatusOK || r.corpo["cognome"] != "Modificato" {
				t.Errorf("l'operatore modifica i dati di un cliente: %d %s", r.stato, r.grezzo)
			}
		}
		for _, corpo := range []map[string]any{{"attivo": false}, {"password": "PasswordNuova9"}, {"nome": "X", "attivo": true}} {
			if r := f.chiama(t, http.MethodPatch, percorsoDest, corpo, op); r.stato != http.StatusForbidden {
				t.Errorf("l'operatore %v: atteso 403, ottenuto %d %s", corpo, r.stato, r.grezzo)
			}
		}

		// il solo mittente non accede all'app: niente stato né password
		for _, corpo := range []map[string]any{{"attivo": false}, {"password": "PasswordNuova9"}} {
			if r := f.chiama(t, http.MethodPatch, percorsoMitt, corpo, admin); r.stato != http.StatusConflict {
				t.Errorf("solo mittente %v: atteso 409, ottenuto %d %s", corpo, r.stato, r.grezzo)
			}
		}

		// il destinatario: nuova password, poi disattivazione
		if r := f.chiama(t, http.MethodPatch, percorsoDest, map[string]any{"password": "PasswordNuova9"}, admin); r.stato != http.StatusOK {
			t.Fatalf("nuova password al destinatario: %d %s", r.stato, r.grezzo)
		}
		sessione := f.login(t, map[string]any{"email": f.emailDi("clientedest"), "password": "PasswordNuova9"}, 200)
		if r := f.chiama(t, http.MethodPatch, percorsoDest, map[string]any{"attivo": false}, admin); r.stato != http.StatusOK || r.corpo["attivo"] != false {
			t.Fatalf("disattivazione del destinatario: %d %s", r.stato, r.grezzo)
		}
		f.login(t, map[string]any{"email": f.emailDi("clientedest"), "password": "PasswordNuova9"}, 403)
		if r := f.chiama(t, http.MethodGet, "/api/spedizioni", nil, conToken(sessione)); r.stato != http.StatusForbidden {
			t.Errorf("il token del destinatario disattivato non vale più: %d", r.stato)
		}

		// un destinatario disattivato non riceve spedizioni nuove, né per modifica
		if r := spedizione("CLI-OFF", idDest); r.stato != http.StatusConflict || !contiene(campiInErrore(r), "id_cliente_destinatario") {
			t.Errorf("spedizione a un destinatario disattivato: atteso 409, ottenuto %d %s", r.stato, r.grezzo)
		}
		altra := f.nuovaSpedizione(t, "CLI-MOD")
		if r := f.chiama(t, http.MethodPatch, "/api/spedizioni/"+strconv.Itoa(altra), map[string]any{"id_cliente_destinatario": idDest}, op); r.stato != http.StatusConflict {
			t.Errorf("modifica verso un destinatario disattivato: atteso 409, ottenuto %d %s", r.stato, r.grezzo)
		}

		// riattivato, torna tutto come prima
		if r := f.chiama(t, http.MethodPatch, percorsoDest, map[string]any{"attivo": true}, admin); r.stato != http.StatusOK {
			t.Fatalf("riattivazione: %d %s", r.stato, r.grezzo)
		}
		f.login(t, map[string]any{"email": f.emailDi("clientedest"), "password": "PasswordNuova9"}, 200)
		if r := spedizione("CLI-ON", idDest); r.stato != http.StatusCreated {
			t.Errorf("spedizione al destinatario riattivato: %d %s", r.stato, r.grezzo)
		}

		// gli amministratori non si modificano da qui
		var idUtenteAdmin int32
		if err := f.db.QueryRow(t.Context(), `SELECT id_utente FROM amministratori WHERE id = $1`, f.idAmministratore).Scan(&idUtenteAdmin); err != nil {
			t.Fatal(err)
		}
		if r := f.chiama(t, http.MethodPatch, percorsoUtenti+"/"+strconv.Itoa(int(idUtenteAdmin)), map[string]any{"nome": "X"}, admin); r.stato != http.StatusNotFound {
			t.Errorf("amministratore: atteso 404, ottenuto %d", r.stato)
		}
	})
}
