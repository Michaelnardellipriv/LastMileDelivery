//go:build integration

package apitest

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type fixture struct {
	url              string
	db               *pgxpool.Pool
	suffisso         string
	idOperatore      int32
	idCorriere1      int32
	idCorriere2      int32
	idMittente       int32
	idDestinatario   int32
	idRitiro         int32
	idConsegna       int32
	idAmministratore int32
}

var (
	fx      *fixture
	fxUnico sync.Once
)

func preparaFixture(t *testing.T) *fixture {
	t.Helper()
	fxUnico.Do(func() { fx = creaFixture(t) })
	if fx == nil {
		t.Fatal("preparazione dei dati di prova fallita (vedi il primo errore)")
	}
	return fx
}

func creaFixture(t *testing.T) *fixture {
	url := strings.TrimRight(valore("TEST_API_URL", "http://localhost:8080"), "/")
	connessione := os.Getenv("TEST_DATABASE_URL")
	if connessione == "" {
		t.Fatal("imposta TEST_DATABASE_URL con lo stesso database del backend")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, connessione)
	if err != nil {
		t.Fatal(err)
	}

	b := make([]byte, 4)
	rand.Read(b)
	f := &fixture{url: url, db: db, suffisso: hex.EncodeToString(b)}

	inserisci := func(sql string, args ...any) int32 {
		var id int32
		if err := db.QueryRow(ctx, sql, args...).Scan(&id); err != nil {
			t.Fatalf("dati di prova: %v", err)
		}
		return id
	}
	contatore := 0
	utente := func(nome string) int32 {
		contatore++
		telefono := fmt.Sprintf("8%s%d", f.suffisso[:6], contatore)
		telefono = strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'f' {
				return '0' + (r - 'a')
			}
			return r
		}, telefono)
		return inserisci(`INSERT INTO utenti (nome, cognome, prefisso, nr_telefono, email)
			VALUES ($1, 'Test', '+39', $2, $3) RETURNING id`, nome, telefono, nome+"."+f.suffisso+"@test.it")
	}

	f.idOperatore = inserisci(`INSERT INTO operatori_logistici (pwd_hash, id_utente) VALUES ('x', $1) RETURNING id`, utente("operatore"))
	f.idCorriere1 = inserisci(`INSERT INTO corrieri (pwd_hash, id_utente) VALUES ('x', $1) RETURNING id`, utente("corriere1"))
	f.idCorriere2 = inserisci(`INSERT INTO corrieri (pwd_hash, id_utente) VALUES ('x', $1) RETURNING id`, utente("corriere2"))
	f.idMittente = inserisci(`INSERT INTO clienti (id_utente) VALUES ($1) RETURNING id`, utente("mittente"))
	f.idDestinatario = inserisci(`INSERT INTO clienti (id_utente) VALUES ($1) RETURNING id`, utente("destinatario"))
	f.idAmministratore = inserisci(`INSERT INTO amministratori (pwd_hash, id_utente) VALUES ('x', $1) RETURNING id`, utente("amministratore"))

	nazione := inserisci(`INSERT INTO nazioni (nome) VALUES ($1) RETURNING id`, "Nazione "+f.suffisso)
	regione := inserisci(`INSERT INTO regioni (nome, id_nazione) VALUES ('Regione', $1) RETURNING id`, nazione)
	citta := inserisci(`INSERT INTO citta (nome, cod_postale, id_regione) VALUES ('Trento', '38121', $1) RETURNING id`, regione)
	f.idRitiro = inserisci(`INSERT INTO indirizzi (via, civico) VALUES ('Via Roma', '1') RETURNING id`)
	f.idConsegna = inserisci(`INSERT INTO indirizzi (via, civico) VALUES ('Via Museo', '12') RETURNING id`)
	inserisci(`INSERT INTO indirizzi_in_citta (id_citta, id_indirizzo) VALUES ($1, $2) RETURNING id`, citta, f.idRitiro)
	inserisci(`INSERT INTO indirizzi_in_citta (id_citta, id_indirizzo) VALUES ($1, $2) RETURNING id`, citta, f.idConsegna)
	return f
}

func valore(nome, predefinito string) string {
	if v := strings.TrimSpace(os.Getenv(nome)); v != "" {
		return v
	}
	return predefinito
}

type risposta struct {
	stato  int
	header http.Header
	corpo  map[string]any
	grezzo []byte
}

func (f *fixture) chiama(t *testing.T, metodo, percorso string, corpo any, header map[string]string) risposta {
	t.Helper()
	var dati []byte
	switch c := corpo.(type) {
	case nil:
	case string:
		dati = []byte(c)
	default:
		dati, _ = json.Marshal(c)
	}
	req, err := http.NewRequest(metodo, f.url+percorso, bytes.NewReader(dati))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for k, v := range header {
		req.Header.Set(k, v)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("chiamata al server fallita: %v. Controlla che il backend sia avviato", err)
	}
	defer res.Body.Close()
	grezzo, _ := io.ReadAll(res.Body)
	var j map[string]any
	json.Unmarshal(grezzo, &j)
	return risposta{stato: res.StatusCode, header: res.Header, corpo: j, grezzo: grezzo}
}

func comeOperatore(f *fixture) map[string]string {
	return map[string]string{"X-Id-Operatore": strconv.Itoa(int(f.idOperatore))}
}

func comeCliente(id int32) map[string]string {
	return map[string]string{"X-Id-Cliente": strconv.Itoa(int(id))}
}

func comeAmministratore(f *fixture) map[string]string {
	return map[string]string{"X-Id-Amministratore": strconv.Itoa(int(f.idAmministratore))}
}

func (f *fixture) retrodata(t *testing.T, id, giorni int) {
	t.Helper()
	ctx := context.Background()
	tx, err := f.db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT set_config('app.id_operatore', $1, true)`, strconv.Itoa(int(f.idOperatore))); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE spedizioni SET creata_il = now() - make_interval(days => $2) WHERE id = $1`, id, giorni); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) retrodataStorico(t *testing.T, id, giorni int) {
	t.Helper()
	ctx := context.Background()
	tx, err := f.db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for _, sql := range []string{
		`ALTER TABLE cambiamenti_spedizioni DISABLE TRIGGER trg_cambiamenti_spedizioni_immutabile`,
		`UPDATE cambiamenti_spedizioni SET modificato_il = now() - make_interval(days => $2) WHERE id_spedizione = $1`,
		`ALTER TABLE cambiamenti_spedizioni ENABLE TRIGGER trg_cambiamenti_spedizioni_immutabile`,
	} {
		var err error
		if strings.Contains(sql, "$1") {
			_, err = tx.Exec(ctx, sql, id, giorni)
		} else {
			_, err = tx.Exec(ctx, sql)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) consegnaCompleta(t *testing.T, id int) {
	t.Helper()
	c1 := comeCorriere(f.idCorriere1)
	f.assegna(t, id, f.idCorriere1, 201)
	f.stato(t, id, c1, map[string]any{"stato": "ritirata"}, 200)
	f.stato(t, id, c1, map[string]any{"stato": "in consegna"}, 200)
	f.stato(t, id, c1, map[string]any{"stato": "consegnata"}, 200)
}

func (f *fixture) dataRoma(t *testing.T, giorniDaOggi int) string {
	t.Helper()
	var d string
	if err := f.db.QueryRow(context.Background(),
		`SELECT ((now() AT TIME ZONE 'Europe/Rome')::date + $1::int)::text`, giorniDaOggi).Scan(&d); err != nil {
		t.Fatal(err)
	}
	return d
}

func (f *fixture) elenco(t *testing.T, chi map[string]string, query string, atteso int) (map[int]map[string]any, risposta) {
	t.Helper()
	r := f.chiama(t, http.MethodGet, "/api/spedizioni?"+query, nil, chi)
	if r.stato != atteso {
		t.Fatalf("GET /api/spedizioni?%s: atteso %d, ottenuto %d: %s", query, atteso, r.stato, r.grezzo)
	}
	voci := map[int]map[string]any{}
	if dati, ok := r.corpo["dati"].([]any); ok {
		for _, d := range dati {
			v := d.(map[string]any)
			voci[int(v["id"].(float64))] = v
		}
	}
	return voci, r
}

func comeCorriere(id int32) map[string]string {
	return map[string]string{"X-Id-Corriere": strconv.Itoa(int(id))}
}

func (f *fixture) corpoValido(codice string) map[string]any {
	return map[string]any{
		"cod_ordine":                   codice + "-" + f.suffisso,
		"id_cliente_mittente":          f.idMittente,
		"id_cliente_destinatario":      f.idDestinatario,
		"dataora_richiesta_spedizione": time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
		"id_indirizzo_ritiro":          f.idRitiro,
		"id_indirizzo_spedizione":      f.idConsegna,
	}
}

func (f *fixture) nuovaSpedizione(t *testing.T, codice string) int {
	t.Helper()
	r := f.chiama(t, http.MethodPost, "/api/spedizioni", f.corpoValido(codice), comeOperatore(f))
	if r.stato != http.StatusCreated {
		t.Fatalf("creazione: atteso 201, ottenuto %d: %s", r.stato, r.grezzo)
	}
	return int(r.corpo["id"].(float64))
}

func (f *fixture) invia(t *testing.T, metodo string, id int, sotto string, chi map[string]string, corpo map[string]any, atteso int) risposta {
	t.Helper()
	r := f.chiama(t, metodo, "/api/spedizioni/"+strconv.Itoa(id)+sotto, corpo, chi)
	if r.stato != atteso {
		t.Fatalf("%s %s %v: atteso %d, ottenuto %d: %s", metodo, sotto, corpo, atteso, r.stato, r.grezzo)
	}
	return r
}

func (f *fixture) stato(t *testing.T, id int, chi map[string]string, corpo map[string]any, atteso int) risposta {
	t.Helper()
	return f.invia(t, http.MethodPatch, id, "/stato", chi, corpo, atteso)
}

func (f *fixture) assegna(t *testing.T, id int, idCorriere int32, atteso int) risposta {
	t.Helper()
	return f.invia(t, http.MethodPost, id, "/assegnazioni", comeOperatore(f), map[string]any{"id_corriere": idCorriere}, atteso)
}

func (f *fixture) tentativo(t *testing.T, id int, chi map[string]string, corpo map[string]any, atteso int) risposta {
	t.Helper()
	return f.invia(t, http.MethodPost, id, "/tentativi", chi, corpo, atteso)
}

func (f *fixture) modifica(t *testing.T, id int, chi map[string]string, corpo map[string]any, atteso int) risposta {
	t.Helper()
	return f.invia(t, http.MethodPatch, id, "", chi, corpo, atteso)
}

func (f *fixture) storico(t *testing.T, id int) int {
	t.Helper()
	return f.conta(t, `SELECT count(*) FROM cambiamenti_spedizioni WHERE id_spedizione = $1`, id)
}

func campiInErrore(r risposta) []string {
	var campi []string
	dettagli, _ := r.corpo["dettagli"].([]any)
	for _, d := range dettagli {
		campi = append(campi, d.(map[string]any)["campo"].(string))
	}
	return campi
}

func contiene(lista []string, x string) bool {
	for _, v := range lista {
		if v == x {
			return true
		}
	}
	return false
}

func (f *fixture) conta(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := f.db.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

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
		prima := f.chiama(t, http.MethodPost, "/api/spedizioni", f.corpoValido("IDEM"), h)
		seconda := f.chiama(t, http.MethodPost, "/api/spedizioni", f.corpoValido("IDEM"), h)
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

func TestModificaDati(t *testing.T) {
	f := preparaFixture(t)
	op, c1 := comeOperatore(f), comeCorriere(f.idCorriere1)

	t.Run("modifica_prima_del_ritiro_200_e_storico", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "MOD")
		data := time.Now().Add(72 * time.Hour).UTC().Truncate(time.Second)
		corpo := map[string]any{"dataora_richiesta_spedizione": data.Format(time.RFC3339)}
		r := f.modifica(t, id, op, corpo, 200)
		if r.corpo["dataora_richiesta_spedizione"] != data.Format(time.RFC3339) {
			t.Fatalf("dati non aggiornati: %s", r.grezzo)
		}
		if n := f.conta(t, `SELECT count(*) FROM cambiamenti_spedizioni WHERE id_spedizione = $1
			AND operazione = 'modifica' AND id_operatore = $2 AND valore_nuovo ? 'dataora_richiesta_spedizione'`, id, f.idOperatore); n != 1 {
			t.Fatalf("modifica non registrata nello storico con l'operatore")
		}

		f.assegna(t, id, f.idCorriere1, 201)
		prima := f.storico(t, id)
		f.modifica(t, id, op, corpo, 200)
		if f.storico(t, id) != prima {
			t.Fatal("una modifica con gli stessi valori non deve scrivere nello storico")
		}
	})

	t.Run("dopo_il_ritiro_409", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "MODRIT")
		f.assegna(t, id, f.idCorriere1, 201)
		f.stato(t, id, c1, map[string]any{"stato": "ritirata"}, 200)
		f.modifica(t, id, op, map[string]any{"dataora_richiesta_spedizione": time.Now().Add(96 * time.Hour).UTC().Format(time.RFC3339)}, 409)
	})

	t.Run("reso_solo_se_annullata_persa_o_non_consegnata", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "RESOANN")
		f.modifica(t, id, op, map[string]any{"is_reso": true}, 409)
		f.assegna(t, id, f.idCorriere1, 201)
		f.modifica(t, id, op, map[string]any{"is_reso": true}, 409)
		r := f.stato(t, id, op, map[string]any{"stato": "annullata", "motivo": "richiesta del destinatario", "nota": "il cliente ha chiamato"}, 200)
		if r.corpo["is_reso"] != false {
			t.Fatalf("l'annullamento non deve impostare is_reso: %s", r.grezzo)
		}
		f.modifica(t, id, c1, map[string]any{"is_reso": true}, 403)
		f.modifica(t, id, op, map[string]any{"is_reso": true, "dataora_richiesta_spedizione": time.Now().Add(96 * time.Hour).UTC().Format(time.RFC3339)}, 409)
		r = f.modifica(t, id, op, map[string]any{"is_reso": true}, 200)
		if r.corpo["is_reso"] != true || r.corpo["stato"] != "annullata" {
			t.Fatalf("reso su spedizione annullata: %s", r.grezzo)
		}
		if n := f.conta(t, `SELECT count(*) FROM cambiamenti_spedizioni WHERE id_spedizione = $1
			AND operazione = 'modifica' AND id_operatore = $2 AND valore_nuovo = '{"is_reso": true}'`, id, f.idOperatore); n != 1 {
			t.Fatal("il reso deve comparire nello storico con l'operatore che l'ha impostato")
		}
	})

	t.Run("errori_400_403", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "MODERR")
		f.modifica(t, id, op, map[string]any{}, 400)
		f.modifica(t, id, op, map[string]any{"cod_ordine": "NUOVO"}, 400)
		f.modifica(t, id, op, map[string]any{"stato": "consegnata"}, 400)
		f.modifica(t, id, op, map[string]any{"id_indirizzo_spedizione": f.idRitiro}, 400)
		f.modifica(t, id, op, map[string]any{"dataora_richiesta_spedizione": "domani"}, 400)
		r := f.modifica(t, id, op, map[string]any{"id_cliente_destinatario": 999999999}, 400)
		if !contiene(campiInErrore(r), "id_cliente_destinatario") {
			t.Fatalf("atteso errore su id_cliente_destinatario: %s", r.grezzo)
		}
		f.modifica(t, id, c1, map[string]any{"is_reso": true}, 403)
		f.modifica(t, 999999999, op, map[string]any{"is_reso": true}, 404)
	})
}

func TestAssegnazioni(t *testing.T) {
	f := preparaFixture(t)
	op := comeOperatore(f)
	c1, c2 := comeCorriere(f.idCorriere1), comeCorriere(f.idCorriere2)

	t.Run("assegna_e_riassegna_prima_e_dopo_il_ritiro", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "ASS")
		r := f.assegna(t, id, f.idCorriere1, 201)
		if r.corpo["stato"] != "assegnata" || int32(r.corpo["id_corriere"].(float64)) != f.idCorriere1 {
			t.Fatalf("assegnazione: %s", r.grezzo)
		}
		f.assegna(t, id, f.idCorriere1, 200)
		r = f.assegna(t, id, f.idCorriere2, 201)
		if r.corpo["stato"] != "assegnata" || int32(r.corpo["id_corriere"].(float64)) != f.idCorriere2 {
			t.Fatalf("riassegnazione prima del ritiro: %s", r.grezzo)
		}
		f.stato(t, id, c1, map[string]any{"stato": "ritirata"}, 403)
		f.stato(t, id, c2, map[string]any{"stato": "ritirata"}, 200)

		r = f.assegna(t, id, f.idCorriere1, 201)
		if r.corpo["stato"] != "ritirata" || int32(r.corpo["id_corriere"].(float64)) != f.idCorriere1 {
			t.Fatalf("riassegnazione dopo il ritiro: %s", r.grezzo)
		}
		f.stato(t, id, c2, map[string]any{"stato": "in consegna"}, 403)
		f.stato(t, id, c1, map[string]any{"stato": "in consegna"}, 200)
		f.assegna(t, id, f.idCorriere2, 409)

		if n := f.conta(t, `SELECT count(*) FROM assegnazioni a JOIN spedizioni s ON s.id_assegnazione_corrente = a.id
			WHERE s.id = $1 AND a.id_operatore = $2`, id, f.idOperatore); n != 1 {
			t.Fatal("l'assegnazione corrente deve riportare l'operatore che l'ha fatta")
		}
	})

	t.Run("errori", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "ASSERR")
		f.invia(t, http.MethodPost, id, "/assegnazioni", op, map[string]any{}, 400)
		r := f.assegna(t, id, 999999999, 400)
		if !contiene(campiInErrore(r), "id_corriere") {
			t.Fatalf("atteso errore su id_corriere inesistente: %s", r.grezzo)
		}
		f.invia(t, http.MethodPost, id, "/assegnazioni", c1, map[string]any{"id_corriere": f.idCorriere1}, 403)
		f.assegna(t, 999999999, f.idCorriere1, 404)
	})
}

func TestTentativi(t *testing.T) {
	f := preparaFixture(t)
	op, c1, c2 := comeOperatore(f), comeCorriere(f.idCorriere1), comeCorriere(f.idCorriere2)
	ritiroFallito := map[string]any{"tipo": "ritiro", "nota": "mittente assente"}
	consegnaFallita := map[string]any{"tipo": "consegna", "nota": "citofono rotto", "motivo": "destinatario assente"}
	consegnaSenzaMotivo := map[string]any{"tipo": "consegna", "nota": "traffico, ripasso domani"}

	t.Run("tre_ritiri_falliti_poi_annullata_dall_operatore", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "RITIRO")
		f.assegna(t, id, f.idCorriere1, 201)

		for tentativo := 1; tentativo <= 3; tentativo++ {
			r := f.tentativo(t, id, c1, ritiroFallito, 201)
			if r.corpo["stato"] != "assegnata" || int(r.corpo["tentativi_ritiro_falliti"].(float64)) != tentativo {
				t.Fatalf("ritiro fallito %d: %s", tentativo, r.grezzo)
			}
		}
		f.tentativo(t, id, c1, ritiroFallito, 409)
		f.stato(t, id, c1, map[string]any{"stato": "ritirata"}, 409)
		f.assegna(t, id, f.idCorriere2, 409)
		f.stato(t, id, c1, map[string]any{"stato": "annullata", "motivo": "altro", "nota": "x"}, 403)
		f.stato(t, id, op, map[string]any{"stato": "annullata"}, 400)
		f.stato(t, id, op, map[string]any{"stato": "annullata", "motivo": "ritiro non riuscito", "nota": "3 ritiri falliti"}, 200)

		if n := f.conta(t, `SELECT count(*) FROM tentativi t JOIN note n ON n.id = t.id_nota
			WHERE t.id_spedizione = $1 AND t.tipo = 'ritiro' AND n.id_corriere_creata = $2
			  AND n.nota = 'mittente assente' AND n.is_interna`, id, f.idCorriere1); n != 3 {
			t.Fatalf("attesi 3 tentativi di ritiro con nota del corriere, trovati %d", n)
		}
	})

	t.Run("tentativi_di_consegna_fino_a_non_consegnata", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "TENT")
		f.assegna(t, id, f.idCorriere1, 201)
		f.stato(t, id, c1, map[string]any{"stato": "ritirata"}, 200)

		attesi := []string{"in consegna", "in consegna", "non consegnata"}
		for i, atteso := range attesi {
			f.stato(t, id, c1, map[string]any{"stato": "in consegna"}, 200)
			corpo := consegnaSenzaMotivo
			if atteso == "non consegnata" {
				f.tentativo(t, id, c1, consegnaSenzaMotivo, 400)
				corpo = consegnaFallita
			}
			r := f.tentativo(t, id, c1, corpo, 201)
			if r.corpo["stato"] != atteso || int(r.corpo["tentativi_consegna_falliti"].(float64)) != i+1 || r.corpo["is_reso"] != false {
				t.Fatalf("tentativo %d: atteso %q, ottenuto %s", i+1, atteso, r.grezzo)
			}
			if atteso == "in consegna" {
				f.tentativo(t, id, c1, consegnaSenzaMotivo, 409)
				f.stato(t, id, c1, map[string]any{"stato": "ritirata"}, 403)
				f.stato(t, id, op, map[string]any{"stato": "ritirata"}, 200)
			}
		}
		if n := f.conta(t, `SELECT count(*) FROM tentativi t JOIN note n ON n.id = t.id_nota
			WHERE t.id_spedizione = $1 AND t.tipo = 'consegna' AND n.id_corriere_creata = $2 AND n.is_interna`, id, f.idCorriere1); n != 3 {
			t.Fatalf("attesi 3 tentativi di consegna con la nota del corriere, trovati %d", n)
		}
		if n := f.conta(t, `SELECT count(*) FROM tentativi t JOIN note n ON n.id = t.id_nota
			WHERE t.id_spedizione = $1 AND n.motivo = 'destinatario assente'`, id); n != 1 {
			t.Fatalf("atteso il motivo sul tentativo finale, trovati %d", n)
		}
		f.stato(t, id, c1, map[string]any{"stato": "in consegna"}, 409)
		if r := f.modifica(t, id, comeOperatore(f), map[string]any{"is_reso": true}, 200); r.corpo["is_reso"] != true {
			t.Fatalf("reso su spedizione non consegnata: %s", r.grezzo)
		}
	})

	t.Run("regole_e_dati_400_403_409", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "TENTERR")
		f.tentativo(t, id, c1, ritiroFallito, 403)
		f.assegna(t, id, f.idCorriere1, 201)
		f.tentativo(t, id, c2, ritiroFallito, 403)
		f.tentativo(t, id, op, ritiroFallito, 403)
		f.tentativo(t, id, c1, consegnaFallita, 409)

		r := f.tentativo(t, id, c1, map[string]any{"tipo": "ritiro"}, 400)
		if campi := campiInErrore(r); !contiene(campi, "nota") || contiene(campi, "motivo") {
			t.Fatalf("atteso solo l'errore sulla nota: %v", campi)
		}
		f.tentativo(t, id, c1, map[string]any{"nota": "x"}, 400)
		f.tentativo(t, id, c1, map[string]any{"tipo": "ritiro", "nota": "x", "motivo": "destinatario assente"}, 400)
		r = f.tentativo(t, id, c1, map[string]any{"tipo": "ritiro", "nota": "x", "motivo": "pacco non pronto"}, 201)
		if int(r.corpo["tentativi_ritiro_falliti"].(float64)) != 1 {
			t.Fatalf("ritiro con motivo libero: %s", r.grezzo)
		}
	})

	t.Run("tentativo_ripetuto_con_idempotency_key", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "IDEMRIT")
		f.assegna(t, id, f.idCorriere1, 201)
		header := comeCorriere(f.idCorriere1)
		header["Idempotency-Key"] = "ritiro-" + f.suffisso + "-" + strconv.Itoa(id)
		percorso := "/api/spedizioni/" + strconv.Itoa(id) + "/tentativi"

		primo := f.chiama(t, http.MethodPost, percorso, ritiroFallito, header)
		secondo := f.chiama(t, http.MethodPost, percorso, ritiroFallito, header)
		if primo.stato != 201 || secondo.stato != 201 || secondo.header.Get("Idempotent-Replayed") != "true" {
			t.Fatalf("attesa ripetizione della stessa risposta: %d / %d %v", primo.stato, secondo.stato, secondo.header)
		}
		if n := f.conta(t, `SELECT count(*) FROM tentativi WHERE id_spedizione = $1`, id); n != 1 {
			t.Fatalf("con la stessa Idempotency-Key il tentativo va registrato una volta sola, trovati %d", n)
		}
	})
}

func TestCambioStato(t *testing.T) {
	f := preparaFixture(t)
	op, c1 := comeOperatore(f), comeCorriere(f.idCorriere1)

	t.Run("ciclo_completo_fino_a_consegnata", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "CICLO")
		f.assegna(t, id, f.idCorriere1, 201)
		f.stato(t, id, c1, map[string]any{"stato": "ritirata"}, 200)
		f.stato(t, id, c1, map[string]any{"stato": "in consegna"}, 200)
		r := f.stato(t, id, c1, map[string]any{"stato": "consegnata"}, 200)
		if r.corpo["stato"] != "consegnata" || r.corpo["is_reso"] != false {
			t.Fatalf("stato finale errato: %s", r.grezzo)
		}
		if n := f.storico(t, id); n != 5 {
			t.Fatalf("attese 5 righe di storico, trovate %d", n)
		}
		if n := f.conta(t, `SELECT count(*) FROM cambiamenti_spedizioni WHERE id_spedizione = $1 AND id_corriere = $2`, id, f.idCorriere1); n != 3 {
			t.Fatalf("attese 3 righe di storico del corriere, trovate %d", n)
		}
		f.stato(t, id, c1, map[string]any{"stato": "in consegna"}, 409)
		f.modifica(t, id, op, map[string]any{"is_reso": true}, 409)
	})

	t.Run("ripetere_lo_stesso_stato_non_cambia_niente", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "RIPETI")
		f.assegna(t, id, f.idCorriere1, 201)
		f.stato(t, id, c1, map[string]any{"stato": "ritirata"}, 200)
		prima := f.storico(t, id)
		f.stato(t, id, c1, map[string]any{"stato": "ritirata"}, 200)
		if dopo := f.storico(t, id); dopo != prima {
			t.Fatalf("la ripetizione non deve scrivere nello storico: da %d a %d", prima, dopo)
		}
	})

	t.Run("annullamento_solo_prima_del_ritiro", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "ANN")
		f.assegna(t, id, f.idCorriere1, 201)
		r := f.stato(t, id, op, map[string]any{"stato": "annullata", "motivo": "richiesta del mittente", "nota": "il negozio ha annullato l'ordine"}, 200)
		if r.corpo["annullata_il"] == nil {
			t.Fatalf("manca la data di annullamento: %s", r.grezzo)
		}
		if n := f.conta(t, `SELECT count(*) FROM cambiamenti_spedizioni WHERE id_spedizione = $1 AND operazione = 'annullamento' AND id_operatore = $2`, id, f.idOperatore); n != 1 {
			t.Fatalf("annullamento non registrato nello storico con l'operatore")
		}

		id2 := f.nuovaSpedizione(t, "ANN2")
		f.assegna(t, id2, f.idCorriere1, 201)
		f.stato(t, id2, c1, map[string]any{"stato": "ritirata"}, 200)
		f.stato(t, id2, op, map[string]any{"stato": "annullata", "motivo": "altro", "nota": "x"}, 409)
	})

	t.Run("persa_solo_operatore_con_nota", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "PERSA")
		f.assegna(t, id, f.idCorriere1, 201)
		f.stato(t, id, op, map[string]any{"stato": "persa", "motivo": "furto", "nota": "non ancora ritirata"}, 409)
		f.stato(t, id, c1, map[string]any{"stato": "ritirata"}, 200)
		f.stato(t, id, c1, map[string]any{"stato": "persa", "motivo": "furto", "nota": "pacco non trovato"}, 403)
		f.stato(t, id, op, map[string]any{"stato": "persa", "motivo": "furto"}, 400)
		r := f.stato(t, id, op, map[string]any{"stato": "persa", "nota": "non trovato nel furgone"}, 400)
		if !contiene(campiInErrore(r), "motivo") {
			t.Fatalf("atteso errore sul motivo: %s", r.grezzo)
		}
		f.stato(t, id, op, map[string]any{"stato": "persa", "motivo": "smarrita durante il trasporto", "nota": "non trovato nel furgone"}, 200)
		if n := f.conta(t, `SELECT count(*) FROM note WHERE id_spedizione = $1 AND motivo = 'smarrita durante il trasporto'
			AND nota = 'non trovato nel furgone' AND id_operatore_logistico_creata = $2`, id, f.idOperatore); n != 1 {
			t.Fatal("motivo e nota della perdita devono essere salvati con l'operatore")
		}
		if r := f.modifica(t, id, op, map[string]any{"is_reso": true}, 200); r.corpo["is_reso"] != true {
			t.Fatalf("reso su spedizione persa: %s", r.grezzo)
		}
		f.stato(t, id, c1, map[string]any{"stato": "in consegna"}, 409)
	})

	t.Run("stati_che_si_impostano_altrove_400", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "ALTROVE")
		for _, stato := range []string{"assegnata", "non consegnata", "creata", "volata"} {
			f.stato(t, id, op, map[string]any{"stato": stato}, 400)
		}
		f.stato(t, id, op, map[string]any{}, 400)
		f.stato(t, id, op, map[string]any{"stato": "annullata", "id_corriere": 1}, 400)
	})

	t.Run("transizioni_non_ammesse_409", func(t *testing.T) {
		id := f.nuovaSpedizione(t, "SALTO")
		f.stato(t, id, c1, map[string]any{"stato": "consegnata"}, 409)
		f.assegna(t, id, f.idCorriere1, 201)
		f.stato(t, id, c1, map[string]any{"stato": "consegnata"}, 409)
	})

	t.Run("spedizione_inesistente_404_e_id_non_valido_400", func(t *testing.T) {
		if r := f.chiama(t, http.MethodPatch, "/api/spedizioni/999999999/stato", map[string]any{"stato": "annullata", "motivo": "altro", "nota": "x"}, op); r.stato != http.StatusNotFound {
			t.Fatalf("atteso 404, ottenuto %d", r.stato)
		}
		if r := f.chiama(t, http.MethodPatch, "/api/spedizioni/abc/stato", map[string]any{"stato": "annullata"}, op); r.stato != http.StatusBadRequest {
			t.Fatalf("atteso 400, ottenuto %d", r.stato)
		}
	})
}

func TestElencoSpedizioni(t *testing.T) {
	f := preparaFixture(t)
	op, c1, c2 := comeOperatore(f), comeCorriere(f.idCorriere1), comeCorriere(f.idCorriere2)
	dest, mitt, admin := comeCliente(f.idDestinatario), comeCliente(f.idMittente), comeAmministratore(f)

	creata := f.nuovaSpedizione(t, "EL-CREATA")
	assegnata := f.nuovaSpedizione(t, "EL-ASSEGNATA")
	f.assegna(t, assegnata, f.idCorriere1, 201)
	inConsegna := f.nuovaSpedizione(t, "EL-INCONSEGNA")
	f.assegna(t, inConsegna, f.idCorriere1, 201)
	f.stato(t, inConsegna, c1, map[string]any{"stato": "ritirata"}, 200)
	f.stato(t, inConsegna, c1, map[string]any{"stato": "in consegna"}, 200)
	consegnata := f.nuovaSpedizione(t, "EL-CONSEGNATA")
	f.assegna(t, consegnata, f.idCorriere1, 201)
	f.stato(t, consegnata, c1, map[string]any{"stato": "ritirata"}, 200)
	f.stato(t, consegnata, c1, map[string]any{"stato": "in consegna"}, 200)
	f.stato(t, consegnata, c1, map[string]any{"stato": "consegnata"}, 200)
	annullata := f.nuovaSpedizione(t, "EL-ANNULLATA")
	f.assegna(t, annullata, f.idCorriere1, 201)
	f.stato(t, annullata, op, map[string]any{"stato": "annullata", "motivo": "richiesta del mittente", "nota": "x"}, 200)
	altroCorriere := f.nuovaSpedizione(t, "EL-ALTRO")
	f.assegna(t, altroCorriere, f.idCorriere2, 201)
	consegnataDa10 := f.nuovaSpedizione(t, "EL-CONS10")
	f.consegnaCompleta(t, consegnataDa10)
	f.retrodataStorico(t, consegnataDa10, 10)
	consegnataDa40 := f.nuovaSpedizione(t, "EL-CONS40")
	f.consegnaCompleta(t, consegnataDa40)
	f.retrodataStorico(t, consegnataDa40, 40)
	annullataDa40 := f.nuovaSpedizione(t, "EL-ANN40")
	f.stato(t, annullataDa40, op, map[string]any{"stato": "annullata", "motivo": "richiesta del mittente", "nota": "x"}, 200)
	f.retrodataStorico(t, annullataDa40, 40)
	inCorsoDa40 := f.nuovaSpedizione(t, "EL-INCORSO40")
	f.assegna(t, inCorsoDa40, f.idCorriere1, 201)
	f.retrodata(t, inCorsoDa40, 40)
	f.retrodataStorico(t, inCorsoDa40, 40)
	if _, err := f.db.Exec(context.Background(), `INSERT INTO note (nota, is_interna, id_operatore_logistico_creata, id_spedizione)
		VALUES ('nota interna', true, $1, $2), ('nota per il cliente', false, $1, $2)`, f.idOperatore, assegnata); err != nil {
		t.Fatal(err)
	}

	presenti := func(t *testing.T, voci map[int]map[string]any, ids ...int) {
		t.Helper()
		for _, id := range ids {
			if _, ok := voci[id]; !ok {
				t.Errorf("la spedizione %d deve essere visibile", id)
			}
		}
	}
	assenti := func(t *testing.T, voci map[int]map[string]any, ids ...int) {
		t.Helper()
		for _, id := range ids {
			if _, ok := voci[id]; ok {
				t.Errorf("la spedizione %d non deve essere visibile", id)
			}
		}
	}

	t.Run("operatore_e_amministratore_vedono_tutto", func(t *testing.T) {
		for _, chi := range []map[string]string{op, admin} {
			voci, _ := f.elenco(t, chi, "per_pagina=100", 200)
			presenti(t, voci, creata, assegnata, inConsegna, consegnata, annullata, altroCorriere, consegnataDa10, consegnataDa40)
			if voci[assegnata]["id_corriere"] == nil {
				t.Errorf("l'operatore vede il corriere assegnato")
			}
			if len(voci[assegnata]["note"].([]any)) != 2 {
				t.Errorf("l'operatore vede anche le note interne: %v", voci[assegnata]["note"])
			}
		}
	})

	t.Run("corriere_vede_solo_le_sue_non_concluse", func(t *testing.T) {
		voci, _ := f.elenco(t, c1, "per_pagina=100", 200)
		presenti(t, voci, assegnata, inConsegna)
		assenti(t, voci, creata, consegnata, annullata, altroCorriere, consegnataDa10)
		for id, v := range voci {
			if _, c := v["id_corriere"]; c {
				t.Errorf("il corriere non riceve id_corriere (spedizione %d)", id)
			}
		}
		voci, _ = f.elenco(t, c2, "per_pagina=100", 200)
		presenti(t, voci, altroCorriere)
		assenti(t, voci, assegnata, inConsegna)
	})

	t.Run("cliente_vede_le_sue_fino_a_30_giorni_dalla_chiusura", func(t *testing.T) {
		voci, _ := f.elenco(t, dest, "per_pagina=100", 200)
		presenti(t, voci, creata, assegnata, inConsegna, consegnata, annullata, consegnataDa10, inCorsoDa40)
		assenti(t, voci, consegnataDa40, annullataDa40)
		if voci[consegnata]["conclusa_il"] == nil || voci[assegnata]["conclusa_il"] != nil {
			t.Errorf("conclusa_il valorizzata solo per le spedizioni chiuse: %v / %v", voci[consegnata]["conclusa_il"], voci[assegnata]["conclusa_il"])
		}
		note := voci[assegnata]["note"].([]any)
		if len(note) != 1 || note[0].(map[string]any)["nota"] != "nota per il cliente" {
			t.Errorf("il cliente vede solo le note non interne: %v", note)
		}
		if _, c := voci[assegnata]["id_corriere"]; c {
			t.Errorf("il cliente non riceve id_corriere")
		}
		voci, _ = f.elenco(t, mitt, "per_pagina=100", 200)
		assenti(t, voci, creata, assegnata, consegnata, consegnataDa10)
	})

	t.Run("filtro_per_stato_e_paginazione", func(t *testing.T) {
		voci, _ := f.elenco(t, op, "stato=consegnata&per_pagina=100", 200)
		presenti(t, voci, consegnata)
		for id, v := range voci {
			if v["stato"] != "consegnata" {
				t.Errorf("con il filtro arrivano solo spedizioni consegnate (spedizione %d: %v)", id, v["stato"])
			}
		}
		voci, r := f.elenco(t, op, "pagina=2&per_pagina=1", 200)
		if len(voci) != 1 || r.corpo["pagina"].(float64) != 2 || r.corpo["per_pagina"].(float64) != 1 {
			t.Errorf("paginazione: %s", r.grezzo)
		}
		voci, _ = f.elenco(t, c1, "stato=consegnata&per_pagina=100", 200)
		assenti(t, voci, consegnata)
	})

	t.Run("filtri_corriere_data_riferimento", func(t *testing.T) {
		voci, _ := f.elenco(t, op, "id_corriere="+strconv.Itoa(int(f.idCorriere2))+"&per_pagina=100", 200)
		presenti(t, voci, altroCorriere)
		assenti(t, voci, creata, assegnata)
		for id, v := range voci {
			if int32(v["id_corriere"].(float64)) != f.idCorriere2 {
				t.Errorf("filtro per corriere: la spedizione %d è di un altro corriere", id)
			}
		}

		voci, _ = f.elenco(t, op, "cod_ordine="+url.QueryEscape(strings.ToLower("EL-ASSEGNATA-"+f.suffisso)), 200)
		if len(voci) != 1 || voci[assegnata] == nil {
			t.Errorf("filtro per riferimento (senza distinguere maiuscole): attesa solo la spedizione %d, ottenute %v", assegnata, len(voci))
		}
		voci, _ = f.elenco(t, op, "cod_ordine="+f.suffisso+"&per_pagina=100", 200)
		presenti(t, voci, creata, assegnata, consegnata)

		oggi, domani := f.dataRoma(t, 0), f.dataRoma(t, 1)
		voci, _ = f.elenco(t, op, "dal="+oggi+"&al="+oggi+"&per_pagina=100", 200)
		presenti(t, voci, creata, assegnata)
		assenti(t, voci, inCorsoDa40)
		voci, _ = f.elenco(t, op, "dal="+domani+"&per_pagina=100", 200)
		assenti(t, voci, creata, assegnata)
		voci, _ = f.elenco(t, op, "al="+f.dataRoma(t, -30)+"&cod_ordine="+f.suffisso+"&per_pagina=100", 200)
		presenti(t, voci, inCorsoDa40)
		assenti(t, voci, creata)

		voci, _ = f.elenco(t, op, "stato=assegnata&id_corriere="+strconv.Itoa(int(f.idCorriere1))+"&cod_ordine="+f.suffisso+"&per_pagina=100", 200)
		presenti(t, voci, assegnata, inCorsoDa40)
		assenti(t, voci, inConsegna, consegnata, altroCorriere)

		f.elenco(t, dest, "id_corriere=1", 403)
		f.elenco(t, c1, "id_corriere=1", 403)
	})

	t.Run("parametri_non_validi_400", func(t *testing.T) {
		for _, q := range []string{"stato=volata", "pagina=0", "per_pagina=101", "per_pagina=abc",
			"id_corriere=abc", "id_corriere=0", "dal=01/11/2026", "al=ieri", "dal=2026-11-02&al=2026-11-01", "cod_ordine="} {
			f.elenco(t, op, q, 400)
		}
	})

	t.Run("cliente_e_amministratore_non_modificano", func(t *testing.T) {
		if r := f.chiama(t, http.MethodPost, "/api/spedizioni", f.corpoValido("EL-ADMIN"), admin); r.stato != http.StatusForbidden {
			t.Errorf("l'amministratore non crea spedizioni: %d", r.stato)
		}
		if r := f.chiama(t, http.MethodPost, "/api/spedizioni", f.corpoValido("EL-CLI"), dest); r.stato != http.StatusForbidden {
			t.Errorf("il cliente non crea spedizioni: %d", r.stato)
		}
		f.stato(t, assegnata, dest, map[string]any{"stato": "ritirata"}, 403)
		f.stato(t, assegnata, admin, map[string]any{"stato": "ritirata"}, 403)
	})
}

func TestStoricoSpedizione(t *testing.T) {
	f := preparaFixture(t)
	op, c1, admin := comeOperatore(f), comeCorriere(f.idCorriere1), comeAmministratore(f)

	id := f.nuovaSpedizione(t, "STORICO")
	f.assegna(t, id, f.idCorriere1, 201)
	f.stato(t, id, c1, map[string]any{"stato": "ritirata"}, 200)
	percorso := "/api/spedizioni/" + strconv.Itoa(id) + "/storico"

	for _, chi := range []map[string]string{admin, op} {
		r := f.chiama(t, http.MethodGet, percorso, nil, chi)
		if r.stato != http.StatusOK {
			t.Fatalf("storico: atteso 200, ottenuto %d: %s", r.stato, r.grezzo)
		}
		dati := r.corpo["dati"].([]any)
		if len(dati) != 3 {
			t.Fatalf("attese 3 righe (creazione, assegnazione, ritiro), trovate %d: %s", len(dati), r.grezzo)
		}
		primo, ultimo := dati[0].(map[string]any), dati[2].(map[string]any)
		autorePrimo, autoreUltimo := primo["autore"].(map[string]any), ultimo["autore"].(map[string]any)
		if primo["operazione"] != "creazione" || primo["valore_vecchio"] != nil ||
			autorePrimo["ruolo"] != "operatore" || int32(autorePrimo["id"].(float64)) != f.idOperatore {
			t.Errorf("prima riga: creazione da parte dell'operatore: %v", primo)
		}
		vecchio, nuovo := ultimo["valore_vecchio"].(map[string]any), ultimo["valore_nuovo"].(map[string]any)
		if vecchio["stato"] != "assegnata" || nuovo["stato"] != "ritirata" ||
			autoreUltimo["ruolo"] != "corriere" || int32(autoreUltimo["id"].(float64)) != f.idCorriere1 {
			t.Errorf("ultima riga: ritiro da parte del corriere: %v", ultimo)
		}
	}

	for _, chi := range []map[string]string{c1, comeCliente(f.idDestinatario)} {
		if r := f.chiama(t, http.MethodGet, percorso, nil, chi); r.stato != http.StatusForbidden {
			t.Errorf("solo amministratore e operatore vedono lo storico: %d", r.stato)
		}
	}
	if r := f.chiama(t, http.MethodGet, "/api/spedizioni/999999999/storico", nil, admin); r.stato != http.StatusNotFound {
		t.Errorf("spedizione inesistente: atteso 404, ottenuto %d", r.stato)
	}
	if r := f.chiama(t, http.MethodGet, "/api/spedizioni/abc/storico", nil, admin); r.stato != http.StatusBadRequest {
		t.Errorf("id non valido: atteso 400, ottenuto %d", r.stato)
	}
}
