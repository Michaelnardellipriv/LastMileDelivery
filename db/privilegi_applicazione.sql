-- Permessi dell'utente con cui si collega il backend (least privilege): solo quello che
-- il codice fa davvero. Vale per tutti e due i database (le tabelle che non ci sono si
-- saltano) e si può rieseguire: prima toglie tutto, poi dà il necessario.
--   psql -U <proprietario> -d <database> -v app=<APP_DB_USER> -f db/privilegi_applicazione.sql
-- Lo esegue anche db/ruolo_applicazione.sh alla creazione del database.
-- (dentro DO $$ ... $$ psql non sostituisce :app: il nome passa da un'impostazione)
SELECT set_config('lastmile.app', :'app', false);
DO $$
DECLARE
    app  text := current_setting('lastmile.app');
    t    record;
BEGIN
    -- di base: leggere, aggiungere e modificare; niente DELETE né TRUNCATE
    FOR t IN SELECT tablename FROM pg_tables WHERE schemaname = 'public' LOOP
        EXECUTE format('REVOKE ALL ON TABLE public.%I FROM %I', t.tablename, app);
        EXECUTE format('GRANT SELECT, INSERT, UPDATE ON TABLE public.%I TO %I', t.tablename, app);
    END LOOP;

    -- sola aggiunta: lo storico e quello che documenta una spedizione non si modifica
    -- (e gli indirizzi: si aggiungono, non si cambiano, in tutti e due i database)
    FOR t IN SELECT unnest(ARRAY['cambiamenti_spedizioni', 'note', 'tentativi', 'indirizzi']) AS tablename LOOP
        IF to_regclass('public.' || t.tablename) IS NOT NULL THEN
            EXECUTE format('REVOKE UPDATE ON TABLE public.%I FROM %I', t.tablename, app);
        END IF;
    END LOOP;

    -- sola lettura: il gruppo del database lo imposta chi crea il database
    IF to_regclass('public.gruppo_database') IS NOT NULL THEN
        EXECUTE format('REVOKE INSERT, UPDATE ON TABLE public.gruppo_database FROM %I', app);
    END IF;
    IF to_regprocedure('public.fn_imposta_gruppo(integer)') IS NOT NULL THEN
        EXECUTE format('REVOKE EXECUTE ON FUNCTION public.fn_imposta_gruppo(integer) FROM %I', app);
    END IF;

    -- DELETE solo dove il backend cancella: chiavi di idempotenza scadute, sessioni
    -- scadute da tempo, sessioni chiuse oltre la loro scadenza
    FOR t IN SELECT unnest(ARRAY['chiavi_idempotenza', 'sessioni', 'sessioni_revocate']) AS tablename LOOP
        IF to_regclass('public.' || t.tablename) IS NOT NULL THEN
            EXECUTE format('GRANT DELETE ON TABLE public.%I TO %I', t.tablename, app);
        END IF;
    END LOOP;
END $$;
