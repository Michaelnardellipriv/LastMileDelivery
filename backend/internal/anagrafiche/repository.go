package anagrafiche

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lastmiledelivery/backend/internal/database"
	"lastmiledelivery/backend/internal/errori"
	"lastmiledelivery/backend/internal/identita"
)

type Repository struct {
	db *pgxpool.Pool
}

func NuovoRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// destinatario: il cliente ha ricevuto almeno una spedizione (la stessa regola del
// login); altrimenti è un mittente, anche se appena creato.
const selezioneCliente = `
	SELECT cl.id, u.id, u.nome, u.cognome, u.email, u.prefisso, u.nr_telefono, u.attivo,
	       EXISTS (SELECT 1 FROM spedizioni s WHERE s.id_cliente_destinatario = cl.id)
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
func (r *Repository) conta(ctx context.Context, daDove string, testo string) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT count(*) `+daDove, testo).Scan(&n)
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

func (r *Repository) EmailUsata(ctx context.Context, email string, escludi int32) (bool, error) {
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

	var idUtente, id int32
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
// a chi assegnare una spedizione.
func (r *Repository) CercaCorrieri(ctx context.Context, ric Ricerca) ([]Corriere, int, error) {
	const daDove = `FROM corrieri co JOIN utenti u ON u.id = co.id_utente AND u.attivo` + filtroPersona
	totale, err := r.conta(ctx, daDove, ric.Testo)
	if err != nil {
		return nil, 0, err
	}
	righe, err := r.db.Query(ctx, `
		SELECT co.id, u.id, u.nome, u.cognome, u.email, u.prefisso, u.nr_telefono `+daDove+`
		ORDER BY u.cognome, u.nome, co.id
		LIMIT $2 OFFSET $3`, ric.Testo, ric.PerPagina, (ric.Pagina-1)*ric.PerPagina)
	if err != nil {
		return nil, 0, err
	}
	corrieri, err := pgx.CollectRows(righe, func(riga pgx.CollectableRow) (Corriere, error) {
		var c Corriere
		err := riga.Scan(&c.ID, &c.IDUtente, &c.Nome, &c.Cognome, &c.Email, &c.Prefisso, &c.NrTelefono)
		return c, err
	})
	return corrieri, totale, err
}

// Corrieri e operatori in un'unica tabella, con il ruolo e l'id nella tabella del ruolo.
// Corrieri, operatori e clienti; per i clienti "destinatario" dice se hanno ricevuto
// almeno una spedizione (la stessa regola del login), per gli altri è NULL.
const selezioneUtente = `
	SELECT u.id, g.ruolo, g.id_ruolo, u.nome, u.cognome, u.email, u.prefisso, u.nr_telefono, u.attivo,
	       CASE WHEN g.ruolo = 'cliente'
	            THEN EXISTS (SELECT 1 FROM spedizioni s WHERE s.id_cliente_destinatario = g.id_ruolo) END
	FROM (
		SELECT id_utente, 'corriere' AS ruolo, id AS id_ruolo FROM corrieri
		UNION ALL
		SELECT id_utente, 'operatore', id FROM operatori_logistici
		UNION ALL
		SELECT id_utente, 'cliente', id FROM clienti
	) AS g
	JOIN utenti u ON u.id = g.id_utente`

func scansionaUtente(riga pgx.CollectableRow) (Utente, error) {
	var u Utente
	var ruolo string
	err := riga.Scan(&u.ID, &ruolo, &u.IDRuolo, &u.Nome, &u.Cognome, &u.Email, &u.Prefisso, &u.NrTelefono, &u.Attivo, &u.Destinatario)
	u.Ruolo = identita.Ruolo(ruolo)
	return u, err
}

func (r *Repository) CercaUtenti(ctx context.Context, ric RicercaUtenti) ([]Utente, int, error) {
	const filtro = `
		WHERE ` + condizionePersona + `
		  AND ($2::text IS NULL OR g.ruolo = $2)
		  AND ($3::boolean IS NULL OR u.attivo = $3)`
	var ruolo *string
	if ric.Ruolo != nil {
		ruolo = (*string)(ric.Ruolo)
	}
	var totale int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM (`+selezioneUtente+filtro+`) AS trovati`,
		ric.Testo, ruolo, ric.Attivo).Scan(&totale); err != nil {
		return nil, 0, err
	}
	righe, err := r.db.Query(ctx, selezioneUtente+filtro+`
		ORDER BY u.cognome, u.nome, u.id
		LIMIT $4 OFFSET $5`, ric.Testo, ruolo, ric.Attivo, ric.PerPagina, (ric.Pagina-1)*ric.PerPagina)
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

func (r *Repository) CreaUtente(ctx context.Context, ruolo identita.Ruolo, c Cliente, pwdHash string) (Utente, error) {
	tabella, ok := tabellaDelRuolo[ruolo]
	if !ok {
		return Utente{}, errors.New("ruolo non gestito: " + string(ruolo))
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Utente{}, err
	}
	defer tx.Rollback(ctx)

	var idUtente int32
	if err := tx.QueryRow(ctx, `
		INSERT INTO utenti (nome, cognome, prefisso, nr_telefono, email, pwd_hash)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		c.Nome, c.Cognome, c.Prefisso, c.NrTelefono, c.Email, pwdHash).Scan(&idUtente); err != nil {
		return Utente{}, traduciErrore(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO `+tabella+` (id_utente) VALUES ($1)`, idUtente); err != nil {
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
func (r *Repository) UtentePerID(ctx context.Context, idUtente int32) (Utente, error) {
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
// operatore o un cliente.
func (r *Repository) ModificaUtente(ctx context.Context, idUtente int32, d DatiUtente) (Utente, error) {
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

func traduciErrore(err error) error {
	codice, vincolo, ok := database.ErroreVincolo(err)
	if !ok || codice != database.CodiceUnicita {
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
	}
	return err
}

const selezioneIndirizzo = `
	SELECT i.id, i.via, i.civico, ci.nome, ci.cod_postale, re.nome, na.nome
	FROM indirizzi i
	JOIN indirizzi_in_citta ic ON ic.id_indirizzo = i.id
	JOIN citta ci   ON ci.id = ic.id_citta
	JOIN regioni re ON re.id = ci.id_regione
	JOIN nazioni na ON na.id = re.id_nazione`

func scansionaIndirizzo(riga pgx.CollectableRow) (Indirizzo, error) {
	var i Indirizzo
	err := riga.Scan(&i.ID, &i.Via, &i.Civico, &i.Citta, &i.CAP, &i.Regione, &i.Nazione)
	return i, err
}

func (r *Repository) CercaIndirizzi(ctx context.Context, ric Ricerca) ([]Indirizzo, int, error) {
	const filtro = `
		WHERE $1 = ''
		   OR position(lower($1) in lower(i.via || ' ' || i.civico || ' ' || ci.nome || ' ' || ci.cod_postale)) > 0`
	totale, err := r.conta(ctx, "FROM ("+selezioneIndirizzo+filtro+") AS trovati", ric.Testo)
	if err != nil {
		return nil, 0, err
	}
	righe, err := r.db.Query(ctx, selezioneIndirizzo+filtro+`
		ORDER BY ci.nome, i.via, i.civico, i.id
		LIMIT $2 OFFSET $3`, ric.Testo, ric.PerPagina, (ric.Pagina-1)*ric.PerPagina)
	if err != nil {
		return nil, 0, err
	}
	indirizzi, err := pgx.CollectRows(righe, scansionaIndirizzo)
	return indirizzi, totale, err
}

func (r *Repository) TrovaOCreaIndirizzo(ctx context.Context, i Indirizzo) (Indirizzo, bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Indirizzo{}, false, err
	}
	defer tx.Rollback(ctx)

	trovaOCrea := func(cerca string, inserisci string, args ...any) (int32, error) {
		var id int32
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
		`SELECT id FROM citta WHERE lower(nome) = lower($1) AND upper(cod_postale) = upper($2) AND id_regione = $3 ORDER BY id LIMIT 1`,
		`INSERT INTO citta (nome, cod_postale, id_regione) VALUES ($1, $2, $3)
		 ON CONFLICT ON CONSTRAINT uq_citta DO UPDATE SET nome = EXCLUDED.nome RETURNING id`,
		i.Citta, i.CAP, idRegione)
	if err != nil {
		return Indirizzo{}, false, err
	}
	if _, err := tx.Exec(ctx, `SELECT 1 FROM citta WHERE id = $1 FOR UPDATE`, idCitta); err != nil {
		return Indirizzo{}, false, err
	}

	var idIndirizzo int32
	creato := false
	err = tx.QueryRow(ctx, `
		SELECT i.id FROM indirizzi i JOIN indirizzi_in_citta ic ON ic.id_indirizzo = i.id
		WHERE ic.id_citta = $1 AND lower(i.via) = lower($2) AND lower(i.civico) = lower($3)
		ORDER BY i.id LIMIT 1`, idCitta, i.Via, i.Civico).Scan(&idIndirizzo)
	if errors.Is(err, pgx.ErrNoRows) {
		creato = true
		if err = tx.QueryRow(ctx, `INSERT INTO indirizzi (via, civico) VALUES ($1, $2) RETURNING id`,
			i.Via, i.Civico).Scan(&idIndirizzo); err == nil {
			_, err = tx.Exec(ctx, `INSERT INTO indirizzi_in_citta (id_citta, id_indirizzo) VALUES ($1, $2)`, idCitta, idIndirizzo)
		}
	}
	if err != nil {
		return Indirizzo{}, false, err
	}

	righe, err := tx.Query(ctx, selezioneIndirizzo+` WHERE i.id = $1 AND ci.id = $2`, idIndirizzo, idCitta)
	if err != nil {
		return Indirizzo{}, false, err
	}
	salvato, err := pgx.CollectExactlyOneRow(righe, scansionaIndirizzo)
	if err != nil {
		return Indirizzo{}, false, err
	}
	return salvato, creato, tx.Commit(ctx)
}

var _ archivio = (*Repository)(nil)
