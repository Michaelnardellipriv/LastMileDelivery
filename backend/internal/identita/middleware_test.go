package identita

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"lastmiledelivery/backend/internal/database"
)

// utentiFinti: la sede che il database dà a ogni utente
type utentiFinti map[int64]int64

func (u utentiFinti) Stato(_ context.Context, _ Ruolo, id int64) (bool, bool, int64, error) {
	sede, ok := u[id]
	return ok, ok, sede, nil
}

// tokenFinto restituisce sempre lo stesso richiedente, con la sede del momento del login.
type tokenFinto struct{ r Richiedente }

func (t tokenFinto) Verifica(string) (Richiedente, error) { return t.r, nil }

// sessioniFinte: le sessioni aperte e la sede attuale di ogni utente nel database.
type sessioniFinte struct {
	aperte map[string]bool
	sede   int64
}

func (s sessioniFinte) Controlla(_ context.Context, r Richiedente) (Richiedente, error) {
	if !s.aperte[r.Sessione] {
		return r, errSessioneChiusa
	}
	r.IDSede = s.sede
	return r, nil
}

func TestLaSedeVieneDalDatabaseNonDalToken(t *testing.T) {
	// al login lavorava a Trento (10), poi l'amministratore l'ha spostato a Bolzano (20)
	token := tokenFinto{Richiedente{Ruolo: Corriere, ID: 5, IDSede: 10, Sessione: "s1"}}
	var visto Richiedente
	var ambito database.Ambito
	h := Richiedi(sessioniFinte{aperte: map[string]bool{"s1": true}, sede: 20}, utentiFinti{5: 20}, token, false)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		visto, ambito = DalContesto(r.Context()), database.AmbitoDalContesto(r.Context())
	}))

	r := httptest.NewRequest(http.MethodGet, "/api/spedizioni", nil)
	r.Header.Set("Authorization", "Bearer qualcosa")
	h.ServeHTTP(httptest.NewRecorder(), r)

	if visto.IDSede != 20 || ambito != database.SoloSede(20) {
		t.Fatalf("vale la sede attuale: richiedente %+v, ambito %+v", visto, ambito)
	}
}

func TestAmbitoPerRuolo(t *testing.T) {
	casi := []struct {
		r      Richiedente
		atteso database.Ambito
	}{
		{Richiedente{Ruolo: Operatore, ID: 1, IDSede: 7}, database.SoloSede(7)},
		{Richiedente{Ruolo: Corriere, ID: 1, IDSede: 7}, database.SoloSede(7)},
		{Richiedente{Ruolo: Amministratore, ID: 1}, database.TutteLeSedi()},
		{Richiedente{Ruolo: Cliente, ID: 1}, database.TutteLeSedi()},
		// senza identità non si vede nessuna sede
		{Richiedente{}, database.Ambito{}},
	}
	for _, c := range casi {
		if a := c.r.Ambito(); a != c.atteso {
			t.Errorf("%q: ambito %+v, atteso %+v", c.r.Ruolo, a, c.atteso)
		}
		if c.atteso.TutteLeSedi != (c.atteso.Ammette(1) && c.atteso.Ammette(2)) {
			t.Errorf("%q: Ammette non coerente con l'ambito", c.r.Ruolo)
		}
	}
}

// Il token vale solo finché la sua sessione è aperta nel database: chiusa (logout,
// disattivazione, cambio di sede, nuova password) → 401 subito, anche se il token non è
// scaduto; un token senza sessione non vale.
func TestTokenValeSoloConLaSessioneAperta(t *testing.T) {
	sessioni := sessioniFinte{aperte: map[string]bool{"aperta": true}, sede: 10}
	prova := func(r Richiedente) int {
		h := Richiedi(sessioni, utentiFinti{}, tokenFinto{r}, false)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		req := httptest.NewRequest(http.MethodGet, "/api/spedizioni", nil)
		req.Header.Set("Authorization", "Bearer qualcosa")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w.Code
	}
	if c := prova(Richiedente{Ruolo: Corriere, ID: 5, Sessione: "aperta"}); c != http.StatusOK {
		t.Errorf("sessione aperta: atteso 200, ottenuto %d", c)
	}
	if c := prova(Richiedente{Ruolo: Corriere, ID: 5, Sessione: "chiusa"}); c != http.StatusUnauthorized {
		t.Errorf("sessione chiusa: atteso 401, ottenuto %d", c)
	}
	if c := prova(Richiedente{Ruolo: Corriere, ID: 5}); c != http.StatusUnauthorized {
		t.Errorf("token senza sessione: atteso 401, ottenuto %d", c)
	}

	// gli header X-Id-* di sviluppo non hanno sessione: si controlla l'utente
	h := Richiedi(sessioni, utentiFinti{}, tokenFinto{}, true)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/api/spedizioni", nil)
	req.Header.Set("X-Id-Corriere", "5")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("header X-Id di un utente sconosciuto: atteso 401, ottenuto %d", w.Code)
	}
}
