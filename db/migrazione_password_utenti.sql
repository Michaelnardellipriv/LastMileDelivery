-- Migrazione: password nella tabella utenti e recupero password via mail
--
-- Da eseguire UNA volta su un database già creato con una versione precedente
-- di schema_LastMileDb.sql (non cancella spedizioni né utenti). Sui database
-- nuovi non serve: le stesse modifiche sono già dentro schema_LastMileDb.sql.
--
--   psql -U <utente> -d <database> -f migrazione_password_utenti.sql
--
-- Le vecchie colonne pwd_hash delle tabelle dei ruoli contenevano solo valori
-- segnaposto ('x', 'da-sostituire'), non password vere: vengono eliminate.
--
-- La password è obbligatoria (NOT NULL). Agli utenti già presenti viene messo il
-- valore '!', che non corrisponde a nessuna password: non possono entrare finché
-- l'operatore o l'amministratore non imposta la loro password, per esempio con
--   UPDATE utenti SET pwd_hash = '<hash bcrypt>' WHERE email = '...';
-- Sui database di sviluppo è più semplice ricrearli con schema_LastMileDb.sql e i seed.

BEGIN;

ALTER TABLE utenti ADD COLUMN IF NOT EXISTS pwd_hash VARCHAR(255) NULL;
UPDATE utenti SET pwd_hash = '!' WHERE pwd_hash IS NULL OR pwd_hash = '';
ALTER TABLE utenti ALTER COLUMN pwd_hash SET NOT NULL;
ALTER TABLE utenti DROP CONSTRAINT IF EXISTS ck_utenti_pwd_hash;
ALTER TABLE utenti ADD CONSTRAINT ck_utenti_pwd_hash CHECK (pwd_hash <> '');

ALTER TABLE corrieri            DROP COLUMN IF EXISTS pwd_hash;
ALTER TABLE operatori_logistici DROP COLUMN IF EXISTS pwd_hash;
ALTER TABLE amministratori      DROP COLUMN IF EXISTS pwd_hash;

CREATE TABLE IF NOT EXISTS recuperi_password (
    id          INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_utente   INTEGER NOT NULL
                REFERENCES utenti (id) ON UPDATE CASCADE ON DELETE CASCADE,
    token_hash  CHAR(64)    NOT NULL,
    creato_il   TIMESTAMPTZ NOT NULL DEFAULT now(),
    scade_il    TIMESTAMPTZ NOT NULL,
    usato_il    TIMESTAMPTZ NULL,
    CONSTRAINT uq_recuperi_password_token UNIQUE (token_hash),
    CONSTRAINT ck_recuperi_password_scadenza CHECK (scade_il > creato_il)
);

CREATE INDEX IF NOT EXISTS ix_recuperi_password_utente ON recuperi_password (id_utente);

COMMIT;
