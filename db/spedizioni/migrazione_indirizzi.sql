-- Database delle spedizioni, versione 2.4.0 → 2.5.0: gli indirizzi di ritiro e consegna
-- passano dal database globale a quello del gruppo della loro sede. FASE 1 di 2.
-- Da eseguire su OGNI database delle spedizioni (ogni gruppo), con l'utente proprietario
-- delle tabelle, PRIMA di aggiornare i servizi:
--   kubectl exec -i deploy/db-spedizioni -- sh -c \
--     'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f -' < db/spedizioni/migrazione_indirizzi.sql
-- Poi i permessi (db/privilegi_applicazione.sql) e la copia degli indirizzi dal database
-- globale (db/migra_indirizzi.sh <gruppo>). Dopo l'aggiornamento dei servizi: di nuovo la
-- copia (prende quelli creati dai Pod vecchi durante l'aggiornamento) e la fase 2,
-- db/spedizioni/migrazione_indirizzi_vincoli.sql.
-- Si può rieseguire. Rollback: i servizi 2.4.0 ignorano la tabella (gli indirizzi nel
-- database globale restano dove sono).
BEGIN;

-- ===== Indirizzi di ritiro e consegna =====

-- Gli indirizzi delle spedizioni stanno qui, nel database del gruppo della loro sede: li
-- crea l'operatore (POST /api/indirizzi), sempre nella città della sua sede, crescono con
-- le spedizioni e non si modificano. Città, regione e nazione sono quelle della sede (nel
-- database globale, con l'indirizzo della sede stessa): qui c'è l'id della città.
CREATE TABLE IF NOT EXISTS indirizzi (
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
CREATE UNIQUE INDEX IF NOT EXISTS uq_indirizzi_via_civico ON indirizzi (id_sede, lower(via), lower(civico));


ALTER TABLE indirizzi ENABLE ROW LEVEL SECURITY;
ALTER TABLE indirizzi FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS pol_indirizzi_sede ON indirizzi;
CREATE POLICY pol_indirizzi_sede ON indirizzi
    USING (fn_sede_visibile(id_sede)) WITH CHECK (fn_sede_visibile(id_sede));

-- Appoggio per la copia (db/migra_indirizzi.sh): le righe del database globale arrivano
-- qui con \copy, poi fn_importa_indirizzi le mette in indirizzi con lo stesso id (le
-- spedizioni già esistenti le citano per id).
CREATE UNLOGGED TABLE IF NOT EXISTS indirizzi_da_importare (
    id           BIGINT       NOT NULL,
    id_sede      BIGINT       NOT NULL,
    id_citta     BIGINT       NOT NULL,
    via          VARCHAR(150) NOT NULL,
    civico       VARCHAR(10)  NOT NULL,
    cod_postale  VARCHAR(10)  NOT NULL
);

CREATE OR REPLACE FUNCTION fn_importa_indirizzi()
RETURNS integer
LANGUAGE plpgsql
AS $$
DECLARE
    v_gruppo   BIGINT := (SELECT codice FROM gruppo_database);
    v_copiati  integer;
    v_prossimo BIGINT;
    v_ultimo   BIGINT;
BEGIN
    PERFORM set_config('app.tutte_le_sedi', 'si', true);  -- RLS: si vedono tutte le righe
    INSERT INTO indirizzi (id, id_sede, id_citta, via, civico, cod_postale) OVERRIDING SYSTEM VALUE
    SELECT id, id_sede, id_citta, via, civico, cod_postale FROM indirizzi_da_importare
    ON CONFLICT DO NOTHING;
    GET DIAGNOSTICS v_copiati = ROW_COUNT;
    TRUNCATE indirizzi_da_importare;
    -- gli indirizzi nuovi: nel gruppo g da g × 10^12; nel gruppo 0 un milione oltre il più
    -- alto copiato, così non si scontrano con quelli che i Pod vecchi creano ancora nel
    -- database globale durante l'aggiornamento (si copiano dopo, con lo stesso id)
    v_prossimo := greatest(v_gruppo * 1000000000000,
                           CASE WHEN v_gruppo = 0 THEN COALESCE((SELECT max(id) FROM indirizzi), 0) + 1000000 ELSE 0 END);
    SELECT last_value INTO v_ultimo FROM indirizzi_id_seq;
    IF v_ultimo < v_prossimo THEN
        PERFORM setval('indirizzi_id_seq', v_prossimo, true);
    END IF;
    RETURN v_copiati;
END;
$$;

REVOKE ALL ON FUNCTION fn_importa_indirizzi() FROM PUBLIC;

-- la sequenza di una tabella nuova parte da 1: nel gruppo g deve partire da g × 10^12
SELECT fn_importa_indirizzi();

COMMIT;
