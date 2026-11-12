package autenticazione

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"lastmiledelivery/backend/internal/errori"
)

const (
	lunghezzaMinimaPassword  = 8
	lunghezzaMassimaPassword = 72
)

var (
	hashFittizio      []byte
	hashFittizioUnico sync.Once
)

func controllaPassword(e *errori.Validazione, campo string, p *string) {
	switch {
	case p == nil:
		e.Aggiungi(campo, "obbligatoria")
	case strings.TrimSpace(*p) == "":
		e.Aggiungi(campo, "non può essere vuota")
	case utf8.RuneCountInString(*p) < lunghezzaMinimaPassword:
		e.Aggiungi(campo, "almeno 8 caratteri")
	case len(*p) > lunghezzaMassimaPassword:
		e.Aggiungi(campo, "al massimo 72 byte")
	}
}

func ControllaPassword(e *errori.Validazione, campo string, p *string) {
	controllaPassword(e, campo, p)
}

func HashPassword(p string) (string, error) {
	return hashPassword(p)
}

func hashPassword(p string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	return string(h), err
}

func passwordCorretta(hash string, p string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(p)) == nil
}

func confrontoFittizio(p string) {
	hashFittizioUnico.Do(func() {
		hashFittizio, _ = bcrypt.GenerateFromPassword([]byte("password-fittizia-per-tempi-uguali"), bcrypt.DefaultCost)
	})
	bcrypt.CompareHashAndPassword(hashFittizio, []byte(p))
}

func nuovoTokenRecupero() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, hashToken(token), nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
