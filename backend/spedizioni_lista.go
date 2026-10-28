package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var errUtenteSenzaRuolo = errors.New("utente senza ruolo")

var statiValidi = map[string]bool{
	"creata": true, "assegnata": true, "ritirata": true, "persa": true,
	"in consegna": true, "consegnata": true, "non consegnata": true, "annullata": true,
}

type IndirizzoSpedizione struct {
	Via    string `json:"via"`
	Civico string `json:"civico"`
}

type SpedizioneLista struct {
	ID                        int32               `json:"id"`
	CodOrdine                 string              `json:"cod_ordine"`
	Stato                     string              `json:"stato"`
	IsReso                    bool                `json:"is_reso"`
	CreataIl                  time.Time           `json:"creata_il"`
	DataOraRichiestaSpedizione time.Time          `json:"dataora_richiesta_spedizione"`
	IndirizzoRitiro           IndirizzoSpedizione `json:"indirizzo_ritiro"`
	IndirizzoSpedizione       IndirizzoSpedizione `json:"indirizzo_spedizione"`
	IDCorriere                *int32              `json:"id_corriere,omitempty"` // solo operatore/admin
	Note                      json.RawMessage     `json:"note"`
}

type FiltriSpedizioni struct {
	Stato  string
	Limit  int
	Offset int
}

func elencaSpedizioni(ctx context.Context, db *pgxpool.Pool, utente UtenteAutenticato, f FiltriSpedizioni) ([]SpedizioneLista, error) {
	args := []any{}
	addArg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}

	// Cosa vede ciascun ruolo
	where := "TRUE"
	filtroNote := "" // clausola extra sulle note
	mostraCorriere := true

	switch utente.Ruolo {
	case RuoloCorriere:
		where = "a.id_corriere = " + addArg(utente.IDRuolo) +
			" AND s.stato IN ('ritirata', 'in consegna')"
		mostraCorriere = false
	case RuoloCliente:
		where = "s.id_cliente_destinatario = " + addArg(utente.IDRuolo)
		filtroNote = "AND n.is_interna = FALSE" // mai le note interne
		mostraCorriere = false
	case RuoloOperatore, RuoloAmministratore:
		// tutte le spedizioni, in tutti gli stati
	default:
		return nil, errUtenteSenzaRuolo
	}

	if f.Stato != "" {
		where += " AND s.stato = " + addArg(f.Stato) + "::stato_spedizione"
	}

	query := fmt.Sprintf(`
		SELECT
			s.id, s.cod_ordine, s.stato::text, s.is_reso, s.creata_il,
			s.dataora_richiesta_spedizione,
			ir.via, ir.civico,
			ic.via, ic.civico,
			a.id_corriere,
			COALESCE((
				SELECT json_agg(json_build_object(
					'id', n.id,
					'nota', n.nota,
					'is_interna', n.is_interna,
					'motivo', n.motivo,
					'creata_il', n.creata_il
				) ORDER BY n.creata_il)
				FROM note n
				WHERE n.id_spedizione = s.id %s
			), '[]'::json)
		FROM spedizioni s
		JOIN indirizzi ir ON ir.id = s.id_indirizzo_ritiro
		JOIN indirizzi ic ON ic.id = s.id_indirizzo_spedizione
		LEFT JOIN assegnazioni a ON a.id = s.id_assegnazione_corrente
		WHERE %s
		ORDER BY s.creata_il DESC, s.id DESC
		LIMIT %s OFFSET %s
	`, filtroNote, where, addArg(f.Limit), addArg(f.Offset))

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	risultato := []SpedizioneLista{}

	for rows.Next() {
		var s SpedizioneLista
		var note []byte

		err := rows.Scan(
			&s.ID, &s.CodOrdine, &s.Stato, &s.IsReso, &s.CreataIl,
			&s.DataOraRichiestaSpedizione,
			&s.IndirizzoRitiro.Via, &s.IndirizzoRitiro.Civico,
			&s.IndirizzoSpedizione.Via, &s.IndirizzoSpedizione.Civico,
			&s.IDCorriere,
			&note,
		)
		if err != nil {
			return nil, err
		}

		s.Note = json.RawMessage(note)
		if !mostraCorriere {
			s.IDCorriere = nil
		}

		risultato = append(risultato, s)
	}

	return risultato, rows.Err()
}

// TEMPORANEO: finché non c'è l'autenticazione vera (lezione 10) l'utente
// arriva nell'header X-Utente-Email. Va sostituito con il token/sessione:
// il resto dell'endpoint non cambia, basta cambiare questa funzione.
func utenteDallaRichiesta(r *http.Request, db *pgxpool.Pool) (UtenteAutenticato, int, error) {
	email := r.Header.Get("X-Utente-Email")
	if email == "" {
		return UtenteAutenticato{}, http.StatusUnauthorized, errors.New("non autenticato")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	utente, err := trovaUtentePerEmail(ctx, db, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, errUtenteSenzaRuolo) {
			return UtenteAutenticato{}, http.StatusUnauthorized, errors.New("utente non valido")
		}
		log.Println("Recupero utente fallito:", err)
		return UtenteAutenticato{}, http.StatusInternalServerError, errors.New("errore interno")
	}

	return utente, 0, nil
}

func ottieniSpedizioni(w http.ResponseWriter, r *http.Request, db *pgxpool.Pool) {
	utente, status, err := utenteDallaRichiesta(r, db)
	if err != nil {
		http.Error(w, err.Error(), status)
		return
	}

	q := r.URL.Query()
	filtri := FiltriSpedizioni{Limit: 20, Offset: 0}

	if stato := q.Get("stato"); stato != "" {
		if !statiValidi[stato] {
			http.Error(w, "stato non valido", http.StatusBadRequest)
			return
		}
		filtri.Stato = stato
	}

	if v := q.Get("pageSize"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			http.Error(w, "pageSize deve essere tra 1 e 100", http.StatusBadRequest)
			return
		}
		filtri.Limit = n
	}

	if v := q.Get("page"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil || p < 1 {
			http.Error(w, "page deve essere >= 1", http.StatusBadRequest)
			return
		}
		filtri.Offset = (p - 1) * filtri.Limit
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	spedizioni, err := elencaSpedizioni(ctx, db, utente, filtri)
	if err != nil {
		log.Println("Elenco spedizioni fallito:", err)
		http.Error(w, "Impossibile recuperare le spedizioni", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"dati": spedizioni}); err != nil {
		log.Println("Errore durante l'invio della risposta:", err)
	}
}
