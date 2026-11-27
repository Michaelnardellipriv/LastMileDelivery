-- Migrazione: sedi per città, chiavi BIGINT, Row-Level Security, idempotenza nel
-- database e solo l'ultima posizione dei corrieri.
--
-- Da eseguire UNA volta su un database creato con lo schema precedente (non cancella le
-- spedizioni). Sui database nuovi non serve: è già tutto in schema_LastMileDb.sql.
-- Va eseguita con l'utente proprietario delle tabelle (POSTGRES_USER):
--
--   psql -U <utente> -d <database> -f migrazione_sedi.sql
--
-- Cosa fa con i dati che ci sono già:
--   - ogni indirizzo resta nella prima città a cui era collegato; quelli senza città
--     finiscono nella città "Da verificare" (da correggere a mano);
--   - le città con lo stesso nome e la stessa regione (prima distinte dal CAP) diventano
--     una sola; il CAP passa sull'indirizzo;
--   - nasce una sede per ogni città in cui c'è un indirizzo di ritiro; ogni spedizione va
--     alla sede della città di ritiro;
--   - operatori e corrieri vanno nella sede in cui hanno lavorato di più (o nella prima);
--   - le impostazioni globali diventano le impostazioni di ogni sede;
--   - delle posizioni dei corrieri resta solo l'ultima.
-- Le spedizioni vecchie con la consegna in un'altra città restano com'erano: il vincolo
-- "ritiro e consegna nella città della sede" vale solo per quelle nuove (NOTICE in fondo).
--
-- Dopo la migrazione create l'utente del backend (vedi db/ruolo_applicazione.sh) e
-- usatelo in DATABASE_URL: con POSTGRES_USER la Row-Level Security non scatta.

BEGIN;

-- I trigger si tolgono e si ricreano alla fine (tutto nella stessa transazione):
--   - PostgreSQL non cambia il tipo di una colonna usata da un trigger (id_utente, ...);
--   - qui si aggiornano righe dello storico e delle spedizioni, che i trigger dello
--     storico (sola scrittura, autore obbligatorio) rifiuterebbero.
DROP TRIGGER IF EXISTS trg_clienti_un_solo_ruolo              ON clienti;
DROP TRIGGER IF EXISTS trg_corrieri_un_solo_ruolo             ON corrieri;
DROP TRIGGER IF EXISTS trg_operatori_un_solo_ruolo            ON operatori_logistici;
DROP TRIGGER IF EXISTS trg_amministratori_un_solo_ruolo       ON amministratori;
DROP TRIGGER IF EXISTS trg_spedizioni_storico_insert          ON spedizioni;
DROP TRIGGER IF EXISTS trg_spedizioni_storico_update          ON spedizioni;
DROP TRIGGER IF EXISTS trg_spedizioni_controlla_annullamento  ON spedizioni;
DROP TRIGGER IF EXISTS trg_cambiamenti_spedizioni_immutabile  ON cambiamenti_spedizioni;


-- ===== 1. Tutte le chiavi diventano BIGINT =====

DO $$
DECLARE c record;
BEGIN
    FOR c IN
        SELECT table_name, column_name
        FROM information_schema.columns
        WHERE table_schema = 'public' AND data_type = 'integer'
          AND (column_name = 'id' OR column_name LIKE 'id\_%')
          AND table_name IN (SELECT table_name FROM information_schema.tables
                             WHERE table_schema = 'public' AND table_type = 'BASE TABLE')
    LOOP
        EXECUTE format('ALTER TABLE %I ALTER COLUMN %I TYPE BIGINT', c.table_name, c.column_name);
    END LOOP;
END $$;


-- ===== 2. Ogni indirizzo in UNA città, CAP sull'indirizzo =====

ALTER TABLE indirizzi ADD COLUMN cod_postale VARCHAR(10);
ALTER TABLE indirizzi ADD COLUMN id_citta BIGINT;

UPDATE indirizzi i
SET id_citta = prima.id_citta, cod_postale = prima.cod_postale
FROM (
    SELECT DISTINCT ON (ic.id_indirizzo) ic.id_indirizzo, ic.id_citta, c.cod_postale
    FROM indirizzi_in_citta ic JOIN citta c ON c.id = ic.id_citta
    ORDER BY ic.id_indirizzo, ic.id_citta
) AS prima
WHERE prima.id_indirizzo = i.id;

-- indirizzi senza città: in una città segnaposto, da correggere a mano
DO $$
DECLARE v_naz BIGINT; v_reg BIGINT; v_citta BIGINT; v_quanti INT;
BEGIN
    SELECT count(*) INTO v_quanti FROM indirizzi WHERE id_citta IS NULL;
    IF v_quanti > 0 THEN
        INSERT INTO nazioni (nome) VALUES ('Da verificare')
            ON CONFLICT (nome) DO UPDATE SET nome = EXCLUDED.nome RETURNING id INTO v_naz;
        INSERT INTO regioni (nome, id_nazione) VALUES ('Da verificare', v_naz)
            ON CONFLICT ON CONSTRAINT uq_regioni_nome_nazione DO UPDATE SET nome = EXCLUDED.nome RETURNING id INTO v_reg;
        INSERT INTO citta (nome, cod_postale, id_regione) VALUES ('Da verificare', '00000', v_reg) RETURNING id INTO v_citta;
        UPDATE indirizzi SET id_citta = v_citta, cod_postale = '00000' WHERE id_citta IS NULL;
        RAISE NOTICE '% indirizzi non avevano una città: ora sono nella città "Da verificare"', v_quanti;
    END IF;
END $$;

-- città con lo stesso nome e la stessa regione (prima distinte solo dal CAP) → una sola
UPDATE indirizzi i
SET id_citta = canonica.id
FROM citta c
JOIN LATERAL (
    SELECT min(c2.id) AS id FROM citta c2 WHERE c2.nome = c.nome AND c2.id_regione = c.id_regione
) AS canonica ON TRUE
WHERE i.id_citta = c.id AND canonica.id <> c.id;

DROP TABLE indirizzi_in_citta;

DELETE FROM citta c
WHERE EXISTS (SELECT 1 FROM citta c2 WHERE c2.nome = c.nome AND c2.id_regione = c.id_regione AND c2.id < c.id);

ALTER TABLE citta DROP CONSTRAINT uq_citta;
ALTER TABLE citta DROP COLUMN cod_postale;
ALTER TABLE citta ADD CONSTRAINT uq_citta UNIQUE (nome, id_regione);

-- indirizzi doppi nella stessa città (stessa via e civico): restano quelli con l'id più basso
CREATE TEMP TABLE doppi_indirizzi AS
SELECT i.id AS doppio, min(i2.id) AS tenuto
FROM indirizzi i
JOIN indirizzi i2 ON i2.id_citta = i.id_citta AND lower(i2.via) = lower(i.via)
                 AND lower(i2.civico) = lower(i.civico) AND i2.id < i.id
GROUP BY i.id;

UPDATE spedizioni s SET id_indirizzo_ritiro = d.tenuto FROM doppi_indirizzi d WHERE s.id_indirizzo_ritiro = d.doppio;
UPDATE spedizioni s SET id_indirizzo_spedizione = d.tenuto FROM doppi_indirizzi d WHERE s.id_indirizzo_spedizione = d.doppio;
DELETE FROM indirizzi WHERE id IN (SELECT doppio FROM doppi_indirizzi);

ALTER TABLE indirizzi ALTER COLUMN cod_postale SET NOT NULL;
ALTER TABLE indirizzi ALTER COLUMN id_citta SET NOT NULL;
ALTER TABLE indirizzi ADD CONSTRAINT indirizzi_id_citta_fkey
    FOREIGN KEY (id_citta) REFERENCES citta (id) ON UPDATE CASCADE ON DELETE RESTRICT;
ALTER TABLE indirizzi ADD CONSTRAINT uq_indirizzi_id_citta UNIQUE (id, id_citta);
CREATE UNIQUE INDEX uq_indirizzi_via_civico ON indirizzi (id_citta, lower(via), lower(civico));


-- ===== 3. Sedi: una per ogni città in cui si ritira =====

CREATE TABLE sedi (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nome          VARCHAR(100) NOT NULL,
    id_citta      BIGINT NOT NULL
                  REFERENCES citta (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    id_indirizzo  BIGINT NOT NULL,
    attiva        BOOLEAN NOT NULL DEFAULT TRUE,
    creata_il     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_sedi_citta    UNIQUE (id_citta),
    CONSTRAINT uq_sedi_id_citta UNIQUE (id, id_citta),
    CONSTRAINT ck_sedi_nome     CHECK (btrim(nome) <> ''),
    CONSTRAINT fk_sedi_indirizzo_stessa_citta
        FOREIGN KEY (id_indirizzo, id_citta)
        REFERENCES indirizzi (id, id_citta) ON UPDATE CASCADE ON DELETE RESTRICT
);

CREATE UNIQUE INDEX uq_sedi_nome ON sedi (lower(nome));

-- città con spedizioni (per città di ritiro); se non ce n'è nessuna, la prima città con
-- un indirizzo, perché operatori e corrieri devono avere una sede
INSERT INTO sedi (nome, id_citta, id_indirizzo)
SELECT 'Veloce24 ' || c.nome || ' (' || r.nome || ')', c.id,
       (SELECT min(i2.id) FROM indirizzi i2 WHERE i2.id_citta = c.id)
FROM citta c JOIN regioni r ON r.id = c.id_regione
WHERE c.id IN (SELECT i.id_citta FROM spedizioni s JOIN indirizzi i ON i.id = s.id_indirizzo_ritiro)
ORDER BY c.id;

INSERT INTO sedi (nome, id_citta, id_indirizzo)
SELECT 'Veloce24 ' || c.nome || ' (' || r.nome || ')', c.id, min(i.id)
FROM citta c JOIN regioni r ON r.id = c.id_regione JOIN indirizzi i ON i.id_citta = c.id
WHERE NOT EXISTS (SELECT 1 FROM sedi)
GROUP BY c.id, c.nome, r.nome
ORDER BY c.id
LIMIT 1;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM sedi)
       AND (EXISTS (SELECT 1 FROM corrieri) OR EXISTS (SELECT 1 FROM operatori_logistici)) THEN
        RAISE EXCEPTION 'Nessun indirizzo nel database: impossibile creare una sede per operatori e corrieri'
            USING HINT = 'Inserire almeno un indirizzo con la sua città e rieseguire la migrazione.';
    END IF;
END $$;


-- ===== 4. Spedizioni nella sede della città di ritiro =====

ALTER TABLE spedizioni ADD COLUMN id_sede BIGINT;
ALTER TABLE spedizioni ADD COLUMN id_citta BIGINT;

UPDATE spedizioni s
SET id_citta = i.id_citta, id_sede = se.id
FROM indirizzi i JOIN sedi se ON se.id_citta = i.id_citta
WHERE i.id = s.id_indirizzo_ritiro;

ALTER TABLE spedizioni ALTER COLUMN id_sede SET NOT NULL;
ALTER TABLE spedizioni ALTER COLUMN id_citta SET NOT NULL;
ALTER TABLE spedizioni ADD CONSTRAINT uq_spedizioni_id_sede UNIQUE (id, id_sede);


-- ===== 5. Operatori e corrieri nella sede in cui hanno lavorato di più =====

ALTER TABLE operatori_logistici ADD COLUMN id_sede BIGINT;
ALTER TABLE corrieri ADD COLUMN id_sede BIGINT;

UPDATE operatori_logistici o
SET id_sede = COALESCE((
    SELECT s.id_sede FROM spedizioni s WHERE s.id_operatore_ha_creato = o.id
    GROUP BY s.id_sede ORDER BY count(*) DESC, s.id_sede LIMIT 1
), (SELECT min(id) FROM sedi));

UPDATE corrieri c
SET id_sede = COALESCE((
    SELECT s.id_sede FROM spedizioni s JOIN assegnazioni a ON a.id = s.id_assegnazione_corrente
    WHERE a.id_corriere = c.id
    GROUP BY s.id_sede ORDER BY count(*) DESC, s.id_sede LIMIT 1
), (SELECT min(id) FROM sedi));

ALTER TABLE operatori_logistici ALTER COLUMN id_sede SET NOT NULL;
ALTER TABLE corrieri ALTER COLUMN id_sede SET NOT NULL;
ALTER TABLE operatori_logistici ADD CONSTRAINT operatori_logistici_id_sede_fkey
    FOREIGN KEY (id_sede) REFERENCES sedi (id) ON UPDATE CASCADE ON DELETE RESTRICT;
ALTER TABLE corrieri ADD CONSTRAINT corrieri_id_sede_fkey
    FOREIGN KEY (id_sede) REFERENCES sedi (id) ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX ix_operatori_sede ON operatori_logistici (id_sede);
CREATE INDEX ix_corrieri_sede ON corrieri (id_sede);


-- ===== 6. Assegnazioni, note, tentativi e storico con la sede della spedizione =====

ALTER TABLE assegnazioni ADD COLUMN id_sede BIGINT;
-- l'assegnazione corrente ha la sede della sua spedizione, le vecchie quella del corriere
UPDATE assegnazioni a SET id_sede = s.id_sede FROM spedizioni s WHERE s.id_assegnazione_corrente = a.id;
UPDATE assegnazioni a SET id_sede = c.id_sede FROM corrieri c WHERE c.id = a.id_corriere AND a.id_sede IS NULL;
ALTER TABLE assegnazioni ALTER COLUMN id_sede SET NOT NULL;
ALTER TABLE assegnazioni ADD CONSTRAINT assegnazioni_id_sede_fkey
    FOREIGN KEY (id_sede) REFERENCES sedi (id) ON UPDATE CASCADE ON DELETE RESTRICT;
ALTER TABLE assegnazioni ADD CONSTRAINT uq_assegnazioni_id_sede UNIQUE (id, id_sede);
CREATE INDEX ix_assegnazioni_corriere ON assegnazioni (id_corriere);

ALTER TABLE note ADD COLUMN id_sede BIGINT;
UPDATE note n SET id_sede = s.id_sede FROM spedizioni s WHERE s.id = n.id_spedizione;
ALTER TABLE note ALTER COLUMN id_sede SET NOT NULL;

ALTER TABLE tentativi ADD COLUMN id_sede BIGINT;
UPDATE tentativi t SET id_sede = s.id_sede FROM spedizioni s WHERE s.id = t.id_spedizione;
ALTER TABLE tentativi ALTER COLUMN id_sede SET NOT NULL;

ALTER TABLE cambiamenti_spedizioni ADD COLUMN id_sede BIGINT;
UPDATE cambiamenti_spedizioni c SET id_sede = s.id_sede FROM spedizioni s WHERE s.id = c.id_spedizione;
ALTER TABLE cambiamenti_spedizioni ALTER COLUMN id_sede SET NOT NULL;

-- le vecchie chiavi esterne semplici diventano composte (spedizione, sede)
ALTER TABLE note DROP CONSTRAINT IF EXISTS note_id_spedizione_fkey;
ALTER TABLE note ADD CONSTRAINT fk_note_spedizione
    FOREIGN KEY (id_spedizione, id_sede) REFERENCES spedizioni (id, id_sede) ON UPDATE CASCADE ON DELETE CASCADE;
ALTER TABLE tentativi DROP CONSTRAINT IF EXISTS tentativi_id_spedizione_fkey;
ALTER TABLE tentativi ADD CONSTRAINT fk_tentativi_spedizione
    FOREIGN KEY (id_spedizione, id_sede) REFERENCES spedizioni (id, id_sede) ON UPDATE CASCADE ON DELETE RESTRICT;
ALTER TABLE cambiamenti_spedizioni DROP CONSTRAINT IF EXISTS cambiamenti_spedizioni_id_spedizione_fkey;
ALTER TABLE cambiamenti_spedizioni ADD CONSTRAINT fk_cambiamenti_spedizione
    FOREIGN KEY (id_spedizione, id_sede) REFERENCES spedizioni (id, id_sede) ON UPDATE CASCADE ON DELETE RESTRICT;

CREATE INDEX ix_note_spedizione ON note (id_spedizione);
CREATE INDEX ix_tentativi_spedizione ON tentativi (id_spedizione, tipo);
CREATE INDEX ix_cambiamenti_spedizione ON cambiamenti_spedizioni (id_spedizione, modificato_il);


-- ===== 7. Vincoli di città e sede sulle spedizioni =====

ALTER TABLE spedizioni DROP CONSTRAINT IF EXISTS spedizioni_id_indirizzo_ritiro_fkey;
ALTER TABLE spedizioni DROP CONSTRAINT IF EXISTS spedizioni_id_indirizzo_spedizione_fkey;
ALTER TABLE spedizioni DROP CONSTRAINT IF EXISTS spedizioni_id_assegnazione_corrente_fkey;

ALTER TABLE spedizioni ADD CONSTRAINT fk_spedizioni_sede_citta
    FOREIGN KEY (id_sede, id_citta) REFERENCES sedi (id, id_citta) ON UPDATE CASCADE ON DELETE RESTRICT;
ALTER TABLE spedizioni ADD CONSTRAINT fk_spedizioni_ritiro_in_citta
    FOREIGN KEY (id_indirizzo_ritiro, id_citta) REFERENCES indirizzi (id, id_citta) ON UPDATE CASCADE ON DELETE RESTRICT;
ALTER TABLE spedizioni ADD CONSTRAINT fk_spedizioni_assegnazione_stessa_sede
    FOREIGN KEY (id_assegnazione_corrente, id_sede) REFERENCES assegnazioni (id, id_sede)
    ON UPDATE CASCADE ON DELETE SET NULL (id_assegnazione_corrente);
-- la consegna nella stessa città: NOT VALID non controlla le spedizioni vecchie, solo le nuove
ALTER TABLE spedizioni ADD CONSTRAINT fk_spedizioni_consegna_in_citta
    FOREIGN KEY (id_indirizzo_spedizione, id_citta) REFERENCES indirizzi (id, id_citta)
    ON UPDATE CASCADE ON DELETE RESTRICT NOT VALID;

DO $$
BEGIN
    ALTER TABLE spedizioni VALIDATE CONSTRAINT fk_spedizioni_consegna_in_citta;
EXCEPTION WHEN foreign_key_violation THEN
    RAISE NOTICE 'Ci sono spedizioni vecchie con la consegna in un''altra città: restano com''erano, il vincolo vale per le nuove';
END $$;

CREATE INDEX ix_spedizioni_sede_stato ON spedizioni (id_sede, stato, creata_il DESC);
CREATE INDEX ix_spedizioni_sede_creata ON spedizioni (id_sede, creata_il DESC, id DESC);
CREATE INDEX ix_spedizioni_destinatario ON spedizioni (id_cliente_destinatario);


-- ===== 8. Storico: il trigger scrive anche la sede =====

CREATE OR REPLACE FUNCTION fn_traccia_cambiamenti_spedizioni()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    v_operazione   tipo_operazione_spedizione;
    v_id_corriere  BIGINT;
    v_id_operatore BIGINT;
    v_vecchio      JSONB;
    v_nuovo        JSONB;
BEGIN
    IF TG_OP = 'INSERT' THEN
        v_operazione   := 'creazione';
        v_id_operatore := NEW.id_operatore_ha_creato;
        v_nuovo        := to_jsonb(NEW);
    ELSE
        SELECT jsonb_object_agg(o.key, o.value),
               jsonb_object_agg(o.key, n.value)
          INTO v_vecchio, v_nuovo
          FROM jsonb_each(to_jsonb(OLD)) AS o
          JOIN jsonb_each(to_jsonb(NEW)) AS n ON n.key = o.key
         WHERE o.value IS DISTINCT FROM n.value;

        IF v_nuovo IS NULL THEN
            RETURN NEW;
        END IF;

        IF OLD.annullata_il IS NULL AND NEW.annullata_il IS NOT NULL THEN
            v_operazione   := 'annullamento';
            v_id_operatore := NEW.id_operatore_ha_annullato;
        ELSE
            v_operazione   := 'modifica';
            v_id_corriere  := NULLIF(current_setting('app.id_corriere',  true), '')::BIGINT;
            v_id_operatore := NULLIF(current_setting('app.id_operatore', true), '')::BIGINT;
            IF num_nonnulls(v_id_corriere, v_id_operatore) <> 1 THEN
                RAISE EXCEPTION 'Autore della modifica non valido per la spedizione %', NEW.id
                    USING HINT = 'Impostare uno solo tra app.id_operatore e app.id_corriere '
                                 'con SET LOCAL prima dell''UPDATE.';
            END IF;
        END IF;
    END IF;

    INSERT INTO cambiamenti_spedizioni
        (id_sede, operazione, valore_vecchio, valore_nuovo,
         id_corriere, id_operatore, id_spedizione)
    VALUES
        (NEW.id_sede, v_operazione, v_vecchio, v_nuovo,
         v_id_corriere, v_id_operatore, NEW.id);

    RETURN NEW;
END;
$$;


-- ===== 9. Impostazioni per sede =====

-- copia dei valori globali; la tabella vecchia si toglie prima di crearne una con lo
-- stesso nome (anche gli indici hanno nomi che devono essere unici)
CREATE TEMP TABLE impostazioni_globali AS SELECT * FROM impostazioni;
DROP TABLE impostazioni;

CREATE TABLE impostazioni (
    id_sede                     BIGINT PRIMARY KEY
                                REFERENCES sedi (id) ON UPDATE CASCADE ON DELETE CASCADE,
    intervallo_posizione_sec    INTEGER NOT NULL DEFAULT 30,
    giorni_visibilita_cliente   INTEGER NOT NULL DEFAULT 30,
    max_tentativi_ritiro        INTEGER NOT NULL DEFAULT 3,
    max_tentativi_consegna      INTEGER NOT NULL DEFAULT 3,
    modificate_il               TIMESTAMPTZ NOT NULL DEFAULT now(),
    id_amministratore_modifica  BIGINT NULL
                                REFERENCES amministratori (id) ON UPDATE CASCADE ON DELETE SET NULL,
    CONSTRAINT ck_impostazioni_intervallo CHECK (intervallo_posizione_sec BETWEEN 5 AND 3600),
    CONSTRAINT ck_impostazioni_giorni     CHECK (giorni_visibilita_cliente BETWEEN 1 AND 365),
    CONSTRAINT ck_impostazioni_ritiro     CHECK (max_tentativi_ritiro BETWEEN 1 AND 10),
    CONSTRAINT ck_impostazioni_consegna   CHECK (max_tentativi_consegna BETWEEN 1 AND 10)
);

-- ogni sede parte con i valori globali di prima
INSERT INTO impostazioni (id_sede, intervallo_posizione_sec, giorni_visibilita_cliente,
                          max_tentativi_ritiro, max_tentativi_consegna, modificate_il, id_amministratore_modifica)
SELECT s.id, g.intervallo_posizione_sec, g.giorni_visibilita_cliente,
       g.max_tentativi_ritiro, g.max_tentativi_consegna, g.modificate_il, g.id_amministratore_modifica
FROM sedi s CROSS JOIN impostazioni_globali g;

-- sedi senza riga (tabella globale vuota): valori predefiniti
INSERT INTO impostazioni (id_sede)
SELECT id FROM sedi WHERE id NOT IN (SELECT id_sede FROM impostazioni);


-- ===== 10. Solo l'ultima posizione di ogni corriere =====

CREATE TABLE ultime_posizioni_corrieri (
    id_corriere  BIGINT PRIMARY KEY
                 REFERENCES corrieri (id) ON UPDATE CASCADE ON DELETE CASCADE,
    id_sede      BIGINT NOT NULL
                 REFERENCES sedi (id) ON UPDATE CASCADE ON DELETE CASCADE,
    latitudine   DOUBLE PRECISION NOT NULL,
    longitudine  DOUBLE PRECISION NOT NULL,
    rilevata_il  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_posizioni_latitudine  CHECK (latitudine  BETWEEN -90  AND 90),
    CONSTRAINT ck_posizioni_longitudine CHECK (longitudine BETWEEN -180 AND 180)
);

CREATE INDEX ix_ultime_posizioni_sede ON ultime_posizioni_corrieri (id_sede);

INSERT INTO ultime_posizioni_corrieri (id_corriere, id_sede, latitudine, longitudine, rilevata_il)
SELECT DISTINCT ON (p.id_corriere) p.id_corriere, c.id_sede, p.latitudine, p.longitudine, p.rilevata_il
FROM posizioni_corrieri p JOIN corrieri c ON c.id = p.id_corriere
ORDER BY p.id_corriere, p.rilevata_il DESC;

DROP TABLE posizioni_corrieri;


-- ===== 11. Idempotenza nel database =====

CREATE TABLE chiavi_idempotenza (
    chiave        TEXT        PRIMARY KEY,
    hash_corpo    BYTEA       NOT NULL,
    stato_http    INTEGER     NULL,
    intestazioni  JSONB       NULL,
    corpo         BYTEA       NULL,
    creata_il     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_chiavi_idempotenza_stato CHECK (stato_http IS NULL OR stato_http BETWEEN 200 AND 299)
);

CREATE INDEX ix_chiavi_idempotenza_creata ON chiavi_idempotenza (creata_il);


-- ===== 12. Row-Level Security =====

CREATE OR REPLACE FUNCTION fn_sede_visibile(p_id_sede BIGINT)
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

ALTER TABLE spedizioni                FORCE ROW LEVEL SECURITY;
ALTER TABLE assegnazioni              FORCE ROW LEVEL SECURITY;
ALTER TABLE note                      FORCE ROW LEVEL SECURITY;
ALTER TABLE tentativi                 FORCE ROW LEVEL SECURITY;
ALTER TABLE cambiamenti_spedizioni    FORCE ROW LEVEL SECURITY;
ALTER TABLE impostazioni              FORCE ROW LEVEL SECURITY;
ALTER TABLE ultime_posizioni_corrieri FORCE ROW LEVEL SECURITY;

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


-- ===== 13. I trigger tornano come prima =====

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

CREATE TRIGGER trg_spedizioni_storico_insert
AFTER INSERT ON spedizioni
FOR EACH ROW
EXECUTE FUNCTION fn_traccia_cambiamenti_spedizioni();

CREATE TRIGGER trg_spedizioni_storico_update
AFTER UPDATE ON spedizioni
FOR EACH ROW
WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION fn_traccia_cambiamenti_spedizioni();

CREATE TRIGGER trg_spedizioni_controlla_annullamento
BEFORE UPDATE OF stato ON spedizioni
FOR EACH ROW
WHEN (NEW.stato = 'annullata' AND OLD.stato <> 'annullata')
EXECUTE FUNCTION fn_controlla_annullamento();

CREATE TRIGGER trg_cambiamenti_spedizioni_immutabile
BEFORE UPDATE OR DELETE ON cambiamenti_spedizioni
FOR EACH ROW
EXECUTE FUNCTION fn_blocca_modifica_storico();

COMMIT;

-- Riepilogo: le sedi create e quante spedizioni ha ciascuna.
SELECT se.id, se.nome, count(s.id) AS spedizioni
FROM sedi se LEFT JOIN spedizioni s ON s.id_sede = se.id
GROUP BY se.id, se.nome ORDER BY se.id;
