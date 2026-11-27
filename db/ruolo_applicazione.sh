#!/bin/sh
# Crea l'utente del database con cui si collega il backend.
#
# Gira una volta sola, alla prima creazione del container (docker-entrypoint-initdb.d),
# dopo lo schema. Serve perché l'utente POSTGRES_USER dell'immagine è un SUPERUTENTE e
# un superutente ignora la Row-Level Security: se il backend usasse lui, la rete di
# sicurezza sulle sedi non scatterebbe mai.
#
# Variabili (stesso file .env / Secret del database):
#   APP_DB_USER      nome dell'utente del backend (es. lastmile_app)
#   APP_DB_PASSWORD  la sua password
# Il backend usa poi DATABASE_URL=postgres://APP_DB_USER:APP_DB_PASSWORD@.../POSTGRES_DB
#
# Least privilege: i permessi sulle tabelle sono in privilegi_applicazione.sql (stesso
# file per i due database e per le migrazioni): niente DELETE se non dove il backend
# cancella davvero, storico, note e tentativi in sola aggiunta.
set -eu

if [ -z "${APP_DB_USER:-}" ] || [ -z "${APP_DB_PASSWORD:-}" ]; then
    echo "ATTENZIONE: APP_DB_USER o APP_DB_PASSWORD non impostate: utente del backend non creato." >&2
    echo "Il backend dovrà usare POSTGRES_USER, che ignora la Row-Level Security." >&2
    exit 0
fi

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
     -v utente="$APP_DB_USER" -v password="$APP_DB_PASSWORD" -v db="$POSTGRES_DB" <<'SQL'
-- utente normale: niente SUPERUSER, niente BYPASSRLS, non proprietario delle tabelle
CREATE ROLE :"utente" LOGIN PASSWORD :'password' NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
GRANT CONNECT ON DATABASE :"db" TO :"utente";
GRANT USAGE ON SCHEMA public TO :"utente";
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO :"utente";
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO :"utente";
\set app :utente
\i /usr/local/share/lastmile/privilegi_applicazione.sql
SQL

echo "utente del backend $APP_DB_USER creato"
