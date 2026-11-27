-- Dati dimostrativi del database delle SPEDIZIONI: spedizioni in tutti gli stati, con
-- storico, note, tentativi e ultima posizione dei corrieri.
--
-- Da eseguire su un database VUOTO, subito dopo spedizioni/schema.sql, quando il database
-- globale è stato riempito con globale/seed_demo.sql.
--
-- Le spedizioni non vengono inserite già "finite": nascono 'creata' e avanzano con gli
-- stessi passaggi che farebbe il backend (assegnazione, ritiro, uscita in consegna,
-- tentativi falliti, annullamento...). Così i trigger scrivono lo storico e i contatori
-- dei tentativi tornano con quelli che il backend calcola.

BEGIN;

-- ===== Id del database globale (creati da globale/seed_demo.sql) =====
-- Persone, sedi e indirizzi stanno nell'altro database: qui servono solo i loro id.

CREATE TEMP TABLE persone_demo (email text PRIMARY KEY, ruolo text, id bigint, id_sede bigint) ON COMMIT DROP;
INSERT INTO persone_demo VALUES
  ('operatore@veloce24.test',    'operatore', 1, 1),
  ('operatore2@veloce24.test',   'operatore', 2, 2),
  ('corriere@veloce24.test',     'corriere',  1, 1),
  ('corriere2@veloce24.test',    'corriere',  2, 2),
  ('corriere3@veloce24.test',    'corriere',  3, 1),
  ('corriere4@veloce24.test',    'corriere',  4, 2),
  ('mittente@veloce24.test',     'cliente',   1, NULL),
  ('sport@veloce24.test',        'cliente',   2, NULL),
  ('libreria@veloce24.test',     'cliente',   3, NULL),
  ('destinatario@veloce24.test', 'cliente',   4, NULL),
  ('giulia.rossi@veloce24.test', 'cliente',   5, NULL),
  ('luca.bianchi@veloce24.test', 'cliente',   6, NULL),
  ('sara.verdi@veloce24.test',   'cliente',   7, NULL),
  ('paolo.neri@veloce24.test',   'cliente',   8, NULL),
  ('elena.gallo@veloce24.test',  'cliente',   9, NULL);

-- sede 1 = Veloce24 Trento (città 1), sede 2 = Veloce24 Bolzano (città 2)
CREATE TEMP TABLE sedi_demo (id bigint PRIMARY KEY, id_citta bigint) ON COMMIT DROP;
INSERT INTO sedi_demo VALUES (1, 1), (2, 2);

CREATE TEMP TABLE indirizzi_demo (via text PRIMARY KEY, id bigint) ON COMMIT DROP;
INSERT INTO indirizzi_demo VALUES
  ('Via Belenzani', 1), ('Via Manci', 2), ('Via Brennero', 3), ('Viale Verona', 4), ('Via Rosmini', 5),
  ('Via Garibaldi', 6), ('Via Mazzini', 7),
  ('Via Cavour', 8), ('Piazza Walther', 9), ('Via Museo', 10), ('Corso Libertà', 11), ('Via Druso', 12),
  ('Via Roma', 13);

INSERT INTO impostazioni (id_sede) SELECT id FROM sedi_demo ORDER BY id;

-- indirizzi di ritiro e consegna (stessi id di globale/seed_demo.sql): 1-7 Trento, 8-13 Bolzano
INSERT INTO indirizzi (id, id_sede, id_citta, via, civico, cod_postale) OVERRIDING SYSTEM VALUE VALUES
  (1, 1, 1, 'Via Belenzani', '1', '38122'), (2, 1, 1, 'Via Manci', '20', '38122'),
  (3, 1, 1, 'Via Brennero', '150', '38121'), (4, 1, 1, 'Viale Verona', '190', '38123'),
  (5, 1, 1, 'Via Rosmini', '45', '38122'), (6, 1, 1, 'Via Garibaldi', '12', '38122'),
  (7, 1, 1, 'Via Mazzini', '5', '38122'),
  (8, 2, 2, 'Via Cavour', '3', '39100'), (9, 2, 2, 'Piazza Walther', '8', '39100'),
  (10, 2, 2, 'Via Museo', '30', '39100'), (11, 2, 2, 'Corso Libertà', '10', '39100'),
  (12, 2, 2, 'Via Druso', '50', '39100'), (13, 2, 2, 'Via Roma', '77', '39100');
SELECT setval('indirizzi_id_seq', 13);

-- ===== Funzioni di appoggio (temporanee: spariscono a fine sessione) =====

CREATE FUNCTION pg_temp.id_operatore(p_email text) RETURNS bigint LANGUAGE sql AS
$$ SELECT id FROM persone_demo WHERE email = p_email AND ruolo = 'operatore' $$;

CREATE FUNCTION pg_temp.id_corriere(p_email text) RETURNS bigint LANGUAGE sql AS
$$ SELECT id FROM persone_demo WHERE email = p_email AND ruolo = 'corriere' $$;

CREATE FUNCTION pg_temp.id_cliente(p_email text) RETURNS bigint LANGUAGE sql AS
$$ SELECT id FROM persone_demo WHERE email = p_email AND ruolo = 'cliente' $$;

CREATE FUNCTION pg_temp.id_indirizzo(p_via text) RETURNS bigint LANGUAGE sql AS
$$ SELECT id FROM indirizzi_demo WHERE via = p_via $$;


CREATE FUNCTION pg_temp.sede_di(p_sped bigint) RETURNS bigint LANGUAGE sql AS
$$ SELECT id_sede FROM spedizioni WHERE id = p_sped $$;

-- Dice al trigger dello storico chi sta facendo la modifica (uno solo tra operatore e corriere).
CREATE FUNCTION pg_temp.autore(p_ruolo text, p_id bigint) RETURNS void LANGUAGE plpgsql AS $$
BEGIN
    PERFORM set_config('app.id_operatore', CASE WHEN p_ruolo = 'operatore' THEN p_id::text ELSE '' END, true);
    PERFORM set_config('app.id_corriere',  CASE WHEN p_ruolo = 'corriere'  THEN p_id::text ELSE '' END, true);
END $$;

-- Nuova spedizione 'creata' nella sede dell'operatore, inserita p_giorni_fa giorni fa e
-- richiesta due ore prima (la richiesta non è mai nel futuro).
CREATE FUNCTION pg_temp.nuova(p_cod text, p_operatore text, p_mittente text, p_destinatario text,
                              p_ritiro text, p_consegna text, p_giorni_fa integer) RETURNS bigint LANGUAGE plpgsql AS $$
DECLARE v_id bigint;
BEGIN
    INSERT INTO spedizioni (id_sede, id_citta, cod_ordine, id_operatore_ha_creato, id_cliente_mittente, id_cliente_destinatario,
                            creata_il, dataora_richiesta_spedizione, id_indirizzo_ritiro, id_indirizzo_spedizione)
    SELECT s.id, s.id_citta, p_cod, o.id, pg_temp.id_cliente(p_mittente), pg_temp.id_cliente(p_destinatario),
           now() - make_interval(days => p_giorni_fa),
           now() - make_interval(days => p_giorni_fa) - interval '2 hours',
           pg_temp.id_indirizzo(p_ritiro), pg_temp.id_indirizzo(p_consegna)
    FROM persone_demo o JOIN sedi_demo s ON s.id = o.id_sede
    WHERE o.email = p_operatore AND o.ruolo = 'operatore'
    RETURNING id INTO v_id;
    RETURN v_id;
END $$;

-- L'operatore assegna (o riassegna) la spedizione a un corriere della stessa sede.
CREATE FUNCTION pg_temp.assegna(p_sped bigint, p_operatore text, p_corriere text) RETURNS void LANGUAGE plpgsql AS $$
DECLARE v_ass bigint;
BEGIN
    INSERT INTO assegnazioni (id_sede, id_operatore, id_corriere)
    VALUES (pg_temp.sede_di(p_sped), pg_temp.id_operatore(p_operatore), pg_temp.id_corriere(p_corriere))
    RETURNING id INTO v_ass;
    PERFORM pg_temp.autore('operatore', pg_temp.id_operatore(p_operatore));
    UPDATE spedizioni
    SET stato = CASE WHEN stato = 'creata' THEN 'assegnata' ELSE stato END,
        id_assegnazione_corrente = v_ass
    WHERE id = p_sped;
END $$;

-- Cambio di stato fatto da un operatore o da un corriere; al primo passaggio in
-- 'ritirata' viene registrata la data del ritiro, come fa il backend.
CREATE FUNCTION pg_temp.stato(p_sped bigint, p_ruolo text, p_email text, p_stato text) RETURNS void LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_temp.autore(p_ruolo, CASE WHEN p_ruolo = 'operatore' THEN pg_temp.id_operatore(p_email)
                                         ELSE pg_temp.id_corriere(p_email) END);
    UPDATE spedizioni
    SET stato = p_stato::stato_spedizione,
        ritirata_il = CASE WHEN p_stato = 'ritirata' THEN COALESCE(ritirata_il, now()) ELSE ritirata_il END
    WHERE id = p_sped;
END $$;

-- Tentativo di ritiro o di consegna non riuscito: nota pubblica (motivo + nota) del corriere.
CREATE FUNCTION pg_temp.fallito(p_sped bigint, p_corriere text, p_tipo text, p_motivo text, p_nota text) RETURNS void LANGUAGE plpgsql AS $$
DECLARE v_nota bigint;
BEGIN
    INSERT INTO note (id_sede, nota, is_interna, motivo, id_corriere_creata, id_spedizione)
    VALUES (pg_temp.sede_di(p_sped), p_nota, FALSE, p_motivo, pg_temp.id_corriere(p_corriere), p_sped)
    RETURNING id INTO v_nota;
    INSERT INTO tentativi (id_sede, tipo, id_spedizione, id_nota)
    VALUES (pg_temp.sede_di(p_sped), p_tipo::tipo_tentativo, p_sped, v_nota);
END $$;

-- L'operatore chiude la spedizione come 'annullata' o 'persa'; con il motivo scrive anche la nota pubblica.
CREATE FUNCTION pg_temp.chiudi(p_sped bigint, p_operatore text, p_stato text, p_motivo text, p_nota text) RETURNS void LANGUAGE plpgsql AS $$
DECLARE v_op bigint := pg_temp.id_operatore(p_operatore);
BEGIN
    IF p_motivo IS NOT NULL THEN
        INSERT INTO note (id_sede, nota, is_interna, motivo, id_operatore_logistico_creata, id_spedizione)
        VALUES (pg_temp.sede_di(p_sped), p_nota, FALSE, p_motivo, v_op, p_sped);
    END IF;
    PERFORM pg_temp.autore('operatore', v_op);
    IF p_stato = 'annullata' THEN
        UPDATE spedizioni SET stato = 'annullata', id_operatore_ha_annullato = v_op, annullata_il = now() WHERE id = p_sped;
    ELSE
        UPDATE spedizioni SET stato = p_stato::stato_spedizione WHERE id = p_sped;
    END IF;
END $$;

-- Nota interna libera di un operatore o di un corriere.
CREATE FUNCTION pg_temp.nota(p_sped bigint, p_ruolo text, p_email text, p_testo text) RETURNS void LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO note (id_sede, nota, is_interna, id_operatore_logistico_creata, id_corriere_creata, id_spedizione)
    VALUES (pg_temp.sede_di(p_sped), p_testo, TRUE,
            CASE WHEN p_ruolo = 'operatore' THEN pg_temp.id_operatore(p_email) END,
            CASE WHEN p_ruolo = 'corriere'  THEN pg_temp.id_corriere(p_email)  END,
            p_sped);
END $$;

-- ===== Spedizioni =====
-- op1, c1 e c3 lavorano a Trento; op2, c2 e c4 a Bolzano.

DO $$
DECLARE
    op1 CONSTANT text := 'operatore@veloce24.test';
    op2 CONSTANT text := 'operatore2@veloce24.test';
    c1  CONSTANT text := 'corriere@veloce24.test';
    c2  CONSTANT text := 'corriere2@veloce24.test';
    c3  CONSTANT text := 'corriere3@veloce24.test';
    c4  CONSTANT text := 'corriere4@veloce24.test';
    marco    CONSTANT text := 'mittente@veloce24.test';
    sport    CONSTANT text := 'sport@veloce24.test';
    libreria CONSTANT text := 'libreria@veloce24.test';
    dora   CONSTANT text := 'destinatario@veloce24.test';
    giulia CONSTANT text := 'giulia.rossi@veloce24.test';
    luca   CONSTANT text := 'luca.bianchi@veloce24.test';
    sara   CONSTANT text := 'sara.verdi@veloce24.test';
    paolo  CONSTANT text := 'paolo.neri@veloce24.test';
    elena  CONSTANT text := 'elena.gallo@veloce24.test';
    s bigint;
BEGIN
    -- ---- create: ancora senza corriere ----
    PERFORM pg_temp.nuova('ORD-2026-0001', op1, marco, giulia, 'Via Belenzani', 'Via Garibaldi', 0);
    PERFORM pg_temp.nuova('ORD-2026-0002', op2, sport, luca, 'Via Cavour', 'Piazza Walther', 0);
    s := pg_temp.nuova('ORD-2026-0003', op1, libreria, sara, 'Via Rosmini', 'Via Mazzini', 1);
    PERFORM pg_temp.nota(s, 'operatore', op1, 'Il cliente chiede la consegna dopo le 17');

    -- ---- assegnate: il corriere deve ancora ritirare ----
    s := pg_temp.nuova('ORD-2026-0004', op1, marco, dora, 'Via Belenzani', 'Via Manci', 1);
    PERFORM pg_temp.assegna(s, op1, c1);

    s := pg_temp.nuova('ORD-2026-0005', op2, sport, paolo, 'Via Cavour', 'Via Druso', 2);
    PERFORM pg_temp.assegna(s, op2, c2);
    PERFORM pg_temp.fallito(s, c2, 'ritiro', 'mittente assente', 'Negozio chiuso per pausa pranzo');

    s := pg_temp.nuova('ORD-2026-0006', op1, libreria, elena, 'Via Rosmini', 'Viale Verona', 3);
    PERFORM pg_temp.assegna(s, op1, c1);
    PERFORM pg_temp.fallito(s, c1, 'ritiro', 'pacco non pronto', 'Il pacco non era ancora imballato');
    PERFORM pg_temp.fallito(s, c1, 'ritiro', 'pacco non pronto', 'Secondo passaggio: manca ancora la bolla');

    -- tre ritiri falliti: ora tocca all'operatore annullarla
    s := pg_temp.nuova('ORD-2026-0007', op2, sport, giulia, 'Via Cavour', 'Corso Libertà', 4);
    PERFORM pg_temp.assegna(s, op2, c4);
    PERFORM pg_temp.fallito(s, c4, 'ritiro', 'mittente assente', 'Magazzino chiuso');
    PERFORM pg_temp.fallito(s, c4, 'ritiro', 'accesso impossibile', 'Strada chiusa per lavori');
    PERFORM pg_temp.fallito(s, c4, 'ritiro', 'mittente assente', 'Terzo passaggio, nessuno risponde');

    -- ---- ritirate: pacco sul furgone, non ancora in consegna ----
    s := pg_temp.nuova('ORD-2026-0008', op1, marco, luca, 'Via Belenzani', 'Via Brennero', 2);
    PERFORM pg_temp.assegna(s, op1, c1);
    PERFORM pg_temp.stato(s, 'corriere', c1, 'ritirata');

    s := pg_temp.nuova('ORD-2026-0009', op2, libreria, sara, 'Via Roma', 'Via Museo', 3);
    PERFORM pg_temp.assegna(s, op2, c2);
    PERFORM pg_temp.stato(s, 'corriere', c2, 'ritirata');
    PERFORM pg_temp.nota(s, 'corriere', c2, 'Pacco fragile: tenere in piano');

    -- ritirata dopo una consegna fallita: l'operatore l'ha riportata in ritirata
    s := pg_temp.nuova('ORD-2026-0010', op1, sport, dora, 'Via Brennero', 'Via Manci', 4);
    PERFORM pg_temp.assegna(s, op1, c3);
    PERFORM pg_temp.stato(s, 'corriere', c3, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c3, 'in consegna');
    PERFORM pg_temp.fallito(s, c3, 'consegna', 'destinatario assente', 'Citofono senza risposta');
    PERFORM pg_temp.stato(s, 'operatore', op1, 'ritirata');

    -- come sopra, poi riassegnata a un altro corriere della stessa sede
    s := pg_temp.nuova('ORD-2026-0011', op2, marco, paolo, 'Piazza Walther', 'Via Druso', 5);
    PERFORM pg_temp.assegna(s, op2, c4);
    PERFORM pg_temp.stato(s, 'corriere', c4, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c4, 'in consegna');
    PERFORM pg_temp.fallito(s, c4, 'consegna', 'indirizzo errato', 'Al civico indicato non risulta il destinatario');
    PERFORM pg_temp.stato(s, 'operatore', op2, 'ritirata');
    PERFORM pg_temp.nota(s, 'operatore', op2, 'Sentito il cliente: il civico giusto è 50/A. Riassegno a Chiara');
    PERFORM pg_temp.assegna(s, op2, c2);

    -- ---- in consegna: il corriere trasmette la posizione ----
    s := pg_temp.nuova('ORD-2026-0012', op1, marco, dora, 'Via Belenzani', 'Via Manci', 1);
    PERFORM pg_temp.assegna(s, op1, c1);
    PERFORM pg_temp.stato(s, 'corriere', c1, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c1, 'in consegna');

    s := pg_temp.nuova('ORD-2026-0013', op2, sport, luca, 'Via Cavour', 'Via Museo', 2);
    PERFORM pg_temp.assegna(s, op2, c2);
    PERFORM pg_temp.stato(s, 'corriere', c2, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c2, 'in consegna');

    -- seconda uscita in consegna dopo un tentativo fallito
    s := pg_temp.nuova('ORD-2026-0014', op1, libreria, elena, 'Via Rosmini', 'Via Mazzini', 4);
    PERFORM pg_temp.assegna(s, op1, c3);
    PERFORM pg_temp.stato(s, 'corriere', c3, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c3, 'in consegna');
    PERFORM pg_temp.fallito(s, c3, 'consegna', 'destinatario assente', 'Nessuno in casa, lasciato avviso');
    PERFORM pg_temp.stato(s, 'operatore', op1, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c3, 'in consegna');

    -- ---- consegnate ----
    s := pg_temp.nuova('ORD-2026-0015', op1, marco, giulia, 'Via Belenzani', 'Via Garibaldi', 6);
    PERFORM pg_temp.assegna(s, op1, c1);
    PERFORM pg_temp.stato(s, 'corriere', c1, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c1, 'in consegna');
    PERFORM pg_temp.stato(s, 'corriere', c1, 'consegnata');

    s := pg_temp.nuova('ORD-2026-0016', op2, sport, sara, 'Via Cavour', 'Corso Libertà', 7);
    PERFORM pg_temp.assegna(s, op2, c2);
    PERFORM pg_temp.stato(s, 'corriere', c2, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c2, 'in consegna');
    PERFORM pg_temp.stato(s, 'corriere', c2, 'consegnata');

    -- consegnata al secondo tentativo
    s := pg_temp.nuova('ORD-2026-0017', op1, libreria, dora, 'Via Rosmini', 'Via Manci', 8);
    PERFORM pg_temp.assegna(s, op1, c3);
    PERFORM pg_temp.stato(s, 'corriere', c3, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c3, 'in consegna');
    PERFORM pg_temp.fallito(s, c3, 'consegna', 'destinatario assente', 'Portineria chiusa');
    PERFORM pg_temp.stato(s, 'operatore', op1, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c3, 'in consegna');
    PERFORM pg_temp.stato(s, 'corriere', c3, 'consegnata');

    -- ---- non consegnate: tre consegne fallite ----
    s := pg_temp.nuova('ORD-2026-0018', op2, marco, paolo, 'Piazza Walther', 'Via Roma', 9);
    PERFORM pg_temp.assegna(s, op2, c4);
    PERFORM pg_temp.stato(s, 'corriere', c4, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c4, 'in consegna');
    PERFORM pg_temp.fallito(s, c4, 'consegna', 'destinatario assente', 'Primo passaggio: nessuno');
    PERFORM pg_temp.stato(s, 'operatore', op2, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c4, 'in consegna');
    PERFORM pg_temp.fallito(s, c4, 'consegna', 'destinatario assente', 'Secondo passaggio: nessuno');
    PERFORM pg_temp.stato(s, 'operatore', op2, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c4, 'in consegna');
    PERFORM pg_temp.fallito(s, c4, 'consegna', 'rifiutata dal destinatario', 'Il destinatario non vuole più il pacco');
    PERFORM pg_temp.stato(s, 'corriere', c4, 'non consegnata');

    -- non consegnata e poi segnata come reso
    s := pg_temp.nuova('ORD-2026-0019', op1, sport, elena, 'Via Brennero', 'Via Mazzini', 10);
    PERFORM pg_temp.assegna(s, op1, c1);
    PERFORM pg_temp.stato(s, 'corriere', c1, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c1, 'in consegna');
    PERFORM pg_temp.fallito(s, c1, 'consegna', 'accesso impossibile', 'Zona a traffico limitato');
    PERFORM pg_temp.stato(s, 'operatore', op1, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c1, 'in consegna');
    PERFORM pg_temp.fallito(s, c1, 'consegna', 'destinatario assente', 'Negozio del destinatario chiuso');
    PERFORM pg_temp.stato(s, 'operatore', op1, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c1, 'in consegna');
    PERFORM pg_temp.fallito(s, c1, 'consegna', 'destinatario assente', 'Terzo tentativo a vuoto');
    PERFORM pg_temp.stato(s, 'corriere', c1, 'non consegnata');
    PERFORM pg_temp.autore('operatore', pg_temp.id_operatore(op1));
    UPDATE spedizioni SET is_reso = TRUE WHERE id = s;

    -- ---- annullate ----
    s := pg_temp.nuova('ORD-2026-0020', op1, marco, luca, 'Via Belenzani', 'Via Brennero', 5);
    PERFORM pg_temp.chiudi(s, op1, 'annullata', 'richiesta del mittente', 'Il negozio ha annullato l''ordine per telefono');

    -- annullata dopo tre ritiri falliti (i tentativi hanno già i loro motivi)
    s := pg_temp.nuova('ORD-2026-0021', op2, libreria, giulia, 'Via Roma', 'Via Cavour', 6);
    PERFORM pg_temp.assegna(s, op2, c4);
    PERFORM pg_temp.fallito(s, c4, 'ritiro', 'mittente assente', 'Libreria chiusa');
    PERFORM pg_temp.fallito(s, c4, 'ritiro', 'mittente assente', 'Libreria chiusa per inventario');
    PERFORM pg_temp.fallito(s, c4, 'ritiro', 'pacco non pronto', 'Il libro non è ancora arrivato dal grossista');
    PERFORM pg_temp.chiudi(s, op2, 'annullata', NULL, NULL);

    -- ---- perse ----
    s := pg_temp.nuova('ORD-2026-0022', op1, sport, sara, 'Via Brennero', 'Via Mazzini', 7);
    PERFORM pg_temp.assegna(s, op1, c1);
    PERFORM pg_temp.stato(s, 'corriere', c1, 'ritirata');
    PERFORM pg_temp.chiudi(s, op1, 'persa', 'smarrita in deposito', 'Non risulta più sullo scaffale B3 dopo l''inventario');

    s := pg_temp.nuova('ORD-2026-0023', op2, marco, paolo, 'Piazza Walther', 'Via Druso', 8);
    PERFORM pg_temp.assegna(s, op2, c4);
    PERFORM pg_temp.stato(s, 'corriere', c4, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c4, 'in consegna');
    PERFORM pg_temp.chiudi(s, op2, 'persa', 'furto', 'Furgone forzato durante la sosta: denuncia presentata');
END $$;

-- ===== Ultima posizione dei corrieri in consegna =====

INSERT INTO ultime_posizioni_corrieri (id_corriere, id_sede, latitudine, longitudine, rilevata_il)
SELECT c.id, c.id_sede, p.lat, p.lon, now() - make_interval(mins => p.minuti_fa)
FROM (VALUES
  ('corriere@veloce24.test',  46.0671, 11.1225, 0),   -- Carlo, a Trento
  ('corriere2@veloce24.test', 46.4983, 11.3548, 1),   -- Chiara, a Bolzano
  ('corriere3@veloce24.test', 46.0590, 11.1290, 2)    -- Kevin, a Trento
) AS p(email, lat, lon, minuti_fa)
JOIN persone_demo c ON c.email = p.email AND c.ruolo = 'corriere';

COMMIT;

-- ===== Riepilogo (sede 1 = Trento, sede 2 = Bolzano) =====

SELECT id_sede, stato, count(*) AS spedizioni
FROM spedizioni
GROUP BY id_sede, stato ORDER BY id_sede, stato;
