BEGIN;



DROP TABLE IF EXISTS cambiamenti_spedizioni CASCADE;
DROP TABLE IF EXISTS tentativi              CASCADE;
DROP TABLE IF EXISTS note                   CASCADE;
DROP TABLE IF EXISTS spedizioni             CASCADE;
DROP TABLE IF EXISTS assegnazioni           CASCADE;
DROP TABLE IF EXISTS indirizzi_in_citta     CASCADE;
DROP TABLE IF EXISTS indirizzi              CASCADE;
DROP TABLE IF EXISTS citta                  CASCADE;
DROP TABLE IF EXISTS regioni                CASCADE;
DROP TABLE IF EXISTS nazioni                CASCADE;
DROP TABLE IF EXISTS amministratori         CASCADE;
DROP TABLE IF EXISTS operatori_logistici    CASCADE;
DROP TABLE IF EXISTS corrieri               CASCADE;
DROP TABLE IF EXISTS clienti                CASCADE;
DROP TABLE IF EXISTS utenti                 CASCADE;

DROP FUNCTION IF EXISTS fn_traccia_cambiamenti_spedizioni() CASCADE;
DROP FUNCTION IF EXISTS fn_blocca_modifica_storico()        CASCADE;
DROP FUNCTION IF EXISTS fn_controlla_annullamento()         CASCADE;

DROP TYPE IF EXISTS tipo_operazione_spedizione CASCADE;
DROP TYPE IF EXISTS stato_spedizione           CASCADE;
DROP TYPE IF EXISTS tipo_tentativo             CASCADE;


CREATE TYPE tipo_tentativo AS ENUM ('consegna', 'ritiro');

CREATE TYPE stato_spedizione AS ENUM (
    'creata',
    'assegnata',
    'ritirata',
    'persa',
    'in consegna',
    'consegnata',
    'non consegnata',
    'annullata'
);


CREATE TYPE tipo_operazione_spedizione AS ENUM (
    'creazione',
    'modifica',
    'annullamento'
);


CREATE TABLE utenti (
    id           INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nome         VARCHAR(100) NOT NULL,
    cognome      VARCHAR(100) NOT NULL,
    prefisso     VARCHAR(5)   NOT NULL,          -- es. '+39'
    nr_telefono  VARCHAR(20)  NOT NULL,
    pwd_hash     VARCHAR(255) NOT NULL,
    email        VARCHAR(254) NOT NULL,
    CONSTRAINT uq_utenti_email    UNIQUE (email),
    CONSTRAINT uq_utenti_telefono UNIQUE (prefisso, nr_telefono),
    CONSTRAINT ck_utenti_prefisso CHECK (prefisso ~ '^\+[0-9]{1,4}$'),
    CONSTRAINT ck_utenti_telefono CHECK (nr_telefono ~ '^[0-9]{4,15}$'),
    CONSTRAINT ck_utenti_email    CHECK (email ~* '^[^@\s]+@[^@\s]+\.[^@\s]+$')
);

CREATE TABLE clienti (
    id         INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_utente  INTEGER NOT NULL UNIQUE
               REFERENCES utenti (id) ON UPDATE CASCADE ON DELETE RESTRICT
);

CREATE TABLE corrieri (
    id         INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_utente  INTEGER NOT NULL UNIQUE
               REFERENCES utenti (id) ON UPDATE CASCADE ON DELETE RESTRICT
);

CREATE TABLE operatori_logistici (
    id         INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_utente  INTEGER NOT NULL UNIQUE
               REFERENCES utenti (id) ON UPDATE CASCADE ON DELETE RESTRICT
);

CREATE TABLE amministratori (
    id         INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_utente  INTEGER NOT NULL UNIQUE
               REFERENCES utenti (id) ON UPDATE CASCADE ON DELETE RESTRICT
);


CREATE TABLE nazioni (
    id    INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nome  VARCHAR(100) NOT NULL UNIQUE
);

CREATE TABLE regioni (
    id          INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nome        VARCHAR(100) NOT NULL,
    id_nazione  INTEGER NOT NULL
                REFERENCES nazioni (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT uq_regioni_nome_nazione UNIQUE (nome, id_nazione)
);

CREATE TABLE citta (
    id           INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nome         VARCHAR(100) NOT NULL,
    cod_postale  VARCHAR(10)  NOT NULL,
    id_regione   INTEGER NOT NULL
                 REFERENCES regioni (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT uq_citta UNIQUE (nome, cod_postale, id_regione)
);

CREATE TABLE indirizzi (
    id      INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    via     VARCHAR(150) NOT NULL,
    civico  VARCHAR(10)  NOT NULL
);

CREATE TABLE indirizzi_in_citta (
    id            INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_citta      INTEGER NOT NULL
                  REFERENCES citta (id)     ON UPDATE CASCADE ON DELETE RESTRICT,
    id_indirizzo  INTEGER NOT NULL
                  REFERENCES indirizzi (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT uq_indirizzi_in_citta UNIQUE (id_citta, id_indirizzo)
);


CREATE TABLE assegnazioni (
    id           INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_operatore INTEGER NOT NULL
                 REFERENCES operatori_logistici (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    id_corriere  INTEGER NOT NULL
                 REFERENCES corrieri (id)            ON UPDATE CASCADE ON DELETE RESTRICT,
    creata_il    TIMESTAMPTZ NOT NULL DEFAULT now()
);


CREATE TABLE spedizioni (
    id                            INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    stato                         stato_spedizione NOT NULL DEFAULT 'creata',
    cod_ordine                    VARCHAR(50) NOT NULL,
    is_reso                       BOOLEAN     NOT NULL DEFAULT FALSE,
    id_operatore_ha_annullato     INTEGER NULL
                                  REFERENCES operatori_logistici (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    annullata_il                  TIMESTAMPTZ NULL,
    id_operatore_ha_creato        INTEGER NOT NULL
                                  REFERENCES operatori_logistici (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    creata_il                     TIMESTAMPTZ NOT NULL DEFAULT now(),
    id_cliente_mittente           INTEGER NOT NULL
                                  REFERENCES clienti (id)   ON UPDATE CASCADE ON DELETE RESTRICT,
    id_cliente_destinatario       INTEGER NOT NULL
                                  REFERENCES clienti (id)   ON UPDATE CASCADE ON DELETE RESTRICT,
    dataora_richiesta_spedizione  TIMESTAMPTZ NOT NULL,
    id_indirizzo_ritiro           INTEGER NOT NULL
                                  REFERENCES indirizzi (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    id_indirizzo_spedizione       INTEGER NOT NULL
                                  REFERENCES indirizzi (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    id_assegnazione_corrente      INTEGER NULL
                                  REFERENCES assegnazioni (id) ON UPDATE CASCADE ON DELETE SET NULL,

    CONSTRAINT uq_spedizioni_cod_ordine          UNIQUE (cod_ordine),
    CONSTRAINT uq_spedizioni_assegnazione_corr   UNIQUE (id_assegnazione_corrente),

    -- annullamento: operatore e data presenti entrambi o assenti entrambi
    CONSTRAINT ck_spedizioni_annullamento        CHECK (
        (id_operatore_ha_annullato IS NULL) = (annullata_il IS NULL)
    ),
    -- la data di annullamento c'è se e solo se lo stato è 'annullata'
    CONSTRAINT ck_spedizioni_annullamento_stato  CHECK (
        (stato = 'annullata') = (annullata_il IS NOT NULL)
    ),
    CONSTRAINT ck_spedizioni_date_annullamento   CHECK (annullata_il IS NULL OR annullata_il >= creata_il),
    -- da 'assegnata' in poi serve l'assegnazione (e quindi il corriere);
    -- possono non averla solo le spedizioni 'creata' e 'annullata'
    CONSTRAINT ck_spedizioni_assegnazione_stato  CHECK (
        stato IN ('creata', 'annullata') OR id_assegnazione_corrente IS NOT NULL
    )
);


CREATE TABLE note (
    id                             INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nota                           TEXT        NOT NULL,
    is_interna                     BOOLEAN     NOT NULL DEFAULT FALSE,
    motivo                         VARCHAR(100) NULL,
    creata_il                      TIMESTAMPTZ NOT NULL DEFAULT now(),
    id_operatore_logistico_creata  INTEGER NULL
                                   REFERENCES operatori_logistici (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    id_corriere_creata             INTEGER NULL
                                   REFERENCES corrieri (id)            ON UPDATE CASCADE ON DELETE RESTRICT,
    id_spedizione  INTEGER NOT NULL
                                   REFERENCES spedizioni (id) ON UPDATE CASCADE ON DELETE CASCADE,
    -- esattamente uno tra operatore e corriere
    CONSTRAINT ck_note_autore CHECK (
        num_nonnulls(id_operatore_logistico_creata, id_corriere_creata) = 1
    ),
    -- serve alla chiave esterna composta di tentativi (nota + spedizione)
    CONSTRAINT uq_note_id_spedizione UNIQUE (id, id_spedizione)
);



CREATE TABLE tentativi (
    id             INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tipo           tipo_tentativo NOT NULL,       -- era "isConsegna"
    effettuato_il  TIMESTAMPTZ    NOT NULL DEFAULT now(),
    id_spedizione  INTEGER NOT NULL
                   REFERENCES spedizioni (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    id_nota        INTEGER NOT NULL,
    -- la nota del tentativo deve essere della stessa spedizione del tentativo
    CONSTRAINT fk_tentativi_nota_stessa_spedizione
        FOREIGN KEY (id_nota, id_spedizione)
        REFERENCES note (id, id_spedizione) ON UPDATE CASCADE ON DELETE RESTRICT
);



CREATE TABLE cambiamenti_spedizioni (
    id                       INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    operazione               tipo_operazione_spedizione NOT NULL,
    valore_vecchio           JSONB NULL,
    valore_nuovo             JSONB NOT NULL,
    modificato_il            TIMESTAMPTZ NOT NULL DEFAULT now(),
    id_corriere              INTEGER NULL
                             REFERENCES corrieri (id)            ON UPDATE CASCADE ON DELETE RESTRICT,
    id_operatore             INTEGER NULL
                             REFERENCES operatori_logistici (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    id_spedizione            INTEGER NOT NULL
                             REFERENCES spedizioni (id)          ON UPDATE CASCADE ON DELETE RESTRICT,

    CONSTRAINT ck_cambiamenti_autore CHECK (
        num_nonnulls(id_corriere, id_operatore) = 1
    ),
    --check su annullamento e creazione che non le può fare un corriere
    CONSTRAINT ck_cambiamenti_autore_operazione CHECK (
        operazione = 'modifica' OR id_operatore IS NOT NULL
    ),
    -- mi assicuro che ci siano json chiave -> valore ad es. stato-> consegnata
    CONSTRAINT ck_cambiamenti_json_oggetti CHECK (
        jsonb_typeof(valore_nuovo) = 'object'
        AND (valore_vecchio IS NULL OR jsonb_typeof(valore_vecchio) = 'object')
    ),
    CONSTRAINT ck_cambiamenti_valori CHECK (
        CASE operazione
            WHEN 'creazione' THEN valore_vecchio IS NULL
            ELSE valore_vecchio IS NOT NULL AND valore_vecchio <> valore_nuovo
        END
    )
);

-- controllo se non ci siano piu stesse spedizioni con stato creazione
CREATE UNIQUE INDEX uq_cambiamenti_creazione
    ON cambiamenti_spedizioni (id_spedizione)
    WHERE operazione = 'creazione';





CREATE FUNCTION fn_traccia_cambiamenti_spedizioni()
RETURNS trigger --funzione che si può collegare solo ad un trigger
LANGUAGE plpgsql --definiscon il linguaggio che voglio usare per la funzione questo è il linguaggio procedurale di postgres
AS $$ --i dollari delimitano il corpo della funzione
--dichiaro delle variabili
DECLARE
    v_operazione   tipo_operazione_spedizione;
    v_id_corriere  INTEGER;
    v_id_operatore INTEGER;
    v_vecchio      JSONB;
    v_nuovo        JSONB;
    --new e old contengono i valori vecchi e nuovi dell operazione
BEGIN
    IF TG_OP = 'INSERT' THEN
        v_operazione   := 'creazione';
        v_id_operatore := NEW.id_operatore_ha_creato;
        v_nuovo        := to_jsonb(NEW);
    ELSE
        --jsonb_each(to_jsonb(OLD)) trasforma la riga vecchia in una tabella con una riga per campo
        --creo una tabella con solo le modifiche che ha subito la spezione attributo valore nuovo
        -- e attributo valore vecchio visualizzati come due oggetti json e poi salvo i valori nuovo e vecchio
        -- in due variabili v_vecchio e v_nuovo
        SELECT jsonb_object_agg(o.key, o.value),
               jsonb_object_agg(o.key, n.value)
          INTO v_vecchio, v_nuovo
          FROM jsonb_each(to_jsonb(OLD)) AS o
          JOIN jsonb_each(to_jsonb(NEW)) AS n ON n.key = o.key
         WHERE o.value IS DISTINCT FROM n.value;

        --se non ci sono nuove modifiche ma rimodifico con gli stessi valori gli attributi esco
        IF v_nuovo IS NULL THEN
            RETURN NEW;
        END IF;

        --salvo nello storico
        IF OLD.annullata_il IS NULL AND NEW.annullata_il IS NOT NULL THEN
            v_operazione   := 'annullamento';
            v_id_operatore := NEW.id_operatore_ha_annullato;
        ELSE
            --setto le variabili per contenere il corriere o l'operatore corretto
            v_operazione   := 'modifica';
            v_id_corriere  := NULLIF(current_setting('app.id_corriere',  true), '')::INTEGER;
            v_id_operatore := NULLIF(current_setting('app.id_operatore', true), '')::INTEGER;
            --num_nonnulls(…) conta quanti argomenti non sono NULL
            IF num_nonnulls(v_id_corriere, v_id_operatore) <> 1 THEN
                RAISE EXCEPTION 'Autore della modifica non valido per la spedizione %', NEW.id
                    USING HINT = 'Impostare uno solo tra app.id_operatore e app.id_corriere '
                                 'con SET LOCAL prima dell''UPDATE.';
            END IF;
        END IF;
    END IF;

    INSERT INTO cambiamenti_spedizioni
        (operazione, valore_vecchio, valore_nuovo,
         id_corriere, id_operatore, id_spedizione)
    VALUES
        (v_operazione, v_vecchio, v_nuovo,
         v_id_corriere, v_id_operatore, NEW.id);

    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_spedizioni_storico_insert
AFTER INSERT ON spedizioni
FOR EACH ROW
EXECUTE FUNCTION fn_traccia_cambiamenti_spedizioni();

CREATE TRIGGER trg_spedizioni_storico_update
AFTER UPDATE ON spedizioni
FOR EACH ROW
WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION fn_traccia_cambiamenti_spedizioni();


-- Una spedizione si può annullare solo se è ancora 'creata' o 'assegnata'.
-- Serve un trigger perché un CHECK vede solo la riga nuova, non lo stato precedente.
CREATE FUNCTION fn_controlla_annullamento()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.stato NOT IN ('creata', 'assegnata') THEN
        RAISE EXCEPTION 'La spedizione % non può essere annullata: è nello stato "%"', NEW.id, OLD.stato
            USING HINT = 'Si possono annullare solo spedizioni in stato "creata" o "assegnata".';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_spedizioni_controlla_annullamento
BEFORE UPDATE OF stato ON spedizioni
FOR EACH ROW
WHEN (NEW.stato = 'annullata' AND OLD.stato <> 'annullata')
EXECUTE FUNCTION fn_controlla_annullamento();


-- Lo storico è in sola scrittura: le righe non si possono modificare né cancellare.
CREATE FUNCTION fn_blocca_modifica_storico()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'Lo storico delle spedizioni non può essere modificato o cancellato';
END;
$$;

CREATE TRIGGER trg_cambiamenti_spedizioni_immutabile
BEFORE UPDATE OR DELETE ON cambiamenti_spedizioni
FOR EACH ROW
EXECUTE FUNCTION fn_blocca_modifica_storico();



COMMIT;
