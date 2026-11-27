//go:build integration

package apitest

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Il ruolo di un utente non si cambia: né dall'API (400 sul campo ruolo) né con un UPDATE
// o un DELETE diretto nel database (trigger).
func TestRuoloNonModificabile(t *testing.T) {
	f := preparaFixture(t)
	admin := comeAmministratore(f)
	var idUtente int64
	if err := f.db.QueryRow(t.Context(), `SELECT id_utente FROM corrieri WHERE id = $1`, f.idCorriere2).Scan(&idUtente); err != nil {
		t.Fatal(err)
	}
	percorso := percorsoUtenti + "/" + strconv.FormatInt(idUtente, 10)

	for _, ruolo := range []string{"amministratore", "operatore", "corriere"} {
		r := f.chiama(t, http.MethodPatch, percorso, map[string]any{"ruolo": ruolo}, admin)
		if r.stato != http.StatusBadRequest || !contiene(campiInErrore(r), "ruolo") {
			t.Errorf("PATCH con ruolo %q: atteso 400 sul campo ruolo, ottenuto %d %s", ruolo, r.stato, r.grezzo)
		}
	}
	r := f.chiama(t, http.MethodGet, percorsoUtenti+"?q="+f.emailDi("corriere2"), nil, admin)
	if u := r.corpo["dati"].([]any)[0].(map[string]any); u["ruolo"] != "corriere" {
		t.Fatalf("il ruolo è rimasto corriere: %v", u)
	}

	ctx := context.Background()
	for nome, sql := range map[string]string{
		"spostare la riga a un altro utente": `UPDATE corrieri SET id_utente = id_utente WHERE id = $1`,
		"togliere il ruolo":                  `DELETE FROM corrieri WHERE id = $1`,
	} {
		_, err := f.db.Exec(ctx, sql, f.idCorriere2)
		if err == nil || !strings.Contains(err.Error(), "ruolo") {
			t.Errorf("%s nel database: deve essere rifiutato dal trigger, ottenuto %v", nome, err)
		}
	}
}

// Least privilege: l'utente con cui si collega il backend non cancella (se non le chiavi
// di idempotenza e le sessioni scadute) e non modifica lo storico, le note e i tentativi.
func TestPrivilegiDelBackend(t *testing.T) {
	f := preparaFixture(t)
	app := valore("TEST_APP_DB_USER", "lastmile_app")
	prova := func(db *pgxpool.Pool, sql string) error {
		t.Helper()
		ctx := context.Background()
		tx, err := db.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx) // annulla anche il cambio di ruolo
		if _, err := tx.Exec(ctx, `SET LOCAL ROLE `+app); err != nil {
			t.Fatal(err)
		}
		_, err = tx.Exec(ctx, sql)
		return err
	}
	negati := []struct {
		db  *pgxpool.Pool
		sql string
	}{
		{f.db, `DELETE FROM utenti WHERE false`},
		{f.db, `DELETE FROM sedi WHERE false`},
		{f.db, `TRUNCATE recuperi_password`},
		{f.sped, `DELETE FROM spedizioni WHERE false`},
		{f.sped, `UPDATE cambiamenti_spedizioni SET id_sede = id_sede WHERE false`},
		{f.sped, `UPDATE note SET nota = nota WHERE false`},
		{f.sped, `UPDATE tentativi SET tipo = tipo WHERE false`},
	}
	for _, n := range negati {
		if err := prova(n.db, n.sql); err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Errorf("%s: deve essere negato all'utente del backend, ottenuto %v", n.sql, err)
		}
	}
	ammessi := []struct {
		db  *pgxpool.Pool
		sql string
	}{
		{f.db, `DELETE FROM chiavi_idempotenza WHERE false`},
		{f.db, `DELETE FROM sessioni WHERE false`},
		{f.sped, `DELETE FROM sessioni_revocate WHERE false`},
		{f.sped, `UPDATE spedizioni SET is_reso = is_reso WHERE false`},
	}
	for _, a := range ammessi {
		if err := prova(a.db, a.sql); err != nil {
			t.Errorf("%s: serve al backend, ottenuto %v", a.sql, err)
		}
	}
}

// Un token firmato ma senza sessione (o con una sessione che non esiste) non vale: il
// token da solo non basta, conta la sessione nel database.
func TestTokenSenzaSessioneValida(t *testing.T) {
	f := preparaFixture(t)
	f.impostaPassword(t, f.emailDi("operatore"), "PasswordOperatore1")
	sessione := f.login(t, map[string]any{"email": f.emailDi("operatore"), "password": "PasswordOperatore1"}, 200)
	// la sessione chiusa direttamente nel database (come farebbe un'altra replica):
	// il servizio utenti la vede chiusa alla richiesta successiva
	token := sessione.corpo["token"].(string)
	if _, err := f.db.Exec(t.Context(), `
		UPDATE sessioni SET revocata_il = now(), motivo_revoca = 'prova'
		WHERE id = (SELECT id FROM sessioni WHERE id_ruolo = $1 AND ruolo = 'operatore' ORDER BY creata_il DESC LIMIT 1)`,
		f.idOperatore); err != nil {
		t.Fatal(err)
	}
	if r := f.chiama(t, http.MethodGet, "/api/clienti", nil, map[string]string{"Authorization": "Bearer " + token}); r.stato != http.StatusUnauthorized {
		t.Errorf("sessione chiusa nel database: atteso 401, ottenuto %d", r.stato)
	}
}
