-- Database globale, versione 2.2.0 → 2.3.0: sessioni nel database (il token vale solo
-- finché la sua sessione è aperta) e ruolo non modificabile.
-- Da eseguire una volta, con l'utente proprietario delle tabelle e il nome dell'utente
-- del backend (APP_DB_USER), PRIMA di aggiornare il servizio utenti:
--   kubectl exec -i deploy/db-globale -- sh -c \
--     'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v app="$APP_DB_USER" -f -' \
--     < db/globale/migrazione_sessioni.sql
-- Poi i permessi: lo stesso comando con db/privilegi_applicazione.sql.
-- Dopo l'aggiornamento i token emessi prima non hanno una sessione: si rifà il login.
BEGIN;
-- ===== Sessioni =====

-- Una riga per ogni login. Il token porta l'id della sessione (claim "sid") e vale solo
-- finché la sessione è aperta: il servizio utenti la controlla qui a ogni richiesta, il
-- servizio spedizioni riceve le sessioni chiuse (tabella sessioni_revocate del suo
-- database). Si chiude con il logout, la disattivazione dell'utente, un cambio di sede o
-- una password nuova; scade dopo DURATA_SESSIONE dal login.
CREATE TABLE IF NOT EXISTS sessioni (
    id             CHAR(32)     PRIMARY KEY,                 -- 128 bit casuali
    id_utente      BIGINT       NOT NULL
                   REFERENCES utenti (id) ON UPDATE CASCADE ON DELETE CASCADE,
    ruolo          VARCHAR(20)  NOT NULL,
    id_ruolo       BIGINT       NOT NULL,                    -- id nella tabella del ruolo
    creata_il      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    rinnovata_il   TIMESTAMPTZ  NULL,
    scade_il       TIMESTAMPTZ  NOT NULL,
    revocata_il    TIMESTAMPTZ  NULL,
    motivo_revoca  VARCHAR(50)  NULL,
    CONSTRAINT ck_sessioni_ruolo  CHECK (ruolo IN ('operatore', 'corriere', 'cliente', 'amministratore')),
    CONSTRAINT ck_sessioni_scade  CHECK (scade_il > creata_il),
    CONSTRAINT ck_sessioni_revoca CHECK ((revocata_il IS NULL) = (motivo_revoca IS NULL))
);

CREATE INDEX IF NOT EXISTS ix_sessioni_utente_aperte ON sessioni (id_utente) WHERE revocata_il IS NULL;
CREATE INDEX IF NOT EXISTS ix_sessioni_scadenza ON sessioni (scade_il);

-- Il ruolo di un utente non si cambia e non si toglie (né dall'API né con un UPDATE
-- diretto): un utente che non deve più lavorare si disattiva. Senza questo controllo,
-- spostare una riga da corrieri ad amministratori darebbe a un corriere i permessi di
-- amministratore.
CREATE OR REPLACE FUNCTION fn_ruolo_immutabile()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'Il ruolo di un utente non si cambia né si toglie (tabella %): si disattiva l''utente', TG_TABLE_NAME
        USING ERRCODE = 'restrict_violation';
END;
$$;

CREATE OR REPLACE TRIGGER trg_clienti_ruolo_immutabile
BEFORE UPDATE OF id, id_utente OR DELETE ON clienti
FOR EACH ROW EXECUTE FUNCTION fn_ruolo_immutabile();

CREATE OR REPLACE TRIGGER trg_corrieri_ruolo_immutabile
BEFORE UPDATE OF id, id_utente OR DELETE ON corrieri
FOR EACH ROW EXECUTE FUNCTION fn_ruolo_immutabile();

CREATE OR REPLACE TRIGGER trg_operatori_ruolo_immutabile
BEFORE UPDATE OF id, id_utente OR DELETE ON operatori_logistici
FOR EACH ROW EXECUTE FUNCTION fn_ruolo_immutabile();

CREATE OR REPLACE TRIGGER trg_amministratori_ruolo_immutabile
BEFORE UPDATE OF id, id_utente OR DELETE ON amministratori
FOR EACH ROW EXECUTE FUNCTION fn_ruolo_immutabile();

-- utenti.pwd_cambiata_il (2.2.0) non serve più, ma resta: la 2.2.0 la legge, e una
-- migrazione non toglie niente finché si può ancora tornare alla versione precedente
-- (docs/deployment.md, "Rollback"). Si toglierà in un rilascio successivo.

COMMIT;
