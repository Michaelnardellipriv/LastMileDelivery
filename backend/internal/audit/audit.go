// Package audit scrive nel log le operazioni significative per la sicurezza: accessi
// riusciti e falliti, accessi negati, sessioni chiuse, cambi di stato, di sede e di
// password. Una riga per evento, con chi, cosa e su cosa:
//
//	audit utente=operatore:3 azione=spedizione_stato spedizione=42 da=assegnata a=ritirata
//
// Mai password, token, segreti o dati personali che non servono a ricostruire l'azione:
// chi chiama passa solo id e nomi di campi, non valori.
package audit

import (
	"context"
	"fmt"
	"log"
	"strings"
)

type chiave struct{}

// ConUtente mette nel contesto chi sta facendo la richiesta ("ruolo:id"): lo fa il
// middleware di identità, così ogni evento della richiesta lo riporta.
func ConUtente(ctx context.Context, utente string) context.Context {
	return context.WithValue(ctx, chiave{}, utente)
}

func utente(ctx context.Context) string {
	if u, ok := ctx.Value(chiave{}).(string); ok && u != "" {
		return u
	}
	return "-"
}

// Evento scrive una riga di audit. campi sono coppie nome, valore.
func Evento(ctx context.Context, azione string, campi ...any) {
	var b strings.Builder
	b.WriteString("audit utente=")
	b.WriteString(utente(ctx))
	b.WriteString(" azione=")
	b.WriteString(azione)
	for i := 0; i+1 < len(campi); i += 2 {
		fmt.Fprintf(&b, " %v=%s", campi[i], pulisci(fmt.Sprint(campi[i+1])))
	}
	log.Print(b.String())
}

// pulisci: niente a capo né spazi nei valori, così una riga resta una riga (un valore
// scelto da chi chiama non può fingere un altro evento).
func pulisci(v string) string {
	v = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return '_'
		}
		return r
	}, v)
	if len(v) > 80 {
		v = v[:80]
	}
	return v
}
