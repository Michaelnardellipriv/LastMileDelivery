package autenticazione

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"lastmiledelivery/backend/internal/identita"
)

var (
	intestazioneJWT    = base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	errTokenNonValido  = errors.New("token non valido o scaduto")
	errSessioneScaduta = errors.New("sessione scaduta: rifare il login")
	ruoliAmmessi       = map[identita.Ruolo]bool{
		identita.Operatore: true, identita.Corriere: true, identita.Cliente: true, identita.Amministratore: true,
	}
)

// durataSessionePredefinita: per quanto tempo dopo il login un token si può rinnovare.
// Oltre, si rifà il login: un token rubato non si allunga all'infinito.
const durataSessionePredefinita = 12 * time.Hour

type Firmatario struct {
	segreto        []byte
	durata         time.Duration // durata di un token (DURATA_TOKEN)
	durataSessione time.Duration // durata massima della sessione (DURATA_SESSIONE)
	adesso         func() time.Time
}

func NuovoFirmatario(segreto []byte, durata time.Duration) *Firmatario {
	return &Firmatario{segreto: segreto, durata: durata, durataSessione: durataSessionePredefinita, adesso: time.Now}
}

// ConDurataSessione imposta quanto dura una sessione dal login (DURATA_SESSIONE).
func (f *Firmatario) ConDurataSessione(d time.Duration) *Firmatario {
	f.durataSessione = d
	return f
}

func (f *Firmatario) DurataSessione() time.Duration { return f.durataSessione }

type rivendicazioni struct {
	Soggetto string `json:"sub"`
	Ruolo    string `json:"ruolo"`
	ID       int64  `json:"id"`
	// sede di operatori e corrieri quando il token è stato emesso (assente per gli altri
	// ruoli). Un cambio di sede chiude le sessioni dell'utente, quindi un token con la sede
	// vecchia smette subito di valere.
	Sede     int64 `json:"sede,omitempty"`
	Emesso   int64 `json:"iat"`
	Scadenza int64 `json:"exp"`
	// la sessione del login (tabella sessioni del database globale): il token vale solo
	// finché è aperta, e il rinnovo la conserva
	Sessione string `json:"sid"`
}

// Firma emette il token della sessione r.Sessione (login o rinnovo).
func (f *Firmatario) Firma(idUtente int64, r identita.Richiedente) (string, time.Time, error) {
	return f.FirmaFinoA(idUtente, r, time.Time{})
}

// FirmaFinoA come Firma, ma il token non scade dopo limite (la fine della sessione): così
// un token non vale mai più a lungo della sessione da cui viene.
func (f *Firmatario) FirmaFinoA(idUtente int64, r identita.Richiedente, limite time.Time) (string, time.Time, error) {
	adesso := f.adesso()
	scadenza := adesso.Add(f.durata)
	if !limite.IsZero() && limite.Before(scadenza) {
		scadenza = limite
	}
	corpo, err := json.Marshal(rivendicazioni{
		Soggetto: strconv.FormatInt(idUtente, 10), Ruolo: string(r.Ruolo), ID: r.ID, Sede: r.IDSede,
		Emesso: adesso.Unix(), Scadenza: scadenza.Unix(), Sessione: r.Sessione,
	})
	if err != nil {
		return "", time.Time{}, err
	}
	firmato := intestazioneJWT + "." + base64.RawURLEncoding.EncodeToString(corpo)
	return firmato + "." + f.firma(firmato), scadenza, nil
}

// Verifica controlla firma e scadenza: è il controllo di ogni richiesta.
func (f *Firmatario) Verifica(token string) (identita.Richiedente, error) {
	c, err := f.leggi(token)
	if err != nil || f.adesso().Unix() >= c.Scadenza {
		return identita.Richiedente{}, errTokenNonValido
	}
	return richiedente(c), nil
}

// VerificaPerRinnovo controlla la firma ma accetta anche un token scaduto, purché non da
// più della durata massima di una sessione: così un'app rimasta senza rete o sospesa per
// più di un quarto d'ora rinnova senza rifare il login. Il token scaduto non vale per
// nient'altro, e il rinnovo controlla nel database che la sessione sia ancora aperta e
// l'utente attivo (Servizio.Rinnova).
func (f *Firmatario) VerificaPerRinnovo(token string) (identita.Richiedente, error) {
	c, err := f.leggi(token)
	if err != nil {
		return identita.Richiedente{}, err
	}
	if !f.adesso().Before(time.Unix(c.Scadenza, 0).Add(f.durataSessione)) {
		return identita.Richiedente{}, errSessioneScaduta
	}
	return richiedente(c), nil
}

func richiedente(c rivendicazioni) identita.Richiedente {
	return identita.Richiedente{Ruolo: identita.Ruolo(c.Ruolo), ID: c.ID, IDSede: c.Sede, Sessione: c.Sessione}
}

// leggi controlla la firma e il ruolo e restituisce le rivendicazioni.
func (f *Firmatario) leggi(token string) (rivendicazioni, error) {
	var c rivendicazioni
	parti := strings.Split(token, ".")
	if len(parti) != 3 || parti[0] != intestazioneJWT {
		return c, errTokenNonValido
	}
	if !hmac.Equal([]byte(parti[2]), []byte(f.firma(parti[0]+"."+parti[1]))) {
		return c, errTokenNonValido
	}
	corpo, err := base64.RawURLEncoding.DecodeString(parti[1])
	if err != nil {
		return c, errTokenNonValido
	}
	if err := json.Unmarshal(corpo, &c); err != nil {
		return c, errTokenNonValido
	}
	if !ruoliAmmessi[identita.Ruolo(c.Ruolo)] || c.ID <= 0 || c.Sessione == "" {
		return c, errTokenNonValido
	}
	return c, nil
}

func (f *Firmatario) firma(dati string) string {
	m := hmac.New(sha256.New, f.segreto)
	m.Write([]byte(dati))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
