//go:build integration

package apitest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// Mappa sede → database delle spedizioni: le sedi sono divise in gruppi e ogni gruppo ha il
// suo database. Questi test creano una sede nel gruppo 1 e controllano che le sue spedizioni
// finiscano (e si leggano) nel database del gruppo 1, non in quello principale (gruppo 0).
//
// Servono il backend avviato con due gruppi (DATABASE_SPEDIZIONI_URL e
// DATABASE_SPEDIZIONI_URL_1) e TEST_DATABASE_SPEDIZIONI_URL_1 con lo stesso database del
// gruppo 1; senza, i test vengono saltati.

// passoGruppo: gli id del gruppo g partono da g × 10^12 + 1, quindi il gruppo di un id è id / 10^12.
const passoGruppo = int64(1_000_000_000_000)

// payloadToken legge (senza verificarla) la parte centrale del JWT: serve l'id della sessione (sid).
func payloadToken(t *testing.T, chi map[string]string) map[string]any {
	t.Helper()
	parti := strings.Split(strings.TrimPrefix(chi["Authorization"], "Bearer "), ".")
	if len(parti) != 3 {
		t.Fatalf("token non valido: %q", chi["Authorization"])
	}
	grezzo, err := base64.RawURLEncoding.DecodeString(parti[1])
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(grezzo, &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

// attendi riprova la condizione per qualche secondo (le copie verso gli altri gruppi
// partono subito dopo la modifica, ma non nella stessa richiesta).
func attendi(t *testing.T, durata time.Duration, condizione func() bool) bool {
	t.Helper()
	limite := time.Now().Add(durata)
	for {
		if condizione() {
			return true
		}
		if time.Now().After(limite) {
			return false
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func TestSedeInUnAltroGruppo(t *testing.T) {
	connessione := os.Getenv("TEST_DATABASE_SPEDIZIONI_URL_1")
	if connessione == "" {
		t.Skip("imposta TEST_DATABASE_SPEDIZIONI_URL_1 con il database delle spedizioni del gruppo 1 (DATABASE_SPEDIZIONI_URL_1 del backend)")
	}
	f := preparaFixture(t)
	ctx := context.Background()
	g1, err := pgxpool.New(ctx, connessione)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(g1.Close)
	admin := comeAmministratore(f)
	trento := comeOperatore(f)

	// contaIn esegue un count(*) nel database indicato (gruppo 0 = f.sped, gruppo 1 = g1, globale = f.db)
	contaIn := func(t *testing.T, db *pgxpool.Pool, sql string, args ...any) int {
		t.Helper()
		var n int
		if err := db.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// 1. la sede nuova, nel gruppo 1
	nuovaSede := func(citta string, gruppo any) map[string]any {
		corpo := map[string]any{"nome": "Sede " + citta + " " + f.suffisso, "indirizzo": map[string]any{
			"via": "Via Portici", "civico": "1", "citta": citta, "cap": "39012", "regione": "Regione", "nazione": f.nazione}}
		if gruppo != nil {
			corpo["gruppo_spedizioni"] = gruppo
		}
		return corpo
	}
	r := f.chiama(t, http.MethodPost, "/api/sedi", nuovaSede("Merano", 1), admin)
	if r.stato != http.StatusCreated || r.corpo["gruppo_spedizioni"] != float64(1) {
		t.Fatalf("nuova sede nel gruppo 1: atteso 201 con gruppo_spedizioni 1, ottenuto %d %s", r.stato, r.grezzo)
	}
	idSede := int64(r.corpo["id"].(float64))
	percorsoSede := "/api/sedi/" + strconv.FormatInt(idSede, 10)
	var idCitta int64
	if err := f.db.QueryRow(ctx, `SELECT id_citta FROM sedi WHERE id = $1`, idSede).Scan(&idCitta); err != nil {
		t.Fatal(err)
	}

	t.Run("gruppo_della_sede", func(t *testing.T) {
		for _, gruppo := range []any{5, -1, 9007} {
			r := f.chiama(t, http.MethodPost, "/api/sedi", nuovaSede("Bressanone", gruppo), admin)
			if r.stato != http.StatusBadRequest || !contiene(campiInErrore(r), "gruppo_spedizioni") {
				t.Errorf("gruppo %v (senza database o fuori da 0..9006): atteso 400 su gruppo_spedizioni, ottenuto %d %s", gruppo, r.stato, r.grezzo)
			}
		}
		if r := f.chiama(t, http.MethodPatch, percorsoSede, map[string]any{"gruppo_spedizioni": 0}, admin); r.stato != http.StatusBadRequest ||
			!contiene(campiInErrore(r), "gruppo_spedizioni") {
			t.Errorf("il gruppo non si cambia: atteso 400 su gruppo_spedizioni, ottenuto %d %s", r.stato, r.grezzo)
		}
		if r := f.chiama(t, http.MethodGet, percorsoSede, nil, admin); r.stato != http.StatusOK || r.corpo["gruppo_spedizioni"] != float64(1) {
			t.Errorf("la sede resta nel gruppo 1: %d %s", r.stato, r.grezzo)
		}
		if r := f.chiama(t, http.MethodGet, "/api/sedi/"+strconv.FormatInt(f.idSede, 10), nil, admin); r.stato != http.StatusOK ||
			r.corpo["gruppo_spedizioni"] != float64(0) {
			t.Errorf("senza gruppo la sede è nel principale (0): %d %s", r.stato, r.grezzo)
		}
		if n := contaIn(t, f.db, `SELECT count(*) FROM sedi WHERE id = $1 AND gruppo_spedizioni = 1`, idSede); n != 1 {
			t.Errorf("sedi.gruppo_spedizioni = 1 nel database globale: %d", n)
		}
	})

	// 2. operatori, corriere, cliente e indirizzi della sede nuova, direttamente nel database
	// globale come fa la fixture (ma con una password, per entrare con il token)
	const password = "PasswordGruppo1"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	inserisci := func(sql string, args ...any) int64 {
		t.Helper()
		var id int64
		if err := f.db.QueryRow(ctx, sql, args...).Scan(&id); err != nil {
			t.Fatalf("dati di prova: %v", err)
		}
		return id
	}
	contatore := 0
	utente := func(nome string) int64 {
		contatore++
		telefono := strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'f' {
				return '0' + (r - 'a')
			}
			return r
		}, "7"+f.suffisso[:6]+strconv.Itoa(contatore))
		return inserisci(`INSERT INTO utenti (nome, cognome, prefisso, nr_telefono, email, pwd_hash)
			VALUES ($1, 'Gruppo', '+39', $2, $3, $4) RETURNING id`, nome, telefono, f.emailDi(nome), string(hash))
	}
	ruolo := func(tabella, nome string) (idRuolo, idUtente int64) {
		idUtente = utente(nome)
		idRuolo = inserisci(`INSERT INTO `+tabella+` (id_utente, id_sede) VALUES ($1, $2) RETURNING id`, idUtente, idSede)
		return idRuolo, idUtente
	}
	accedi := func(nome string) map[string]string {
		t.Helper()
		return conToken(f.login(t, map[string]any{"email": f.emailDi(nome), "password": password}, http.StatusOK))
	}
	_, _ = ruolo("operatori_logistici", "operatoregruppo")
	idCorriere, idUtenteCorriere := ruolo("corrieri", "corrieregruppo")
	_, idUtenteRevocato := ruolo("operatori_logistici", "operatorerevocato")
	idCliente := inserisci(`INSERT INTO clienti (id_utente) VALUES ($1) RETURNING id`, utente("clientegruppi"))
	// indirizzi di ritiro e consegna della sede nuova: nel database del suo gruppo
	indirizzo := func(via, civico string) int64 {
		var id int64
		if err := g1.QueryRow(ctx, `
			INSERT INTO indirizzi (id_sede, id_citta, via, civico, cod_postale) VALUES ($1, $2, $3, $4, '39012') RETURNING id`,
			idSede, idCitta, via, civico).Scan(&id); err != nil {
			t.Fatalf("indirizzo nel gruppo 1: %v", err)
		}
		return id
	}
	idRitiro, idConsegna := indirizzo("Via Portici", "10"), indirizzo("Corso Liberta", "5")
	op, corriere := accedi("operatoregruppo"), accedi("corrieregruppo")

	corpo := func(codice string) map[string]any {
		return map[string]any{
			"cod_ordine":                   codice + "-" + f.suffisso,
			"id_cliente_mittente":          f.idMittente,
			"id_cliente_destinatario":      idCliente,
			"dataora_richiesta_spedizione": time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
			"id_indirizzo_ritiro":          idRitiro,
			"id_indirizzo_spedizione":      idConsegna,
		}
	}
	crea := func(t *testing.T, chi map[string]string, corpo map[string]any) int {
		t.Helper()
		r := f.chiama(t, http.MethodPost, "/api/spedizioni", corpo, chi)
		if r.stato != http.StatusCreated {
			t.Fatalf("creazione %v: atteso 201, ottenuto %d %s", corpo["cod_ordine"], r.stato, r.grezzo)
		}
		return int(r.corpo["id"].(float64))
	}
	// una spedizione di Trento (gruppo 0) per lo stesso cliente
	diTrento := func(t *testing.T, codice string) int {
		t.Helper()
		c := f.corpoValido(codice)
		c["id_cliente_destinatario"] = idCliente
		return crea(t, trento, c)
	}

	id := crea(t, op, corpo("GR1-A"))
	percorso := "/api/spedizioni/" + strconv.Itoa(id)

	t.Run("spedizione_nel_database_del_gruppo", func(t *testing.T) {
		if int64(id) < passoGruppo || int64(id) >= 2*passoGruppo {
			t.Fatalf("l'id di una spedizione del gruppo 1 è tra 10^12 e 2×10^12: %d", id)
		}
		if n := contaIn(t, g1, `SELECT count(*) FROM spedizioni WHERE id = $1 AND id_sede = $2`, id, idSede); n != 1 {
			t.Errorf("la spedizione è nel database del gruppo 1: %d righe", n)
		}
		if n := contaIn(t, g1, `SELECT count(*) FROM cambiamenti_spedizioni WHERE id_spedizione = $1`, id); n == 0 {
			t.Errorf("anche lo storico è nel database del gruppo 1")
		}
		if n := contaIn(t, f.sped, `SELECT count(*) FROM spedizioni WHERE id = $1 OR id_sede = $2`, id, idSede); n != 0 {
			t.Errorf("nel database principale (gruppo 0) non c'è: %d righe", n)
		}
		if idTrento := diTrento(t, "GR0-CONTROLLO"); int64(idTrento) >= passoGruppo {
			t.Errorf("le spedizioni di Trento restano nel gruppo 0: id %d", idTrento)
		}
	})

	t.Run("operatore_e_corriere_lavorano_nel_gruppo", func(t *testing.T) {
		voci, _ := f.elenco(t, op, "per_pagina=100", http.StatusOK)
		if _, c := voci[id]; !c {
			t.Fatalf("la spedizione è nell'elenco dell'operatore della sede")
		}
		for idVoce, v := range voci {
			if int64(v["id_sede"].(float64)) != idSede {
				t.Errorf("l'operatore vede solo la sua sede (spedizione %d della sede %v)", idVoce, v["id_sede"])
			}
		}
		if r := f.invia(t, http.MethodGet, id, "", op, nil, http.StatusOK); int64(r.corpo["id_sede"].(float64)) != idSede {
			t.Errorf("dettaglio con la sede: %s", r.grezzo)
		}
		f.invia(t, http.MethodPost, id, "/assegnazioni", op, map[string]any{"id_corriere": idCorriere}, http.StatusCreated)
		f.invia(t, http.MethodPost, id, "/tentativi", corriere, map[string]any{"tipo": "ritiro", "nota": "chiuso", "motivo": "mittente assente"}, http.StatusCreated)
		f.invia(t, http.MethodPatch, id, "/stato", corriere, map[string]any{"stato": "ritirata"}, http.StatusOK)
		voci, _ = f.elenco(t, corriere, "per_pagina=100", http.StatusOK)
		if v, c := voci[id]; !c || v["stato"] != "ritirata" {
			t.Errorf("il corriere vede la sua spedizione ritirata: %v", v)
		}
		f.invia(t, http.MethodGet, id, "", corriere, nil, http.StatusOK)
		storico := f.invia(t, http.MethodGet, id, "/storico", op, nil, http.StatusOK)
		if n := contaIn(t, g1, `SELECT count(*) FROM cambiamenti_spedizioni WHERE id_spedizione = $1`, id); n < 3 {
			t.Errorf("creazione, assegnazione e ritiro nello storico del gruppo 1: %d righe (%s)", n, storico.grezzo)
		}
		if n := contaIn(t, g1, `SELECT count(*) FROM tentativi WHERE id_spedizione = $1`, id); n != 1 {
			t.Errorf("il tentativo è nel database del gruppo 1: %d", n)
		}

		annullata := crea(t, op, corpo("GR1-ANN"))
		f.invia(t, http.MethodPatch, annullata, "/stato", op, map[string]any{"stato": "annullata", "motivo": "richiesta del mittente", "nota": "x"}, http.StatusOK)
		f.invia(t, http.MethodPost, annullata, "/note", op, map[string]any{"nota": "nota nel gruppo 1"}, http.StatusCreated)

		// Trento e Bolzano (gruppo 0) non vedono la spedizione: 404, come se non esistesse
		for _, chi := range []map[string]string{trento, comeOperatoreAltro(f)} {
			for _, sotto := range []string{"", "/storico"} {
				if r := f.chiama(t, http.MethodGet, percorso+sotto, nil, chi); r.stato != http.StatusNotFound {
					t.Errorf("GET %s da un'altra sede: atteso 404, ottenuto %d %s", sotto, r.stato, r.grezzo)
				}
			}
		}
		voci, _ = f.elenco(t, trento, "per_pagina=100&cod_ordine=GR1-A-"+f.suffisso, http.StatusOK)
		if len(voci) != 0 {
			t.Errorf("la spedizione del gruppo 1 non compare nell'elenco di Trento")
		}

		// le spedizioni aperte del corriere si cercano in tutti i gruppi
		r := f.chiama(t, http.MethodPatch, percorsoUtenti+"/"+strconv.FormatInt(idUtenteCorriere, 10), map[string]any{"id_sede": f.idSedeAltra}, admin)
		if r.stato != http.StatusConflict || !contiene(campiInErrore(r), "id_sede") {
			t.Errorf("corriere con una spedizione aperta nel gruppo 1: atteso 409 su id_sede, ottenuto %d %s", r.stato, r.grezzo)
		}
	})

	t.Run("amministratore_sceglie_la_sede", func(t *testing.T) {
		idTrento := diTrento(t, "GR0-ADMIN")
		voci, _ := f.elenco(t, admin, "id_sede="+strconv.FormatInt(idSede, 10)+"&per_pagina=100", http.StatusOK)
		if _, c := voci[id]; !c {
			t.Errorf("scelta la sede nuova, l'amministratore vede la sua spedizione")
		}
		if _, c := voci[idTrento]; c {
			t.Errorf("scelta la sede nuova, niente spedizioni di Trento")
		}
		for idVoce, v := range voci {
			if int64(v["id_sede"].(float64)) != idSede {
				t.Errorf("solo spedizioni della sede scelta (spedizione %d della sede %v)", idVoce, v["id_sede"])
			}
		}
		voci, _ = f.elenco(t, admin, "id_sede="+strconv.FormatInt(f.idSede, 10)+"&per_pagina=100", http.StatusOK)
		if _, c := voci[id]; c {
			t.Errorf("scelta Trento, la spedizione del gruppo 1 non c'è")
		}
		for _, sotto := range []string{"", "/storico"} {
			if r := f.chiama(t, http.MethodGet, percorso+sotto, nil, admin); r.stato != http.StatusOK {
				t.Errorf("GET %s: l'amministratore legge per id (l'id dice il database), atteso 200, ottenuto %d %s", sotto, r.stato, r.grezzo)
			}
		}
		_, r := f.elenco(t, admin, "per_pagina=100", http.StatusBadRequest)
		if !contiene(campiInErrore(r), "id_sede") {
			t.Errorf("senza sede: atteso l'errore sul campo id_sede: %s", r.grezzo)
		}
	})

	t.Run("cliente_vede_tutti_i_gruppi", func(t *testing.T) {
		// tre spedizioni create una dopo l'altra: Trento, sede nuova, Trento
		a := diTrento(t, "GR-CLI-A")
		b := crea(t, op, corpo("GR-CLI-B"))
		c := diTrento(t, "GR-CLI-C")
		cliente := accedi("clientegruppi")
		_, r := f.elenco(t, cliente, "per_pagina=100&cod_ordine=GR-CLI-", http.StatusOK)
		ordine := func(r risposta) []int {
			var ids []int
			for _, d := range r.corpo["dati"].([]any) {
				ids = append(ids, int(d.(map[string]any)["id"].(float64)))
			}
			return ids
		}
		atteso := []int{c, b, a} // creata_il decrescente
		if got := ordine(r); len(got) != 3 || got[0] != c || got[1] != b || got[2] != a || r.corpo["totale_elementi"] != float64(3) {
			t.Fatalf("il cliente vede le spedizioni dei due gruppi in ordine %v (totale 3), ottenuto %v: %s", atteso, got, r.grezzo)
		}
		var prima time.Time
		for i, d := range r.corpo["dati"].([]any) {
			creata, err := time.Parse(time.RFC3339Nano, d.(map[string]any)["creata_il"].(string))
			if err != nil {
				t.Fatal(err)
			}
			if i > 0 && creata.After(prima) {
				t.Errorf("ordine per creata_il decrescente: %v dopo %v", creata, prima)
			}
			prima = creata
		}

		// senza filtro: tutte le spedizioni del cliente (anche quelle dei sottotest precedenti,
		// tutte recenti), totale = somma dei due gruppi
		_, tutte := f.elenco(t, cliente, "per_pagina=100", http.StatusOK)
		nGruppo0 := contaIn(t, f.sped, `SELECT count(*) FROM spedizioni WHERE id_cliente_destinatario = $1`, idCliente)
		nGruppo1 := contaIn(t, g1, `SELECT count(*) FROM spedizioni WHERE id_cliente_destinatario = $1`, idCliente)
		if tutte.corpo["totale_elementi"] != float64(nGruppo0+nGruppo1) || nGruppo1 == 0 || nGruppo0 == 0 {
			t.Errorf("totale_elementi = %d (gruppo 0) + %d (gruppo 1): %s", nGruppo0, nGruppo1, tutte.grezzo)
		}

		// una per pagina: le pagine seguono lo stesso ordine, senza ripetizioni
		for pagina, idAtteso := range atteso {
			_, r := f.elenco(t, cliente, "per_pagina=1&cod_ordine=GR-CLI-&pagina="+strconv.Itoa(pagina+1), http.StatusOK)
			got := ordine(r)
			if len(got) != 1 || got[0] != idAtteso || r.corpo["totale_elementi"] != float64(3) || r.corpo["totale_pagine"] != float64(3) {
				t.Errorf("pagina %d: attesa la spedizione %d (3 in tutto), ottenuto %v %s", pagina+1, idAtteso, got, r.grezzo)
			}
		}
		_, r = f.elenco(t, cliente, "per_pagina=1&cod_ordine=GR-CLI-&pagina=4", http.StatusOK)
		if got := ordine(r); len(got) != 0 || r.corpo["totale_elementi"] != float64(3) {
			t.Errorf("oltre l'ultima pagina: nessun dato ma i totali, ottenuto %s", r.grezzo)
		}
		if r := f.chiama(t, http.MethodGet, "/api/spedizioni/"+strconv.Itoa(b), nil, cliente); r.stato != http.StatusOK {
			t.Errorf("il cliente apre la spedizione del gruppo 1: %d %s", r.stato, r.grezzo)
		}
	})

	t.Run("motivi_copiati_nel_gruppo", func(t *testing.T) {
		descrizione := "motivo dei gruppi " + f.suffisso
		r := f.chiama(t, http.MethodPost, percorsoMotiviAdmin, map[string]any{"descrizione": descrizione}, admin)
		if r.stato != http.StatusCreated {
			t.Fatalf("nuovo motivo: %d %s", r.stato, r.grezzo)
		}
		idMotivo := int64(r.corpo["id"].(float64))
		percorsoMotivo := percorsoMotiviAdmin + "/" + strconv.FormatInt(idMotivo, 10)
		t.Cleanup(func() { f.chiama(t, http.MethodPatch, percorsoMotivo, map[string]any{"attivo": false}, admin) })
		copiato := func(attivo bool) func() bool {
			return func() bool {
				return contaIn(t, g1, `SELECT count(*) FROM motivi WHERE id = $1 AND descrizione = $2 AND attivo = $3`, idMotivo, descrizione, attivo) == 1
			}
		}
		if !attendi(t, 5*time.Second, copiato(true)) {
			t.Fatalf("il motivo %d (attivo) è copiato nel database del gruppo 1 entro pochi secondi", idMotivo)
		}
		// il corriere del gruppo 1 lo usa subito
		usata := crea(t, op, corpo("GR1-MOTIVO"))
		f.invia(t, http.MethodPost, usata, "/assegnazioni", op, map[string]any{"id_corriere": idCorriere}, http.StatusCreated)
		f.invia(t, http.MethodPost, usata, "/tentativi", corriere, map[string]any{"tipo": "ritiro", "nota": "chiuso", "motivo": descrizione}, http.StatusCreated)

		if r := f.chiama(t, http.MethodPatch, percorsoMotivo, map[string]any{"attivo": false}, admin); r.stato != http.StatusOK {
			t.Fatalf("disattivazione: %d %s", r.stato, r.grezzo)
		}
		if !attendi(t, 5*time.Second, copiato(false)) {
			t.Fatalf("anche la disattivazione è copiata nel gruppo 1")
		}
	})

	t.Run("revoca_nel_gruppo_del_token", func(t *testing.T) {
		token := accedi("operatorerevocato")
		sid, _ := payloadToken(t, token)["sid"].(string)
		if sid == "" {
			t.Fatalf("il token ha l'id della sessione (sid)")
		}
		if r := f.chiama(t, http.MethodGet, "/api/spedizioni", nil, token); r.stato != http.StatusOK {
			t.Fatalf("prima della disattivazione il token vale: %d %s", r.stato, r.grezzo)
		}
		r := f.chiama(t, http.MethodPatch, percorsoUtenti+"/"+strconv.FormatInt(idUtenteRevocato, 10), map[string]any{"attivo": false}, admin)
		if r.stato != http.StatusOK {
			t.Fatalf("disattivazione: %d %s", r.stato, r.grezzo)
		}
		if r := f.chiama(t, http.MethodGet, "/api/spedizioni", nil, token); r.stato != http.StatusForbidden {
			t.Errorf("le spedizioni della sede (gruppo 1) con il token revocato: atteso 403 subito, ottenuto %d %s", r.stato, r.grezzo)
		}
		// la revoca va solo nel database che controlla quel token: il gruppo della sede del
		// login; un gruppo diverso (anche giù) non blocca le revoche di questo
		if n := contaIn(t, g1, `SELECT count(*) FROM sessioni_revocate WHERE id = $1`, sid); n != 1 {
			t.Errorf("la sessione revocata è nel database del gruppo 1: %d", n)
		}
		if n := contaIn(t, f.sped, `SELECT count(*) FROM sessioni_revocate WHERE id = $1`, sid); n != 0 {
			t.Errorf("e non nel gruppo 0, che non controlla quel token: %d", n)
		}
		var sedeSessione *int64
		if err := f.db.QueryRow(context.Background(), `SELECT id_sede FROM sessioni WHERE id = $1`, sid).Scan(&sedeSessione); err != nil ||
			sedeSessione == nil || *sedeSessione != idSede {
			t.Errorf("la sessione ricorda la sede del login: %v %v", sedeSessione, err)
		}
	})

	t.Run("impostazioni_della_sede_nel_gruppo", func(t *testing.T) {
		percorsoImp := percorsoImpostazioni(idSede)
		if r := f.chiama(t, http.MethodGet, percorsoImp, nil, admin); r.stato != http.StatusOK || r.corpo["max_tentativi_ritiro"] != float64(3) {
			t.Fatalf("la sede nuova ha le impostazioni predefinite: %d %s", r.stato, r.grezzo)
		}
		if r := f.chiama(t, http.MethodPatch, percorsoImp, map[string]any{"max_tentativi_ritiro": 4}, admin); r.stato != http.StatusOK ||
			r.corpo["max_tentativi_ritiro"] != float64(4) {
			t.Fatalf("modifica: %d %s", r.stato, r.grezzo)
		}
		if r := f.chiama(t, http.MethodGet, percorsoImp, nil, admin); r.stato != http.StatusOK || r.corpo["max_tentativi_ritiro"] != float64(4) {
			t.Errorf("rilettura: %d %s", r.stato, r.grezzo)
		}
		if n := contaIn(t, g1, `SELECT count(*) FROM impostazioni WHERE id_sede = $1 AND max_tentativi_ritiro = 4`, idSede); n != 1 {
			t.Errorf("le impostazioni sono nel database del gruppo 1: %d", n)
		}
		if n := contaIn(t, f.sped, `SELECT count(*) FROM impostazioni WHERE id_sede = $1`, idSede); n != 0 {
			t.Errorf("nel database principale non ci sono: %d", n)
		}
	})
}
