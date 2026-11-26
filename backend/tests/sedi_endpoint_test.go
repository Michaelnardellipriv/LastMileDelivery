//go:build integration

package apitest

import (
	"context"
	"net/http"
	"strconv"
	"testing"
)

func TestSediAmministratore(t *testing.T) {
	f := preparaFixture(t)
	admin, op, c1, dest := comeAmministratore(f), comeOperatore(f), comeCorriere(f.idCorriere1), comeCliente(f.idDestinatario)
	nuova := func(nome, citta string) map[string]any {
		return map[string]any{"nome": nome + " " + f.suffisso, "indirizzo": map[string]any{
			"via": "Corso Rosmini", "civico": "45", "citta": citta, "cap": "38068", "regione": "Regione", "nazione": f.nazione}}
	}

	t.Run("crea_sede_201", func(t *testing.T) {
		r := f.chiama(t, http.MethodPost, "/api/sedi", nuova("Sede Rovereto", "Rovereto"), admin)
		if r.stato != http.StatusCreated {
			t.Fatalf("nuova sede: atteso 201, ottenuto %d %s", r.stato, r.grezzo)
		}
		ind := r.corpo["indirizzo"].(map[string]any)
		if ind["citta"] != "Rovereto" || ind["nazione"] != f.nazione || r.corpo["id"] == nil {
			t.Fatalf("la sede ha città, regione e nazione dall'indirizzo: %s", r.grezzo)
		}
		id := strconv.Itoa(int(r.corpo["id"].(float64)))
		imp := f.chiama(t, http.MethodGet, "/api/sedi/"+id+"/impostazioni", nil, admin)
		if imp.stato != http.StatusOK || imp.corpo["max_tentativi_ritiro"] != float64(3) {
			t.Fatalf("la sede nasce con le impostazioni predefinite: %d %s", imp.stato, imp.grezzo)
		}
	})

	t.Run("una_sede_per_citta_409", func(t *testing.T) {
		r := f.chiama(t, http.MethodPost, "/api/sedi", nuova("Seconda Trento", "Trento"), admin)
		if r.stato != http.StatusConflict {
			t.Fatalf("Trento ha già una sede: atteso 409, ottenuto %d %s", r.stato, r.grezzo)
		}
	})

	t.Run("validazione_400", func(t *testing.T) {
		senzaNome := nuova("x", "Arco")
		delete(senzaNome, "nome")
		for _, corpo := range []map[string]any{senzaNome, {"nome": "Sede", "indirizzo": map[string]any{"via": "Via X"}}} {
			if r := f.chiama(t, http.MethodPost, "/api/sedi", corpo, admin); r.stato != http.StatusBadRequest {
				t.Errorf("%v: atteso 400, ottenuto %d %s", corpo, r.stato, r.grezzo)
			}
		}
	})

	t.Run("solo_amministratore_crea_403", func(t *testing.T) {
		for _, chi := range []map[string]string{op, c1, dest} {
			if r := f.chiama(t, http.MethodPost, "/api/sedi", nuova("Vietata", "Arco"), chi); r.stato != http.StatusForbidden {
				t.Errorf("%v crea una sede: atteso 403, ottenuto %d", chi, r.stato)
			}
		}
	})

	t.Run("chi_vede_quali_sedi", func(t *testing.T) {
		r := f.chiama(t, http.MethodGet, "/api/sedi?per_pagina=100&q="+f.suffisso, nil, admin)
		if r.stato != http.StatusOK || len(r.corpo["dati"].([]any)) < 2 {
			t.Fatalf("l'amministratore vede tutte le sedi: %d %s", r.stato, r.grezzo)
		}
		r = f.chiama(t, http.MethodGet, "/api/sedi", nil, op)
		dati := r.corpo["dati"].([]any)
		if r.stato != http.StatusOK || len(dati) != 1 || int64(dati[0].(map[string]any)["id"].(float64)) != f.idSede {
			t.Fatalf("l'operatore vede solo la sua sede: %d %s", r.stato, r.grezzo)
		}
		altra := "/api/sedi/" + strconv.FormatInt(f.idSedeAltra, 10)
		if r := f.chiama(t, http.MethodGet, altra, nil, op); r.stato != http.StatusForbidden {
			t.Errorf("l'operatore non vede un'altra sede: atteso 403, ottenuto %d", r.stato)
		}
		if r := f.chiama(t, http.MethodGet, altra, nil, admin); r.stato != http.StatusOK {
			t.Errorf("l'amministratore vede ogni sede: atteso 200, ottenuto %d", r.stato)
		}
		if r := f.chiama(t, http.MethodGet, "/api/sedi/999999999", nil, admin); r.stato != http.StatusNotFound {
			t.Errorf("sede inesistente: atteso 404, ottenuto %d", r.stato)
		}
	})
}

func TestUnaSedeNonVedeLeSpedizioniDiUnAltra(t *testing.T) {
	f := preparaFixture(t)
	admin, altro := comeAmministratore(f), comeOperatoreAltro(f)
	id := f.nuovaSpedizione(t, "SEDE-A")
	percorso := "/api/spedizioni/" + strconv.Itoa(id)

	t.Run("operatore_di_un_altra_sede_404", func(t *testing.T) {
		voci, _ := f.elenco(t, altro, "per_pagina=100", http.StatusOK)
		if _, c := voci[id]; c {
			t.Fatal("la spedizione di Trento non compare nell'elenco di Bolzano")
		}
		for _, c := range []struct {
			metodo, sotto string
			corpo         any
		}{
			{http.MethodGet, "", nil},
			{http.MethodGet, "/storico", nil},
			{http.MethodPatch, "", map[string]any{"is_reso": true}},
			{http.MethodPatch, "/stato", map[string]any{"stato": "annullata", "motivo": "altro", "nota": "x"}},
			{http.MethodPost, "/note", map[string]any{"nota": "x"}},
		} {
			if r := f.chiama(t, c.metodo, percorso+c.sotto, c.corpo, altro); r.stato != http.StatusNotFound {
				t.Errorf("%s %s da un'altra sede: atteso 404 (come se non esistesse), ottenuto %d %s", c.metodo, c.sotto, r.stato, r.grezzo)
			}
		}
	})

	t.Run("amministratore_sceglie_sempre_una_sede", func(t *testing.T) {
		// non c'è più la vista "tutte le sedi": senza id_sede la richiesta è rifiutata
		_, r := f.elenco(t, admin, "per_pagina=100&cod_ordine=SEDE-A-"+f.suffisso, http.StatusBadRequest)
		if !contiene(campiInErrore(r), "id_sede") {
			t.Fatalf("senza sede: atteso l'errore sul campo id_sede: %s", r.grezzo)
		}
		_, r = f.elenco(t, admin, "id_sede=999999999", http.StatusNotFound)
		if r.corpo["errore"] != "sede non trovata" {
			t.Errorf("sede inesistente: atteso \"sede non trovata\", ottenuto %s", r.grezzo)
		}
		f.elenco(t, admin, "id_sede=0", http.StatusBadRequest)

		trento := "id_sede=" + strconv.FormatInt(f.idSede, 10)
		voci, _ := f.elenco(t, admin, trento+"&per_pagina=100&cod_ordine=SEDE-A-"+f.suffisso, http.StatusOK)
		v, c := voci[id]
		if !c || int64(v["id_sede"].(float64)) != f.idSede || v["sede"] == "" {
			t.Fatalf("l'amministratore vede la spedizione con la sua sede: %v", v)
		}
		voci, _ = f.elenco(t, admin, trento+"&per_pagina=100", http.StatusOK)
		for idVoce, v := range voci {
			if int64(v["id_sede"].(float64)) != f.idSede {
				t.Errorf("scelta Trento arrivano solo spedizioni di Trento (spedizione %d della sede %v)", idVoce, v["id_sede"])
			}
		}
		voci, _ = f.elenco(t, admin, "id_sede="+strconv.FormatInt(f.idSedeAltra, 10)+"&cod_ordine=SEDE-A-"+f.suffisso, http.StatusOK)
		if len(voci) != 0 {
			t.Fatal("filtrando su Bolzano la spedizione di Trento non c'è")
		}
	})

	t.Run("filtro_sede_solo_per_amministratore", func(t *testing.T) {
		f.elenco(t, comeOperatore(f), "id_sede="+strconv.FormatInt(f.idSedeAltra, 10), http.StatusForbidden)
	})
}

func TestConsegneDentroLaCittaDellaSede(t *testing.T) {
	f := preparaFixture(t)

	t.Run("corriere_di_un_altra_sede_409", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "SEDE-ASS")
		r := f.assegna(t, id, f.idCorriereAltro, http.StatusConflict)
		if !contiene(campiInErrore(r), "id_corriere") {
			t.Fatalf("errore sul campo id_corriere: %s", r.grezzo)
		}
		corpo := f.corpoValido("SEDE-ASS-NEW")
		corpo["id_corriere"] = f.idCorriereAltro
		if r := f.chiama(t, http.MethodPost, "/api/spedizioni", corpo, comeOperatore(f)); r.stato != http.StatusConflict {
			t.Fatalf("creazione con un corriere di un'altra sede: atteso 409, ottenuto %d %s", r.stato, r.grezzo)
		}
	})

	t.Run("indirizzo_in_un_altra_citta_400", func(t *testing.T) {
		corpo := f.corpoValido("SEDE-IND")
		corpo["id_indirizzo_spedizione"] = f.idIndirizzoAltro
		r := f.chiama(t, http.MethodPost, "/api/spedizioni", corpo, comeOperatore(f))
		if r.stato != http.StatusBadRequest || !contiene(campiInErrore(r), "id_indirizzo_spedizione") {
			t.Fatalf("consegna a Bolzano per una sede di Trento: atteso 400, ottenuto %d %s", r.stato, r.grezzo)
		}
		id := f.nuovaSpedizione(t, "SEDE-IND-MOD")
		r = f.modifica(t, id, comeOperatore(f), map[string]any{"id_indirizzo_ritiro": f.idIndirizzoAltro}, http.StatusBadRequest)
		if !contiene(campiInErrore(r), "id_indirizzo_ritiro") {
			t.Fatalf("modifica con ritiro a Bolzano: %s", r.grezzo)
		}
	})

	t.Run("corriere_con_spedizioni_aperte_non_cambia_sede", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "SEDE-SPOSTA")
		f.assegna(t, id, f.idCorriere2, http.StatusCreated)
		var idUtente int64
		if err := f.db.QueryRow(t.Context(), `SELECT id_utente FROM corrieri WHERE id = $1`, f.idCorriere2).Scan(&idUtente); err != nil {
			t.Fatal(err)
		}
		percorso := percorsoUtenti + "/" + strconv.FormatInt(idUtente, 10)
		r := f.chiama(t, http.MethodPatch, percorso, map[string]any{"id_sede": f.idSedeAltra}, comeAmministratore(f))
		if r.stato != http.StatusConflict || !contiene(campiInErrore(r), "id_sede") {
			t.Fatalf("spostare un corriere con spedizioni aperte: atteso 409, ottenuto %d %s", r.stato, r.grezzo)
		}
	})
}

func TestIdempotenzaNelDatabase(t *testing.T) {
	f := preparaFixture(t)
	corpo := f.corpoValido("IDEM-DB")
	chi := comeOperatore(f)
	chi["Idempotency-Key"] = "idem-db-" + f.suffisso

	prima := f.chiama(t, http.MethodPost, "/api/spedizioni", corpo, chi)
	seconda := f.chiama(t, http.MethodPost, "/api/spedizioni", corpo, chi)
	if prima.stato != http.StatusCreated || seconda.stato != http.StatusCreated || seconda.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("la seconda richiesta restituisce la prima risposta: %d %d %v", prima.stato, seconda.stato, seconda.header)
	}
	if prima.corpo["id"] != seconda.corpo["id"] {
		t.Fatalf("stessa spedizione nelle due risposte: %v %v", prima.corpo["id"], seconda.corpo["id"])
	}
	if n := f.conta(t, `SELECT count(*) FROM spedizioni WHERE cod_ordine = $1`, corpo["cod_ordine"]); n != 1 {
		t.Fatalf("una sola spedizione creata, trovate %d", n)
	}
	// la chiave sta nel database: vale per tutte le repliche e dopo un riavvio
	if n := f.conta(t, `SELECT count(*) FROM chiavi_idempotenza WHERE chiave LIKE '%' || $1 AND stato_http = 201`, chi["Idempotency-Key"]); n != 1 {
		t.Fatalf("risposta salvata nella tabella chiavi_idempotenza: %d", n)
	}
	diverso := f.corpoValido("IDEM-DB-ALTRO")
	if r := f.chiama(t, http.MethodPost, "/api/spedizioni", diverso, chi); r.stato != http.StatusConflict {
		t.Fatalf("stessa chiave con dati diversi: atteso 409, ottenuto %d", r.stato)
	}
}

// La Row-Level Security è la rete di sicurezza: anche una query senza filtro per sede,
// eseguita da un utente normale, vede solo le righe della sede dichiarata.
func TestRowLevelSecurity(t *testing.T) {
	f := preparaFixture(t)
	id := f.nuovaSpedizione(t, "RLS")
	ctx := context.Background()
	tx, err := f.sped.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) // annulla anche il ruolo di prova

	ruolo := "rls_prova_" + f.suffisso
	for _, sql := range []string{
		`CREATE ROLE ` + ruolo + ` NOLOGIN NOSUPERUSER NOBYPASSRLS`,
		`GRANT SELECT ON spedizioni, note, cambiamenti_spedizioni TO ` + ruolo,
		`SET LOCAL ROLE ` + ruolo,
	} {
		if _, err := tx.Exec(ctx, sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	visibili := func(tutte, sede string) int {
		t.Helper()
		if _, err := tx.Exec(ctx, `SELECT set_config('app.tutte_le_sedi', $1, true), set_config('app.id_sede', $2, true)`, tutte, sede); err != nil {
			t.Fatal(err)
		}
		var n int
		// nessun filtro per sede nella query: filtra il database
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM spedizioni WHERE id = $1`, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := visibili("", strconv.FormatInt(f.idSede, 10)); n != 1 {
		t.Errorf("la sede di Trento vede la sua spedizione: %d", n)
	}
	if n := visibili("", strconv.FormatInt(f.idSedeAltra, 10)); n != 0 {
		t.Errorf("la sede di Bolzano non vede la spedizione di Trento: %d", n)
	}
	if n := visibili("si", ""); n != 1 {
		t.Errorf("l'amministratore vede tutte le sedi: %d", n)
	}
	if n := visibili("", ""); n != 0 {
		t.Errorf("senza sede dichiarata non si vede nulla: %d", n)
	}
}

func TestModificaEDisattivazioneSede(t *testing.T) {
	f := preparaFixture(t)
	admin, op := comeAmministratore(f), comeOperatore(f)
	sede := "/api/sedi/" + strconv.FormatInt(f.idSede, 10)
	var citta string
	if err := f.db.QueryRow(t.Context(), `SELECT ci.nome FROM sedi se JOIN citta ci ON ci.id = se.id_citta WHERE se.id = $1`, f.idSede).Scan(&citta); err != nil {
		t.Fatal(err)
	}
	indirizzo := func(citta string) map[string]any {
		return map[string]any{"via": "Via Nuova", "civico": "9", "citta": citta, "cap": "38122", "regione": "Regione", "nazione": f.nazione}
	}
	// un corriere da spostare: creato dall'amministratore nella sede altra, senza spedizioni
	nuovoCorriere := func(nome string, idSede int64) risposta {
		return f.chiama(t, http.MethodPost, percorsoUtenti, map[string]any{"ruolo": "corriere", "id_sede": idSede, "nome": nome,
			"cognome": "Sede", "prefisso": "+39", "nr_telefono": telefonoUnico(), "email": f.emailDi(nome), "password": "PasswordCorriere1"}, admin)
	}
	r := nuovoCorriere("spostato", f.idSedeAltra)
	if r.stato != http.StatusCreated {
		t.Fatalf("nuovo corriere: %d %s", r.stato, r.grezzo)
	}
	utente := r.corpo["utente"].(map[string]any)
	percorsoCorriere := percorsoUtenti + "/" + strconv.Itoa(int(utente["id"].(float64)))
	if int64(utente["id_sede"].(float64)) != f.idSedeAltra || utente["sede"] == nil {
		t.Fatalf("l'utente creato ha la sua sede: %v", utente)
	}

	t.Run("modifica_nome_e_indirizzo", func(t *testing.T) {
		r := f.chiama(t, http.MethodPatch, sede, map[string]any{"nome": "Rinominata " + f.suffisso, "indirizzo": indirizzo(citta)}, admin)
		if r.stato != http.StatusOK || r.corpo["nome"] != "Rinominata "+f.suffisso || r.corpo["indirizzo"].(map[string]any)["via"] != "Via Nuova" {
			t.Fatalf("nuovo nome e nuovo indirizzo nella stessa città: %d %s", r.stato, r.grezzo)
		}
		r = f.chiama(t, http.MethodPatch, sede, map[string]any{"indirizzo": indirizzo("Altrove" + f.suffisso)}, admin)
		if r.stato != http.StatusBadRequest || !contiene(campiInErrore(r), "indirizzo.citta") {
			t.Fatalf("indirizzo in un'altra città: atteso 400, ottenuto %d %s", r.stato, r.grezzo)
		}
		if r := f.chiama(t, http.MethodPatch, sede, map[string]any{}, admin); r.stato != http.StatusBadRequest {
			t.Errorf("nessun campo: atteso 400, ottenuto %d", r.stato)
		}
		if r := f.chiama(t, http.MethodPatch, sede, map[string]any{"attiva": false}, op); r.stato != http.StatusForbidden {
			t.Errorf("l'operatore non modifica la sede: atteso 403, ottenuto %d", r.stato)
		}
	})

	t.Run("sposta_corriere_in_altra_sede", func(t *testing.T) {
		// il token porta la sede: spostando il corriere, le sue sessioni si chiudono subito
		token := conToken(f.login(t, map[string]any{"email": f.emailDi("spostato"), "password": "PasswordCorriere1"}, 200))
		if r := f.chiama(t, http.MethodGet, "/api/spedizioni", nil, token); r.stato != http.StatusOK {
			t.Fatalf("prima dello spostamento il token vale: %d", r.stato)
		}
		r := f.chiama(t, http.MethodPatch, percorsoCorriere, map[string]any{"id_sede": f.idSede}, admin)
		if r.stato != http.StatusOK || int64(r.corpo["id_sede"].(float64)) != f.idSede {
			t.Fatalf("l'amministratore sposta il corriere: %d %s", r.stato, r.grezzo)
		}
		if r := f.chiama(t, http.MethodGet, "/api/spedizioni", nil, token); r.stato != http.StatusUnauthorized {
			t.Errorf("dopo lo spostamento il token con la sede vecchia non vale: atteso 401, ottenuto %d", r.stato)
		}
		r = f.chiama(t, http.MethodGet, percorsoUtenti+"?ruolo=corriere&id_sede="+strconv.FormatInt(f.idSede, 10)+"&q="+f.emailDi("spostato"), nil, admin)
		if r.stato != http.StatusOK || len(r.corpo["dati"].([]any)) != 1 {
			t.Fatalf("filtrando per la nuova sede il corriere c'è: %d %s", r.stato, r.grezzo)
		}
		if r := f.chiama(t, http.MethodPatch, percorsoCorriere, map[string]any{"id_sede": 999999999}, admin); r.stato != http.StatusBadRequest {
			t.Errorf("sede inesistente: atteso 400, ottenuto %d %s", r.stato, r.grezzo)
		}
	})

	t.Run("disattivata", func(t *testing.T) {
		r := f.chiama(t, http.MethodPatch, sede, map[string]any{"attiva": false}, admin)
		if r.stato != http.StatusOK || r.corpo["attiva"] != false {
			t.Fatalf("disattivazione: %d %s", r.stato, r.grezzo)
		}
		r = f.chiama(t, http.MethodGet, "/api/sedi?attiva=false&per_pagina=100", nil, admin)
		trovata := false
		for _, v := range r.corpo["dati"].([]any) {
			trovata = trovata || int64(v.(map[string]any)["id"].(float64)) == f.idSede
		}
		if r.stato != http.StatusOK || !trovata {
			t.Fatalf("filtro attiva=false: %d %s", r.stato, r.grezzo)
		}
		// niente spedizioni nuove, niente nuovi corrieri o operatori
		if r := f.chiama(t, http.MethodPost, "/api/spedizioni", f.corpoValido("SEDE-OFF"), op); r.stato != http.StatusConflict {
			t.Errorf("spedizione in una sede disattivata: atteso 409, ottenuto %d %s", r.stato, r.grezzo)
		}
		if r := nuovoCorriere("insedeoff", f.idSede); r.stato != http.StatusConflict || !contiene(campiInErrore(r), "id_sede") {
			t.Errorf("nuovo corriere in una sede disattivata: atteso 409, ottenuto %d %s", r.stato, r.grezzo)
		}
		if r := f.chiama(t, http.MethodPatch, percorsoCorriere, map[string]any{"id_sede": f.idSedeAltra}, admin); r.stato != http.StatusOK {
			t.Errorf("da una sede disattivata si sposta via: %d %s", r.stato, r.grezzo)
		}
		if r := f.chiama(t, http.MethodPatch, percorsoCorriere, map[string]any{"id_sede": f.idSede}, admin); r.stato != http.StatusConflict {
			t.Errorf("verso una sede disattivata: atteso 409, ottenuto %d %s", r.stato, r.grezzo)
		}
		// l'operatore della sede entra ancora e vede le sue spedizioni
		f.elenco(t, op, "per_pagina=5", http.StatusOK)

		if r := f.chiama(t, http.MethodPatch, sede, map[string]any{"attiva": true}, admin); r.stato != http.StatusOK || r.corpo["attiva"] != true {
			t.Fatalf("riattivazione: %d %s", r.stato, r.grezzo)
		}
		f.nuovaSpedizione(t, "SEDE-ON")
	})

	t.Run("clienti_senza_sede", func(t *testing.T) {
		var idUtente int64
		if err := f.db.QueryRow(t.Context(), `SELECT id_utente FROM clienti WHERE id = $1`, f.idDestinatario).Scan(&idUtente); err != nil {
			t.Fatal(err)
		}
		percorso := percorsoUtenti + "/" + strconv.FormatInt(idUtente, 10)
		if r := f.chiama(t, http.MethodPatch, percorso, map[string]any{"id_sede": f.idSede}, admin); r.stato != http.StatusBadRequest {
			t.Errorf("sede a un cliente: atteso 400, ottenuto %d %s", r.stato, r.grezzo)
		}
		if r := f.chiama(t, http.MethodPatch, percorso, map[string]any{"cognome": "DaOperatore"}, op); r.stato != http.StatusOK || r.corpo["id_sede"] != nil {
			t.Errorf("l'operatore modifica i dati del cliente, che non ha sede: %d %s", r.stato, r.grezzo)
		}
	})
}
