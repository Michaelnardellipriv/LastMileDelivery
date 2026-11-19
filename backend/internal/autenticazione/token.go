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
	intestazioneJWT   = base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	errTokenNonValido = errors.New("token non valido o scaduto")
	ruoliAmmessi      = map[identita.Ruolo]bool{
		identita.Operatore: true, identita.Corriere: true, identita.Cliente: true, identita.Amministratore: true,
	}
)

type Firmatario struct {
	segreto []byte
	durata  time.Duration
	adesso  func() time.Time
}

func NuovoFirmatario(segreto []byte, durata time.Duration) *Firmatario {
	return &Firmatario{segreto: segreto, durata: durata, adesso: time.Now}
}

type rivendicazioni struct {
	Soggetto string `json:"sub"`
	Ruolo    string `json:"ruolo"`
	ID       int32  `json:"id"`
	Emesso   int64  `json:"iat"`
	Scadenza int64  `json:"exp"`
}

func (f *Firmatario) Firma(idUtente int32, r identita.Richiedente) (string, time.Time, error) {
	adesso := f.adesso()
	scadenza := adesso.Add(f.durata)
	corpo, err := json.Marshal(rivendicazioni{
		Soggetto: strconv.Itoa(int(idUtente)), Ruolo: string(r.Ruolo), ID: r.ID,
		Emesso: adesso.Unix(), Scadenza: scadenza.Unix(),
	})
	if err != nil {
		return "", time.Time{}, err
	}
	firmato := intestazioneJWT + "." + base64.RawURLEncoding.EncodeToString(corpo)
	return firmato + "." + f.firma(firmato), scadenza, nil
}

func (f *Firmatario) Verifica(token string) (identita.Richiedente, error) {
	parti := strings.Split(token, ".")
	if len(parti) != 3 || parti[0] != intestazioneJWT {
		return identita.Richiedente{}, errTokenNonValido
	}
	if !hmac.Equal([]byte(parti[2]), []byte(f.firma(parti[0]+"."+parti[1]))) {
		return identita.Richiedente{}, errTokenNonValido
	}
	corpo, err := base64.RawURLEncoding.DecodeString(parti[1])
	if err != nil {
		return identita.Richiedente{}, errTokenNonValido
	}
	var c rivendicazioni
	if err := json.Unmarshal(corpo, &c); err != nil {
		return identita.Richiedente{}, errTokenNonValido
	}
	ruolo := identita.Ruolo(c.Ruolo)
	if !ruoliAmmessi[ruolo] || c.ID <= 0 || f.adesso().Unix() >= c.Scadenza {
		return identita.Richiedente{}, errTokenNonValido
	}
	return identita.Richiedente{Ruolo: ruolo, ID: c.ID}, nil
}

func (f *Firmatario) firma(dati string) string {
	m := hmac.New(sha256.New, f.segreto)
	m.Write([]byte(dati))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
