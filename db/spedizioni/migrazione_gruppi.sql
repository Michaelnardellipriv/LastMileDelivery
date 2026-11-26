-- Database delle spedizioni, versione 2.3.0 → 2.4.0: mappa sede → database (gruppi).
-- Un database esistente diventa il gruppo 0 e i suoi id non cambiano.
-- Da eseguire una volta, con l'utente proprietario delle tabelle, PRIMA di aggiornare il
-- servizio spedizioni (che all'avvio controlla gruppo_database):
--   kubectl exec -i deploy/db-spedizioni -- sh -c --     'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f -' < db/spedizioni/migrazione_gruppi.sql
-- Poi i permessi: db/privilegi_applicazione.sql con -v app="$APP_DB_USER".
BEGIN;
-- ===== Gruppo di sedi di questo database (mappa sede → database) =====

-- Le sedi sono divise in gruppi, ognuno con il suo database delle spedizioni (le città
-- grandi da sole, le piccole insieme): il gruppo di una sede è sedi.gruppo_spedizioni nel
-- database globale. Questa tabella dice quale gruppo tiene QUESTO database: il servizio
-- spedizioni la controlla all'avvio, così un URL sbagliato non mescola i dati di due
-- gruppi. Una sola riga.
CREATE TABLE IF NOT EXISTS gruppo_database (
    unica   BOOLEAN  NOT NULL DEFAULT TRUE PRIMARY KEY,
    codice  SMALLINT NOT NULL,
    CONSTRAINT ck_gruppo_database_unica  CHECK (unica),
    -- (codice + 1) × 10^12 resta sotto 2^53: gli id sono numeri esatti anche in JavaScript
    CONSTRAINT ck_gruppo_database_codice CHECK (codice BETWEEN 0 AND 9006)
);

INSERT INTO gruppo_database (codice) VALUES (0) ON CONFLICT DO NOTHING;

-- Imposta il gruppo di un database nuovo e fa partire gli id di ogni tabella da
-- gruppo × 10^12 + 1: dall'id di una spedizione si sa in quale database sta, e gli id non
-- si ripetono tra gruppi. Si esegue una volta, alla creazione del database
-- (db/gruppo_spedizioni.sh con GRUPPO_SPEDIZIONI), prima di qualunque dato.
CREATE OR REPLACE FUNCTION fn_imposta_gruppo(p_codice INTEGER)
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


COMMIT;
