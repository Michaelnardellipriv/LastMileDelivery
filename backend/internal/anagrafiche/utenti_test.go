package anagrafiche

import (
	"context"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

func nuovoUtente(ruolo *string, email string) NuovoUtente {
	return NuovoUtente{Ruolo: ruolo, NuovoCliente: nuovoCliente(email, p("PasswordNuova1"))}
}

func TestCreaUtente(t *testing.T) {
	ctx := context.Background()
	a := &archivioFinto{emailUsate: map[string]bool{"usata@test.it": true}}
	postino := &postinoFinto{}
	s := NuovoServizio(a, postino)

	for _, ruolo := range []identita.Ruolo{identita.Corriere, identita.Operatore} {
		esito, err := s.CreaUtente(ctx, amministratore, nuovoUtente(p(string(ruolo)), "nuovo@test.it"))
		if err != nil || esito.Utente.Ruolo != ruolo || esito.Utente.Nome != "Dora" || !esito.MailInviata {
			t.Fatalf("l'amministratore crea un %s: %+v %v", ruolo, esito, err)
		}
		if bcrypt.CompareHashAndPassword([]byte(a.hash), []byte("PasswordNuova1")) != nil {
			t.Fatal("nel database va l'hash della password scelta dall'amministratore")
		}
		if postino.destinatario != "nuovo@test.it" || !strings.Contains(postino.testo, "PasswordNuova1") {
			t.Fatalf("la mail con le credenziali va al nuovo utente: %q", postino.testo)
		}
	}

	for _, r := range []identita.Richiedente{operatore, corriere, cliente} {
		if _, err := s.CreaUtente(ctx, r, nuovoUtente(p("corriere"), "x@test.it")); tipoErrore(err) != "403" {
			t.Errorf("%s non crea corrieri e operatori: %v", r.Ruolo, err)
		}
	}

	prima := a.creati
	for nome, c := range map[string]struct {
		n     NuovoUtente
		campo string
	}{
		"senza ruolo":          {nuovoUtente(nil, "x@test.it"), "ruolo"},
		"ruolo cliente":        {nuovoUtente(p("cliente"), "x@test.it"), "ruolo"},
		"ruolo amministratore": {nuovoUtente(p("amministratore"), "x@test.it"), "ruolo"},
		"email non valida":     {nuovoUtente(p("corriere"), "non-una-mail"), "email"},
		"senza password":       {NuovoUtente{Ruolo: p("corriere"), NuovoCliente: nuovoCliente("x@test.it", nil)}, "password"},
	} {
		_, err := s.CreaUtente(ctx, amministratore, c.n)
		if tipoErrore(err) != "400" || !campoSegnalato(err, c.campo) {
			t.Errorf("%s: atteso 400 sul campo %s, ottenuto %v", nome, c.campo, err)
		}
	}
	if _, err := s.CreaUtente(ctx, amministratore, nuovoUtente(p("operatore"), "Usata@test.it")); tipoErrore(err) != "409" {
		t.Errorf("email già usata: atteso 409, ottenuto %v", err)
	}
	if a.creati != prima {
		t.Fatal("con dati non validi non si crea nessun utente")
	}
}

func TestCercaUtenti(t *testing.T) {
	ctx := context.Background()
	a := &archivioFinto{}
	s := NuovoServizio(a, &postinoFinto{})
	ric := Ricerca{Pagina: 1, PerPagina: 20}

	if _, _, err := s.CercaUtenti(ctx, amministratore, ric, p("corriere"), p(false)); err != nil ||
		a.utenti.Ruolo == nil || *a.utenti.Ruolo != identita.Corriere || a.utenti.Attivo == nil || *a.utenti.Attivo {
		t.Fatalf("filtri per ruolo e stato: %+v %v", a.utenti, err)
	}
	if _, _, err := s.CercaUtenti(ctx, amministratore, ric, p("cliente"), nil); tipoErrore(err) != "400" {
		t.Errorf("si cercano solo corrieri e operatori: %v", err)
	}
	for _, r := range []identita.Richiedente{operatore, corriere, cliente} {
		if _, _, err := s.CercaUtenti(ctx, r, ric, nil, nil); tipoErrore(err) != "403" {
			t.Errorf("%s non consulta gli utenti: %v", r.Ruolo, err)
		}
	}
}

func TestAbilitaDisattivaUtente(t *testing.T) {
	ctx := context.Background()
	a := &archivioFinto{}
	s := NuovoServizio(a, &postinoFinto{})

	for _, attivo := range []bool{false, true} {
		u, err := s.ModificaUtente(ctx, amministratore, 7, ModificaUtente{Attivo: p(attivo)})
		if err != nil || u.ID != 7 || u.Attivo != attivo || *a.modifica.Attivo != attivo || a.modifica.Nome != nil {
			t.Fatalf("attivo=%v: %+v %+v %v", attivo, u, a.modifica, err)
		}
	}
	for _, r := range []identita.Richiedente{operatore, corriere, cliente} {
		if _, err := s.ModificaUtente(ctx, r, 7, ModificaUtente{Attivo: p(false)}); tipoErrore(err) != "403" {
			t.Errorf("%s non disattiva utenti: %v", r.Ruolo, err)
		}
	}
}

func TestModificaDatiUtente(t *testing.T) {
	ctx := context.Background()
	a := &archivioFinto{
		emailUsate:  map[string]bool{"altro@test.it": true, "carlo@test.it": true},
		proprietari: map[string]int32{"altro@test.it": 8, "carlo@test.it": 7},
	}
	postino := &postinoFinto{}
	s := NuovoServizio(a, postino)

	u, err := s.ModificaUtente(ctx, amministratore, 7, ModificaUtente{Nome: p("  Carla "), NrTelefono: p("3339998877")})
	if err != nil || u.Nome != "Carla" || *a.modifica.Nome != "Carla" || *a.modifica.NrTelefono != "3339998877" ||
		a.modifica.Cognome != nil || a.modifica.Email != nil || a.modifica.PwdHash != nil || a.modifica.Attivo != nil {
		t.Fatalf("si cambiano solo i campi indicati, ripuliti dagli spazi: %+v %+v %v", u, a.modifica, err)
	}
	if postino.destinatario != "" {
		t.Fatal("senza nuova password non parte nessuna mail")
	}

	// la propria email, anche con maiuscole diverse, non è "già usata"
	if _, err := s.ModificaUtente(ctx, amministratore, 7, ModificaUtente{Email: p("Carlo@Test.it")}); err != nil {
		t.Fatalf("l'utente tiene la sua email: %v", err)
	}
	if _, err := s.ModificaUtente(ctx, amministratore, 7, ModificaUtente{Email: p("ALTRO@test.it")}); tipoErrore(err) != "409" {
		t.Fatalf("email di un altro utente: atteso 409, ottenuto %v", err)
	}

	if _, err := s.ModificaUtente(ctx, amministratore, 7, ModificaUtente{Password: p("NuovaPassword9")}); err != nil {
		t.Fatal(err)
	}
	if a.modifica.PwdHash == nil || bcrypt.CompareHashAndPassword([]byte(*a.modifica.PwdHash), []byte("NuovaPassword9")) != nil {
		t.Fatal("nel database va l'hash della nuova password")
	}
	if postino.destinatario != "carlo@test.it" || !strings.Contains(postino.testo, "NuovaPassword9") {
		t.Fatalf("la nuova password arriva per mail all'utente: %q %q", postino.destinatario, postino.testo)
	}

	for nome, c := range map[string]struct {
		m     ModificaUtente
		campo string
	}{
		"corpo vuoto":      {ModificaUtente{}, "corpo"},
		"nome vuoto":       {ModificaUtente{Nome: p("  ")}, "nome"},
		"prefisso errato":  {ModificaUtente{Prefisso: p("39")}, "prefisso"},
		"telefono errato":  {ModificaUtente{NrTelefono: p("12 34")}, "nr_telefono"},
		"email non valida": {ModificaUtente{Email: p("non-una-mail")}, "email"},
		"password corta":   {ModificaUtente{Password: p("corta")}, "password"},
	} {
		a.modifica = nil
		_, err := s.ModificaUtente(ctx, amministratore, 7, c.m)
		if tipoErrore(err) != "400" || !campoSegnalato(err, c.campo) || a.modifica != nil {
			t.Errorf("%s: atteso 400 sul campo %s senza salvare, ottenuto %v", nome, c.campo, err)
		}
	}
}

func campoSegnalato(err error, campo string) bool {
	v, ok := err.(*errori.Validazione)
	return ok && v.Segnalato(campo)
}
