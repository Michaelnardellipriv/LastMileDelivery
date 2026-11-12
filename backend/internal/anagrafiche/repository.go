package anagrafiche

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lastmiledelivery/backend/internal/database"
	"lastmiledelivery/backend/internal/errori"
)

type Repository struct {
	db *pgxpool.Pool
}

func NuovoRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

const selezioneCliente = `
	SELECT cl.id, u.id, u.nome, u.cognome, u.email, u.prefisso, u.nr_telefono
	FROM clienti cl JOIN utenti u ON u.id = cl.id_utente`

func scansionaCliente(riga pgx.CollectableRow) (Cliente, error) {
	var c Cliente
	err := riga.Scan(&c.ID, &c.IDUtente, &c.Nome, &c.Cognome, &c.Email, &c.Prefisso, &c.NrTelefono)
	return c, err
}

func (r *Repository) CercaClienti(ctx context.Context, ric Ricerca) ([]Cliente, error) {
	righe, err := r.db.Query(ctx, selezioneCliente+`
		WHERE $1 = ''
		   OR position(lower($1) in lower(u.nome || ' ' || u.cognome || ' ' || u.cognome || ' ' || u.nome)) > 0
		   OR position(lower($1) in lower(u.email)) > 0
		   OR position($1 in u.nr_telefono) > 0
		ORDER BY u.cognome, u.nome, cl.id
		LIMIT $2 OFFSET $3`, ric.Testo, ric.PerPagina, (ric.Pagina-1)*ric.PerPagina)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(righe, scansionaCliente)
}

func (r *Repository) EmailUsata(ctx context.Context, email string) (bool, error) {
	var usata bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM utenti WHERE lower(email) = lower($1))`, email).Scan(&usata)
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

func (r *Repository) CercaCorrieri(ctx context.Context, ric Ricerca) ([]Corriere, error) {
	righe, err := r.db.Query(ctx, `
		SELECT co.id, u.id, u.nome, u.cognome, u.email, u.prefisso, u.nr_telefono
		FROM corrieri co JOIN utenti u ON u.id = co.id_utente
		WHERE $1 = ''
		   OR position(lower($1) in lower(u.nome || ' ' || u.cognome || ' ' || u.cognome || ' ' || u.nome)) > 0
		   OR position(lower($1) in lower(u.email)) > 0
		   OR position($1 in u.nr_telefono) > 0
		ORDER BY u.cognome, u.nome, co.id
		LIMIT $2 OFFSET $3`, ric.Testo, ric.PerPagina, (ric.Pagina-1)*ric.PerPagina)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(righe, func(riga pgx.CollectableRow) (Corriere, error) {
		var c Corriere
		err := riga.Scan(&c.ID, &c.IDUtente, &c.Nome, &c.Cognome, &c.Email, &c.Prefisso, &c.NrTelefono)
		return c, err
	})
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

func (r *Repository) CercaIndirizzi(ctx context.Context, ric Ricerca) ([]Indirizzo, error) {
	righe, err := r.db.Query(ctx, selezioneIndirizzo+`
		WHERE $1 = ''
		   OR position(lower($1) in lower(i.via || ' ' || i.civico || ' ' || ci.nome || ' ' || ci.cod_postale)) > 0
		ORDER BY ci.nome, i.via, i.civico, i.id
		LIMIT $2 OFFSET $3`, ric.Testo, ric.PerPagina, (ric.Pagina-1)*ric.PerPagina)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(righe, scansionaIndirizzo)
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
