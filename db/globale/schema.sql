-- Database GLOBALE (servizio utenti): utenti e ruoli, sedi, indirizzi e geografia.
-- Lo usano il login e l'amministrazione. Le spedizioni sono in un altro database
-- (db/spedizioni/schema.sql, servizio spedizioni): tra i due non ci sono chiavi esterne,
-- i servizi si chiedono i dati con le rotte /interno (backend/internal/interno).
BEGIN;

DROP TABLE IF EXISTS chiavi_idempotenza        CASCADE;
DROP TABLE IF EXISTS sessioni                  CASCADE;
DROP TABLE IF EXISTS recuperi_password         CASCADE;
DROP TABLE IF EXISTS amministratori            CASCADE;
DROP TABLE IF EXISTS operatori_logistici       CASCADE;
DROP TABLE IF EXISTS corrieri                  CASCADE;
DROP TABLE IF EXISTS clienti                   CASCADE;
DROP TABLE IF EXISTS sedi                      CASCADE;
DROP TABLE IF EXISTS indirizzi                 CASCADE;
DROP TABLE IF EXISTS citta                     CASCADE;
DROP TABLE IF EXISTS regioni                   CASCADE;
DROP TABLE IF EXISTS nazioni                   CASCADE;
DROP TABLE IF EXISTS utenti                    CASCADE;

DROP FUNCTION IF EXISTS fn_un_solo_ruolo() CASCADE;
DROP FUNCTION IF EXISTS fn_ruolo_immutabile() CASCADE;
DROP FUNCTION IF EXISTS fn_gruppo_immutabile() CASCADE;


-- Tutte le chiavi primarie ed esterne sono BIGINT, come nel database delle spedizioni
-- che le usa come riferimenti.

CREATE TABLE utenti (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nome         VARCHAR(100) NOT NULL,
    cognome      VARCHAR(100) NOT NULL,
    prefisso     VARCHAR(5)   NOT NULL,          -- es. '+39'
    nr_telefono  VARCHAR(20)  NOT NULL,
    email        VARCHAR(254) NOT NULL,
    -- hash bcrypt della password, obbligatorio: ogni utente nasce con la sua password
    pwd_hash     VARCHAR(255) NOT NULL,
    -- l'amministratore può disattivare un utente: non entra più e i suoi token non valgono
    attivo       BOOLEAN      NOT NULL DEFAULT TRUE,
    CONSTRAINT uq_utenti_email    UNIQUE (email),
    CONSTRAINT uq_utenti_telefono UNIQUE (prefisso, nr_telefono),
    CONSTRAINT ck_utenti_prefisso CHECK (prefisso ~ '^\+[0-9]{1,4}$'),
    CONSTRAINT ck_utenti_telefono CHECK (nr_telefono ~ '^[0-9]{4,15}$'),
    CONSTRAINT ck_utenti_email    CHECK (email ~* '^[^@\s]+@[^@\s]+\.[^@\s]+$'),
    CONSTRAINT ck_utenti_pwd_hash CHECK (pwd_hash <> '')
);


-- ===== Geografia =====

CREATE TABLE nazioni (
    id    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nome  VARCHAR(100) NOT NULL UNIQUE
);

CREATE TABLE regioni (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nome        VARCHAR(100) NOT NULL,
    id_nazione  BIGINT NOT NULL
                REFERENCES nazioni (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT uq_regioni_nome_nazione UNIQUE (nome, id_nazione)
);

-- Una città è nome + regione. Il CAP sta sull'indirizzo: una città grande ne ha molti.
CREATE TABLE citta (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nome         VARCHAR(100) NOT NULL,
    id_regione   BIGINT NOT NULL
                 REFERENCES regioni (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT uq_citta UNIQUE (nome, id_regione)
);

-- Ogni indirizzo sta in UNA città (prima il legame era molti-a-molti).
CREATE TABLE indirizzi (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    via          VARCHAR(150) NOT NULL,
    civico       VARCHAR(10)  NOT NULL,
    cod_postale  VARCHAR(10)  NOT NULL,
    id_citta     BIGINT NOT NULL
                 REFERENCES citta (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    -- serve alla chiave esterna composta (indirizzo, città) delle sedi
    CONSTRAINT uq_indirizzi_id_citta UNIQUE (id, id_citta)
);

CREATE UNIQUE INDEX uq_indirizzi_via_civico
    ON indirizzi (id_citta, lower(via), lower(civico));


-- ===== Sedi =====

-- Una sede operativa per città: spedizioni, operatori e corrieri appartengono a una sede
-- e le consegne restano dentro la sua città. Le impostazioni della sede (tentativi,
-- giorni di visibilità, intervallo della posizione) sono nel database delle spedizioni.
CREATE TABLE sedi (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nome          VARCHAR(100) NOT NULL,
    id_citta      BIGINT NOT NULL
                  REFERENCES citta (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    id_indirizzo  BIGINT NOT NULL,
    -- una sede disattivata non accetta nuove spedizioni né nuovi corrieri o operatori;
    -- chi ci lavora porta a termine le spedizioni già aperte
    attiva        BOOLEAN NOT NULL DEFAULT TRUE,
    -- mappa sede → database delle spedizioni: il gruppo di sedi il cui database tiene le
    -- spedizioni di questa sede (0 = il database principale). Non cambia dopo la
    -- creazione: le spedizioni della sede sono in quel database (fn_gruppo_immutabile)
    gruppo_spedizioni SMALLINT NOT NULL DEFAULT 0,
    creata_il     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_sedi_gruppo_spedizioni CHECK (gruppo_spedizioni BETWEEN 0 AND 9006),
    CONSTRAINT uq_sedi_citta    UNIQUE (id_citta),
    CONSTRAINT uq_sedi_id_citta UNIQUE (id, id_citta),
    CONSTRAINT ck_sedi_nome     CHECK (btrim(nome) <> ''),
    -- l'indirizzo della sede è nella città della sede
    CONSTRAINT fk_sedi_indirizzo_stessa_citta
        FOREIGN KEY (id_indirizzo, id_citta)
        REFERENCES indirizzi (id, id_citta) ON UPDATE CASCADE ON DELETE RESTRICT
);

CREATE UNIQUE INDEX uq_sedi_nome ON sedi (lower(nome));

-- Il gruppo di una sede non si cambia con un UPDATE: le sue spedizioni stanno nel database
-- di quel gruppo e il servizio spedizioni tiene la mappa in memoria. Spostare una sede in
-- un altro gruppo è una migrazione dei dati (docs/microservizi.md).
CREATE FUNCTION fn_gruppo_immutabile()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'Il gruppo delle spedizioni della sede % non si cambia', OLD.id
        USING ERRCODE = 'check_violation', CONSTRAINT = 'ck_sedi_gruppo_immutabile';
END;
$$;

CREATE TRIGGER trg_sedi_gruppo_immutabile
BEFORE UPDATE OF gruppo_spedizioni ON sedi
FOR EACH ROW WHEN (OLD.gruppo_spedizioni IS DISTINCT FROM NEW.gruppo_spedizioni)
EXECUTE FUNCTION fn_gruppo_immutabile();


-- ===== Ruoli =====

CREATE TABLE clienti (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_utente     BIGINT NOT NULL UNIQUE
                  REFERENCES utenti (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    -- il cliente ha ricevuto almeno una spedizione: solo allora può entrare a seguirla.
    -- Le spedizioni sono nell'altro database: lo segna il servizio spedizioni quando il
    -- cliente diventa destinatario (POST /interno/clienti/{id}/destinatario)
    destinatario  BOOLEAN NOT NULL DEFAULT FALSE
);

-- Corrieri e operatori lavorano per una sede; clienti e amministratori no.
CREATE TABLE corrieri (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_utente  BIGINT NOT NULL UNIQUE
               REFERENCES utenti (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    id_sede    BIGINT NOT NULL
               REFERENCES sedi (id) ON UPDATE CASCADE ON DELETE RESTRICT
);

CREATE INDEX ix_corrieri_sede ON corrieri (id_sede);

CREATE TABLE operatori_logistici (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_utente  BIGINT NOT NULL UNIQUE
               REFERENCES utenti (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    id_sede    BIGINT NOT NULL
               REFERENCES sedi (id) ON UPDATE CASCADE ON DELETE RESTRICT
);

CREATE INDEX ix_operatori_sede ON operatori_logistici (id_sede);

CREATE TABLE amministratori (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_utente  BIGINT NOT NULL UNIQUE
               REFERENCES utenti (id) ON UPDATE CASCADE ON DELETE RESTRICT
);

-- Un utente (cioè un'email) ha un solo ruolo: non può essere insieme cliente, corriere,
-- operatore o amministratore. Il blocco sulla riga dell'utente mette in fila due
-- inserimenti contemporanei nelle tabelle di ruoli diversi.
CREATE FUNCTION fn_un_solo_ruolo()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM 1 FROM utenti WHERE id = NEW.id_utente FOR UPDATE;
    IF (SELECT count(*) FROM (
            SELECT id_utente FROM clienti             WHERE id_utente = NEW.id_utente
            UNION ALL
            SELECT id_utente FROM corrieri            WHERE id_utente = NEW.id_utente
            UNION ALL
            SELECT id_utente FROM operatori_logistici WHERE id_utente = NEW.id_utente
            UNION ALL
            SELECT id_utente FROM amministratori      WHERE id_utente = NEW.id_utente
        ) AS ruoli) > 1 THEN
        RAISE EXCEPTION 'L''utente % ha già un ruolo: ogni email può avere un solo ruolo', NEW.id_utente
            USING ERRCODE = 'unique_violation', CONSTRAINT = 'uq_utenti_un_solo_ruolo';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_clienti_un_solo_ruolo
AFTER INSERT OR UPDATE OF id_utente ON clienti
FOR EACH ROW EXECUTE FUNCTION fn_un_solo_ruolo();

CREATE TRIGGER trg_corrieri_un_solo_ruolo
AFTER INSERT OR UPDATE OF id_utente ON corrieri
FOR EACH ROW EXECUTE FUNCTION fn_un_solo_ruolo();

CREATE TRIGGER trg_operatori_un_solo_ruolo
AFTER INSERT OR UPDATE OF id_utente ON operatori_logistici
FOR EACH ROW EXECUTE FUNCTION fn_un_solo_ruolo();

CREATE TRIGGER trg_amministratori_un_solo_ruolo
AFTER INSERT OR UPDATE OF id_utente ON amministratori
FOR EACH ROW EXECUTE FUNCTION fn_un_solo_ruolo();



-- ===== Recupero password =====

-- Una riga per ogni richiesta di recupero: nel database c'è solo l'hash SHA-256
-- del token inviato per mail, così chi legge la tabella non può usarlo.
CREATE TABLE recuperi_password (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_utente   BIGINT NOT NULL
                REFERENCES utenti (id) ON UPDATE CASCADE ON DELETE CASCADE,
    token_hash  CHAR(64)    NOT NULL,
    creato_il   TIMESTAMPTZ NOT NULL DEFAULT now(),
    scade_il    TIMESTAMPTZ NOT NULL,
    usato_il    TIMESTAMPTZ NULL,
    CONSTRAINT uq_recuperi_password_token UNIQUE (token_hash),
    CONSTRAINT ck_recuperi_password_scadenza CHECK (scade_il > creato_il)
);

CREATE INDEX ix_recuperi_password_utente ON recuperi_password (id_utente);


-- ===== Sessioni =====

-- Una riga per ogni login. Il token porta l'id della sessione (claim "sid") e vale solo
-- finché la sessione è aperta: il servizio utenti la controlla qui a ogni richiesta, il
-- servizio spedizioni riceve le sessioni chiuse (tabella sessioni_revocate del suo
-- database). Si chiude con il logout, la disattivazione dell'utente, un cambio di sede o
-- una password nuova; scade dopo DURATA_SESSIONE dal login.
CREATE TABLE sessioni (
    id             CHAR(32)     PRIMARY KEY,                 -- 128 bit casuali
    id_utente      BIGINT       NOT NULL
                   REFERENCES utenti (id) ON UPDATE CASCADE ON DELETE CASCADE,
    ruolo          VARCHAR(20)  NOT NULL,
    id_ruolo       BIGINT       NOT NULL,                    -- id nella tabella del ruolo
    -- sede del login (operatori e corrieri), la stessa del token: la revoca va al database
    -- delle spedizioni del suo gruppo, l'unico che controlla quel token
    id_sede        BIGINT       NULL,
    creata_il      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    rinnovata_il   TIMESTAMPTZ  NULL,
    scade_il       TIMESTAMPTZ  NOT NULL,
    revocata_il    TIMESTAMPTZ  NULL,
    motivo_revoca  VARCHAR(50)  NULL,
    CONSTRAINT ck_sessioni_ruolo  CHECK (ruolo IN ('operatore', 'corriere', 'cliente', 'amministratore')),
    CONSTRAINT ck_sessioni_scade  CHECK (scade_il > creata_il),
    CONSTRAINT ck_sessioni_revoca CHECK ((revocata_il IS NULL) = (motivo_revoca IS NULL))
);

CREATE INDEX ix_sessioni_utente_aperte ON sessioni (id_utente) WHERE revocata_il IS NULL;
CREATE INDEX ix_sessioni_scadenza ON sessioni (scade_il);


-- Il ruolo di un utente non si cambia e non si toglie (né dall'API né con un UPDATE
-- diretto): un utente che non deve più lavorare si disattiva. Senza questo controllo,
-- spostare una riga da corrieri ad amministratori darebbe a un corriere i permessi di
-- amministratore.
CREATE FUNCTION fn_ruolo_immutabile()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'Il ruolo di un utente non si cambia né si toglie (tabella %): si disattiva l''utente', TG_TABLE_NAME
        USING ERRCODE = 'restrict_violation';
END;
$$;

CREATE TRIGGER trg_clienti_ruolo_immutabile
BEFORE UPDATE OF id, id_utente OR DELETE ON clienti
FOR EACH ROW EXECUTE FUNCTION fn_ruolo_immutabile();

CREATE TRIGGER trg_corrieri_ruolo_immutabile
BEFORE UPDATE OF id, id_utente OR DELETE ON corrieri
FOR EACH ROW EXECUTE FUNCTION fn_ruolo_immutabile();

CREATE TRIGGER trg_operatori_ruolo_immutabile
BEFORE UPDATE OF id, id_utente OR DELETE ON operatori_logistici
FOR EACH ROW EXECUTE FUNCTION fn_ruolo_immutabile();

CREATE TRIGGER trg_amministratori_ruolo_immutabile
BEFORE UPDATE OF id, id_utente OR DELETE ON amministratori
FOR EACH ROW EXECUTE FUNCTION fn_ruolo_immutabile();


-- ===== Idempotenza =====

-- Le risposte alle richieste con Idempotency-Key. Stanno nel database, non nella memoria
-- del backend: così una richiesta ripetuta trova la risposta anche se arriva a un'altra
-- replica o dopo un riavvio. stato_http NULL = richiesta ancora in corso.
CREATE TABLE chiavi_idempotenza (
    chiave        TEXT        PRIMARY KEY,   -- ruolo:id:METODO percorso:Idempotency-Key
    hash_corpo    BYTEA       NOT NULL,
    stato_http    INTEGER     NULL,
    intestazioni  JSONB       NULL,
    corpo         BYTEA       NULL,
    creata_il     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_chiavi_idempotenza_stato CHECK (stato_http IS NULL OR stato_http BETWEEN 200 AND 299)
);

CREATE INDEX ix_chiavi_idempotenza_creata ON chiavi_idempotenza (creata_il);


COMMIT;
