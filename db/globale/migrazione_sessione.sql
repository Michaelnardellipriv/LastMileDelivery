-- Database globale: data del cambio password (rinnovo dei token, API 0.43).
-- I token delle sessioni iniziate prima di un cambio password non si rinnovano più.
-- Da eseguire una volta, con l'utente proprietario delle tabelle, su un database creato
-- con la versione precedente di db/globale/schema.sql:
--   kubectl exec -i deploy/db-globale -- sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"' < db/globale/migrazione_sessione.sql
ALTER TABLE utenti ADD COLUMN IF NOT EXISTS pwd_cambiata_il TIMESTAMPTZ NOT NULL DEFAULT now();
