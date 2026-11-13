-- Migrazione: data e ora del ritiro registrate dal sistema
--
-- Da eseguire UNA volta su un database già creato con una versione precedente
-- di schema_LastMileDb.sql. Sui database nuovi non serve: le stesse modifiche sono
-- già dentro schema_LastMileDb.sql.
--
--   psql -U <utente> -d <database> -f migrazione_data_ritiro.sql
--
-- Cosa cambia:
--   - nuova colonna ritirata_il: data e ora del ritiro, registrata dal sistema quando
--     il corriere porta la spedizione in 'ritirata'. Per le spedizioni già ritirate
--     viene ricavata dallo storico (primo passaggio in 'ritirata').

BEGIN;

ALTER TABLE spedizioni
    ADD COLUMN ritirata_il TIMESTAMPTZ NULL;

-- l'aggiornamento non deve finire nello storico come modifica di un utente
ALTER TABLE spedizioni DISABLE TRIGGER trg_spedizioni_storico_update;

UPDATE spedizioni s
SET ritirata_il = r.primo_ritiro
FROM (
    SELECT id_spedizione, min(modificato_il) AS primo_ritiro
    FROM cambiamenti_spedizioni
    WHERE valore_nuovo ->> 'stato' = 'ritirata'
    GROUP BY id_spedizione
) r
WHERE r.id_spedizione = s.id
  AND s.stato NOT IN ('creata', 'assegnata');

ALTER TABLE spedizioni ENABLE TRIGGER trg_spedizioni_storico_update;

ALTER TABLE spedizioni
    ADD CONSTRAINT ck_spedizioni_date_ritiro  CHECK (ritirata_il IS NULL OR ritirata_il >= creata_il),
    ADD CONSTRAINT ck_spedizioni_ritiro_stato CHECK (stato NOT IN ('creata', 'assegnata') OR ritirata_il IS NULL);

COMMIT;
