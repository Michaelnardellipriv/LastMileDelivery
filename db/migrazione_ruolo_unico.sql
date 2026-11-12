-- Migrazione: ogni utente (cioè ogni email) ha un solo ruolo
--
-- Da eseguire UNA volta su un database già creato con una versione precedente
-- di schema_LastMileDb.sql. Sui database nuovi non serve: le stesse modifiche sono
-- già dentro schema_LastMileDb.sql.
--
--   psql -U <utente> -d <database> -f migrazione_ruolo_unico.sql
--
-- Se ci sono già utenti con più ruoli la migrazione si ferma e li elenca: vanno
-- sistemati a mano (togliendo il ruolo in più) prima di rieseguirla.

BEGIN;

DO $$
DECLARE
    v_utenti TEXT;
BEGIN
    SELECT string_agg(id_utente::text, ', ') INTO v_utenti
    FROM (
        SELECT id_utente FROM (
            SELECT id_utente FROM clienti
            UNION ALL SELECT id_utente FROM corrieri
            UNION ALL SELECT id_utente FROM operatori_logistici
            UNION ALL SELECT id_utente FROM amministratori
        ) AS ruoli
        GROUP BY id_utente HAVING count(*) > 1
    ) AS doppi;
    IF v_utenti IS NOT NULL THEN
        RAISE EXCEPTION 'Utenti con più di un ruolo (id_utente): %', v_utenti;
    END IF;
END;
$$;

DROP FUNCTION IF EXISTS fn_un_solo_ruolo() CASCADE;

-- Un utente (cioè un'email) ha un solo ruolo: non può essere insieme cliente, corriere,
-- operatore o amministratore. Il blocco sulla riga dell'utente mette in fila due
-- inserimenti contemporanei nelle tabelle di ruoli diversi.
CREATE FUNCTION fn_un_solo_ruolo()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM 1 FROM utenti WHERE id = NEW.id_utente FOR UPDATE;
    IF (SELECT count(*) FROM (
            SELECT id_utente FROM clienti             WHERE id_utente = NEW.id_utente
            UNION ALL
            SELECT id_utente FROM corrieri            WHERE id_utente = NEW.id_utente
            UNION ALL
            SELECT id_utente FROM operatori_logistici WHERE id_utente = NEW.id_utente
            UNION ALL
            SELECT id_utente FROM amministratori      WHERE id_utente = NEW.id_utente
        ) AS ruoli) > 1 THEN
        RAISE EXCEPTION 'L''utente % ha già un ruolo: ogni email può avere un solo ruolo', NEW.id_utente
            USING ERRCODE = 'unique_violation', CONSTRAINT = 'uq_utenti_un_solo_ruolo';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_clienti_un_solo_ruolo
AFTER INSERT OR UPDATE OF id_utente ON clienti
FOR EACH ROW EXECUTE FUNCTION fn_un_solo_ruolo();

CREATE TRIGGER trg_corrieri_un_solo_ruolo
AFTER INSERT OR UPDATE OF id_utente ON corrieri
FOR EACH ROW EXECUTE FUNCTION fn_un_solo_ruolo();

CREATE TRIGGER trg_operatori_un_solo_ruolo
AFTER INSERT OR UPDATE OF id_utente ON operatori_logistici
FOR EACH ROW EXECUTE FUNCTION fn_un_solo_ruolo();

CREATE TRIGGER trg_amministratori_un_solo_ruolo
AFTER INSERT OR UPDATE OF id_utente ON amministratori
FOR EACH ROW EXECUTE FUNCTION fn_un_solo_ruolo();

COMMIT;
