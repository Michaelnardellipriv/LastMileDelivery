package spedizioni

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/httpx"
	"lastmiledelivery/backend/internal/identita"
	"lastmiledelivery/backend/internal/indirizzo"
	"lastmiledelivery/backend/internal/interno"
)

// Indirizzi di ritiro e consegna (GET e POST /api/indirizzi). Stanno nel database del
// gruppo della sede, sempre nella città della sede: città, regione e nazione sono quelle
// della sede (database globale, /interno/risolvi), qui c'è l'id della città.

const (
	indirizziPerPaginaPredefinito = 20
	indirizziPerPaginaMassimo     = 100
)

// IndirizzoSede è un indirizzo di ritiro o consegna di una sede.
type IndirizzoSede struct {
	ID      int64
	IDSede  int64
	IDCitta int64
	Via     string
	Civico  string
	CAP     string
	// della sede
	Citta, Regione, Nazione string
}

type RicercaIndirizzi struct {
	Testo     string
	IDSede    *int64
	Pagina    int
	PerPagina int
}

// archivioIndirizzi: Repository.
type archivioIndirizzi interface {
	Sede(ctx context.Context, idSede int64) (interno.Sede, bool, error)
	CercaIndirizzi(ctx context.Context, idSede int64, citta, testo string, pagina, perPagina int) ([]IndirizzoSede, int, error)
	TrovaOCreaIndirizzo(ctx context.Context, idSede int64, sede interno.Sede, d indirizzo.Dati) (IndirizzoSede, bool, error)
}

type ServizioIndirizzi struct {
	archivio archivioIndirizzi
}

func NuovoServizioIndirizzi(a archivioIndirizzi) *ServizioIndirizzi {
	return &ServizioIndirizzi{archivio: a}
}

var errSedeNonTrovata = &errori.NonTrovato{Messaggio: "sede non trovata"}

// Cerca: l'operatore vede gli indirizzi della sua sede (gli unici che può usare per
// ritiro e consegna); l'amministratore quelli della sede che sceglie (id_sede
// obbligatorio: ogni sede ha i suoi nel database del suo gruppo).
func (s *ServizioIndirizzi) Cerca(ctx context.Context, r identita.Richiedente, ric RicercaIndirizzi) ([]IndirizzoSede, int, error) {
	e := &errori.Validazione{}
	var idSede int64
	switch r.Ruolo {
	case identita.Operatore:
		if ric.IDSede != nil && *ric.IDSede != r.IDSede {
			return nil, 0, &errori.NonAutorizzato{Messaggio: "il filtro per sede è per gli amministratori: si vede solo la propria sede"}
		}
		idSede = r.IDSede
	case identita.Amministratore:
		switch {
		case ric.IDSede == nil:
			e.Aggiungi("id_sede", "obbligatorio per l'amministratore: scegliere una sede")
		case *ric.IDSede <= 0:
			e.Aggiungi("id_sede", "deve essere un id positivo")
		default:
			idSede = *ric.IDSede
		}
	default:
		return nil, 0, &errori.NonAutorizzato{Messaggio: "clienti e indirizzi sono consultabili solo da operatori e amministratori"}
	}
	if ric.Pagina < 1 {
		e.Aggiungi("pagina", "deve essere almeno 1")
	} else if ric.Pagina > PaginaMassima {
		e.Aggiungi("pagina", "al massimo "+strconv.Itoa(PaginaMassima))
	}
	if ric.PerPagina < 1 || ric.PerPagina > indirizziPerPaginaMassimo {
		e.Aggiungi("per_pagina", "deve essere tra 1 e 100")
	}
	if err := e.Err(); err != nil {
		return nil, 0, err
	}
	sede, ok, err := s.archivio.Sede(ctx, idSede)
	switch {
	case err != nil:
		return nil, 0, err
	case !ok:
		return nil, 0, errSedeNonTrovata
	}
	trovati, totale, err := s.archivio.CercaIndirizzi(ctx, idSede, sede.Citta, ric.Testo, ric.Pagina, ric.PerPagina)
	for i := range trovati {
		trovati[i].Citta, trovati[i].Regione, trovati[i].Nazione = sede.Citta, sede.Regione, sede.Nazione
	}
	return trovati, totale, err
}

// TrovaOCrea: l'operatore inserisce un indirizzo nella città della sua sede; se c'è già
// (stessa via e civico, senza badare alle maiuscole) restituisce quello (creato = false).
func (s *ServizioIndirizzi) TrovaOCrea(ctx context.Context, r identita.Richiedente, n indirizzo.Nuovo) (IndirizzoSede, bool, error) {
	if r.Ruolo != identita.Operatore {
		return IndirizzoSede{}, false, &errori.NonAutorizzato{Messaggio: "solo un operatore logistico può inserire un indirizzo"}
	}
	e := &errori.Validazione{}
	d := indirizzo.Valida(e, "", n)
	if err := e.Err(); err != nil {
		return IndirizzoSede{}, false, err
	}
	sede, ok, err := s.archivio.Sede(ctx, r.IDSede)
	switch {
	case err != nil:
		return IndirizzoSede{}, false, err
	case !ok:
		return IndirizzoSede{}, false, &errori.NonAutorizzato{Messaggio: "l'operatore non ha una sede valida"}
	}
	// ritiro e consegna restano nella città della sede: un indirizzo altrove non servirebbe
	luogoSede := indirizzo.Dati{Citta: sede.Citta, Regione: sede.Regione, Nazione: sede.Nazione}
	if !indirizzo.StessoLuogo(luogoSede, d) {
		e.Aggiungi("citta", "la sede lavora a "+sede.Citta+" ("+sede.Regione+", "+sede.Nazione+
			"): ritiro e consegna restano in questa città")
		return IndirizzoSede{}, false, e
	}
	i, creato, err := s.archivio.TrovaOCreaIndirizzo(ctx, r.IDSede, sede, d)
	i.Citta, i.Regione, i.Nazione = sede.Citta, sede.Regione, sede.Nazione
	return i, creato, err
}

// ===== Repository =====

// Sede: nome, città e gruppo della sede (servizio utenti).
func (r *Repository) Sede(ctx context.Context, idSede int64) (interno.Sede, bool, error) {
	ris, err := r.anagrafe.Risolvi(ctx, interno.Richiesta{Sedi: []int64{idSede}})
	if err != nil {
		return interno.Sede{}, false, err
	}
	s, ok := ris.Sedi[idSede]
	return s, ok, nil
}

// CercaIndirizzi cerca per via, civico, città o CAP tra gli indirizzi della sede, nel
// database del suo gruppo (citta è il nome della città della sede, per la ricerca).
func (r *Repository) CercaIndirizzi(ctx context.Context, idSede int64, citta, testo string, pagina, perPagina int) ([]IndirizzoSede, int, error) {
	db, err := r.db.PerSede(ctx, idSede)
	if err != nil {
		return nil, 0, err
	}
	const filtro = `
		FROM indirizzi i
		WHERE i.id_sede = $1
		  AND ($2 = '' OR position(lower($2) in lower(i.via || ' ' || i.civico || ' ' || $3 || ' ' || i.cod_postale)) > 0)`
	var totale int
	if err := db.QueryRow(ctx, `SELECT count(*) `+filtro, idSede, testo, citta).Scan(&totale); err != nil {
		return nil, 0, err
	}
	righe, err := db.Query(ctx, `
		SELECT i.id, i.id_sede, i.id_citta, i.via, i.civico, i.cod_postale `+filtro+`
		ORDER BY i.via, i.civico, i.id
		LIMIT $4 OFFSET $5`, idSede, testo, citta, perPagina, (pagina-1)*perPagina)
	if err != nil {
		return nil, 0, err
	}
	trovati, err := pgx.CollectRows(righe, scansionaIndirizzoSede)
	return trovati, totale, err
}

func scansionaIndirizzoSede(riga pgx.CollectableRow) (IndirizzoSede, error) {
	var i IndirizzoSede
	err := riga.Scan(&i.ID, &i.IDSede, &i.IDCitta, &i.Via, &i.Civico, &i.CAP)
	return i, err
}

// TrovaOCreaIndirizzo: l'indirizzo della sede con quella via e civico, creato se manca.
// L'indice unico (sede, via, civico) mette in fila due inserimenti contemporanei.
func (r *Repository) TrovaOCreaIndirizzo(ctx context.Context, idSede int64, sede interno.Sede, d indirizzo.Dati) (IndirizzoSede, bool, error) {
	db, err := r.db.PerSede(ctx, idSede)
	if err != nil {
		return IndirizzoSede{}, false, err
	}
	righe, err := db.Query(ctx, `
		INSERT INTO indirizzi (id_sede, id_citta, via, civico, cod_postale) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id_sede, lower(via), lower(civico)) DO NOTHING
		RETURNING id, id_sede, id_citta, via, civico, cod_postale`,
		idSede, sede.IDCitta, d.Via, d.Civico, d.CAP)
	if err != nil {
		return IndirizzoSede{}, false, err
	}
	creato, err := pgx.CollectExactlyOneRow(righe, scansionaIndirizzoSede)
	if err == nil {
		return creato, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return IndirizzoSede{}, false, err
	}
	righe, err = db.Query(ctx, `
		SELECT id, id_sede, id_citta, via, civico, cod_postale FROM indirizzi
		WHERE id_sede = $1 AND lower(via) = lower($2) AND lower(civico) = lower($3)`, idSede, d.Via, d.Civico)
	if err != nil {
		return IndirizzoSede{}, false, err
	}
	esistente, err := pgx.CollectExactlyOneRow(righe, scansionaIndirizzoSede)
	return esistente, false, err
}

// ===== HTTP =====

type HandlerIndirizzi struct {
	servizio *ServizioIndirizzi
}

func NuovoHandlerIndirizzi(s *ServizioIndirizzi) *HandlerIndirizzi {
	return &HandlerIndirizzi{servizio: s}
}

func (h *HandlerIndirizzi) Registra(rotte *httpx.Rotte, identifica, idempotente httpx.Middleware) {
	rotte.Gestisci(http.MethodGet, "/api/indirizzi", http.HandlerFunc(h.cerca), identifica)
	rotte.Gestisci(http.MethodPost, "/api/indirizzi", http.HandlerFunc(h.trovaOCrea), identifica, idempotente)
}

type indirizzoSedeDTO struct {
	ID      int64  `json:"id"`
	IDCitta int64  `json:"id_citta"`
	Via     string `json:"via"`
	Civico  string `json:"civico"`
	Citta   string `json:"citta"`
	CAP     string `json:"cap"`
	Regione string `json:"regione"`
	Nazione string `json:"nazione"`
}

func versoIndirizzoSedeDTO(i IndirizzoSede) indirizzoSedeDTO {
	return indirizzoSedeDTO{ID: i.ID, IDCitta: i.IDCitta, Via: i.Via, Civico: i.Civico, Citta: i.Citta, CAP: i.CAP,
		Regione: i.Regione, Nazione: i.Nazione}
}

type paginaIndirizzi struct {
	Dati           []indirizzoSedeDTO `json:"dati"`
	Pagina         int                `json:"pagina"`
	PerPagina      int                `json:"per_pagina"`
	TotaleElementi int                `json:"totale_elementi"`
	TotalePagine   int                `json:"totale_pagine"`
}

type richiestaIndirizzo struct {
	Via     *string `json:"via"`
	Civico  *string `json:"civico"`
	Citta   *string `json:"citta"`
	CAP     *string `json:"cap"`
	Regione *string `json:"regione"`
	Nazione *string `json:"nazione"`
}

func (h *HandlerIndirizzi) cerca(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ric := RicercaIndirizzi{Testo: strings.TrimSpace(q.Get("q")), Pagina: 1, PerPagina: indirizziPerPaginaPredefinito}
	e := &errori.Validazione{}
	for _, c := range []struct {
		nome string
		dest *int
	}{{"pagina", &ric.Pagina}, {"per_pagina", &ric.PerPagina}} {
		if v := q.Get(c.nome); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				e.Aggiungi(c.nome, "deve essere un numero intero")
				continue
			}
			*c.dest = n
		}
	}
	if v := q.Get("id_sede"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			e.Aggiungi("id_sede", "deve essere un numero intero")
		} else {
			ric.IDSede = &n
		}
	}
	if err := e.Err(); err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	trovati, totale, err := h.servizio.Cerca(r.Context(), identita.DalContesto(r.Context()), ric)
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	risposta := paginaIndirizzi{Dati: make([]indirizzoSedeDTO, len(trovati)), Pagina: ric.Pagina, PerPagina: ric.PerPagina,
		TotaleElementi: totale}
	if totale > 0 {
		risposta.TotalePagine = (totale + ric.PerPagina - 1) / ric.PerPagina
	}
	for i, t := range trovati {
		risposta.Dati[i] = versoIndirizzoSedeDTO(t)
	}
	httpx.ScriviJSON(w, http.StatusOK, risposta)
}

func (h *HandlerIndirizzi) trovaOCrea(w http.ResponseWriter, r *http.Request) {
	var req richiestaIndirizzo
	if !httpx.LeggiCorpo(w, r, &req) {
		return
	}
	i, creato, err := h.servizio.TrovaOCrea(r.Context(), identita.DalContesto(r.Context()), indirizzo.Nuovo{
		Via: req.Via, Civico: req.Civico, Citta: req.Citta, CAP: req.CAP, Regione: req.Regione, Nazione: req.Nazione,
	})
	if err != nil {
		httpx.RispondiErrore(w, r, err)
		return
	}
	stato := http.StatusOK
	if creato {
		stato = http.StatusCreated
	}
	httpx.ScriviJSON(w, stato, versoIndirizzoSedeDTO(i))
}
