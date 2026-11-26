-- Database delle SPEDIZIONI (servizio spedizioni): spedizioni, assegnazioni, note,
-- tentativi, storico, impostazioni delle sedi, motivi e ultima posizione dei corrieri.
--
-- Sedi, clienti, corrieri e operatori sono nel database globale (servizio utenti): qui ci
-- sono solo i loro id, senza chiave esterna. Che esistano e che siano attivi lo controlla
-- il servizio spedizioni chiedendolo al servizio utenti (POST /interno/risolvi) prima di
-- scrivere. Gli indirizzi di ritiro e consegna invece sono qui (tabella indirizzi), e la
-- regola "ritiro e consegna nella città della sede" è una chiave esterna.
BEGIN;

DROP TABLE IF EXISTS gruppo_database           CASCADE;
DROP TABLE IF EXISTS chiavi_idempotenza        CASCADE;
DROP TABLE IF EXISTS sessioni_revocate         CASCADE;
DROP TABLE IF EXISTS ultime_posizioni_corrieri CASCADE;
DROP TABLE IF EXISTS impostazioni              CASCADE;
DROP TABLE IF EXISTS motivi                    CASCADE;
DROP TABLE IF EXISTS cambiamenti_spedizioni    CASCADE;
DROP TABLE IF EXISTS tentativi                 CASCADE;
DROP TABLE IF EXISTS note                      CASCADE;
DROP TABLE IF EXISTS spedizioni                CASCADE;
DROP TABLE IF EXISTS assegnazioni              CASCADE;
DROP TABLE IF EXISTS indirizzi                 CASCADE;

DROP FUNCTION IF EXISTS fn_imposta_gruppo(INTEGER) CASCADE;

DROP FUNCTION IF EXISTS fn_traccia_cambiamenti_spedizioni() CASCADE;
DROP FUNCTION IF EXISTS fn_blocca_modifica_storico()        CASCADE;
DROP FUNCTION IF EXISTS fn_controlla_annullamento()         CASCADE;
DROP FUNCTION IF EXISTS fn_sede_visibile(BIGINT)            CASCADE;

DROP TYPE IF EXISTS tipo_operazione_spedizione CASCADE;
DROP TYPE IF EXISTS stato_spedizione           CASCADE;
DROP TYPE IF EXISTS tipo_tentativo             CASCADE;


-- Tutte le chiavi primarie ed esterne sono BIGINT: con molte città le tabelle più
-- grandi (storico, note) supererebbero i 2,1 miliardi di righe di un INTEGER.

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



-- ===== Spedizioni =====

CREATE TABLE assegnazioni (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- sede, operatore e corriere sono nel database globale: qui solo l'id
    id_sede      BIGINT NOT NULL,
    id_operatore BIGINT NOT NULL,
    id_corriere  BIGINT NOT NULL,
    creata_il    TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- serve alla chiave esterna composta (assegnazione, sede) di spedizioni
    CONSTRAINT uq_assegnazioni_id_sede UNIQUE (id, id_sede)
);

CREATE INDEX ix_assegnazioni_corriere ON assegnazioni (id_corriere);


-- ===== Indirizzi di ritiro e consegna =====

-- Gli indirizzi delle spedizioni stanno qui, nel database del gruppo della loro sede: li
-- crea l'operatore (POST /api/indirizzi), sempre nella città della sua sede, crescono con
-- le spedizioni e non si modificano. Città, regione e nazione sono quelle della sede (nel
-- database globale, con l'indirizzo della sede stessa): qui c'è l'id della città.
CREATE TABLE indirizzi (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_sede      BIGINT       NOT NULL,
    id_citta     BIGINT       NOT NULL,
    via          VARCHAR(150) NOT NULL,
    civico       VARCHAR(10)  NOT NULL,
    cod_postale  VARCHAR(10)  NOT NULL,
    creato_il    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    -- serve alle chiavi esterne composte (indirizzo, sede) delle spedizioni
    CONSTRAINT uq_indirizzi_id_sede UNIQUE (id, id_sede)
);

-- lo stesso indirizzo una volta sola per sede (cioè per città)
CREATE UNIQUE INDEX uq_indirizzi_via_civico ON indirizzi (id_sede, lower(via), lower(civico));


CREATE TABLE spedizioni (
    id                            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- la sede che gestisce la spedizione e la sua città: ritiro e consegna sono lì.
    -- Sede, città, clienti, operatori e indirizzi sono nel database globale: qui solo
    -- gli id; li controlla il servizio spedizioni prima di scrivere
    id_sede                       BIGINT NOT NULL,
    id_citta                      BIGINT NOT NULL,
    stato                         stato_spedizione NOT NULL DEFAULT 'creata',
    cod_ordine                    VARCHAR(50) NOT NULL,
    is_reso                       BOOLEAN     NOT NULL DEFAULT FALSE,
    id_operatore_ha_annullato     BIGINT NULL,
    annullata_il                  TIMESTAMPTZ NULL,
    id_operatore_ha_creato        BIGINT NOT NULL,
    creata_il                     TIMESTAMPTZ NOT NULL DEFAULT now(),
    id_cliente_mittente           BIGINT NOT NULL,
    id_cliente_destinatario       BIGINT NOT NULL,
    -- data e ora richieste per la spedizione: le imposta l'operatore
    dataora_richiesta_spedizione  TIMESTAMPTZ NOT NULL,
    -- data e ora del ritiro: le registra il sistema quando il corriere porta la
    -- spedizione in 'ritirata' (resta quella del primo ritiro)
    ritirata_il                   TIMESTAMPTZ NULL,
    id_indirizzo_ritiro           BIGINT NOT NULL,
    id_indirizzo_spedizione       BIGINT NOT NULL,
    id_assegnazione_corrente      BIGINT NULL,

    CONSTRAINT uq_spedizioni_cod_ordine          UNIQUE (cod_ordine),
    CONSTRAINT uq_spedizioni_assegnazione_corr   UNIQUE (id_assegnazione_corrente),
    -- serve alle chiavi esterne composte (spedizione, sede) di note, tentativi e storico
    CONSTRAINT uq_spedizioni_id_sede             UNIQUE (id, id_sede),

    -- ritiro e consegna sono indirizzi della sede della spedizione (cioè della sua città)
    CONSTRAINT fk_spedizioni_ritiro_stessa_sede
        FOREIGN KEY (id_indirizzo_ritiro, id_sede) REFERENCES indirizzi (id, id_sede),
    CONSTRAINT fk_spedizioni_consegna_stessa_sede
        FOREIGN KEY (id_indirizzo_spedizione, id_sede) REFERENCES indirizzi (id, id_sede),

    -- l'assegnazione corrente è della stessa sede
    CONSTRAINT fk_spedizioni_assegnazione_stessa_sede
        FOREIGN KEY (id_assegnazione_corrente, id_sede)
        REFERENCES assegnazioni (id, id_sede) ON UPDATE CASCADE
        ON DELETE SET NULL (id_assegnazione_corrente),

    -- annullamento: operatore e data presenti entrambi o assenti entrambi
    CONSTRAINT ck_spedizioni_annullamento        CHECK (
        (id_operatore_ha_annullato IS NULL) = (annullata_il IS NULL)
    ),
    -- la data di annullamento c'è se e solo se lo stato è 'annullata'
    CONSTRAINT ck_spedizioni_annullamento_stato  CHECK (
        (stato = 'annullata') = (annullata_il IS NOT NULL)
    ),
    CONSTRAINT ck_spedizioni_date_annullamento   CHECK (annullata_il IS NULL OR annullata_il >= creata_il),
    CONSTRAINT ck_spedizioni_date_ritiro         CHECK (ritirata_il IS NULL OR ritirata_il >= creata_il),
    -- prima del ritiro la data del ritiro non c'è
    CONSTRAINT ck_spedizioni_ritiro_stato        CHECK (stato NOT IN ('creata', 'assegnata') OR ritirata_il IS NULL),
    -- da 'assegnata' in poi serve l'assegnazione (e quindi il corriere);
    -- possono non averla solo le spedizioni 'creata' e 'annullata'
    CONSTRAINT ck_spedizioni_assegnazione_stato  CHECK (
        stato IN ('creata', 'annullata') OR id_assegnazione_corrente IS NOT NULL
    )
);

-- elenchi per sede, filtrati per stato e ordinati dalla più recente
CREATE INDEX ix_spedizioni_sede_stato ON spedizioni (id_sede, stato, creata_il DESC);
CREATE INDEX ix_spedizioni_sede_creata ON spedizioni (id_sede, creata_il DESC, id DESC);
CREATE INDEX ix_spedizioni_destinatario ON spedizioni (id_cliente_destinatario);


CREATE TABLE note (
    id                             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_sede                        BIGINT NOT NULL,
    nota                           TEXT        NOT NULL,
    is_interna                     BOOLEAN     NOT NULL DEFAULT FALSE,
    motivo                         VARCHAR(100) NULL,
    creata_il                      TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- autore: un operatore o un corriere (id del database globale)
    id_operatore_logistico_creata  BIGINT NULL,
    id_corriere_creata             BIGINT NULL,
    id_spedizione                  BIGINT NOT NULL,
    CONSTRAINT fk_note_spedizione
        FOREIGN KEY (id_spedizione, id_sede)
        REFERENCES spedizioni (id, id_sede) ON UPDATE CASCADE ON DELETE CASCADE,
    -- esattamente uno tra operatore e corriere
    CONSTRAINT ck_note_autore CHECK (
        num_nonnulls(id_operatore_logistico_creata, id_corriere_creata) = 1
    ),
    -- serve alla chiave esterna composta di tentativi (nota + spedizione)
    CONSTRAINT uq_note_id_spedizione UNIQUE (id, id_spedizione)
);

CREATE INDEX ix_note_spedizione ON note (id_spedizione);


CREATE TABLE tentativi (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_sede        BIGINT NOT NULL,
    tipo           tipo_tentativo NOT NULL,       -- era "isConsegna"
    effettuato_il  TIMESTAMPTZ    NOT NULL DEFAULT now(),
    id_spedizione  BIGINT NOT NULL,
    id_nota        BIGINT NOT NULL,
    CONSTRAINT fk_tentativi_spedizione
        FOREIGN KEY (id_spedizione, id_sede)
        REFERENCES spedizioni (id, id_sede) ON UPDATE CASCADE ON DELETE RESTRICT,
    -- la nota del tentativo deve essere della stessa spedizione del tentativo
    CONSTRAINT fk_tentativi_nota_stessa_spedizione
        FOREIGN KEY (id_nota, id_spedizione)
        REFERENCES note (id, id_spedizione) ON UPDATE CASCADE ON DELETE RESTRICT
);

CREATE INDEX ix_tentativi_spedizione ON tentativi (id_spedizione, tipo);


CREATE TABLE cambiamenti_spedizioni (
    id                       BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_sede                  BIGINT NOT NULL,
    operazione               tipo_operazione_spedizione NOT NULL,
    valore_vecchio           JSONB NULL,
    valore_nuovo             JSONB NOT NULL,
    modificato_il            TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- autore: un corriere o un operatore (id del database globale)
    id_corriere              BIGINT NULL,
    id_operatore             BIGINT NULL,
    id_spedizione            BIGINT NOT NULL,

    CONSTRAINT fk_cambiamenti_spedizione
        FOREIGN KEY (id_spedizione, id_sede)
        REFERENCES spedizioni (id, id_sede) ON UPDATE CASCADE ON DELETE RESTRICT,
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

CREATE INDEX ix_cambiamenti_spedizione ON cambiamenti_spedizioni (id_spedizione, modificato_il);




CREATE FUNCTION fn_traccia_cambiamenti_spedizioni()
RETURNS trigger --funzione che si può collegare solo ad un trigger
LANGUAGE plpgsql --definiscon il linguaggio che voglio usare per la funzione questo è il linguaggio procedurale di postgres
AS $$ --i dollari delimitano il corpo della funzione
--dichiaro delle variabili
DECLARE
    v_operazione   tipo_operazione_spedizione;
    v_id_corriere  BIGINT;
    v_id_operatore BIGINT;
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
            v_id_corriere  := NULLIF(current_setting('app.id_corriere',  true), '')::BIGINT;
            v_id_operatore := NULLIF(current_setting('app.id_operatore', true), '')::BIGINT;
            --num_nonnulls(…) conta quanti argomenti non sono NULL
            IF num_nonnulls(v_id_corriere, v_id_operatore) <> 1 THEN
                RAISE EXCEPTION 'Autore della modifica non valido per la spedizione %', NEW.id
                    USING HINT = 'Impostare uno solo tra app.id_operatore e app.id_corriere '
                                 'con SET LOCAL prima dell''UPDATE.';
            END IF;
        END IF;
    END IF;

    -- lo storico ha la stessa sede della spedizione
    INSERT INTO cambiamenti_spedizioni
        (id_sede, operazione, valore_vecchio, valore_nuovo,
         id_corriere, id_operatore, id_spedizione)
    VALUES
        (NEW.id_sede, v_operazione, v_vecchio, v_nuovo,
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




-- ===== Configurazione dell'amministratore =====

-- Impostazioni di ogni sede, modificabili dall'amministratore: una riga per sede. La sede
-- nasce nel database globale; la sua riga qui la crea il servizio spedizioni la prima
-- volta che servono le impostazioni (con i valori predefiniti).
CREATE TABLE impostazioni (
    id_sede                     BIGINT PRIMARY KEY,
    -- ogni quanti secondi il corriere invia la posizione (RF16)
    intervallo_posizione_sec    INTEGER NOT NULL DEFAULT 30,
    -- per quanti giorni il cliente destinatario vede una spedizione chiusa
    giorni_visibilita_cliente   INTEGER NOT NULL DEFAULT 30,
    -- tentativi falliti oltre i quali la spedizione va annullata (ritiro)
    -- o diventa 'non consegnata' (consegna)
    max_tentativi_ritiro        INTEGER NOT NULL DEFAULT 3,
    max_tentativi_consegna      INTEGER NOT NULL DEFAULT 3,
    modificate_il               TIMESTAMPTZ NOT NULL DEFAULT now(),
    id_amministratore_modifica  BIGINT NULL,
    -- sotto i 5 secondi il database si riempie, sopra l'ora la posizione non serve più
    CONSTRAINT ck_impostazioni_intervallo CHECK (intervallo_posizione_sec BETWEEN 5 AND 3600),
    CONSTRAINT ck_impostazioni_giorni     CHECK (giorni_visibilita_cliente BETWEEN 1 AND 365),
    CONSTRAINT ck_impostazioni_ritiro     CHECK (max_tentativi_ritiro BETWEEN 1 AND 10),
    CONSTRAINT ck_impostazioni_consegna   CHECK (max_tentativi_consegna BETWEEN 1 AND 10)
);

-- Elenco dei motivi pubblici di tentativi falliti, annullamenti e perdite: è unico
-- per tutte le sedi. Un motivo non si cancella né si rinomina: si disattiva, così le
-- note che lo usano restano leggibili. Le note salvano il testo del motivo (note.motivo).
CREATE TABLE motivi (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    descrizione  VARCHAR(100) NOT NULL,
    attivo       BOOLEAN      NOT NULL DEFAULT TRUE,
    creato_il    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT ck_motivi_descrizione CHECK (btrim(descrizione) <> '')
);

CREATE UNIQUE INDEX uq_motivi_descrizione ON motivi (lower(descrizione));

INSERT INTO motivi (descrizione) VALUES
    ('mittente assente'),
    ('pacco non pronto'),
    ('indirizzo di ritiro errato'),
    ('destinatario assente'),
    ('indirizzo errato'),
    ('rifiutata dal destinatario'),
    ('accesso impossibile'),
    ('richiesta del mittente'),
    ('richiesta del destinatario'),
    ('dati della spedizione errati'),
    ('ritiro non riuscito'),
    ('smarrita in deposito'),
    ('smarrita durante il trasporto'),
    ('furto'),
    ('altro');


-- ===== Posizione dei corrieri (RF16) =====

-- Solo l'ULTIMA posizione di ogni corriere: una riga per corriere, aggiornata a ogni
-- invio. La tabella non cresce con il tempo (prima si teneva tutto lo storico).
CREATE TABLE ultime_posizioni_corrieri (
    id_corriere  BIGINT PRIMARY KEY,
    id_sede      BIGINT NOT NULL,
    latitudine   DOUBLE PRECISION NOT NULL,
    longitudine  DOUBLE PRECISION NOT NULL,
    rilevata_il  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_posizioni_latitudine  CHECK (latitudine  BETWEEN -90  AND 90),
    CONSTRAINT ck_posizioni_longitudine CHECK (longitudine BETWEEN -180 AND 180)
);

CREATE INDEX ix_ultime_posizioni_sede ON ultime_posizioni_corrieri (id_sede);


-- ===== Sessioni chiuse =====

-- Le sessioni che il servizio utenti ha chiuso (logout, disattivazione, cambio di sede,
-- password nuova): il servizio spedizioni rifiuta i loro token. Il servizio utenti le
-- comunica prima di confermare l'operazione. Le righe oltre scade_il si cancellano: dopo
-- la fine della sessione il token non varrebbe comunque.
CREATE TABLE sessioni_revocate (
    id        CHAR(32)    PRIMARY KEY,
    scade_il  TIMESTAMPTZ NOT NULL,
    motivo    VARCHAR(50) NOT NULL,
    creata_il TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ix_sessioni_revocate_scadenza ON sessioni_revocate (scade_il);


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


-- ===== Gruppo di sedi di questo database (mappa sede → database) =====

-- Le sedi sono divise in gruppi, ognuno con il suo database delle spedizioni (le città
-- grandi da sole, le piccole insieme): il gruppo di una sede è sedi.gruppo_spedizioni nel
-- database globale. Questa tabella dice quale gruppo tiene QUESTO database: il servizio
-- spedizioni la controlla all'avvio, così un URL sbagliato non mescola i dati di due
-- gruppi. Una sola riga.
CREATE TABLE gruppo_database (
    unica   BOOLEAN  NOT NULL DEFAULT TRUE PRIMARY KEY,
    codice  SMALLINT NOT NULL,
    CONSTRAINT ck_gruppo_database_unica  CHECK (unica),
    -- (codice + 1) × 10^12 resta sotto 2^53: gli id sono numeri esatti anche in JavaScript
    CONSTRAINT ck_gruppo_database_codice CHECK (codice BETWEEN 0 AND 9006)
);

INSERT INTO gruppo_database (codice) VALUES (0);

-- Imposta il gruppo di un database nuovo e fa partire gli id di ogni tabella da
-- gruppo × 10^12 + 1: dall'id di una spedizione si sa in quale database sta, e gli id non
-- si ripetono tra gruppi. Si esegue una volta, alla creazione del database
-- (db/gruppo_spedizioni.sh con GRUPPO_SPEDIZIONI), prima di qualunque dato.
CREATE FUNCTION fn_imposta_gruppo(p_codice INTEGER)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
    v_inizio   BIGINT := p_codice::BIGINT * 1000000000000;
    v_sequenza regclass;
    v_ultimo   BIGINT;
BEGIN
    IF p_codice NOT BETWEEN 0 AND 9006 THEN
        RAISE EXCEPTION 'gruppo % non valido: tra 0 e 9006', p_codice;
    END IF;
    -- spedizioni ha FORCE ROW LEVEL SECURITY: senza questa variabile un proprietario che
    -- non è superutente non vedrebbe nessuna riga e il controllo qui sotto non scatterebbe
    PERFORM set_config('app.tutte_le_sedi', 'si', true);
    IF EXISTS (SELECT 1 FROM spedizioni) AND (SELECT codice FROM gruppo_database) <> p_codice THEN
        RAISE EXCEPTION 'il database ha già delle spedizioni: il gruppo non si cambia';
    END IF;
    UPDATE gruppo_database SET codice = p_codice;
    FOR v_sequenza IN
        SELECT s.seqrelid::regclass
        FROM pg_sequence s
        JOIN pg_class c ON c.oid = s.seqrelid
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'public'
    LOOP
        EXECUTE format('SELECT last_value FROM %s', v_sequenza) INTO v_ultimo;
        IF v_ultimo < v_inizio THEN
            PERFORM setval(v_sequenza, v_inizio, true);  -- il prossimo id è v_inizio + 1
        END IF;
    END LOOP;
END;
$$;

REVOKE ALL ON FUNCTION fn_imposta_gruppo(INTEGER) FROM PUBLIC;


-- ===== Row-Level Security: ogni sede vede solo i suoi dati =====

-- È una rete di sicurezza: il backend filtra già per sede in ogni query. A ogni richiesta
-- il backend dichiara chi sta lavorando con due variabili di sessione:
--   app.tutte_le_sedi = 'si'  amministratori e clienti (i clienti non hanno una sede)
--   app.id_sede       = <id>  operatori e corrieri
-- Senza nessuna delle due (es. richiesta senza login) non si vede nessuna riga.
-- NB: un superutente e chi ha BYPASSRLS ignorano queste regole: il backend deve
-- collegarsi con un utente normale (vedi db/ruolo_applicazione.sh).
CREATE FUNCTION fn_sede_visibile(p_id_sede BIGINT)
RETURNS BOOLEAN
LANGUAGE sql
STABLE
AS $$
    SELECT current_setting('app.tutte_le_sedi', true) = 'si'
        OR p_id_sede = NULLIF(current_setting('app.id_sede', true), '')::BIGINT
$$;

ALTER TABLE spedizioni                ENABLE ROW LEVEL SECURITY;
ALTER TABLE assegnazioni              ENABLE ROW LEVEL SECURITY;
ALTER TABLE note                      ENABLE ROW LEVEL SECURITY;
ALTER TABLE tentativi                 ENABLE ROW LEVEL SECURITY;
ALTER TABLE cambiamenti_spedizioni    ENABLE ROW LEVEL SECURITY;
ALTER TABLE impostazioni              ENABLE ROW LEVEL SECURITY;
ALTER TABLE ultime_posizioni_corrieri ENABLE ROW LEVEL SECURITY;
ALTER TABLE indirizzi                 ENABLE ROW LEVEL SECURITY;

-- FORCE: le regole valgono anche per il proprietario delle tabelle
ALTER TABLE spedizioni                FORCE ROW LEVEL SECURITY;
ALTER TABLE assegnazioni              FORCE ROW LEVEL SECURITY;
ALTER TABLE note                      FORCE ROW LEVEL SECURITY;
ALTER TABLE tentativi                 FORCE ROW LEVEL SECURITY;
ALTER TABLE cambiamenti_spedizioni    FORCE ROW LEVEL SECURITY;
ALTER TABLE impostazioni              FORCE ROW LEVEL SECURITY;
ALTER TABLE ultime_posizioni_corrieri FORCE ROW LEVEL SECURITY;
ALTER TABLE indirizzi                 FORCE ROW LEVEL SECURITY;

CREATE POLICY pol_spedizioni_sede ON spedizioni
    USING (fn_sede_visibile(id_sede)) WITH CHECK (fn_sede_visibile(id_sede));
CREATE POLICY pol_assegnazioni_sede ON assegnazioni
    USING (fn_sede_visibile(id_sede)) WITH CHECK (fn_sede_visibile(id_sede));
CREATE POLICY pol_note_sede ON note
    USING (fn_sede_visibile(id_sede)) WITH CHECK (fn_sede_visibile(id_sede));
CREATE POLICY pol_tentativi_sede ON tentativi
    USING (fn_sede_visibile(id_sede)) WITH CHECK (fn_sede_visibile(id_sede));
CREATE POLICY pol_cambiamenti_sede ON cambiamenti_spedizioni
    USING (fn_sede_visibile(id_sede)) WITH CHECK (fn_sede_visibile(id_sede));
CREATE POLICY pol_impostazioni_sede ON impostazioni
    USING (fn_sede_visibile(id_sede)) WITH CHECK (fn_sede_visibile(id_sede));
CREATE POLICY pol_ultime_posizioni_sede ON ultime_posizioni_corrieri
    USING (fn_sede_visibile(id_sede)) WITH CHECK (fn_sede_visibile(id_sede));
CREATE POLICY pol_indirizzi_sede ON indirizzi
    USING (fn_sede_visibile(id_sede)) WITH CHECK (fn_sede_visibile(id_sede));


COMMIT;
