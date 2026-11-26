-- Database globale, versione 2.3.0 → 2.4.0: mappa sede → database delle spedizioni.
-- Le sedi esistenti restano nel gruppo 0 (il database delle spedizioni di oggi).
-- Da eseguire una volta, con l'utente proprietario delle tabelle, PRIMA di aggiornare i
-- servizi:
--   kubectl exec -i deploy/db-globale -- sh -c --     'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f -' < db/globale/migrazione_gruppi.sql
-- Rollback: i servizi 2.3.0 ignorano la colonna, si può lasciare.
BEGIN;

ALTER TABLE sedi ADD COLUMN IF NOT EXISTS gruppo_spedizioni SMALLINT NOT NULL DEFAULT 0;

-- sede del login nella sessione: la revoca va solo al database del gruppo che controlla il
-- token. Le sessioni aperte prima restano senza sede: la loro revoca va a tutti i gruppi.
ALTER TABLE sessioni ADD COLUMN IF NOT EXISTS id_sede BIGINT NULL;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_sedi_gruppo_spedizioni') THEN
        ALTER TABLE sedi ADD CONSTRAINT ck_sedi_gruppo_spedizioni CHECK (gruppo_spedizioni BETWEEN 0 AND 9006);
    END IF;
END $$;

-- Il gruppo di una sede non si cambia con un UPDATE: le sue spedizioni stanno nel database
-- di quel gruppo e il servizio spedizioni tiene la mappa in memoria. Spostare una sede in
-- un altro gruppo è una migrazione dei dati (docs/microservizi.md).
CREATE OR REPLACE FUNCTION fn_gruppo_immutabile()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'Il gruppo delle spedizioni della sede % non si cambia', OLD.id
        USING ERRCODE = 'check_violation', CONSTRAINT = 'ck_sedi_gruppo_immutabile';
END;
$$;

DROP TRIGGER IF EXISTS trg_sedi_gruppo_immutabile ON sedi;
CREATE TRIGGER trg_sedi_gruppo_immutabile
BEFORE UPDATE OF gruppo_spedizioni ON sedi
FOR EACH ROW WHEN (OLD.gruppo_spedizioni IS DISTINCT FROM NEW.gruppo_spedizioni)
EXECUTE FUNCTION fn_gruppo_immutabile();

COMMIT;
