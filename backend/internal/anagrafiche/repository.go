package anagrafiche

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lastmiledelivery/backend/internal/database"
	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

// spedizioniAperte chiede al servizio spedizioni (l'altro database) se un corriere ha
// ancora spedizioni da chiudere.
type spedizioniAperte interface {
	CorriereHaSpedizioniAperte(ctx context.Context, idCorriere int64) (bool, error)
	// Gruppi: i gruppi di sedi che hanno un database delle spedizioni
	Gruppi(ctx context.Context) ([]int, error)
}

// revocaSessioni chiude le sessioni dell'utente dentro la transazione (package sessioni):
// da quel momento i suoi token non valgono più, su tutti e due i servizi.
type revocaSessioni interface {
	RevocaUtente(ctx context.Context, tx pgx.Tx, idUtente int64, tranne, motivo string) (int, error)
}

type Repository struct {
	db         *pgxpool.Pool
	spedizioni spedizioniAperte
	sessioni   revocaSessioni
}

// NuovoRepository: db è il database globale (utenti, sedi, indirizzi); le spedizioni
// sono nell'altro database e si chiedono al servizio spedizioni.
func NuovoRepository(db *pgxpool.Pool, spedizioni spedizioniAperte, sessioni revocaSessioni) *Repository {
	return &Repository{db: db, spedizioni: spedizioni, sessioni: sessioni}
}

// destinatario: il cliente ha ricevuto almeno una spedizione (la stessa regola del
// login); altrimenti è un mittente, anche se appena creato. Lo segna il servizio
// spedizioni (clienti.destinatario).
const selezioneCliente = `
	SELECT cl.id, u.id, u.nome, u.cognome, u.email, u.prefisso, u.nr_telefono, u.attivo, cl.destinatario
	FROM clienti cl JOIN utenti u ON u.id = cl.id_utente`

func scansionaCliente(riga pgx.CollectableRow) (Cliente, error) {
	var c Cliente
	err := riga.Scan(&c.ID, &c.IDUtente, &c.Nome, &c.Cognome, &c.Email, &c.Prefisso, &c.NrTelefono, &c.Attivo, &c.Destinatario)
	return c, err
}

// Ricerca per nome, cognome, email o telefono (clienti, corrieri e operatori).
const condizionePersona = `($1 = ''
		   OR position(lower($1) in lower(u.nome || ' ' || u.cognome || ' ' || u.cognome || ' ' || u.nome)) > 0
		   OR position(lower($1) in lower(u.email)) > 0
		   OR position($1 in u.nr_telefono) > 0)`

const filtroPersona = `
		WHERE ` + condizionePersona

// conta restituisce quante righe trova la ricerca, su tutte le pagine.
func (r *Repository) conta(ctx context.Context, daDove string, args ...any) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT count(*) `+daDove, args...).Scan(&n)
	return n, err
}

func (r *Repository) CercaClienti(ctx context.Context, ric Ricerca) ([]Cliente, int, error) {
	const daDove = `FROM clienti cl JOIN utenti u ON u.id = cl.id_utente` + filtroPersona
	totale, err := r.conta(ctx, daDove, ric.Testo)
	if err != nil {
		return nil, 0, err
	}
	righe, err := r.db.Query(ctx, selezioneCliente+filtroPersona+`
		ORDER BY u.cognome, u.nome, cl.id
		LIMIT $2 OFFSET $3`, ric.Testo, ric.PerPagina, (ric.Pagina-1)*ric.PerPagina)
	if err != nil {
		return nil, 0, err
	}
	clienti, err := pgx.CollectRows(righe, scansionaCliente)
	return clienti, totale, err
}

func (r *Repository) EmailUsata(ctx context.Context, email string, escludi int64) (bool, error) {
	var usata bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM utenti WHERE lower(email) = lower($1) AND id <> $2)`,
		email, escludi).Scan(&usata)
	return usata, err
}

func (r *Repository) CreaCliente(ctx context.Context, c Cliente, pwdHash string) (Cliente, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Cliente{}, err
	}
	defer tx.Rollback(ctx)

	var idUtente, id int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO utenti (nome, cognome, prefisso, nr_telefono, email, pwd_hash)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		c.Nome, c.Cognome, c.Prefisso, c.NrTelefono, c.Email, pwdHash).Scan(&idUtente); err != nil {
		return Cliente{}, traduciErrore(err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO clienti (id_utente) VALUES ($1) RETURNING id`, idUtente).Scan(&id); err != nil {
		return Cliente{}, traduciErrore(err)
	}
	righe, err := tx.Query(ctx, selezioneCliente+` WHERE cl.id = $1`, id)
	if err != nil {
		return Cliente{}, err
	}
	creato, err := pgx.CollectExactlyOneRow(righe, scansionaCliente)
	if err != nil {
		return Cliente{}, err
	}
	return creato, tx.Commit(ctx)
}

// CercaCorrieri elenca solo i corrieri attivi: è l'elenco da cui l'operatore sceglie
// a chi assegnare una spedizione. ric.IDSede (se c'è) li limita a una sede.
func (r *Repository) CercaCorrieri(ctx context.Context, ric Ricerca) ([]Corriere, int, error) {
	const daDove = `FROM corrieri co JOIN utenti u ON u.id = co.id_utente AND u.attivo` + filtroPersona + `
		  AND ($2::bigint IS NULL OR co.id_sede = $2)`
	totale, err := r.conta(ctx, daDove, ric.Testo, ric.IDSede)
	if err != nil {
		return nil, 0, err
	}
	righe, err := r.db.Query(ctx, `
		SELECT co.id, u.id, co.id_sede, u.nome, u.cognome, u.email, u.prefisso, u.nr_telefono `+daDove+`
		ORDER BY u.cognome, u.nome, co.id
		LIMIT $3 OFFSET $4`, ric.Testo, ric.IDSede, ric.PerPagina, (ric.Pagina-1)*ric.PerPagina)
	if err != nil {
		return nil, 0, err
	}
	corrieri, err := pgx.CollectRows(righe, func(riga pgx.CollectableRow) (Corriere, error) {
		var c Corriere
		err := riga.Scan(&c.ID, &c.IDUtente, &c.IDSede, &c.Nome, &c.Cognome, &c.Email, &c.Prefisso, &c.NrTelefono)
		return c, err
	})
	return corrieri, totale, err
}

// Corrieri, operatori e clienti in un'unica tabella, con il ruolo, l'id nella tabella del
// ruolo e (per corrieri e operatori) la sede in cui lavorano. Per i clienti "destinatario"
// dice se hanno ricevuto almeno una spedizione (la stessa regola del login), per gli
// altri è NULL.
const selezioneUtente = `
	SELECT u.id, g.ruolo, g.id_ruolo, g.id_sede, se.nome, u.nome, u.cognome, u.email, u.prefisso, u.nr_telefono, u.attivo,
	       g.destinatario
	FROM (
		SELECT id_utente, 'corriere' AS ruolo, id AS id_ruolo, id_sede, NULL::boolean AS destinatario FROM corrieri
		UNION ALL
		SELECT id_utente, 'operatore', id, id_sede, NULL FROM operatori_logistici
		UNION ALL
		SELECT id_utente, 'cliente', id, NULL::bigint, destinatario FROM clienti
	) AS g
	JOIN utenti u     ON u.id = g.id_utente
	LEFT JOIN sedi se ON se.id = g.id_sede`

func scansionaUtente(riga pgx.CollectableRow) (Utente, error) {
	var u Utente
	var ruolo string
	err := riga.Scan(&u.ID, &ruolo, &u.IDRuolo, &u.IDSede, &u.NomeSede,
		&u.Nome, &u.Cognome, &u.Email, &u.Prefisso, &u.NrTelefono, &u.Attivo, &u.Destinatario)
	u.Ruolo = identita.Ruolo(ruolo)
	return u, err
}

func (r *Repository) CercaUtenti(ctx context.Context, ric RicercaUtenti) ([]Utente, int, error) {
	const filtro = `
		WHERE ` + condizionePersona + `
		  AND ($2::text IS NULL OR g.ruolo = $2)
		  AND ($3::boolean IS NULL OR u.attivo = $3)
		  AND ($4::bigint IS NULL OR g.id_sede = $4)`
	var ruolo *string
	if ric.Ruolo != nil {
		ruolo = (*string)(ric.Ruolo)
	}
	var totale int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM (`+selezioneUtente+filtro+`) AS trovati`,
		ric.Testo, ruolo, ric.Attivo, ric.IDSede).Scan(&totale); err != nil {
		return nil, 0, err
	}
	righe, err := r.db.Query(ctx, selezioneUtente+filtro+`
		ORDER BY u.cognome, u.nome, u.id
		LIMIT $5 OFFSET $6`, ric.Testo, ruolo, ric.Attivo, ric.IDSede, ric.PerPagina, (ric.Pagina-1)*ric.PerPagina)
	if err != nil {
		return nil, 0, err
	}
	utenti, err := pgx.CollectRows(righe, scansionaUtente)
	return utenti, totale, err
}

var tabellaDelRuolo = map[identita.Ruolo]string{
	identita.Corriere:  "corrieri",
	identita.Operatore: "operatori_logistici",
}

func (r *Repository) CreaUtente(ctx context.Context, ruolo identita.Ruolo, idSede int64, c Cliente, pwdHash string) (Utente, error) {
	tabella, ok := tabellaDelRuolo[ruolo]
	if !ok {
		return Utente{}, errors.New("ruolo non gestito: " + string(ruolo))
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Utente{}, err
	}
	defer tx.Rollback(ctx)

	if err := sedeAttiva(ctx, tx, idSede); err != nil {
		return Utente{}, err
	}
	var idUtente int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO utenti (nome, cognome, prefisso, nr_telefono, email, pwd_hash)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		c.Nome, c.Cognome, c.Prefisso, c.NrTelefono, c.Email, pwdHash).Scan(&idUtente); err != nil {
		return Utente{}, traduciErrore(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO `+tabella+` (id_utente, id_sede) VALUES ($1, $2)`, idUtente, idSede); err != nil {
		return Utente{}, traduciErrore(err)
	}
	righe, err := tx.Query(ctx, selezioneUtente+` WHERE u.id = $1`, idUtente)
	if err != nil {
		return Utente{}, err
	}
	creato, err := pgx.CollectExactlyOneRow(righe, scansionaUtente)
	if err != nil {
		return Utente{}, err
	}
	return creato, tx.Commit(ctx)
}

// UtentePerID legge un corriere, un operatore o un cliente; per gli altri (amministratori)
// e per gli id inesistenti risponde NonTrovato.
func (r *Repository) UtentePerID(ctx context.Context, idUtente int64) (Utente, error) {
	righe, err := r.db.Query(ctx, selezioneUtente+` WHERE u.id = $1`, idUtente)
	if err != nil {
		return Utente{}, err
	}
	u, err := pgx.CollectExactlyOneRow(righe, scansionaUtente)
	if errors.Is(err, pgx.ErrNoRows) {
		return Utente{}, errUtenteNonTrovato
	}
	return u, err
}

var errUtenteNonTrovato = &errori.NonTrovato{Messaggio: "nessun corriere, operatore o cliente con questo id utente"}

// ModificaUtente cambia solo i campi presenti, e solo se l'utente è un corriere, un
// operatore o un cliente. Un corriere cambia sede solo se non ha spedizioni ancora da
// chiudere (quelle restano nella loro città), e solo verso una sede attiva.
func (r *Repository) ModificaUtente(ctx context.Context, idUtente int64, d DatiUtente) (Utente, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Utente{}, err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE utenti SET
			nome        = COALESCE($2, nome),
			cognome     = COALESCE($3, cognome),
			prefisso    = COALESCE($4, prefisso),
			nr_telefono = COALESCE($5, nr_telefono),
			email       = COALESCE($6, email),
			pwd_hash    = COALESCE($7, pwd_hash),
			attivo      = COALESCE($8, attivo)
		WHERE id = $1
		  AND id IN (SELECT id_utente FROM corrieri UNION ALL SELECT id_utente FROM operatori_logistici
		             UNION ALL SELECT id_utente FROM clienti)`,
		idUtente, d.Nome, d.Cognome, d.Prefisso, d.NrTelefono, d.Email, d.PwdHash, d.Attivo)
	if err != nil {
		return Utente{}, traduciErrore(err)
	}
	if tag.RowsAffected() == 0 {
		return Utente{}, errUtenteNonTrovato
	}
	// i link di recupero già mandati al vecchio indirizzo non valgono più
	if d.Email != nil {
		if _, err := tx.Exec(ctx, `
			UPDATE recuperi_password SET usato_il = now()
			WHERE id_utente = $1 AND usato_il IS NULL`, idUtente); err != nil {
			return Utente{}, err
		}
	}
	cambiataSede := false
	if d.IDSede != nil {
		if cambiataSede, err = r.cambiaSede(ctx, tx, idUtente, *d.IDSede); err != nil {
			return Utente{}, err
		}
	}
	// i token dell'utente smettono subito di valere quando l'amministratore gli nega
	// l'accesso, gli cambia sede (il token porta la sede) o gli dà una password nuova
	motivo := ""
	switch {
	case d.Attivo != nil && !*d.Attivo:
		motivo = identita.MotivoDisattivato
	case cambiataSede:
		motivo = identita.MotivoCambioSede
	case d.PwdHash != nil:
		motivo = identita.MotivoPasswordCambiata
	}
	if motivo != "" {
		if _, err := r.sessioni.RevocaUtente(ctx, tx, idUtente, "", motivo); err != nil {
			return Utente{}, err
		}
	}
	righe, err := tx.Query(ctx, selezioneUtente+` WHERE u.id = $1`, idUtente)
	if err != nil {
		return Utente{}, err
	}
	u, err := pgx.CollectExactlyOneRow(righe, scansionaUtente)
	if err != nil {
		return Utente{}, err
	}
	return u, tx.Commit(ctx)
}

// sedeAttiva: corrieri e operatori si creano o si spostano solo in una sede attiva.
func sedeAttiva(ctx context.Context, tx pgx.Tx, idSede int64) error {
	var attiva bool
	err := tx.QueryRow(ctx, `SELECT attiva FROM sedi WHERE id = $1`, idSede).Scan(&attiva)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return &errori.RiferimentoNonTrovato{Campo: "id_sede", Messaggio: "sede non trovata"}
	case err != nil:
		return err
	case !attiva:
		return &errori.Conflitto{
			Messaggio: "la sede è disattivata: non le si assegnano corrieri né operatori",
			Dettagli:  []errori.ErroreCampo{{Campo: "id_sede", Messaggio: "sede disattivata"}},
		}
	}
	return nil
}

// cambiaSede sposta un corriere o un operatore; dice se la sede è davvero cambiata.
func (r *Repository) cambiaSede(ctx context.Context, tx pgx.Tx, idUtente, idSede int64) (bool, error) {
	var attuale int64
	err := tx.QueryRow(ctx, `
		SELECT id_sede FROM corrieri WHERE id_utente = $1
		UNION ALL
		SELECT id_sede FROM operatori_logistici WHERE id_utente = $1`, idUtente).Scan(&attuale)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// un cliente: non ha sede (il servizio lo rifiuta prima di arrivare qui)
		return false, &errori.Validazione{Dettagli: []errori.ErroreCampo{{Campo: "id_sede", Messaggio: "i clienti non hanno una sede"}}}
	case err != nil:
		return false, err
	case attuale == idSede:
		return false, nil // già in questa sede
	}
	if err := sedeAttiva(ctx, tx, idSede); err != nil {
		return false, err
	}
	// le spedizioni sono nell'altro database: lo si chiede al servizio spedizioni. Il
	// blocco sulla riga del corriere tiene ferma la sua sede mentre si decide.
	var idCorriere int64
	err = tx.QueryRow(ctx, `SELECT id FROM corrieri WHERE id_utente = $1 FOR UPDATE`, idUtente).Scan(&idCorriere)
	switch {
	case err == nil:
		aperte, err := r.spedizioni.CorriereHaSpedizioniAperte(ctx, idCorriere)
		if err != nil {
			return false, err
		}
		if aperte {
			return false, &errori.Conflitto{
				Messaggio: "il corriere ha spedizioni assegnate, ritirate o in consegna nella sede attuale: " +
					"riassegnarle a un altro corriere prima di spostarlo",
				Dettagli: []errori.ErroreCampo{{Campo: "id_sede", Messaggio: "spedizioni ancora aperte"}},
			}
		}
	case !errors.Is(err, pgx.ErrNoRows):
		return false, err
	}
	// l'ultima posizione del corriere (database delle spedizioni) resta con la vecchia
	// sede: la nuova sede la vede solo dal prossimo invio
	for _, tabella := range []string{"corrieri", "operatori_logistici"} {
		if _, err := tx.Exec(ctx, `UPDATE `+tabella+` SET id_sede = $2 WHERE id_utente = $1`, idUtente, idSede); err != nil {
			return false, traduciErrore(err)
		}
	}
	return true, nil
}

var errSedeNonTrovata = &errori.NonTrovato{Messaggio: "sede non trovata"}

func traduciErrore(err error) error {
	codice, vincolo, ok := database.ErroreVincolo(err)
	if !ok {
		return err
	}
	switch {
	case codice == database.CodiceChiaveEsterna &&
		(vincolo == "corrieri_id_sede_fkey" || vincolo == "operatori_logistici_id_sede_fkey"):
		return &errori.RiferimentoNonTrovato{Campo: "id_sede", Messaggio: "sede non trovata"}
	case codice != database.CodiceUnicita:
		return err
	}
	switch vincolo {
	case "uq_utenti_telefono":
		return &errori.Conflitto{
			Messaggio: "esiste già un utente con questo numero di telefono",
			Dettagli:  []errori.ErroreCampo{{Campo: "nr_telefono", Messaggio: "già usato"}},
		}
	case "uq_utenti_email", "uq_utenti_un_solo_ruolo":
		return errEmailUsata
	case "uq_sedi_citta":
		return &errori.Conflitto{
			Messaggio: "questa città ha già una sede: ogni città ne ha una sola",
			Dettagli:  []errori.ErroreCampo{{Campo: "indirizzo.citta", Messaggio: "città con una sede"}},
		}
	case "uq_sedi_nome":
		return &errori.Conflitto{
			Messaggio: "esiste già una sede con questo nome",
			Dettagli:  []errori.ErroreCampo{{Campo: "nome", Messaggio: "già usato"}},
		}
	}
	return err
}

// ===== Indirizzi =====

const selezioneIndirizzo = `
	SELECT i.id, i.id_citta, i.via, i.civico, ci.nome, i.cod_postale, re.nome, na.nome
	FROM indirizzi i
	JOIN citta ci   ON ci.id = i.id_citta
	JOIN regioni re ON re.id = ci.id_regione
	JOIN nazioni na ON na.id = re.id_nazione`

func scansionaIndirizzo(riga pgx.CollectableRow) (Indirizzo, error) {
	var i Indirizzo
	err := riga.Scan(&i.ID, &i.IDCitta, &i.Via, &i.Civico, &i.Citta, &i.CAP, &i.Regione, &i.Nazione)
	return i, err
}

// trovaOCreaIndirizzo cerca (o crea) nazione, regione, città e indirizzo dentro la
// transazione tx: l'indirizzo di una sede (gli indirizzi di ritiro e consegna sono nel
// database delle spedizioni del gruppo della sede).
func trovaOCreaIndirizzo(ctx context.Context, tx pgx.Tx, i Indirizzo) (Indirizzo, bool, error) {
	trovaOCrea := func(cerca string, inserisci string, args ...any) (int64, error) {
		var id int64
		err := tx.QueryRow(ctx, cerca, args...).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, inserisci, args...).Scan(&id)
		}
		return id, err
	}

	idNazione, err := trovaOCrea(
		`SELECT id FROM nazioni WHERE lower(nome) = lower($1) ORDER BY id LIMIT 1`,
		`INSERT INTO nazioni (nome) VALUES ($1)
		 ON CONFLICT (nome) DO UPDATE SET nome = EXCLUDED.nome RETURNING id`, i.Nazione)
	if err != nil {
		return Indirizzo{}, false, err
	}
	idRegione, err := trovaOCrea(
		`SELECT id FROM regioni WHERE lower(nome) = lower($1) AND id_nazione = $2 ORDER BY id LIMIT 1`,
		`INSERT INTO regioni (nome, id_nazione) VALUES ($1, $2)
		 ON CONFLICT ON CONSTRAINT uq_regioni_nome_nazione DO UPDATE SET nome = EXCLUDED.nome RETURNING id`,
		i.Regione, idNazione)
	if err != nil {
		return Indirizzo{}, false, err
	}
	idCitta, err := trovaOCrea(
		`SELECT id FROM citta WHERE lower(nome) = lower($1) AND id_regione = $2 ORDER BY id LIMIT 1`,
		`INSERT INTO citta (nome, id_regione) VALUES ($1, $2)
		 ON CONFLICT ON CONSTRAINT uq_citta DO UPDATE SET nome = EXCLUDED.nome RETURNING id`,
		i.Citta, idRegione)
	if err != nil {
		return Indirizzo{}, false, err
	}
	// due inserimenti contemporanei dello stesso indirizzo passano uno alla volta
	if _, err := tx.Exec(ctx, `SELECT 1 FROM citta WHERE id = $1 FOR UPDATE`, idCitta); err != nil {
		return Indirizzo{}, false, err
	}

	var idIndirizzo int64
	creato := false
	err = tx.QueryRow(ctx, `
		SELECT id FROM indirizzi
		WHERE id_citta = $1 AND lower(via) = lower($2) AND lower(civico) = lower($3)`,
		idCitta, i.Via, i.Civico).Scan(&idIndirizzo)
	if errors.Is(err, pgx.ErrNoRows) {
		creato = true
		err = tx.QueryRow(ctx, `
			INSERT INTO indirizzi (via, civico, cod_postale, id_citta) VALUES ($1, $2, $3, $4) RETURNING id`,
			i.Via, i.Civico, i.CAP, idCitta).Scan(&idIndirizzo)
	}
	if err != nil {
		return Indirizzo{}, false, err
	}

	righe, err := tx.Query(ctx, selezioneIndirizzo+` WHERE i.id = $1`, idIndirizzo)
	if err != nil {
		return Indirizzo{}, false, err
	}
	salvato, err := pgx.CollectExactlyOneRow(righe, scansionaIndirizzo)
	return salvato, creato, err
}

// ===== Sedi =====

const selezioneSede = `
	SELECT se.id, se.nome, se.attiva, se.gruppo_spedizioni, se.creata_il,
	       i.id, i.id_citta, i.via, i.civico, ci.nome, i.cod_postale, re.nome, na.nome
	FROM sedi se
	JOIN indirizzi i ON i.id = se.id_indirizzo
	JOIN citta ci    ON ci.id = i.id_citta
	JOIN regioni re  ON re.id = ci.id_regione
	JOIN nazioni na  ON na.id = re.id_nazione`

func scansionaSede(riga pgx.CollectableRow) (Sede, error) {
	var s Sede
	i := &s.Indirizzo
	err := riga.Scan(&s.ID, &s.Nome, &s.Attiva, &s.Gruppo, &s.CreataIl,
		&i.ID, &i.IDCitta, &i.Via, &i.Civico, &i.Citta, &i.CAP, &i.Regione, &i.Nazione)
	return s, err
}

func (r *Repository) CercaSedi(ctx context.Context, ric RicercaSedi) ([]Sede, int, error) {
	const filtro = `
		WHERE ($1 = '' OR position(lower($1) in lower(se.nome || ' ' || ci.nome || ' ' || re.nome || ' ' || na.nome)) > 0)
		  AND ($2::bigint IS NULL OR se.id = $2)
		  AND ($3::boolean IS NULL OR se.attiva = $3)`
	totale, err := r.conta(ctx, "FROM ("+selezioneSede+filtro+") AS trovate", ric.Testo, ric.IDSede, ric.Attiva)
	if err != nil {
		return nil, 0, err
	}
	righe, err := r.db.Query(ctx, selezioneSede+filtro+`
		ORDER BY se.nome, se.id
		LIMIT $4 OFFSET $5`, ric.Testo, ric.IDSede, ric.Attiva, ric.PerPagina, (ric.Pagina-1)*ric.PerPagina)
	if err != nil {
		return nil, 0, err
	}
	sedi, err := pgx.CollectRows(righe, scansionaSede)
	return sedi, totale, err
}

func (r *Repository) Sede(ctx context.Context, id int64) (Sede, error) {
	return sedePerID(ctx, r.db, id)
}

type interrogabile interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func sedePerID(ctx context.Context, q interrogabile, id int64) (Sede, error) {
	righe, err := q.Query(ctx, selezioneSede+` WHERE se.id = $1`, id)
	if err != nil {
		return Sede{}, err
	}
	s, err := pgx.CollectExactlyOneRow(righe, scansionaSede)
	if errors.Is(err, pgx.ErrNoRows) {
		return Sede{}, errSedeNonTrovata
	}
	return s, err
}

// CreaSede crea indirizzo (se manca) e sede in una sola transazione. Le impostazioni
// della sede stanno nel database delle spedizioni: il servizio spedizioni le crea con i
// valori predefiniti la prima volta che servono.
func (r *Repository) CreaSede(ctx context.Context, nome string, i Indirizzo, gruppo int) (Sede, error) {
	// un gruppo diverso dal principale deve avere il suo database delle spedizioni,
	// altrimenti la sede non potrebbe lavorare
	if gruppo != 0 {
		codici, err := r.spedizioni.Gruppi(ctx)
		if err != nil {
			return Sede{}, err
		}
		if !slices.Contains(codici, gruppo) {
			e := &errori.Validazione{}
			e.Aggiungi("gruppo_spedizioni", fmt.Sprintf("il gruppo %d non ha un database delle spedizioni (gruppi configurati: %v)", gruppo, codici))
			return Sede{}, e
		}
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Sede{}, err
	}
	defer tx.Rollback(ctx)

	ind, _, err := trovaOCreaIndirizzo(ctx, tx, i)
	if err != nil {
		return Sede{}, err
	}
	var id int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO sedi (nome, id_citta, id_indirizzo, gruppo_spedizioni) VALUES ($1, $2, $3, $4) RETURNING id`,
		nome, ind.IDCitta, ind.ID, gruppo).Scan(&id); err != nil {
		return Sede{}, traduciErrore(err)
	}
	creata, err := sedePerID(ctx, tx, id)
	if err != nil {
		return Sede{}, err
	}
	return creata, tx.Commit(ctx)
}

// ModificaSede cambia solo i campi presenti. Il nuovo indirizzo deve essere nella città
// della sede: le spedizioni della sede sono legate alla sua città, che non cambia.
func (r *Repository) ModificaSede(ctx context.Context, id int64, d DatiSede) (Sede, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Sede{}, err
	}
	defer tx.Rollback(ctx)

	attuale, err := sedePerID(ctx, tx, id)
	if err != nil {
		return Sede{}, err
	}
	var idIndirizzo *int64
	if d.Indirizzo != nil {
		ind, _, err := trovaOCreaIndirizzo(ctx, tx, *d.Indirizzo)
		if err != nil {
			return Sede{}, err
		}
		if ind.IDCitta != attuale.Indirizzo.IDCitta {
			return Sede{}, &errori.Validazione{Dettagli: []errori.ErroreCampo{{Campo: "indirizzo.citta",
				Messaggio: "la sede resta a " + attuale.Indirizzo.Citta + ": il nuovo indirizzo deve essere nella stessa città"}}}
		}
		idIndirizzo = &ind.ID
	}
	if _, err := tx.Exec(ctx, `
		UPDATE sedi SET
			nome         = COALESCE($2, nome),
			id_indirizzo = COALESCE($3, id_indirizzo),
			attiva       = COALESCE($4, attiva)
		WHERE id = $1`, id, d.Nome, idIndirizzo, d.Attiva); err != nil {
		return Sede{}, traduciErrore(err)
	}
	modificata, err := sedePerID(ctx, tx, id)
	if err != nil {
		return Sede{}, err
	}
	return modificata, tx.Commit(ctx)
}

var _ archivio = (*Repository)(nil)
