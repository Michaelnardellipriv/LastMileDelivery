-- Database delle spedizioni, versione 2.2.0 → 2.3.0: sessioni chiuse dal servizio utenti.
-- Da eseguire una volta, con l'utente proprietario delle tabelle, PRIMA di aggiornare il
-- servizio spedizioni:
--   kubectl exec -i deploy/db-spedizioni -- sh -c \
--     'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f -' < db/spedizioni/migrazione_sessioni.sql
-- Poi i permessi: db/privilegi_applicazione.sql con -v app="$APP_DB_USER".
BEGIN;
-- ===== Sessioni chiuse =====

-- Le sessioni che il servizio utenti ha chiuso (logout, disattivazione, cambio di sede,
-- password nuova): il servizio spedizioni rifiuta i loro token. Il servizio utenti le
-- comunica prima di confermare l'operazione. Le righe oltre scade_il si cancellano: dopo
-- la fine della sessione il token non varrebbe comunque.
CREATE TABLE IF NOT EXISTS sessioni_revocate (
    id        CHAR(32)    PRIMARY KEY,
    scade_il  TIMESTAMPTZ NOT NULL,
    motivo    VARCHAR(50) NOT NULL,
    creata_il TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_sessioni_revocate_scadenza ON sessioni_revocate (scade_il);

COMMIT;
