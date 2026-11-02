package spedizioni

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lastmiledelivery/backend/internal/database"
	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

var errNonTrovata = &errori.NonTrovato{Messaggio: "spedizione non trovata"}

type Repository struct {
	db *pgxpool.Pool
}

func NuovoRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

type interrogabile interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (r *Repository) Crea(ctx context.Context, idOperatore int32, n NuovaSpedizione) (Spedizione, error) {
	var id int32
	err := r.db.QueryRow(ctx, `
		INSERT INTO spedizioni (
			cod_ordine, id_operatore_ha_creato,
			id_cliente_mittente, id_cliente_destinatario,
			dataora_richiesta_spedizione, id_indirizzo_ritiro, id_indirizzo_spedizione
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`,
		n.CodOrdine, idOperatore,
		n.IDClienteMittente, n.IDClienteDestinatario,
		n.DataoraRichiesta, n.IDIndirizzoRitiro, n.IDIndirizzoSpedizione,
	).Scan(&id)
	if err != nil {
		return Spedizione{}, traduciErrore(err)
	}
	return perID(ctx, r.db, id)
}

func (r *Repository) Aggiorna(ctx context.Context, id int32, autore identita.Richiedente,
	decidi func(attuale Spedizione) (Modifica, error)) (Spedizione, bool, error) {

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Spedizione{}, false, err
	}
	defer tx.Rollback(ctx)

	var bloccata int32
	err = tx.QueryRow(ctx, `SELECT id FROM spedizioni WHERE id = $1 FOR UPDATE`, id).Scan(&bloccata)
	if errors.Is(err, pgx.ErrNoRows) {
		return Spedizione{}, false, errNonTrovata
	}
	if err != nil {
		return Spedizione{}, false, err
	}

	attuale, err := perID(ctx, tx, id)
	if err != nil {
		return Spedizione{}, false, err
	}
	m, err := decidi(attuale)
	if err != nil {
		return Spedizione{}, false, err
	}
	if m.NessunaModifica {
		return attuale, false, nil
	}

	chiave := "app.id_operatore"
	if autore.Ruolo == identita.Corriere {
		chiave = "app.id_corriere"
	}
	if _, err := tx.Exec(ctx, `SELECT set_config($1, $2, true)`, chiave, strconv.Itoa(int(autore.ID))); err != nil {
		return Spedizione{}, false, err
	}

	var idNota *int32
	if m.Nota != nil {
		idOperatore, idCorriere := autoreNota(autore)
		if err := tx.QueryRow(ctx, `
			INSERT INTO note (nota, is_interna, motivo, id_operatore_logistico_creata, id_corriere_creata, id_spedizione)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id`,
			m.Nota.Testo, !m.Nota.Pubblica, m.Nota.Motivo, idOperatore, idCorriere, id,
		).Scan(&idNota); err != nil {
			return Spedizione{}, false, err
		}
	}

	if m.Tentativo != "" {
		if _, err := tx.Exec(ctx,
			`INSERT INTO tentativi (tipo, id_spedizione, id_nota) VALUES ($1, $2, $3)`,
			string(m.Tentativo), id, idNota); err != nil {
			return Spedizione{}, false, err
		}
	}

	var idAssegnazione *int32
	if m.NuovoCorriere != nil {
		err := tx.QueryRow(ctx,
			`INSERT INTO assegnazioni (id_operatore, id_corriere) VALUES ($1, $2) RETURNING id`,
			autore.ID, *m.NuovoCorriere,
		).Scan(&idAssegnazione)
		if err != nil {
			return Spedizione{}, false, traduciErrore(err)
		}
	}

	d := m.Dati
	if d == nil {
		d = &DatiSpedizione{}
	}
	_, err = tx.Exec(ctx, `
		UPDATE spedizioni SET
			stato                        = $2,
			id_assegnazione_corrente     = COALESCE($3, id_assegnazione_corrente),
			id_operatore_ha_annullato    = CASE WHEN $4 THEN $5::integer ELSE id_operatore_ha_annullato END,
			annullata_il                 = CASE WHEN $4 THEN now() ELSE annullata_il END,
			is_reso                      = COALESCE($6, is_reso),
			id_cliente_mittente          = COALESCE($7, id_cliente_mittente),
			id_cliente_destinatario      = COALESCE($8, id_cliente_destinatario),
			id_indirizzo_ritiro          = COALESCE($9, id_indirizzo_ritiro),
			id_indirizzo_spedizione      = COALESCE($10, id_indirizzo_spedizione),
			dataora_richiesta_spedizione = COALESCE($11, dataora_richiesta_spedizione)
		WHERE id = $1`,
		id, string(m.NuovoStato), idAssegnazione, m.Annulla, autore.ID,
		d.IsReso, d.IDClienteMittente, d.IDClienteDestinatario,
		d.IDIndirizzoRitiro, d.IDIndirizzoSpedizione, d.DataoraRichiesta)
	if err != nil {
		return Spedizione{}, false, traduciErrore(err)
	}

	aggiornata, err := perID(ctx, tx, id)
	if err != nil {
		return Spedizione{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Spedizione{}, false, err
	}
	return aggiornata, true, nil
}

func autoreNota(autore identita.Richiedente) (idOperatore, idCorriere *int32) {
	id := autore.ID
	if autore.Ruolo == identita.Corriere {
		return nil, &id
	}
	return &id, nil
}

func (r *Repository) AggiungiNota(ctx context.Context, id int32, autore identita.Richiedente, testo string,
	verifica func(attuale Spedizione) error) (Nota, error) {

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Nota{}, err
	}
	defer tx.Rollback(ctx)

	var bloccata int32
	err = tx.QueryRow(ctx, `SELECT id FROM spedizioni WHERE id = $1 FOR SHARE`, id).Scan(&bloccata)
	if errors.Is(err, pgx.ErrNoRows) {
		return Nota{}, errNonTrovata
	}
	if err != nil {
		return Nota{}, err
	}
	attuale, err := perID(ctx, tx, id)
	if err != nil {
		return Nota{}, err
	}
	if err := verifica(attuale); err != nil {
		return Nota{}, err
	}

	n := Nota{Testo: testo, IsInterna: true}
	n.IDOperatore, n.IDCorriere = autoreNota(autore)
	if err := tx.QueryRow(ctx, `
		INSERT INTO note (nota, is_interna, id_operatore_logistico_creata, id_corriere_creata, id_spedizione)
		VALUES ($1, true, $2, $3, $4)
		RETURNING id, creata_il`,
		testo, n.IDOperatore, n.IDCorriere, id,
	).Scan(&n.ID, &n.CreataIl); err != nil {
		return Nota{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Nota{}, err
	}
	return n, nil
}

func perID(ctx context.Context, q interrogabile, id int32) (Spedizione, error) {
	var s Spedizione
	var stato string
	var ritiri, consegne, uscite int64
	err := q.QueryRow(ctx, `
		SELECT s.id, s.cod_ordine, s.stato::text, s.is_reso, s.creata_il,
		       s.dataora_richiesta_spedizione,
		       s.id_cliente_mittente, s.id_cliente_destinatario,
		       s.id_indirizzo_ritiro, s.id_indirizzo_spedizione,
		       a.id_corriere, s.annullata_il,
		       (SELECT count(*) FROM tentativi t WHERE t.id_spedizione = s.id AND t.tipo = 'ritiro'),
		       (SELECT count(*) FROM tentativi t WHERE t.id_spedizione = s.id AND t.tipo = 'consegna'),
		       (SELECT count(*) FROM cambiamenti_spedizioni c
		         WHERE c.id_spedizione = s.id AND c.valore_nuovo ->> 'stato' = 'in consegna')
		FROM spedizioni s
		LEFT JOIN assegnazioni a ON a.id = s.id_assegnazione_corrente
		WHERE s.id = $1`, id,
	).Scan(&s.ID, &s.CodOrdine, &stato, &s.IsReso, &s.CreataIl,
		&s.DataoraRichiesta,
		&s.IDClienteMittente, &s.IDClienteDestinatario,
		&s.IDIndirizzoRitiro, &s.IDIndirizzoSpedizione,
		&s.IDCorriere, &s.AnnullataIl, &ritiri, &consegne, &uscite)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, errNonTrovata
	}
	s.Stato = Stato(stato)
	s.TentativiRitiroFalliti = int(ritiri)
	s.TentativiConsegnaFalliti = int(consegne)
	s.UsciteInConsegna = int(uscite)
	return s, err
}

var riferimenti = map[string]errori.RiferimentoNonTrovato{
	"spedizioni_id_cliente_mittente_fkey":     {Campo: "id_cliente_mittente", Messaggio: "cliente non trovato"},
	"spedizioni_id_cliente_destinatario_fkey": {Campo: "id_cliente_destinatario", Messaggio: "cliente non trovato"},
	"spedizioni_id_indirizzo_ritiro_fkey":     {Campo: "id_indirizzo_ritiro", Messaggio: "indirizzo non trovato"},
	"spedizioni_id_indirizzo_spedizione_fkey": {Campo: "id_indirizzo_spedizione", Messaggio: "indirizzo non trovato"},
	"assegnazioni_id_corriere_fkey":           {Campo: "id_corriere", Messaggio: "corriere non trovato"},
}

func traduciErrore(err error) error {
	codice, vincolo, ok := database.ErroreVincolo(err)
	if !ok {
		return err
	}
	switch {
	case codice == database.CodiceUnicita && vincolo == "uq_spedizioni_cod_ordine":
		return &errori.Conflitto{
			Messaggio: "esiste già una spedizione con questo codice ordine",
			Dettagli:  []errori.ErroreCampo{{Campo: "cod_ordine", Messaggio: "già usato"}},
		}
	case codice == database.CodiceChiaveEsterna:
		if rif, ok := riferimenti[vincolo]; ok {
			return &rif
		}
	}
	return err
}

func (r *Repository) Elenca(ctx context.Context, v Visibilita, f FiltroElenco) ([]VoceElenco, error) {
	return r.elenca(ctx, v, f, nil)
}

func (r *Repository) Dettaglio(ctx context.Context, v Visibilita, id int32) (DettaglioSpedizione, error) {
	voci, err := r.elenca(ctx, v, FiltroElenco{Pagina: 1, PerPagina: 1}, &id)
	if err != nil {
		return DettaglioSpedizione{}, err
	}
	if len(voci) == 0 {
		return DettaglioSpedizione{}, errNonTrovata
	}
	s, err := perID(ctx, r.db, id)
	if err != nil {
		return DettaglioSpedizione{}, err
	}
	return DettaglioSpedizione{
		VoceElenco:               voci[0],
		AnnullataIl:              s.AnnullataIl,
		TentativiRitiroFalliti:   s.TentativiRitiroFalliti,
		TentativiConsegnaFalliti: s.TentativiConsegnaFalliti,
	}, nil
}

func (r *Repository) elenca(ctx context.Context, v Visibilita, f FiltroElenco, id *int32) ([]VoceElenco, error) {
	var args []any
	arg := func(x any) string {
		args = append(args, x)
		return "$" + strconv.Itoa(len(args))
	}

	condizioni := []string{"TRUE"}
	if id != nil {
		condizioni = append(condizioni, "s.id = "+arg(*id))
	}
	if v.IDCorriere != nil {
		condizioni = append(condizioni, "a.id_corriere = "+arg(*v.IDCorriere))
	}
	if len(v.Stati) > 0 {
		nomi := make([]string, len(v.Stati))
		for i, st := range v.Stati {
			nomi[i] = string(st)
		}
		condizioni = append(condizioni, "s.stato::text = ANY("+arg(nomi)+")")
	}
	if v.IDDestinatario != nil {
		condizioni = append(condizioni, "s.id_cliente_destinatario = "+arg(*v.IDDestinatario))
	}
	if v.ConcluseDal != nil {
		condizioni = append(condizioni, "(ch.conclusa_il IS NULL OR ch.conclusa_il >= "+arg(*v.ConcluseDal)+")")
	}
	if f.Stato != nil {
		condizioni = append(condizioni, "s.stato::text = "+arg(string(*f.Stato)))
	}
	if f.IDCorriere != nil {
		condizioni = append(condizioni, "a.id_corriere = "+arg(*f.IDCorriere))
	}
	if f.Dal != nil {
		condizioni = append(condizioni, "(s.creata_il AT TIME ZONE 'Europe/Rome')::date >= "+arg(f.Dal.Format(time.DateOnly))+"::date")
	}
	if f.Al != nil {
		condizioni = append(condizioni, "(s.creata_il AT TIME ZONE 'Europe/Rome')::date <= "+arg(f.Al.Format(time.DateOnly))+"::date")
	}
	if f.CodOrdine != nil {
		condizioni = append(condizioni, "position(lower("+arg(*f.CodOrdine)+") in lower(s.cod_ordine)) > 0")
	}
	filtroNote := ""
	if v.SoloNotePubbliche {
		filtroNote = "AND NOT n.is_interna"
	}

	query := fmt.Sprintf(`
		SELECT s.id, s.cod_ordine, s.stato::text, s.is_reso, s.creata_il,
		       s.dataora_richiesta_spedizione,
		       s.id_cliente_mittente, s.id_cliente_destinatario,
		       ir.via, ir.civico, isp.via, isp.civico,
		       a.id_corriere, ch.conclusa_il,
		       COALESCE((
		           SELECT json_agg(json_build_object(
		                      'id', n.id, 'nota', n.nota, 'is_interna', n.is_interna,
		                      'motivo', n.motivo, 'creata_il', n.creata_il,
		                      'id_operatore', n.id_operatore_logistico_creata, 'id_corriere', n.id_corriere_creata
		                  ) ORDER BY n.creata_il, n.id)
		           FROM note n
		           WHERE n.id_spedizione = s.id %s
		       ), '[]'::json)
		FROM spedizioni s
		JOIN indirizzi ir  ON ir.id  = s.id_indirizzo_ritiro
		JOIN indirizzi isp ON isp.id = s.id_indirizzo_spedizione
		LEFT JOIN assegnazioni a ON a.id = s.id_assegnazione_corrente
		LEFT JOIN LATERAL (
		    SELECT max(c.modificato_il) AS conclusa_il
		    FROM cambiamenti_spedizioni c
		    WHERE c.id_spedizione = s.id AND c.valore_nuovo ->> 'stato' = s.stato::text
		) ch ON s.stato IN ('consegnata', 'non consegnata', 'persa', 'annullata')
		WHERE %s
		ORDER BY s.creata_il DESC, s.id DESC
		LIMIT %s OFFSET %s`,
		filtroNote, strings.Join(condizioni, " AND "), arg(f.PerPagina), arg((f.Pagina-1)*f.PerPagina))

	righe, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer righe.Close()

	voci := []VoceElenco{}
	for righe.Next() {
		var voce VoceElenco
		var stato string
		var note []byte
		if err := righe.Scan(&voce.ID, &voce.CodOrdine, &stato, &voce.IsReso, &voce.CreataIl,
			&voce.DataoraRichiesta,
			&voce.IDClienteMittente, &voce.IDClienteDestinatario,
			&voce.IndirizzoRitiro.Via, &voce.IndirizzoRitiro.Civico,
			&voce.IndirizzoSpedizione.Via, &voce.IndirizzoSpedizione.Civico,
			&voce.IDCorriere, &voce.ConclusaIl, &note); err != nil {
			return nil, err
		}
		voce.Stato = Stato(stato)
		if voce.Note, err = leggiNote(note); err != nil {
			return nil, err
		}
		voci = append(voci, voce)
	}
	return voci, righe.Err()
}

func leggiNote(dati []byte) ([]Nota, error) {
	var grezze []struct {
		ID          int32     `json:"id"`
		Nota        string    `json:"nota"`
		IsInterna   bool      `json:"is_interna"`
		Motivo      *string   `json:"motivo"`
		CreataIl    time.Time `json:"creata_il"`
		IDOperatore *int32    `json:"id_operatore"`
		IDCorriere  *int32    `json:"id_corriere"`
	}
	if err := json.Unmarshal(dati, &grezze); err != nil {
		return nil, err
	}
	note := make([]Nota, len(grezze))
	for i, g := range grezze {
		note[i] = Nota{ID: g.ID, Testo: g.Nota, IsInterna: g.IsInterna, Motivo: g.Motivo, CreataIl: g.CreataIl,
			IDOperatore: g.IDOperatore, IDCorriere: g.IDCorriere}
	}
	return note, nil
}

func (r *Repository) Storico(ctx context.Context, id int32) ([]VoceStorico, error) {
	var esiste bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM spedizioni WHERE id = $1)`, id).Scan(&esiste); err != nil {
		return nil, err
	}
	if !esiste {
		return nil, errNonTrovata
	}

	righe, err := r.db.Query(ctx, `
		SELECT id, operazione::text, valore_vecchio, valore_nuovo, modificato_il, id_operatore, id_corriere
		FROM cambiamenti_spedizioni
		WHERE id_spedizione = $1
		ORDER BY modificato_il, id`, id)
	if err != nil {
		return nil, err
	}
	defer righe.Close()

	voci := []VoceStorico{}
	for righe.Next() {
		var v VoceStorico
		if err := righe.Scan(&v.ID, &v.Operazione, &v.ValoreVecchio, &v.ValoreNuovo,
			&v.ModificatoIl, &v.IDOperatore, &v.IDCorriere); err != nil {
			return nil, err
		}
		voci = append(voci, v)
	}
	return voci, righe.Err()
}

var _ archivio = (*Repository)(nil)
