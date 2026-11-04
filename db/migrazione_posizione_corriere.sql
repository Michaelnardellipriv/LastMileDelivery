-- Migrazione: posizione dei corrieri (RF16)
--
-- Da eseguire UNA volta su un database già creato con schema_LastMileDb.sql
-- (non cancella i dati esistenti). Sui database nuovi non serve: le stesse
-- tabelle sono già dentro schema_LastMileDb.sql.
--
--   psql -U <utente> -d <database> -f migrazione_posizione_corriere.sql

BEGIN;

-- Impostazioni globali dell'applicazione, modificabili dall'amministratore.
-- La tabella ha sempre UNA sola riga: la chiave è un booleano che può
-- valere solo TRUE, quindi una seconda riga violerebbe la PRIMARY KEY.
CREATE TABLE IF NOT EXISTS impostazioni (
    id                          BOOLEAN PRIMARY KEY DEFAULT TRUE,
    intervallo_posizione_sec    INTEGER NOT NULL DEFAULT 30,
    modificate_il               TIMESTAMPTZ NOT NULL DEFAULT now(),
    id_amministratore_modifica  INTEGER NULL
                                REFERENCES amministratori (id) ON UPDATE CASCADE ON DELETE SET NULL,
    CONSTRAINT ck_impostazioni_riga_unica CHECK (id),
    -- sotto i 5 secondi il database si riempie, sopra l'ora la posizione non serve più
    CONSTRAINT ck_impostazioni_intervallo CHECK (intervallo_posizione_sec BETWEEN 5 AND 3600)
);

INSERT INTO impostazioni DEFAULT VALUES ON CONFLICT (id) DO NOTHING;

-- Storico delle posizioni (simulate) inviate dai corrieri.
-- L'ultima posizione di un corriere è la riga con rilevata_il più recente.
CREATE TABLE IF NOT EXISTS posizioni_corrieri (
    id           INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_corriere  INTEGER NOT NULL
                 REFERENCES corrieri (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    latitudine   DOUBLE PRECISION NOT NULL,
    longitudine  DOUBLE PRECISION NOT NULL,
    rilevata_il  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_posizioni_latitudine  CHECK (latitudine  BETWEEN -90  AND 90),
    CONSTRAINT ck_posizioni_longitudine CHECK (longitudine BETWEEN -180 AND 180)
);

-- Rende veloce "ultima posizione di ogni corriere".
CREATE INDEX IF NOT EXISTS ix_posizioni_corrieri_ultima
    ON posizioni_corrieri (id_corriere, rilevata_il DESC);

COMMIT;
