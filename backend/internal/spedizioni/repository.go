package spedizioni

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lastmiledelivery/backend/internal/database"
	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
	"lastmiledelivery/backend/internal/interno"
)

var errNonTrovata = &errori.NonTrovato{Messaggio: "spedizione non trovata"}

// anagrafe dà i dati del database globale (sedi, clienti, corrieri, operatori,
// indirizzi): li ha il servizio utenti, che si chiama con le rotte /interno.
type anagrafe interface {
	Risolvi(ctx context.Context, r interno.Richiesta) (interno.Risposta, error)
	SegnaDestinatario(ctx context.Context, idCliente int64) error
}

type Repository struct {
	db       *database.Gruppi
	anagrafe anagrafe
}

// NuovoRepository: db sono i database delle spedizioni, uno per gruppo di sedi (mappa
// sede → database); sedi, persone e indirizzi si chiedono al servizio utenti (a).
//
// Ogni operazione lavora nel database giusto:
//   - una spedizione nuova in quello della sede dell'operatore;
//   - una spedizione esistente in quello del gruppo del suo id (database.GruppoDellID);
//   - un elenco in quello della sede (operatori, corrieri, amministratore che ha scelto
//     la sede); solo l'elenco di un cliente, che riceve da sedi diverse, li legge tutti.
func NuovoRepository(db *database.Gruppi, a anagrafe) *Repository {
	return &Repository{db: db, anagrafe: a}
}

// perID: il database della spedizione con quell'id; un id di un gruppo che non c'è è una
// spedizione che non esiste.
func (r *Repository) perID(ctx context.Context, id int64) (*pgxpool.Pool, error) {
	db, ok, err := r.db.PerID(ctx, id)
	switch {
	case err != nil:
		return nil, err
	case !ok:
		return nil, errNonTrovata
	}
	return db, nil
}

type interrogabile interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// parametri raccoglie gli argomenti di una query costruita a pezzi ($1, $2, ...).
type parametri struct{ args []any }

func (p *parametri) arg(x any) string {
	p.args = append(p.args, x)
	return "$" + strconv.Itoa(len(p.args))
}

// condizioneSede limita la query alle sedi dell'ambito: è il repository a metterla in
// ogni query, così chi chiama non può dimenticarla. Un ambito vuoto non vede nulla.
func condizioneSede(p *parametri, a database.Ambito, colonna string) string {
	switch {
	case a.TutteLeSedi:
		return "TRUE"
	case a.IDSede > 0:
		return colonna + " = " + p.arg(a.IDSede)
	}
	return "FALSE"
}

// ===== Controlli sul database globale =====

// riferimenti sono i dati di una spedizione che stanno nel database globale e vanno
// controllati prima di scrivere (prima li controllavano le chiavi esterne).
type riferimenti struct {
	idSede int64
	// spedizione nuova: la sede deve essere attiva
	sedeAttiva bool
	mittente   *int64
	// destinatario nuovo (spedizione nuova o destinatario cambiato): deve essere attivo
	destinatario *int64
	ritiro       *int64
	consegna     *int64
	corriere     *int64
}

const messaggioIndirizzoCitta = "indirizzo non trovato o non nella città della sede: ritiro e consegna restano nella città della sede"

// controlla chiede al servizio utenti sede, clienti e corriere in una sola chiamata e
// applica le regole che prima erano chiavi esterne:
//   - la sede esiste (e per una spedizione nuova è attiva);
//   - un destinatario nuovo è attivo;
//   - i clienti esistono;
//   - il corriere esiste, è attivo e lavora nella sede.
//
// Gli indirizzi invece sono in questo database: devono essere della sede (cioè della sua
// città), e lo garantisce anche la chiave esterna; il controllo qui dà l'errore sul campo.
//
// Restituisce la sede, che serve a Crea per la città.
func (r *Repository) controlla(ctx context.Context, rif riferimenti) (interno.Sede, error) {
	ric := interno.Richiesta{Sedi: []int64{rif.idSede}}
	aggiungi := func(lista *[]int64, id *int64) {
		if id != nil {
			*lista = append(*lista, *id)
		}
	}
	aggiungi(&ric.Clienti, rif.mittente)
	aggiungi(&ric.Clienti, rif.destinatario)
	aggiungi(&ric.Corrieri, rif.corriere)
	ris, err := r.anagrafe.Risolvi(ctx, ric)
	if err != nil {
		return interno.Sede{}, err
	}

	sede, ok := ris.Sedi[rif.idSede]
	switch {
	case !ok:
		return sede, &errori.NonAutorizzato{Messaggio: "l'operatore non ha una sede valida"}
	case rif.sedeAttiva && !sede.Attiva:
		return sede, &errori.Conflitto{Messaggio: "la sede è disattivata: non accetta nuove spedizioni"}
	}
	if rif.destinatario != nil {
		if d, ok := ris.Clienti[*rif.destinatario]; ok && !d.Attivo {
			return sede, &errori.Conflitto{
				Messaggio: "il cliente destinatario è disattivato: non può ricevere nuove spedizioni",
				Dettagli:  []errori.ErroreCampo{{Campo: "id_cliente_destinatario", Messaggio: "cliente disattivato"}},
			}
		}
	}
	clienti := []struct {
		id    *int64
		campo string
	}{{rif.mittente, "id_cliente_mittente"}, {rif.destinatario, "id_cliente_destinatario"}}
	for _, c := range clienti {
		if c.id == nil {
			continue
		}
		if _, ok := ris.Clienti[*c.id]; !ok {
			return sede, &errori.RiferimentoNonTrovato{Campo: c.campo, Messaggio: "cliente non trovato"}
		}
	}
	if err := r.controllaIndirizzi(ctx, rif); err != nil {
		return sede, err
	}
	if rif.corriere != nil {
		c, ok := ris.Corrieri[*rif.corriere]
		switch {
		case !ok:
			return sede, &errori.RiferimentoNonTrovato{Campo: "id_corriere", Messaggio: "corriere non trovato"}
		case !c.Attivo || c.IDSede != rif.idSede:
			// un corriere disattivato dall'amministratore non riceve nuove spedizioni
			return sede, &errori.Conflitto{
				Messaggio: "il corriere è disattivato o lavora in un'altra sede: sceglierne un altro",
				Dettagli:  []errori.ErroreCampo{{Campo: "id_corriere", Messaggio: "corriere non assegnabile"}},
			}
		}
	}
	return sede, nil
}

// controllaIndirizzi: ritiro e consegna sono indirizzi della sede della spedizione.
func (r *Repository) controllaIndirizzi(ctx context.Context, rif riferimenti) error {
	indirizzi := []struct {
		id    *int64
		campo string
	}{{rif.ritiro, "id_indirizzo_ritiro"}, {rif.consegna, "id_indirizzo_spedizione"}}
	var ids []int64
	for _, i := range indirizzi {
		if i.id != nil {
			ids = append(ids, *i.id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	db, err := r.db.PerSede(ctx, rif.idSede)
	if err != nil {
		return err
	}
	righe, err := db.Query(ctx, `SELECT id FROM indirizzi WHERE id = ANY($1) AND id_sede = $2`, ids, rif.idSede)
	if err != nil {
		return err
	}
	trovati, err := pgx.CollectRows(righe, pgx.RowTo[int64])
	if err != nil {
		return err
	}
	for _, i := range indirizzi {
		if i.id != nil && !slices.Contains(trovati, *i.id) {
			return &errori.RiferimentoNonTrovato{Campo: i.campo, Messaggio: messaggioIndirizzoCitta}
		}
	}
	return nil
}

func (r *Repository) SedeCorriere(ctx context.Context, idCorriere int64) (int64, bool, error) {
	ris, err := r.anagrafe.Risolvi(ctx, interno.Richiesta{Corrieri: []int64{idCorriere}})
	if err != nil {
		return 0, false, err
	}
	c, ok := ris.Corrieri[idCorriere]
	return c.IDSede, ok, nil
}

// ===== Scritture =====

func (r *Repository) Crea(ctx context.Context, idSede, idOperatore int64, n NuovaSpedizione) (Spedizione, error) {
	// la città della spedizione è quella della sede: indirizzi di ritiro o di consegna in
	// un'altra città vengono rifiutati
	sede, err := r.controlla(ctx, riferimenti{
		idSede: idSede, sedeAttiva: true,
		mittente: &n.IDClienteMittente, destinatario: &n.IDClienteDestinatario,
		ritiro: &n.IDIndirizzoRitiro, consegna: &n.IDIndirizzoSpedizione,
		corriere: n.IDCorriere,
	})
	if err != nil {
		return Spedizione{}, err
	}

	db, err := r.db.PerSede(ctx, idSede)
	if err != nil {
		return Spedizione{}, err
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return Spedizione{}, err
	}
	defer tx.Rollback(ctx)

	var id int64
	err = tx.QueryRow(ctx, `
		INSERT INTO spedizioni (
			id_sede, id_citta, cod_ordine, id_operatore_ha_creato,
			id_cliente_mittente, id_cliente_destinatario,
			dataora_richiesta_spedizione, id_indirizzo_ritiro, id_indirizzo_spedizione
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`,
		idSede, sede.IDCitta, n.CodOrdine, idOperatore,
		n.IDClienteMittente, n.IDClienteDestinatario,
		n.DataoraRichiesta, n.IDIndirizzoRitiro, n.IDIndirizzoSpedizione,
	).Scan(&id)
	if err != nil {
		return Spedizione{}, traduciErrore(err)
	}

	if n.IDCorriere != nil {
		if _, err := tx.Exec(ctx, `SELECT set_config('app.id_operatore', $1, true)`, strconv.FormatInt(idOperatore, 10)); err != nil {
			return Spedizione{}, err
		}
		idAssegnazione, err := nuovaAssegnazione(ctx, tx, idSede, idOperatore, *n.IDCorriere)
		if err != nil {
			return Spedizione{}, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE spedizioni SET stato = 'assegnata', id_assegnazione_corrente = $2 WHERE id = $1`,
			id, idAssegnazione); err != nil {
			return Spedizione{}, traduciErrore(err)
		}
	}

	creata, err := perID(ctx, tx, id)
	if err != nil {
		return Spedizione{}, err
	}
	// il destinatario da adesso può entrare a seguire la spedizione: lo si segna nel
	// database globale prima del commit (se il commit fallisse resterebbe solo segnato,
	// senza danni: vedrebbe un elenco vuoto)
	if err := r.anagrafe.SegnaDestinatario(ctx, n.IDClienteDestinatario); err != nil {
		return Spedizione{}, err
	}
	return creata, tx.Commit(ctx)
}

func (r *Repository) Aggiorna(ctx context.Context, ambito database.Ambito, id int64, autore identita.Richiedente,
	decidi func(attuale Spedizione) (Modifica, error)) (Spedizione, bool, error) {

	db, err := r.perID(ctx, id)
	if err != nil {
		return Spedizione{}, false, err
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return Spedizione{}, false, err
	}
	defer tx.Rollback(ctx)

	if err := blocca(ctx, tx, ambito, id, "FOR UPDATE"); err != nil {
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

	d := m.Dati
	if d == nil {
		d = &DatiSpedizione{}
	}
	// solo un destinatario nuovo: quello che la spedizione ha già resta anche se disattivato
	var nuovoDestinatario *int64
	if d.IDClienteDestinatario != nil && *d.IDClienteDestinatario != attuale.IDClienteDestinatario {
		nuovoDestinatario = d.IDClienteDestinatario
	}
	if m.NuovoCorriere != nil || d.altriCampi() {
		if _, err := r.controlla(ctx, riferimenti{
			idSede: attuale.IDSede, mittente: d.IDClienteMittente, destinatario: nuovoDestinatario,
			ritiro: d.IDIndirizzoRitiro, consegna: d.IDIndirizzoSpedizione, corriere: m.NuovoCorriere,
		}); err != nil {
			return Spedizione{}, false, err
		}
	}

	chiave := "app.id_operatore"
	if autore.Ruolo == identita.Corriere {
		chiave = "app.id_corriere"
	}
	if _, err := tx.Exec(ctx, `SELECT set_config($1, $2, true)`, chiave, strconv.FormatInt(autore.ID, 10)); err != nil {
		return Spedizione{}, false, err
	}

	// note, tentativi e assegnazioni hanno la sede della spedizione
	var idNota *int64
	if m.Nota != nil {
		idOperatore, idCorriere := autoreNota(autore)
		if err := tx.QueryRow(ctx, `
			INSERT INTO note (id_sede, nota, is_interna, motivo, id_operatore_logistico_creata, id_corriere_creata, id_spedizione)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id`,
			attuale.IDSede, m.Nota.Testo, !m.Nota.Pubblica, m.Nota.Motivo, idOperatore, idCorriere, id,
		).Scan(&idNota); err != nil {
			return Spedizione{}, false, err
		}
	}

	if m.Tentativo != "" {
		if _, err := tx.Exec(ctx,
			`INSERT INTO tentativi (id_sede, tipo, id_spedizione, id_nota) VALUES ($1, $2, $3, $4)`,
			attuale.IDSede, string(m.Tentativo), id, idNota); err != nil {
			return Spedizione{}, false, err
		}
	}

	var idAssegnazione *int64
	if m.NuovoCorriere != nil {
		nuova, err := nuovaAssegnazione(ctx, tx, attuale.IDSede, autore.ID, *m.NuovoCorriere)
		if err != nil {
			return Spedizione{}, false, err
		}
		idAssegnazione = &nuova
	}

	_, err = tx.Exec(ctx, `
		UPDATE spedizioni SET
			stato                        = $2,
			id_assegnazione_corrente     = COALESCE($3, id_assegnazione_corrente),
			id_operatore_ha_annullato    = CASE WHEN $4 THEN $5::bigint ELSE id_operatore_ha_annullato END,
			annullata_il                 = CASE WHEN $4 THEN now() ELSE annullata_il END,
			is_reso                      = COALESCE($6, is_reso),
			id_cliente_mittente          = COALESCE($7, id_cliente_mittente),
			id_cliente_destinatario      = COALESCE($8, id_cliente_destinatario),
			id_indirizzo_ritiro          = COALESCE($9, id_indirizzo_ritiro),
			id_indirizzo_spedizione      = COALESCE($10, id_indirizzo_spedizione),
			dataora_richiesta_spedizione = COALESCE($11, dataora_richiesta_spedizione),
			-- data e ora del ritiro: le registra il sistema al primo passaggio in 'ritirata'
			ritirata_il                  = CASE WHEN $2::stato_spedizione = 'ritirata' THEN COALESCE(ritirata_il, now())
			                                    ELSE ritirata_il END
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
	if nuovoDestinatario != nil {
		if err := r.anagrafe.SegnaDestinatario(ctx, *nuovoDestinatario); err != nil {
			return Spedizione{}, false, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Spedizione{}, false, err
	}
	return aggiornata, true, nil
}

// blocca prende il lock sulla spedizione se è nelle sedi dell'ambito: una spedizione di
// un'altra sede risulta "non trovata", come se non esistesse.
func blocca(ctx context.Context, tx pgx.Tx, ambito database.Ambito, id int64, lock string) error {
	p := &parametri{}
	dove := "id = " + p.arg(id) + " AND " + condizioneSede(p, ambito, "id_sede")
	var bloccata int64
	err := tx.QueryRow(ctx, `SELECT id FROM spedizioni WHERE `+dove+` `+lock, p.args...).Scan(&bloccata)
	if errors.Is(err, pgx.ErrNoRows) {
		return errNonTrovata
	}
	return err
}

// nuovaAssegnazione registra l'assegnazione: che il corriere esista, sia attivo e lavori
// nella sede della spedizione l'ha già controllato controlla (database globale).
func nuovaAssegnazione(ctx context.Context, tx pgx.Tx, idSede, idOperatore, idCorriere int64) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, `
		INSERT INTO assegnazioni (id_sede, id_operatore, id_corriere)
		VALUES ($1, $2, $3)
		RETURNING id`, idSede, idOperatore, idCorriere).Scan(&id)
	return id, traduciErrore(err)
}

func autoreNota(autore identita.Richiedente) (idOperatore, idCorriere *int64) {
	id := autore.ID
	if autore.Ruolo == identita.Corriere {
		return nil, &id
	}
	return &id, nil
}

func (r *Repository) AggiungiNota(ctx context.Context, ambito database.Ambito, id int64, autore identita.Richiedente, testo string,
	verifica func(attuale Spedizione) error) (Nota, error) {

	db, err := r.perID(ctx, id)
	if err != nil {
		return Nota{}, err
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return Nota{}, err
	}
	defer tx.Rollback(ctx)

	if err := blocca(ctx, tx, ambito, id, "FOR SHARE"); err != nil {
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
		INSERT INTO note (id_sede, nota, is_interna, id_operatore_logistico_creata, id_corriere_creata, id_spedizione)
		VALUES ($1, $2, true, $3, $4, $5)
		RETURNING id, creata_il`,
		attuale.IDSede, testo, n.IDOperatore, n.IDCorriere, id,
	).Scan(&n.ID, &n.CreataIl); err != nil {
		return Nota{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Nota{}, err
	}
	// nome e cognome dell'autore sono nel database globale
	nomi := nuoviNomi()
	nomi.autore(n.IDOperatore, n.IDCorriere)
	if err := nomi.risolvi(ctx, r.anagrafe); err != nil {
		return Nota{}, err
	}
	n.NomeAutore, n.CognomeAutore = nomi.nomeAutore(n.IDOperatore, n.IDCorriere)
	return n, nil
}

// perID legge la spedizione senza filtrare per sede: si usa solo dentro una transazione
// in cui blocca (o Crea) ha già verificato la sede.
func perID(ctx context.Context, q interrogabile, id int64) (Spedizione, error) {
	var s Spedizione
	var stato string
	var ritiri, consegne int64
	var tentativoInUscita bool
	err := q.QueryRow(ctx, `
		SELECT s.id, s.id_sede, s.cod_ordine, s.stato::text, s.is_reso, s.creata_il,
		       s.dataora_richiesta_spedizione, s.ritirata_il,
		       s.id_cliente_mittente, s.id_cliente_destinatario,
		       s.id_indirizzo_ritiro, s.id_indirizzo_spedizione,
		       a.id_corriere, s.annullata_il,
		       (SELECT count(*) FROM tentativi t WHERE t.id_spedizione = s.id AND t.tipo = 'ritiro'),
		       (SELECT count(*) FROM tentativi t WHERE t.id_spedizione = s.id AND t.tipo = 'consegna'),
		       -- tentativo di consegna fallito dopo l'ultimo passaggio a "in consegna"
		       EXISTS (SELECT 1 FROM tentativi t
		               WHERE t.id_spedizione = s.id AND t.tipo = 'consegna'
		                 AND t.effettuato_il >= (SELECT max(c.modificato_il) FROM cambiamenti_spedizioni c
		                                         WHERE c.id_spedizione = s.id AND c.valore_nuovo ->> 'stato' = 'in consegna'))
		FROM spedizioni s
		LEFT JOIN assegnazioni a ON a.id = s.id_assegnazione_corrente
		WHERE s.id = $1`, id,
	).Scan(&s.ID, &s.IDSede, &s.CodOrdine, &stato, &s.IsReso, &s.CreataIl,
		&s.DataoraRichiesta, &s.RitirataIl,
		&s.IDClienteMittente, &s.IDClienteDestinatario,
		&s.IDIndirizzoRitiro, &s.IDIndirizzoSpedizione,
		&s.IDCorriere, &s.AnnullataIl, &ritiri, &consegne, &tentativoInUscita)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, errNonTrovata
	}
	s.Stato = Stato(stato)
	s.TentativiRitiroFalliti = int(ritiri)
	s.TentativiConsegnaFalliti = int(consegne)
	s.TentativoInUscita = tentativoInUscita
	return s, err
}

func traduciErrore(err error) error {
	codice, vincolo, ok := database.ErroreVincolo(err)
	if ok && codice == database.CodiceUnicita && vincolo == "uq_spedizioni_cod_ordine" {
		return &errori.Conflitto{
			Messaggio: "esiste già una spedizione con questo codice ordine",
			Dettagli:  []errori.ErroreCampo{{Campo: "cod_ordine", Messaggio: "già usato"}},
		}
	}
	return err
}

// CorriereHaSpedizioniAperte: spedizioni assegnate, ritirate o in consegna del corriere,
// in qualunque sede (e quindi in ogni gruppo: un corriere già spostato può averne nel
// database della sede di prima). La chiede il servizio utenti prima di spostarlo.
func (r *Repository) CorriereHaSpedizioniAperte(ctx context.Context, idCorriere int64) (bool, error) {
	ctx = database.ConAmbito(ctx, database.TutteLeSedi())
	tutti, err := r.db.Tutti(ctx)
	if err != nil {
		return false, err
	}
	for _, db := range tutti {
		var aperte bool
		if err := db.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM spedizioni s
				JOIN assegnazioni a ON a.id = s.id_assegnazione_corrente
				WHERE a.id_corriere = $1 AND s.stato IN ('assegnata', 'ritirata', 'in consegna')
			)`, idCorriere).Scan(&aperte); err != nil || aperte {
			return aperte, err
		}
	}
	return false, nil
}

// RegistraSessioniRevocate salva le sessioni chiuse dal servizio utenti e toglie quelle
// finite da più di un giorno: un token non dura mai oltre la fine della sua sessione
// (autenticazione: la scadenza del token è limitata a quella della sessione), quindi dopo
// non varrebbe comunque. Il giorno di margine copre gli orologi non allineati.
//
// Ogni sessione va nel database che controlla il suo token (identita.ControlloRevoche):
// quello del gruppo della sede del login per operatori e corrieri, il principale per
// clienti e amministratori. Così un gruppo che non risponde blocca solo le revoche dei suoi
// utenti, non quelle di tutti. Se un database non risponde si restituisce l'errore e il
// servizio utenti annulla l'operazione; quelle già registrate altrove restano, senza danni
// (ON CONFLICT DO NOTHING al nuovo tentativo).
func (r *Repository) RegistraSessioniRevocate(ctx context.Context, sessioni []interno.SessioneRevocata) error {
	perDatabase := map[*pgxpool.Pool][]interno.SessioneRevocata{}
	var ordine []*pgxpool.Pool
	aggiungi := func(db *pgxpool.Pool, s interno.SessioneRevocata) {
		if _, ok := perDatabase[db]; !ok {
			ordine = append(ordine, db)
		}
		perDatabase[db] = append(perDatabase[db], s)
	}
	for _, s := range sessioni {
		switch {
		case !identita.Ruolo(s.Ruolo).HaSede():
			aggiungi(r.db.Principale(), s)
		case s.IDSede > 0:
			db, err := r.db.PerSede(ctx, s.IDSede)
			if err != nil {
				return err
			}
			aggiungi(db, s)
		default:
			// sessione di operatore o corriere senza sede (aperta prima della mappa): tutti
			tutti, err := r.db.Tutti(ctx)
			if err != nil {
				return err
			}
			for _, db := range tutti {
				aggiungi(db, s)
			}
		}
	}
	for _, db := range ordine {
		if err := registraRevocate(ctx, db, perDatabase[db]); err != nil {
			return err
		}
	}
	return nil
}

// Gruppi: i gruppi che hanno un database delle spedizioni (rotta interna).
func (r *Repository) Gruppi() []int { return r.db.Codici() }

func registraRevocate(ctx context.Context, db *pgxpool.Pool, sessioni []interno.SessioneRevocata) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, s := range sessioni {
		if _, err := tx.Exec(ctx, `
			INSERT INTO sessioni_revocate (id, scade_il, motivo) VALUES ($1, $2, $3)
			ON CONFLICT (id) DO NOTHING`, s.ID, s.ScadeIl, s.Motivo); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM sessioni_revocate WHERE scade_il < now() - interval '1 day'`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ===== Letture =====

func (r *Repository) Elenca(ctx context.Context, ambito database.Ambito, v Visibilita, f FiltroElenco) ([]VoceElenco, error) {
	return r.elenca(ctx, ambito, v, f, nil)
}

func (r *Repository) Dettaglio(ctx context.Context, ambito database.Ambito, v Visibilita, id int64) (DettaglioSpedizione, error) {
	voci, err := r.elenca(ctx, ambito, v, FiltroElenco{Pagina: 1, PerPagina: 1}, &id)
	if err != nil {
		return DettaglioSpedizione{}, err
	}
	if len(voci) == 0 {
		return DettaglioSpedizione{}, errNonTrovata
	}
	// la spedizione è nell'ambito (l'ha appena trovata elenca)
	db, err := r.perID(ctx, id)
	if err != nil {
		return DettaglioSpedizione{}, err
	}
	s, err := perID(ctx, db, id)
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

// Join che servono alle condizioni dell'elenco (corriere assegnato, data di chiusura,
// giorni di visibilità della sede).
const joinCondizioniElenco = `
		LEFT JOIN assegnazioni a ON a.id = s.id_assegnazione_corrente
		LEFT JOIN impostazioni imp ON imp.id_sede = s.id_sede
		LEFT JOIN LATERAL (
		    SELECT max(c.modificato_il) AS conclusa_il
		    FROM cambiamenti_spedizioni c
		    WHERE c.id_spedizione = s.id AND c.valore_nuovo ->> 'stato' = s.stato::text
		) ch ON s.stato IN ('consegnata', 'non consegnata', 'persa', 'annullata')`

// Conta le spedizioni visibili che rispettano i filtri, su tutte le pagine (per un
// cliente, la somma dei database di tutti i gruppi).
func (r *Repository) Conta(ctx context.Context, ambito database.Ambito, v Visibilita, f FiltroElenco) (int, error) {
	db, err := r.perElenco(ctx, ambito, f, nil)
	if err != nil {
		return 0, err
	}
	p := &parametri{}
	dove := condizioniElenco(p, ambito, v, f, nil)
	totale := 0
	for _, d := range db {
		var n int
		if err := d.QueryRow(ctx, `SELECT count(*) FROM spedizioni s`+joinCondizioniElenco+` WHERE `+dove, p.args...).Scan(&n); err != nil {
			return 0, err
		}
		totale += n
	}
	return totale, nil
}

// perElenco: i database da leggere per un elenco.
//   - una spedizione (dettaglio): quello del gruppo del suo id;
//   - operatori e corrieri (ambito di una sede) e l'amministratore, che sceglie sempre la
//     sede (filtro id_sede): quello della sede;
//   - un cliente, che non ha una sede e riceve spedizioni da sedi diverse: tutti.
func (r *Repository) perElenco(ctx context.Context, ambito database.Ambito, f FiltroElenco, id *int64) ([]*pgxpool.Pool, error) {
	switch {
	case id != nil:
		db, ok, err := r.db.PerID(ctx, *id)
		if err != nil || !ok {
			return nil, err // nessun database: nessuna spedizione con quell'id
		}
		return []*pgxpool.Pool{db}, nil
	case !ambito.TutteLeSedi && ambito.IDSede > 0:
		db, err := r.db.PerSede(ctx, ambito.IDSede)
		return []*pgxpool.Pool{db}, err
	case f.IDSede != nil:
		db, err := r.db.PerSede(ctx, *f.IDSede)
		return []*pgxpool.Pool{db}, err
	case ambito.TutteLeSedi:
		return r.db.Tutti(ctx)
	}
	return nil, nil // ambito vuoto: non vede nulla
}

func condizioniElenco(p *parametri, ambito database.Ambito, v Visibilita, f FiltroElenco, id *int64) string {
	arg := p.arg
	condizioni := []string{condizioneSede(p, ambito, "s.id_sede")}
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
	if v.ConcluseEntroGiorni {
		// ogni sede ha i suoi giorni di visibilità
		condizioni = append(condizioni, "(ch.conclusa_il IS NULL OR ch.conclusa_il >= "+arg(v.Adesso)+
			"::timestamptz - make_interval(days => COALESCE(imp.giorni_visibilita_cliente, 30)))")
	}
	if f.IDSede != nil {
		condizioni = append(condizioni, "s.id_sede = "+arg(*f.IDSede))
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
	return strings.Join(condizioni, " AND ")
}

// altriID: gli indirizzi (letti con la spedizione) e l'operatore che ha assegnato, per
// completare la voce con i dati del database globale (città della sede, nomi).
type altriID struct {
	ritiro, consegna Indirizzo
	operatore        *int64
}

func (r *Repository) elenca(ctx context.Context, ambito database.Ambito, v Visibilita, f FiltroElenco, id *int64) ([]VoceElenco, error) {
	db, err := r.perElenco(ctx, ambito, f, id)
	if err != nil {
		return nil, err
	}
	var trovate []voceConAltri
	if len(db) == 1 {
		// un solo database (sempre, tranne l'elenco di un cliente con più gruppi)
		trovate, err = leggiVoci(ctx, db[0], ambito, v, f, id, f.PerPagina, (f.Pagina-1)*f.PerPagina)
		if err != nil {
			return nil, err
		}
	} else {
		// più database: da ognuno le prime pagina × per_pagina voci, poi si uniscono con
		// lo stesso ordine della query e si prende la pagina chiesta. Va bene per gli
		// elenchi dei clienti, che sono corti (le spedizioni destinate a loro).
		for _, d := range db {
			voci, err := leggiVoci(ctx, d, ambito, v, f, id, f.Pagina*f.PerPagina, 0)
			if err != nil {
				return nil, err
			}
			trovate = append(trovate, voci...)
		}
		sort.SliceStable(trovate, func(i, j int) bool {
			a, b := trovate[i].voce, trovate[j].voce
			if !a.CreataIl.Equal(b.CreataIl) {
				return a.CreataIl.After(b.CreataIl)
			}
			return a.ID > b.ID
		})
		inizio := max(0, min((f.Pagina-1)*f.PerPagina, len(trovate)))
		trovate = trovate[inizio:min(inizio+f.PerPagina, len(trovate))]
	}
	voci := make([]VoceElenco, len(trovate))
	altri := make([]altriID, len(trovate))
	for i, t := range trovate {
		voci[i], altri[i] = t.voce, t.altri
	}
	return voci, r.completa(ctx, voci, altri)
}

// voceConAltri: una voce letta dal database con gli id da completare.
type voceConAltri struct {
	voce  VoceElenco
	altri altriID
}

// leggiVoci legge una pagina di voci da un database (solo i dati di quel database: nomi,
// indirizzi e sede vengono dal servizio utenti, in completa).
func leggiVoci(ctx context.Context, db *pgxpool.Pool, ambito database.Ambito, v Visibilita, f FiltroElenco, id *int64,
	limite, salta int) ([]voceConAltri, error) {
	p := &parametri{}
	arg := p.arg
	dove := condizioniElenco(p, ambito, v, f, id)
	filtroNote := ""
	if v.SoloNotePubbliche {
		filtroNote = "AND NOT n.is_interna"
	}

	query := fmt.Sprintf(`
		SELECT s.id, s.id_sede, s.cod_ordine, s.stato::text, s.is_reso, s.creata_il,
		       s.dataora_richiesta_spedizione, s.ritirata_il,
		       s.id_cliente_mittente, s.id_cliente_destinatario,
		       s.id_indirizzo_ritiro, s.id_indirizzo_spedizione,
		       a.id_corriere, a.id_operatore,
		       ch.conclusa_il,
		       COALESCE(ir.via, ''), COALESCE(ir.civico, ''), COALESCE(ir.cod_postale, ''),
		       COALESCE(ic.via, ''), COALESCE(ic.civico, ''), COALESCE(ic.cod_postale, ''),
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
		%s
		LEFT JOIN indirizzi ir ON ir.id = s.id_indirizzo_ritiro
		LEFT JOIN indirizzi ic ON ic.id = s.id_indirizzo_spedizione
		WHERE %s
		ORDER BY s.creata_il DESC, s.id DESC
		LIMIT %s OFFSET %s`,
		filtroNote, joinCondizioniElenco, dove, arg(limite), arg(salta))

	righe, err := db.Query(ctx, query, p.args...)
	if err != nil {
		return nil, err
	}
	defer righe.Close()

	var voci []voceConAltri
	for righe.Next() {
		var voce VoceElenco
		var a altriID
		var stato string
		var note []byte
		var capRitiro, capConsegna string
		var idRitiro, idConsegna int64 // gli indirizzi si leggono con la join
		if err := righe.Scan(&voce.ID, &voce.IDSede, &voce.CodOrdine, &stato, &voce.IsReso, &voce.CreataIl,
			&voce.DataoraRichiesta, &voce.RitirataIl,
			&voce.IDClienteMittente, &voce.IDClienteDestinatario,
			&idRitiro, &idConsegna,
			&voce.IDCorriere, &a.operatore,
			&voce.ConclusaIl,
			&a.ritiro.Via, &a.ritiro.Civico, &capRitiro,
			&a.consegna.Via, &a.consegna.Civico, &capConsegna,
			&note); err != nil {
			return nil, err
		}
		a.ritiro.CAP, a.consegna.CAP = &capRitiro, &capConsegna
		voce.Stato = Stato(stato)
		if voce.Note, err = leggiNote(note); err != nil {
			return nil, err
		}
		voci = append(voci, voceConAltri{voce, a})
	}
	return voci, righe.Err()
}

// completa aggiunge alle voci i dati del database globale (nome della sede, indirizzi,
// mittente, destinatario, corriere, chi ha assegnato, autori delle note) con una sola
// chiamata al servizio utenti per tutta la pagina.
func (r *Repository) completa(ctx context.Context, voci []VoceElenco, altri []altriID) error {
	if len(voci) == 0 {
		return nil
	}
	nomi := nuoviNomi()
	for i, v := range voci {
		nomi.sedi[v.IDSede] = true
		nomi.clienti[v.IDClienteMittente] = true
		nomi.clienti[v.IDClienteDestinatario] = true
		nomi.autore(altri[i].operatore, v.IDCorriere)
		for _, n := range v.Note {
			nomi.autore(n.IDOperatore, n.IDCorriere)
		}
	}
	if err := nomi.risolvi(ctx, r.anagrafe); err != nil {
		return err
	}
	ris := nomi.trovati
	for i := range voci {
		v := &voci[i]
		sede := ris.Sedi[v.IDSede]
		v.NomeSede = sede.Nome
		// ritiro e consegna sono nella città della sede
		v.IndirizzoRitiro = nellaCitta(altri[i].ritiro, sede)
		v.IndirizzoSpedizione = nellaCitta(altri[i].consegna, sede)
		mittente := persona(v.IDClienteMittente, ris.Clienti[v.IDClienteMittente])
		v.Mittente = &mittente
		v.Destinatario = persona(v.IDClienteDestinatario, ris.Clienti[v.IDClienteDestinatario])
		if v.IDCorriere != nil {
			if c, ok := ris.Corrieri[*v.IDCorriere]; ok {
				p := persona(*v.IDCorriere, c)
				v.Corriere = &p
			}
		}
		if op := altri[i].operatore; op != nil {
			if o, ok := ris.Operatori[*op]; ok {
				p := persona(*op, o)
				v.AssegnataDa = &p
			}
		}
		for j := range v.Note {
			n := &v.Note[j]
			n.NomeAutore, n.CognomeAutore = nomi.nomeAutore(n.IDOperatore, n.IDCorriere)
		}
	}
	return nil
}

// nomi raccoglie gli id da chiedere al servizio utenti, senza doppioni.
type nomi struct {
	sedi, indirizzi, clienti, corrieri, operatori map[int64]bool
	trovati                                       interno.Risposta
}

func nuoviNomi() *nomi {
	return &nomi{sedi: map[int64]bool{}, indirizzi: map[int64]bool{}, clienti: map[int64]bool{},
		corrieri: map[int64]bool{}, operatori: map[int64]bool{}}
}

// autore: una nota o una modifica è di un operatore oppure di un corriere.
func (n *nomi) autore(idOperatore, idCorriere *int64) {
	if idOperatore != nil {
		n.operatori[*idOperatore] = true
	}
	if idCorriere != nil {
		n.corrieri[*idCorriere] = true
	}
}

func chiavi(m map[int64]bool) []int64 {
	ids := make([]int64, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	return ids
}

func (n *nomi) risolvi(ctx context.Context, a anagrafe) error {
	var err error
	n.trovati, err = a.Risolvi(ctx, interno.Richiesta{
		Sedi: chiavi(n.sedi), Indirizzi: chiavi(n.indirizzi), Clienti: chiavi(n.clienti),
		Corrieri: chiavi(n.corrieri), Operatori: chiavi(n.operatori),
	})
	return err
}

func (n *nomi) nomeAutore(idOperatore, idCorriere *int64) (nome, cognome *string) {
	var p interno.Persona
	var ok bool
	switch {
	case idOperatore != nil:
		p, ok = n.trovati.Operatori[*idOperatore]
	case idCorriere != nil:
		p, ok = n.trovati.Corrieri[*idCorriere]
	}
	if !ok {
		return nil, nil
	}
	return &p.Nome, &p.Cognome
}

func persona(id int64, p interno.Persona) Persona {
	return Persona{ID: id, Nome: p.Nome, Cognome: p.Cognome, Email: &p.Email, Telefono: &p.Telefono}
}

// nellaCitta completa un indirizzo di ritiro o consegna con città, regione e nazione
// della sede (le sue).
func nellaCitta(i Indirizzo, sede interno.Sede) Indirizzo {
	i.Citta, i.Regione, i.Nazione = &sede.Citta, &sede.Regione, &sede.Nazione
	return i
}

func leggiNote(dati []byte) ([]Nota, error) {
	var grezze []struct {
		ID          int64     `json:"id"`
		Nota        string    `json:"nota"`
		IsInterna   bool      `json:"is_interna"`
		Motivo      *string   `json:"motivo"`
		CreataIl    time.Time `json:"creata_il"`
		IDOperatore *int64    `json:"id_operatore"`
		IDCorriere  *int64    `json:"id_corriere"`
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

func (r *Repository) Storico(ctx context.Context, ambito database.Ambito, id int64) ([]VoceStorico, error) {
	db, err := r.perID(ctx, id)
	if err != nil {
		return nil, err
	}
	p := &parametri{}
	dove := "id = " + p.arg(id) + " AND " + condizioneSede(p, ambito, "id_sede")
	var esiste bool
	if err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM spedizioni WHERE `+dove+`)`, p.args...).Scan(&esiste); err != nil {
		return nil, err
	}
	if !esiste {
		return nil, errNonTrovata
	}

	righe, err := db.Query(ctx, `
		SELECT c.id, c.operazione::text, c.valore_vecchio, c.valore_nuovo, c.modificato_il,
		       c.id_operatore, c.id_corriere
		FROM cambiamenti_spedizioni c
		WHERE c.id_spedizione = $1
		ORDER BY c.modificato_il, c.id`, id)
	if err != nil {
		return nil, err
	}
	defer righe.Close()

	voci := []VoceStorico{}
	nomi := nuoviNomi()
	for righe.Next() {
		var v VoceStorico
		if err := righe.Scan(&v.ID, &v.Operazione, &v.ValoreVecchio, &v.ValoreNuovo,
			&v.ModificatoIl, &v.IDOperatore, &v.IDCorriere); err != nil {
			return nil, err
		}
		nomi.autore(v.IDOperatore, v.IDCorriere)
		voci = append(voci, v)
	}
	if err := righe.Err(); err != nil {
		return nil, err
	}

	// i tentativi falliti (ritiro o consegna) stanno in un'altra tabella: entrano nello
	// storico come voci "tentativo", con il motivo e la nota del corriere
	tentativi, err := db.Query(ctx, `
		SELECT t.id, t.tipo::text, t.effettuato_il, n.motivo, n.nota, n.id_corriere_creata
		FROM tentativi t
		JOIN note n ON n.id = t.id_nota
		WHERE t.id_spedizione = $1`, id)
	if err != nil {
		return nil, err
	}
	defer tentativi.Close()
	for tentativi.Next() {
		var v VoceStorico
		var tipo, nota string
		var motivo *string
		if err := tentativi.Scan(&v.ID, &tipo, &v.ModificatoIl, &motivo, &nota, &v.IDCorriere); err != nil {
			return nil, err
		}
		v.Operazione = OperazioneTentativo
		v.ValoreNuovo = map[string]any{"tipo": tipo, "esito": "fallito", "motivo": motivo, "nota": nota}
		nomi.autore(nil, v.IDCorriere)
		voci = append(voci, v)
	}
	if err := tentativi.Err(); err != nil {
		return nil, err
	}
	// in ordine di tempo; a parità (stessa transazione: il tentativo che porta la spedizione
	// in "non consegnata") prima il tentativo, poi il cambio di stato che ha causato
	sort.SliceStable(voci, func(i, j int) bool {
		a, b := voci[i], voci[j]
		if !a.ModificatoIl.Equal(b.ModificatoIl) {
			return a.ModificatoIl.Before(b.ModificatoIl)
		}
		return a.Operazione == OperazioneTentativo && b.Operazione != OperazioneTentativo
	})

	// nomi degli autori: database globale
	if err := nomi.risolvi(ctx, r.anagrafe); err != nil {
		return nil, err
	}
	for i := range voci {
		voci[i].NomeAutore, voci[i].CognomeAutore = nomi.nomeAutore(voci[i].IDOperatore, voci[i].IDCorriere)
	}
	return voci, nil
}

var _ archivio = (*Repository)(nil)
