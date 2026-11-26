-- Database delle spedizioni, versione 2.4.0 → 2.5.0, FASE 2 di 2: dopo l'aggiornamento
-- dei servizi e dopo l'ultima copia degli indirizzi (db/migra_indirizzi.sh <gruppo>), su
-- OGNI database delle spedizioni:
--   kubectl exec -i deploy/db-spedizioni -- sh -c \
--     'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f -' < db/spedizioni/migrazione_indirizzi_vincoli.sql
-- Attiva le chiavi esterne "ritiro e consegna sono indirizzi della sede della spedizione".
-- Se una spedizione cita un indirizzo che qui non c'è si ferma e dice quale: copiare
-- l'indirizzo (o correggere la spedizione) e rieseguire.
BEGIN;

SELECT set_config('app.tutte_le_sedi', 'si', true);

DO $$
DECLARE
    v_mancanti text;
BEGIN
    SELECT string_agg(format('spedizione %s: indirizzo %s (sede %s)', s.id, x.id_indirizzo, s.id_sede), '; ')
    INTO v_mancanti
    FROM spedizioni s
    CROSS JOIN LATERAL (VALUES (s.id_indirizzo_ritiro), (s.id_indirizzo_spedizione)) AS x(id_indirizzo)
    WHERE NOT EXISTS (SELECT 1 FROM indirizzi i WHERE i.id = x.id_indirizzo AND i.id_sede = s.id_sede);
    IF v_mancanti IS NOT NULL THEN
        RAISE EXCEPTION 'indirizzi mancanti o di un''altra sede: %', v_mancanti;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_spedizioni_ritiro_stessa_sede') THEN
        ALTER TABLE spedizioni ADD CONSTRAINT fk_spedizioni_ritiro_stessa_sede
            FOREIGN KEY (id_indirizzo_ritiro, id_sede) REFERENCES indirizzi (id, id_sede);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_spedizioni_consegna_stessa_sede') THEN
        ALTER TABLE spedizioni ADD CONSTRAINT fk_spedizioni_consegna_stessa_sede
            FOREIGN KEY (id_indirizzo_spedizione, id_sede) REFERENCES indirizzi (id, id_sede);
    END IF;
END $$;

-- la copia è finita
DROP TABLE IF EXISTS indirizzi_da_importare;
DROP FUNCTION IF EXISTS fn_importa_indirizzi();

COMMIT;
