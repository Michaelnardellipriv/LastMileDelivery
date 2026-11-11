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
	"net/http"
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
	idCorrierePos    int32
	idCorriereFermo  int32
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
	f.idCorrierePos = inserisci(`INSERT INTO corrieri (pwd_hash, id_utente) VALUES ('x', $1) RETURNING id`, utente("corrierepos"))
	f.idCorriereFermo = inserisci(`INSERT INTO corrieri (pwd_hash, id_utente) VALUES ('x', $1) RETURNING id`, utente("corrierefermo"))

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

func (f *fixture) dettaglio(t *testing.T, id int, chi map[string]string, atteso int) risposta {
	t.Helper()
	return f.invia(t, http.MethodGet, id, "", chi, nil, atteso)
}

func (f *fixture) nota(t *testing.T, id int, chi map[string]string, corpo map[string]any, atteso int) risposta {
	t.Helper()
	return f.invia(t, http.MethodPost, id, "/note", chi, corpo, atteso)
}

func autoreNota(n map[string]any) (string, int32) {
	a, ok := n["autore"].(map[string]any)
	if !ok {
		return "", 0
	}
	return a["ruolo"].(string), int32(a["id"].(float64))
}
