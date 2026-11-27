package interno

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

// Client chiama le rotte /interno/* dell'altro servizio. Lo stesso tipo serve a tutti e
// due: il servizio spedizioni lo punta al servizio utenti e viceversa.
type Client struct {
	nome   string // per i messaggi: "utenti" o "spedizioni"
	base   string
	chiave string
	http   *http.Client
}

// NuovoClient: base è l'indirizzo dell'altro servizio (es. http://utenti-svc:8080).
// Con trasporto nil si usa la rete; TrasportoLocale serve quando i due servizi girano
// nello stesso processo (sviluppo).
func NuovoClient(nome, base, chiave string, trasporto http.RoundTripper) *Client {
	return &Client{
		nome:   nome,
		base:   strings.TrimRight(base, "/"),
		chiave: chiave,
		http:   &http.Client{Timeout: 5 * time.Second, Transport: trasporto},
	}
}

func (c *Client) chiama(ctx context.Context, metodo, percorso string, corpo, risposta any) error {
	var dati io.Reader
	if corpo != nil {
		b, err := json.Marshal(corpo)
		if err != nil {
			return err
		}
		dati = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, metodo, c.base+percorso, dati)
	if err != nil {
		return err
	}
	req.Header.Set(IntestazioneChiave, c.chiave)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return &errori.ServizioNonDisponibile{Servizio: c.nome, Causa: err}
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusServiceUnavailable:
		return &errori.ServizioNonDisponibile{Servizio: c.nome, Causa: fmt.Errorf("risposta %d", res.StatusCode)}
	case res.StatusCode < 200 || res.StatusCode > 299:
		testo, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("servizio %s: %s %s: risposta %d: %s", c.nome, metodo, percorso, res.StatusCode, testo)
	case risposta == nil:
		return nil
	}
	return json.NewDecoder(res.Body).Decode(risposta)
}

// ===== Servizio utenti =====

func (c *Client) Risolvi(ctx context.Context, r Richiesta) (Risposta, error) {
	var ris Risposta
	if r.Vuota() {
		return ris, nil
	}
	err := c.chiama(ctx, http.MethodPost, PercorsoRisolvi, r, &ris)
	return ris, err
}

// SegnaDestinatario: il cliente riceve una spedizione. Il servizio utenti se lo segna
// (clienti.destinatario), perché da quel momento il cliente può entrare a seguirla.
func (c *Client) SegnaDestinatario(ctx context.Context, idCliente int64) error {
	percorso := strings.Replace(PercorsoDestinatario, "{id}", strconv.FormatInt(idCliente, 10), 1)
	return c.chiama(ctx, http.MethodPost, percorso, nil, nil)
}

// Stato dice se l'utente del token esiste ancora, se è attivo e in quale sede lavora
// adesso: è il controllo che il middleware di identità fa a ogni richiesta.
func (c *Client) Stato(ctx context.Context, ruolo identita.Ruolo, id int64) (esiste, attivo bool, idSede int64, err error) {
	var r Richiesta
	ids := []int64{id}
	switch ruolo {
	case identita.Cliente:
		r.Clienti = ids
	case identita.Corriere:
		r.Corrieri = ids
	case identita.Operatore:
		r.Operatori = ids
	case identita.Amministratore:
		r.Amministratori = ids
	default:
		return false, false, 0, nil
	}
	ris, err := c.Risolvi(ctx, r)
	if err != nil {
		return false, false, 0, err
	}
	var trovate map[int64]Persona
	switch ruolo {
	case identita.Cliente:
		trovate = ris.Clienti
	case identita.Corriere:
		trovate = ris.Corrieri
	case identita.Operatore:
		trovate = ris.Operatori
	default:
		trovate = ris.Amministratori
	}
	p, ok := trovate[id]
	return ok, p.Attivo, p.IDSede, nil
}

// GruppoDellaSede: il gruppo di sedi (e quindi il database delle spedizioni) della sede.
// Lo usa la mappa sede → database del servizio spedizioni (database.Gruppi).
func (c *Client) GruppoDellaSede(ctx context.Context, idSede int64) (int, bool, error) {
	ris, err := c.Risolvi(ctx, Richiesta{Sedi: []int64{idSede}})
	if err != nil {
		return 0, false, err
	}
	s, ok := ris.Sedi[idSede]
	switch {
	case !ok:
		return 0, false, nil
	case s.Gruppo == nil:
		return 0, false, fmt.Errorf("servizio utenti: la sede %d è senza gruppo (versione del servizio utenti precedente alla mappa delle sedi?)", idSede)
	}
	return *s.Gruppo, true, nil
}

// ===== Servizio spedizioni =====

// Gruppi: i gruppi di sedi che hanno un database delle spedizioni.
func (c *Client) Gruppi(ctx context.Context) ([]int, error) {
	var r Gruppi
	err := c.chiama(ctx, http.MethodGet, PercorsoGruppi, nil, &r)
	return r.Codici, err
}

// CorriereHaSpedizioniAperte: un corriere con spedizioni assegnate, ritirate o in
// consegna non si sposta in un'altra sede.
func (c *Client) CorriereHaSpedizioniAperte(ctx context.Context, idCorriere int64) (bool, error) {
	percorso := strings.Replace(PercorsoSpedizioniAperte, "{id}", strconv.FormatInt(idCorriere, 10), 1)
	var r SpedizioniAperte
	err := c.chiama(ctx, http.MethodGet, percorso, nil, &r)
	return r.Aperte, err
}

// RevocaSessioni comunica al servizio spedizioni le sessioni appena chiuse: da quel
// momento i loro token lì valgono 401 (403 se l'utente è stato disattivato).
func (c *Client) RevocaSessioni(ctx context.Context, sessioni []SessioneRevocata) error {
	if len(sessioni) == 0 {
		return nil
	}
	return c.chiama(ctx, http.MethodPost, PercorsoSessioniRevocate, RichiestaRevoca{Sessioni: sessioni}, nil)
}

// TrasportoLocale manda la richiesta direttamente all'handler dell'altro servizio,
// senza rete: serve quando i due servizi girano nello stesso processo (cmd/server).
func TrasportoLocale(h http.Handler) http.RoundTripper {
	return trasportoLocale{h}
}

type trasportoLocale struct{ h http.Handler }

func (t trasportoLocale) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	t.h.ServeHTTP(rec, req)
	return rec.Result(), nil
}
