//go:build integration

package apitest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func (f *fixture) emailDi(nome string) string {
	return nome + "." + f.suffisso + "@test.it"
}

func (f *fixture) impostaPassword(t *testing.T, email, password string) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(t.Context(), `UPDATE utenti SET pwd_hash = $2 WHERE email = $1`, email, string(hash)); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) login(t *testing.T, corpo map[string]any, atteso int) risposta {
	t.Helper()
	r := f.chiama(t, http.MethodPost, "/api/auth/login", corpo, nil)
	if r.stato != atteso {
		t.Fatalf("login %v: atteso %d, ottenuto %d: %s", corpo["email"], atteso, r.stato, r.grezzo)
	}
	return r
}

func conToken(r risposta) map[string]string {
	return map[string]string{"Authorization": "Bearer " + r.corpo["token"].(string)}
}

func TestLogin(t *testing.T) {
	f := preparaFixture(t)
	f.impostaPassword(t, f.emailDi("operatore"), "PasswordOperatore1")
	f.impostaPassword(t, f.emailDi("destinatario"), "PasswordCliente1")

	t.Run("operatore_e_cliente_destinatario", func(t *testing.T) {
		r := f.login(t, map[string]any{"email": f.emailDi("operatore"), "password": "PasswordOperatore1"}, 200)
		utente := r.corpo["utente"].(map[string]any)
		if r.corpo["tipo"] != "Bearer" || r.corpo["scade_il"] == nil || utente["ruolo"] != "operatore" ||
			int32(utente["id"].(float64)) != f.idOperatore || utente["email"] != f.emailDi("operatore") {
			t.Fatalf("risposta del login: %s", r.grezzo)
		}
		if r.header.Get("Cache-Control") != "no-store" {
			t.Errorf("il token non va messo in cache: %q", r.header.Get("Cache-Control"))
		}
		if e := f.chiama(t, http.MethodGet, "/api/spedizioni", nil, conToken(r)); e.stato != 200 {
			t.Fatalf("elenco con il token dell'operatore: %d %s", e.stato, e.grezzo)
		}

		r = f.login(t, map[string]any{"email": strings.ToUpper(f.emailDi("destinatario")), "password": "PasswordCliente1"}, 200)
		utente = r.corpo["utente"].(map[string]any)
		if utente["ruolo"] != "cliente" || int32(utente["id"].(float64)) != f.idDestinatario {
			t.Fatalf("il destinatario entra come cliente, email senza distinguere maiuscole: %s", r.grezzo)
		}
		if e := f.chiama(t, http.MethodGet, "/api/spedizioni", nil, conToken(r)); e.stato != 200 {
			t.Fatalf("elenco con il token del cliente: %d %s", e.stato, e.grezzo)
		}
		if e := f.chiama(t, http.MethodPost, "/api/spedizioni", f.corpoValido("AUTH-CLI"), conToken(r)); e.stato != http.StatusForbidden {
			t.Fatalf("il cliente non crea spedizioni: atteso 403, ottenuto %d", e.stato)
		}
	})

	t.Run("credenziali_sbagliate_401", func(t *testing.T) {
		sbagliata := f.login(t, map[string]any{"email": f.emailDi("operatore"), "password": "Sbagliata123"}, 401)
		inesistente := f.login(t, map[string]any{"email": "nessuno." + f.suffisso + "@test.it", "password": "Sbagliata123"}, 401)
		if sbagliata.corpo["errore"] != inesistente.corpo["errore"] {
			t.Errorf("stesso messaggio per non rivelare quali email esistono: %v / %v",
				sbagliata.corpo["errore"], inesistente.corpo["errore"])
		}
	})

	t.Run("password_obbligatoria_nel_database", func(t *testing.T) {
		for _, pwd := range []string{"NULL", "''"} {
			_, err := f.db.Exec(t.Context(), `INSERT INTO utenti (nome, cognome, prefisso, nr_telefono, email, pwd_hash)
				VALUES ('Senza', 'Password', '+39', $1, $2, `+pwd+`)`,
				fmt.Sprintf("6%d", time.Now().UnixNano()%1e9), "senzapwd."+f.suffisso+"@test.it")
			if err == nil {
				t.Errorf("pwd_hash %s: il database deve rifiutare un utente senza password", pwd)
			}
		}
	})

	t.Run("dati_non_validi_400", func(t *testing.T) {
		f.login(t, map[string]any{"email": f.emailDi("operatore")}, 400)
		f.login(t, map[string]any{"password": "x"}, 400)
		f.login(t, map[string]any{"email": f.emailDi("operatore"), "password": "x", "id": 1}, 400)
	})

	t.Run("token_non_valido_401", func(t *testing.T) {
		for _, h := range []map[string]string{
			{"Authorization": "Bearer abc.def.ghi"},
			{"Authorization": "Basic " + f.emailDi("operatore")},
		} {
			if r := f.chiama(t, http.MethodGet, "/api/spedizioni", nil, h); r.stato != http.StatusUnauthorized {
				t.Errorf("%v: atteso 401, ottenuto %d", h, r.stato)
			}
		}
	})

	t.Run("un_solo_ruolo_per_email", func(t *testing.T) {
		email := "doppio." + f.suffisso + "@test.it"
		var idUtente, idCorriere, idCliente int32
		ctx := t.Context()
		if err := f.db.QueryRow(ctx, `INSERT INTO utenti (nome, cognome, prefisso, nr_telefono, email, pwd_hash)
			VALUES ('Doppio', 'Ruolo', '+39', $1, $2, '!') RETURNING id`, fmt.Sprintf("7%d", time.Now().UnixNano()%1e9), email).Scan(&idUtente); err != nil {
			t.Fatal(err)
		}
		if err := f.db.QueryRow(ctx, `INSERT INTO corrieri (id_utente) VALUES ($1) RETURNING id`, idUtente).Scan(&idCorriere); err != nil {
			t.Fatal(err)
		}
		err := f.db.QueryRow(ctx, `INSERT INTO clienti (id_utente) VALUES ($1) RETURNING id`, idUtente).Scan(&idCliente)
		if err == nil || !strings.Contains(err.Error(), "un solo ruolo") {
			t.Fatalf("il database rifiuta un secondo ruolo per la stessa email, ottenuto %v", err)
		}
		f.impostaPassword(t, email, "PasswordDoppia1")
		r := f.login(t, map[string]any{"email": email, "password": "PasswordDoppia1"}, 200)
		if u := r.corpo["utente"].(map[string]any); u["ruolo"] != "corriere" || int32(u["id"].(float64)) != idCorriere {
			t.Fatalf("resta solo il ruolo di corriere: %s", r.grezzo)
		}
		f.login(t, map[string]any{"email": email, "password": "PasswordDoppia1", "ruolo": "cliente"}, 403)
	})
}

func TestCambioPassword(t *testing.T) {
	f := preparaFixture(t)
	email := f.emailDi("mittente")
	f.impostaPassword(t, email, "PasswordVecchia1")
	token := conToken(f.login(t, map[string]any{"email": email, "password": "PasswordVecchia1"}, 200))
	cambia := func(corpo map[string]any, chi map[string]string, atteso int) risposta {
		t.Helper()
		r := f.chiama(t, http.MethodPost, "/api/auth/cambio-password", corpo, chi)
		if r.stato != atteso {
			t.Fatalf("cambio password %v: atteso %d, ottenuto %d: %s", corpo, atteso, r.stato, r.grezzo)
		}
		return r
	}

	cambia(map[string]any{"password_attuale": "PasswordVecchia1", "nuova_password": "PasswordNuova1"}, nil, 401)
	cambia(map[string]any{"password_attuale": "Sbagliata123", "nuova_password": "PasswordNuova1"}, token, 403)
	if r := cambia(map[string]any{"password_attuale": "PasswordVecchia1", "nuova_password": "corta"}, token, 400); !contiene(campiInErrore(r), "nuova_password") {
		t.Fatalf("errore sul campo nuova_password: %s", r.grezzo)
	}
	cambia(map[string]any{"password_attuale": "PasswordVecchia1", "nuova_password": "PasswordNuova1"}, token, 204)

	f.login(t, map[string]any{"email": email, "password": "PasswordVecchia1"}, 401)
	f.login(t, map[string]any{"email": email, "password": "PasswordNuova1"}, 200)
}

func TestRecuperoPassword(t *testing.T) {
	f := preparaFixture(t)
	email := f.emailDi("corriere2")
	ctx := t.Context()
	var idUtente int32
	if err := f.db.QueryRow(ctx, `SELECT id FROM utenti WHERE email = $1`, email).Scan(&idUtente); err != nil {
		t.Fatal(err)
	}
	recuperi := func() int {
		return f.conta(t, `SELECT count(*) FROM recuperi_password WHERE id_utente = $1`, idUtente)
	}
	richiedi := func(email string) risposta {
		t.Helper()
		r := f.chiama(t, http.MethodPost, "/api/auth/recupero-password", map[string]any{"email": email}, nil)
		if r.stato != http.StatusAccepted {
			t.Fatalf("recupero per %s: atteso 202, ottenuto %d: %s", email, r.stato, r.grezzo)
		}
		return r
	}
	reimposta := func(token, password string, atteso int) {
		t.Helper()
		r := f.chiama(t, http.MethodPost, "/api/auth/reimposta-password", map[string]any{"token": token, "nuova_password": password}, nil)
		if r.stato != atteso {
			t.Fatalf("reimposta: atteso %d, ottenuto %d: %s", atteso, r.stato, r.grezzo)
		}
	}
	nuovoRecupero := func(token string, scadenza time.Duration) {
		t.Helper()
		h := sha256.Sum256([]byte(token))
		if _, err := f.db.Exec(ctx, `INSERT INTO recuperi_password (id_utente, token_hash, creato_il, scade_il)
			VALUES ($1, $2, now() - interval '1 hour', now() + $3::interval)`,
			idUtente, hex.EncodeToString(h[:]), fmt.Sprintf("%d seconds", int(scadenza.Seconds()))); err != nil {
			t.Fatal(err)
		}
	}

	f.impostaPassword(t, email, "PasswordDimenticata1")

	t.Run("richiesta_202_anche_per_email_sconosciute", func(t *testing.T) {
		sconosciuta := richiedi("nessuno." + f.suffisso + "@test.it")
		nota := richiedi(email)
		if sconosciuta.corpo["messaggio"] != nota.corpo["messaggio"] {
			t.Errorf("stessa risposta per email note e sconosciute: %v / %v", sconosciuta.corpo, nota.corpo)
		}
		if n := recuperi(); n != 1 {
			t.Fatalf("una richiesta registrata per l'utente, trovate %d", n)
		}
		var token string
		if err := f.db.QueryRow(ctx, `SELECT token_hash FROM recuperi_password WHERE id_utente = $1`, idUtente).Scan(&token); err != nil || len(token) != 64 {
			t.Fatalf("nel database c'è solo l'hash del token: %q %v", token, err)
		}
	})

	t.Run("al_massimo_3_richieste_all_ora", func(t *testing.T) {
		for i := 0; i < 4; i++ {
			richiedi(email)
		}
		if n := recuperi(); n != 3 {
			t.Fatalf("le richieste oltre la terza nell'ultima ora non creano link: trovate %d", n)
		}
	})

	t.Run("reimposta_con_token_valido_una_volta", func(t *testing.T) {
		token := "token-di-prova-" + f.suffisso
		nuovoRecupero(token, 30*time.Minute)
		reimposta("token-inesistente", "PasswordRecuperata1", 400)
		reimposta(token, "corta", 400)
		reimposta(token, "PasswordRecuperata1", 204)
		f.login(t, map[string]any{"email": email, "password": "PasswordRecuperata1"}, 200)
		reimposta(token, "AltraPassword1", 400)
	})

	t.Run("token_scaduto_400", func(t *testing.T) {
		token := "token-scaduto-" + f.suffisso
		nuovoRecupero(token, -time.Minute)
		reimposta(token, "PasswordScaduta1", 400)
	})
}
