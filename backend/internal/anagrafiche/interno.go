package anagrafiche

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"

	"lastmiledelivery/backend/internal/httpx"
	"lastmiledelivery/backend/internal/interno"
)

// Rotte interne del servizio utenti, chiamate solo dal servizio spedizioni (non passano
// dall'Ingress): danno i dati del database globale che servono alle spedizioni.

// Risolvi legge sedi, indirizzi e persone (per id del ruolo) in poche query.
func (r *Repository) Risolvi(ctx context.Context, ric interno.Richiesta) (interno.Risposta, error) {
	ris := interno.Risposta{
		Sedi:           map[int64]interno.Sede{},
		Indirizzi:      map[int64]interno.Indirizzo{},
		Clienti:        map[int64]interno.Persona{},
		Corrieri:       map[int64]interno.Persona{},
		Operatori:      map[int64]interno.Persona{},
		Amministratori: map[int64]interno.Persona{},
	}
	if len(ric.Sedi) > 0 {
		righe, err := r.db.Query(ctx, `
			SELECT se.id, se.nome, se.attiva, se.id_citta, se.gruppo_spedizioni, ci.nome, re.nome, na.nome
			FROM sedi se
			JOIN citta ci   ON ci.id = se.id_citta
			JOIN regioni re ON re.id = ci.id_regione
			JOIN nazioni na ON na.id = re.id_nazione
			WHERE se.id = ANY($1)`, ric.Sedi)
		if err != nil {
			return ris, err
		}
		var id int64
		var s interno.Sede
		if _, err := pgx.ForEachRow(righe, []any{&id, &s.Nome, &s.Attiva, &s.IDCitta, &s.Gruppo, &s.Citta, &s.Regione, &s.Nazione}, func() error {
			ris.Sedi[id] = s
			return nil
		}); err != nil {
			return ris, err
		}
	}
	if len(ric.Indirizzi) > 0 {
		righe, err := r.db.Query(ctx, selezioneIndirizzo+` WHERE i.id = ANY($1)`, ric.Indirizzi)
		if err != nil {
			return ris, err
		}
		indirizzi, err := pgx.CollectRows(righe, scansionaIndirizzo)
		if err != nil {
			return ris, err
		}
		for _, i := range indirizzi {
			ris.Indirizzi[i.ID] = interno.Indirizzo{Via: i.Via, Civico: i.Civico, CAP: i.CAP, IDCitta: i.IDCitta,
				Citta: i.Citta, Regione: i.Regione, Nazione: i.Nazione}
		}
	}
	persone := []struct {
		ids     []int64
		tabella string
		sede    string
		dove    map[int64]interno.Persona
	}{
		{ric.Clienti, "clienti", "0", ris.Clienti},
		{ric.Corrieri, "corrieri", "r.id_sede", ris.Corrieri},
		{ric.Operatori, "operatori_logistici", "r.id_sede", ris.Operatori},
		{ric.Amministratori, "amministratori", "0", ris.Amministratori},
	}
	for _, p := range persone {
		if len(p.ids) == 0 {
			continue
		}
		righe, err := r.db.Query(ctx, `
			SELECT r.id, u.nome, u.cognome, u.email, u.prefisso || ' ' || u.nr_telefono, u.attivo, `+p.sede+`
			FROM `+p.tabella+` r JOIN utenti u ON u.id = r.id_utente
			WHERE r.id = ANY($1)`, p.ids)
		if err != nil {
			return ris, err
		}
		var id int64
		var pe interno.Persona
		if _, err := pgx.ForEachRow(righe, []any{&id, &pe.Nome, &pe.Cognome, &pe.Email, &pe.Telefono, &pe.Attivo, &pe.IDSede},
			func() error {
				p.dove[id] = pe
				return nil
			}); err != nil {
			return ris, err
		}
	}
	return ris, nil
}

// SegnaDestinatario: il cliente ha ricevuto una spedizione, da adesso può entrare a
// seguirla. Un id inesistente non fa nulla (il servizio spedizioni l'ha già controllato).
func (r *Repository) SegnaDestinatario(ctx context.Context, idCliente int64) error {
	_, err := r.db.Exec(ctx, `UPDATE clienti SET destinatario = TRUE WHERE id = $1 AND NOT destinatario`, idCliente)
	return err
}

type HandlerInterno struct {
	repo *Repository
}

func NuovoHandlerInterno(repo *Repository) *HandlerInterno {
	return &HandlerInterno{repo: repo}
}

func (h *HandlerInterno) Registra(rotte *httpx.Rotte, protetto httpx.Middleware) {
	rotte.Gestisci(http.MethodPost, interno.PercorsoRisolvi, http.HandlerFunc(h.risolvi), protetto)
	rotte.Gestisci(http.MethodPost, interno.PercorsoDestinatario, http.HandlerFunc(h.segnaDestinatario), protetto)
}

func (h *HandlerInterno) risolvi(w http.ResponseWriter, r *http.Request) {
	var ric interno.Richiesta
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&ric); err != nil {
		httpx.ScriviErrore(w, http.StatusBadRequest, "richiesta non valida")
		return
	}
	ris, err := h.repo.Risolvi(r.Context(), ric)
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	httpx.ScriviJSON(w, http.StatusOK, ris)
}

func (h *HandlerInterno) segnaDestinatario(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.ScriviErrore(w, http.StatusBadRequest, "id non valido")
		return
	}
	if err := h.repo.SegnaDestinatario(r.Context(), id); err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
