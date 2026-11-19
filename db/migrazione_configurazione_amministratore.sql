-- Migrazione: configurazione dell'amministratore e utenti disattivabili
--
-- Da eseguire UNA volta su un database già creato con una versione precedente
-- di schema_LastMileDb.sql. Sui database nuovi non serve: le stesse modifiche sono
-- già dentro schema_LastMileDb.sql. Rieseguirla non fa danni.
--
--   psql -U <utente> -d <database> -f migrazione_configurazione_amministratore.sql
--
-- Giorni di visibilità e tentativi massimi non si leggono più dalle variabili
-- d'ambiente GIORNI_VISIBILITA_CLIENTE e MAX_TENTATIVI ma dalla tabella impostazioni:
-- la migrazione mette i valori predefiniti (30 e 3). Se l'ambiente usava valori
-- diversi, l'amministratore li reimposta con PATCH /api/impostazioni.

BEGIN;

ALTER TABLE utenti
    ADD COLUMN IF NOT EXISTS attivo BOOLEAN NOT NULL DEFAULT TRUE;

ALTER TABLE impostazioni
    ADD COLUMN IF NOT EXISTS giorni_visibilita_cliente INTEGER NOT NULL DEFAULT 30,
    ADD COLUMN IF NOT EXISTS max_tentativi_ritiro      INTEGER NOT NULL DEFAULT 3,
    ADD COLUMN IF NOT EXISTS max_tentativi_consegna    INTEGER NOT NULL DEFAULT 3;

ALTER TABLE impostazioni
    DROP CONSTRAINT IF EXISTS ck_impostazioni_giorni,
    DROP CONSTRAINT IF EXISTS ck_impostazioni_ritiro,
    DROP CONSTRAINT IF EXISTS ck_impostazioni_consegna,
    ADD CONSTRAINT ck_impostazioni_giorni   CHECK (giorni_visibilita_cliente BETWEEN 1 AND 365),
    ADD CONSTRAINT ck_impostazioni_ritiro   CHECK (max_tentativi_ritiro BETWEEN 1 AND 10),
    ADD CONSTRAINT ck_impostazioni_consegna CHECK (max_tentativi_consegna BETWEEN 1 AND 10);

INSERT INTO impostazioni DEFAULT VALUES ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS motivi (
    id           INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    descrizione  VARCHAR(100) NOT NULL,
    attivo       BOOLEAN      NOT NULL DEFAULT TRUE,
    creato_il    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT ck_motivi_descrizione CHECK (btrim(descrizione) <> '')
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_motivi_descrizione ON motivi (lower(descrizione));

-- i motivi che finora erano costanti nel codice del backend
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
    ('altro')
ON CONFLICT DO NOTHING;

COMMIT;
