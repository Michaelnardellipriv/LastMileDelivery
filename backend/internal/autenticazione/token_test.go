package autenticazione

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"lastmiledelivery/backend/internal/identita"
)

func firmatarioDiProva(adesso time.Time) *Firmatario {
	f := NuovoFirmatario([]byte("segreto-di-prova-lungo-almeno-32-caratteri"), time.Hour)
	f.adesso = func() time.Time { return adesso }
	return f
}

func TestTokenFirmaEVerifica(t *testing.T) {
	adesso := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	f := firmatarioDiProva(adesso)
	corriere := identita.Richiedente{Ruolo: identita.Corriere, ID: 5}

	token, scadenza, err := f.Firma(12, corriere)
	if err != nil {
		t.Fatal(err)
	}
	if !scadenza.Equal(adesso.Add(time.Hour)) {
		t.Fatalf("scadenza dopo un'ora: %v", scadenza)
	}
	r, err := f.Verifica(token)
	if err != nil || r != corriere {
		t.Fatalf("il token restituisce ruolo e id: %+v %v", r, err)
	}

	f.adesso = func() time.Time { return adesso.Add(time.Hour) }
	if _, err := f.Verifica(token); err == nil {
		t.Fatal("un token scaduto va rifiutato")
	}
}

func TestTokenNonValidi(t *testing.T) {
	adesso := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	f := firmatarioDiProva(adesso)
	token, _, _ := f.Firma(12, identita.Richiedente{Ruolo: identita.Operatore, ID: 1})
	parti := strings.Split(token, ".")

	altro := NuovoFirmatario([]byte("un-altro-segreto-lungo-almeno-32-caratteri"), time.Hour)
	altro.adesso = f.adesso
	tokenAltro, _, _ := altro.Firma(12, identita.Richiedente{Ruolo: identita.Operatore, ID: 1})

	corpoAmministratore := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"12","ruolo":"amministratore","id":1,"iat":0,"exp":9999999999}`))
	senzaAlgoritmo := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))

	for nome, tk := range map[string]string{
		"vuoto":                     "",
		"due parti":                 parti[0] + "." + parti[1],
		"firma di un altro segreto": tokenAltro,
		"corpo cambiato":            parti[0] + "." + corpoAmministratore + "." + parti[2],
		"algoritmo none":            senzaAlgoritmo + "." + parti[1] + ".",
		"firma troncata":            token[:len(token)-2],
	} {
		if _, err := f.Verifica(tk); err == nil {
			t.Errorf("%s: il token va rifiutato", nome)
		}
	}
}
