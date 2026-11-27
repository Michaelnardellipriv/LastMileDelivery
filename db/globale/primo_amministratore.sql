-- Primo amministratore di un database globale vuoto (PROD: l'immagine non ha dati di
-- prova). Da lì in poi utenti e sedi si creano dall'app.
--
-- La password non passa dalla riga di comando né dal repository: psql la chiede a
-- terminale (\prompt) e PostgreSQL ne salva solo l'hash bcrypt (pgcrypto, stesso formato
-- che usa il backend). Va eseguito con l'utente proprietario delle tabelle:
--
--   kubectl -n lastmile-prod exec -it deploy/db-globale -- \
--     sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f -' < db/globale/primo_amministratore.sql
--
-- (con -it psql può fare le domande; su Windows usare Get-Content ... | kubectl ... -i)

\set ON_ERROR_STOP on
\prompt 'Nome: ' nome
\prompt 'Cognome: ' cognome
\prompt 'Email: ' email
\prompt 'Prefisso (es. +39): ' prefisso
\prompt 'Telefono (solo cifre): ' telefono
\prompt 'Password (da 12 caratteri a 72 byte): ' password

BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- la password arriva al controllo come impostazione della sola transazione (senza
-- stamparla); per l'amministratore di PROD si chiedono almeno 12 caratteri, il backend
-- ne accetta da 8 per gli altri utenti; bcrypt usa al massimo 72 byte
\o /dev/null
SELECT set_config('lastmile.password', :'password', true);
\o
DO $$
BEGIN
    IF char_length(current_setting('lastmile.password')) < 12
       OR octet_length(current_setting('lastmile.password')) > 72 THEN
        RAISE EXCEPTION 'password non valida: da 12 caratteri a 72 byte';
    END IF;
END $$;

WITH nuovo AS (
    INSERT INTO utenti (nome, cognome, prefisso, nr_telefono, email, pwd_hash)
    VALUES (:'nome', :'cognome', :'prefisso', :'telefono', lower(:'email'),
            crypt(:'password', gen_salt('bf', 10)))
    RETURNING id
)
INSERT INTO amministratori (id_utente) SELECT id FROM nuovo;

COMMIT;

\echo 'Amministratore creato: entra dall''app con la sua email e la password appena scelta.'
