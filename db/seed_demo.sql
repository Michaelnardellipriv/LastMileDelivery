-- Dati dimostrativi: più utenti, indirizzi e spedizioni in tutti gli stati.
--
-- Da eseguire su un database VUOTO, subito dopo schema_LastMileDb.sql (al posto di
-- seed_dev.sql e seed_posizione_corriere.sql: gli utenti di sviluppo sono già qui).
--
--   psql -U <utente> -d <database> -f schema_LastMileDb.sql   -- svuota e ricrea le tabelle
--   psql -U <utente> -d <database> -f seed_demo.sql
--
-- Tutti gli utenti hanno la password PasswordProva1 (solo sviluppo).
--
-- Le spedizioni non vengono inserite già "finite": nascono 'creata' e avanzano con gli
-- stessi passaggi che farebbe il backend (assegnazione, ritiro, uscita in consegna,
-- tentativi falliti, annullamento...). Così i trigger scrivono lo storico e i contatori
-- dei tentativi tornano con quelli che il backend calcola.

BEGIN;

-- ===== Utenti =====

INSERT INTO utenti (nome, cognome, prefisso, nr_telefono, email, pwd_hash)
SELECT u.nome, u.cognome, '+39', u.telefono, u.email, '$2a$10$SNjhWOVP8LM0v7WcZuAjROVxOjm3QadNZirnsdusfZvt3FjLAl7vG'
FROM (VALUES
  -- operatori logistici
  ('Oscar',   'Operatore',      '3330000002', 'operatore@veloce24.test'),
  ('Olga',    'Operatrice',     '3330000012', 'operatore2@veloce24.test'),
  -- amministratore
  ('Anna',    'Amministratrice','3330000001', 'admin@veloce24.test'),
  -- corrieri
  ('Carlo',   'Corriere',       '3330000003', 'corriere@veloce24.test'),
  ('Chiara',  'Consegne',       '3330000004', 'corriere2@veloce24.test'),
  ('Kevin',   'Furgoni',        '3330000013', 'corriere3@veloce24.test'),
  ('Simona',  'Strada',         '3330000014', 'corriere4@veloce24.test'),
  -- clienti che spediscono (solo mittenti: non possono fare il login)
  ('Marco',   'Mittente',       '3330000005', 'mittente@veloce24.test'),
  ('Negozio', 'Sport Trento',   '3330000015', 'sport@veloce24.test'),
  ('Libreria','Rosmini',        '3330000016', 'libreria@veloce24.test'),
  -- clienti destinatari
  ('Dora',    'Destinataria',   '3330000006', 'destinatario@veloce24.test'),
  ('Giulia',  'Rossi',          '3330000021', 'giulia.rossi@veloce24.test'),
  ('Luca',    'Bianchi',        '3330000022', 'luca.bianchi@veloce24.test'),
  ('Sara',    'Verdi',          '3330000023', 'sara.verdi@veloce24.test'),
  ('Paolo',   'Neri',           '3330000024', 'paolo.neri@veloce24.test'),
  ('Elena',   'Gallo',          '3330000025', 'elena.gallo@veloce24.test')
) AS u(nome, cognome, telefono, email);

INSERT INTO operatori_logistici (id_utente)
SELECT id FROM utenti WHERE email IN ('operatore@veloce24.test', 'operatore2@veloce24.test') ORDER BY id;

INSERT INTO amministratori (id_utente)
SELECT id FROM utenti WHERE email = 'admin@veloce24.test';

INSERT INTO corrieri (id_utente)
SELECT id FROM utenti WHERE email LIKE 'corriere%@veloce24.test' ORDER BY id;

INSERT INTO clienti (id_utente)
SELECT id FROM utenti
WHERE email IN ('mittente@veloce24.test', 'sport@veloce24.test', 'libreria@veloce24.test', 'destinatario@veloce24.test',
                'giulia.rossi@veloce24.test', 'luca.bianchi@veloce24.test', 'sara.verdi@veloce24.test',
                'paolo.neri@veloce24.test', 'elena.gallo@veloce24.test')
ORDER BY id;

-- ===== Luoghi e indirizzi =====

INSERT INTO nazioni (nome) VALUES ('Italia');

INSERT INTO regioni (nome, id_nazione)
SELECT r.nome, n.id
FROM nazioni n, (VALUES ('Trentino-Alto Adige', 1), ('Veneto', 2), ('Lombardia', 3)) AS r(nome, ordine)
WHERE n.nome = 'Italia'
ORDER BY r.ordine;

INSERT INTO citta (nome, cod_postale, id_regione)
SELECT c.nome, c.cap, r.id
FROM (VALUES
  ('Trento',   '38121', 'Trentino-Alto Adige', 1),
  ('Trento',   '38122', 'Trentino-Alto Adige', 2),
  ('Trento',   '38123', 'Trentino-Alto Adige', 3),
  ('Rovereto', '38068', 'Trentino-Alto Adige', 4),
  ('Bolzano',  '39100', 'Trentino-Alto Adige', 5),
  ('Verona',   '37121', 'Veneto',              6),
  ('Milano',   '20121', 'Lombardia',           7),
  ('Milano',   '20124', 'Lombardia',           8)
) AS c(nome, cap, regione, ordine)
JOIN regioni r ON r.nome = c.regione
ORDER BY c.ordine;

WITH nuovi AS (
  SELECT * FROM (VALUES
    ('Via Belenzani',       '1',   'Trento',   '38122',  1),
    ('Via Manci',           '20',  'Trento',   '38122',  2),
    ('Via Brennero',        '150', 'Trento',   '38121',  3),
    ('Viale Verona',        '190', 'Trento',   '38123',  4),
    ('Corso Rosmini',       '45',  'Rovereto', '38068',  5),
    ('Via Garibaldi',       '12',  'Rovereto', '38068',  6),
    ('Piazza Walther',      '8',   'Bolzano',  '39100',  7),
    ('Via Museo',           '30',  'Bolzano',  '39100',  8),
    ('Via Mazzini',         '5',   'Verona',   '37121',  9),
    ('Corso Porta Nuova',   '77',  'Verona',   '37121', 10),
    ('Via Dante',           '14',  'Milano',   '20121', 11),
    ('Corso Buenos Aires',  '60',  'Milano',   '20124', 12)
  ) AS v(via, civico, citta, cap, ordine)
), inseriti AS (
  INSERT INTO indirizzi (via, civico)
  SELECT via, civico FROM nuovi ORDER BY ordine
  RETURNING id, via, civico
)
INSERT INTO indirizzi_in_citta (id_citta, id_indirizzo)
SELECT c.id, i.id
FROM inseriti i
JOIN nuovi n ON n.via = i.via AND n.civico = i.civico
JOIN citta c ON c.nome = n.citta AND c.cod_postale = n.cap
ORDER BY n.ordine;

-- ===== Funzioni di appoggio (temporanee: spariscono a fine sessione) =====

CREATE FUNCTION pg_temp.id_operatore(p_email text) RETURNS integer LANGUAGE sql AS
$$ SELECT o.id FROM operatori_logistici o JOIN utenti u ON u.id = o.id_utente WHERE u.email = p_email $$;

CREATE FUNCTION pg_temp.id_corriere(p_email text) RETURNS integer LANGUAGE sql AS
$$ SELECT c.id FROM corrieri c JOIN utenti u ON u.id = c.id_utente WHERE u.email = p_email $$;

CREATE FUNCTION pg_temp.id_cliente(p_email text) RETURNS integer LANGUAGE sql AS
$$ SELECT c.id FROM clienti c JOIN utenti u ON u.id = c.id_utente WHERE u.email = p_email $$;

CREATE FUNCTION pg_temp.id_indirizzo(p_via text) RETURNS integer LANGUAGE sql AS
$$ SELECT id FROM indirizzi WHERE via = p_via $$;

-- Dice al trigger dello storico chi sta facendo la modifica (uno solo tra operatore e corriere).
CREATE FUNCTION pg_temp.autore(p_ruolo text, p_id integer) RETURNS void LANGUAGE plpgsql AS $$
BEGIN
    PERFORM set_config('app.id_operatore', CASE WHEN p_ruolo = 'operatore' THEN p_id::text ELSE '' END, true);
    PERFORM set_config('app.id_corriere',  CASE WHEN p_ruolo = 'corriere'  THEN p_id::text ELSE '' END, true);
END $$;

-- Nuova spedizione 'creata', inserita p_giorni_fa giorni fa e richiesta due ore prima
-- (la richiesta non è mai nel futuro).
CREATE FUNCTION pg_temp.nuova(p_cod text, p_operatore text, p_mittente text, p_destinatario text,
                              p_ritiro text, p_consegna text, p_giorni_fa integer) RETURNS integer LANGUAGE plpgsql AS $$
DECLARE v_id integer;
BEGIN
    INSERT INTO spedizioni (cod_ordine, id_operatore_ha_creato, id_cliente_mittente, id_cliente_destinatario,
                            creata_il, dataora_richiesta_spedizione, id_indirizzo_ritiro, id_indirizzo_spedizione)
    VALUES (p_cod, pg_temp.id_operatore(p_operatore), pg_temp.id_cliente(p_mittente), pg_temp.id_cliente(p_destinatario),
            now() - make_interval(days => p_giorni_fa),
            now() - make_interval(days => p_giorni_fa) - interval '2 hours',
            pg_temp.id_indirizzo(p_ritiro), pg_temp.id_indirizzo(p_consegna))
    RETURNING id INTO v_id;
    RETURN v_id;
END $$;

-- L'operatore assegna (o riassegna) la spedizione a un corriere.
CREATE FUNCTION pg_temp.assegna(p_sped integer, p_operatore text, p_corriere text) RETURNS void LANGUAGE plpgsql AS $$
DECLARE v_ass integer;
BEGIN
    INSERT INTO assegnazioni (id_operatore, id_corriere)
    VALUES (pg_temp.id_operatore(p_operatore), pg_temp.id_corriere(p_corriere))
    RETURNING id INTO v_ass;
    PERFORM pg_temp.autore('operatore', pg_temp.id_operatore(p_operatore));
    UPDATE spedizioni
    SET stato = CASE WHEN stato = 'creata' THEN 'assegnata' ELSE stato END,
        id_assegnazione_corrente = v_ass
    WHERE id = p_sped;
END $$;

-- Cambio di stato fatto da un operatore o da un corriere; al primo passaggio in
-- 'ritirata' viene registrata la data del ritiro, come fa il backend.
CREATE FUNCTION pg_temp.stato(p_sped integer, p_ruolo text, p_email text, p_stato text) RETURNS void LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_temp.autore(p_ruolo, CASE WHEN p_ruolo = 'operatore' THEN pg_temp.id_operatore(p_email)
                                         ELSE pg_temp.id_corriere(p_email) END);
    UPDATE spedizioni
    SET stato = p_stato::stato_spedizione,
        ritirata_il = CASE WHEN p_stato = 'ritirata' THEN COALESCE(ritirata_il, now()) ELSE ritirata_il END
    WHERE id = p_sped;
END $$;

-- Tentativo di ritiro o di consegna non riuscito: nota pubblica (motivo + nota) del corriere.
CREATE FUNCTION pg_temp.fallito(p_sped integer, p_corriere text, p_tipo text, p_motivo text, p_nota text) RETURNS void LANGUAGE plpgsql AS $$
DECLARE v_nota integer;
BEGIN
    INSERT INTO note (nota, is_interna, motivo, id_corriere_creata, id_spedizione)
    VALUES (p_nota, FALSE, p_motivo, pg_temp.id_corriere(p_corriere), p_sped)
    RETURNING id INTO v_nota;
    INSERT INTO tentativi (tipo, id_spedizione, id_nota) VALUES (p_tipo::tipo_tentativo, p_sped, v_nota);
END $$;

-- L'operatore chiude la spedizione come 'annullata' o 'persa'; con il motivo scrive anche la nota pubblica.
CREATE FUNCTION pg_temp.chiudi(p_sped integer, p_operatore text, p_stato text, p_motivo text, p_nota text) RETURNS void LANGUAGE plpgsql AS $$
DECLARE v_op integer := pg_temp.id_operatore(p_operatore);
BEGIN
    IF p_motivo IS NOT NULL THEN
        INSERT INTO note (nota, is_interna, motivo, id_operatore_logistico_creata, id_spedizione)
        VALUES (p_nota, FALSE, p_motivo, v_op, p_sped);
    END IF;
    PERFORM pg_temp.autore('operatore', v_op);
    IF p_stato = 'annullata' THEN
        UPDATE spedizioni SET stato = 'annullata', id_operatore_ha_annullato = v_op, annullata_il = now() WHERE id = p_sped;
    ELSE
        UPDATE spedizioni SET stato = p_stato::stato_spedizione WHERE id = p_sped;
    END IF;
END $$;

-- Nota interna libera di un operatore o di un corriere.
CREATE FUNCTION pg_temp.nota(p_sped integer, p_ruolo text, p_email text, p_testo text) RETURNS void LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO note (nota, is_interna, id_operatore_logistico_creata, id_corriere_creata, id_spedizione)
    VALUES (p_testo, TRUE,
            CASE WHEN p_ruolo = 'operatore' THEN pg_temp.id_operatore(p_email) END,
            CASE WHEN p_ruolo = 'corriere'  THEN pg_temp.id_corriere(p_email)  END,
            p_sped);
END $$;

-- ===== Spedizioni =====

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
    s integer;
BEGIN
    -- ---- create: ancora senza corriere ----
    PERFORM pg_temp.nuova('ORD-2026-0001', op1, marco, giulia, 'Via Belenzani', 'Via Garibaldi', 0);
    PERFORM pg_temp.nuova('ORD-2026-0002', op2, sport, luca, 'Via Brennero', 'Piazza Walther', 0);
    s := pg_temp.nuova('ORD-2026-0003', op1, libreria, sara, 'Corso Rosmini', 'Via Mazzini', 1);
    PERFORM pg_temp.nota(s, 'operatore', op1, 'Il cliente chiede la consegna dopo le 17');

    -- ---- assegnate: il corriere deve ancora ritirare ----
    s := pg_temp.nuova('ORD-2026-0004', op1, marco, dora, 'Via Belenzani', 'Via Manci', 1);
    PERFORM pg_temp.assegna(s, op1, c1);

    s := pg_temp.nuova('ORD-2026-0005', op2, sport, paolo, 'Via Brennero', 'Viale Verona', 2);
    PERFORM pg_temp.assegna(s, op2, c2);
    PERFORM pg_temp.fallito(s, c2, 'ritiro', 'mittente assente', 'Negozio chiuso per pausa pranzo');

    s := pg_temp.nuova('ORD-2026-0006', op1, libreria, elena, 'Corso Rosmini', 'Via Museo', 3);
    PERFORM pg_temp.assegna(s, op1, c1);
    PERFORM pg_temp.fallito(s, c1, 'ritiro', 'pacco non pronto', 'Il pacco non era ancora imballato');
    PERFORM pg_temp.fallito(s, c1, 'ritiro', 'pacco non pronto', 'Secondo passaggio: manca ancora la bolla');

    -- tre ritiri falliti: ora tocca all'operatore annullarla
    s := pg_temp.nuova('ORD-2026-0007', op2, sport, giulia, 'Via Brennero', 'Via Garibaldi', 4);
    PERFORM pg_temp.assegna(s, op2, c3);
    PERFORM pg_temp.fallito(s, c3, 'ritiro', 'mittente assente', 'Magazzino chiuso');
    PERFORM pg_temp.fallito(s, c3, 'ritiro', 'accesso impossibile', 'Strada chiusa per lavori');
    PERFORM pg_temp.fallito(s, c3, 'ritiro', 'mittente assente', 'Terzo passaggio, nessuno risponde');

    -- ---- ritirate: pacco sul furgone, non ancora in consegna ----
    s := pg_temp.nuova('ORD-2026-0008', op1, marco, luca, 'Via Belenzani', 'Piazza Walther', 2);
    PERFORM pg_temp.assegna(s, op1, c1);
    PERFORM pg_temp.stato(s, 'corriere', c1, 'ritirata');

    s := pg_temp.nuova('ORD-2026-0009', op2, libreria, sara, 'Corso Rosmini', 'Corso Porta Nuova', 3);
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

    -- come sopra, poi riassegnata a un altro corriere
    s := pg_temp.nuova('ORD-2026-0011', op2, marco, paolo, 'Via Belenzani', 'Viale Verona', 5);
    PERFORM pg_temp.assegna(s, op2, c4);
    PERFORM pg_temp.stato(s, 'corriere', c4, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c4, 'in consegna');
    PERFORM pg_temp.fallito(s, c4, 'consegna', 'indirizzo errato', 'Al civico indicato non risulta il destinatario');
    PERFORM pg_temp.stato(s, 'operatore', op2, 'ritirata');
    PERFORM pg_temp.nota(s, 'operatore', op2, 'Sentito il cliente: il civico giusto è 190/A. Riassegno a Carlo');
    PERFORM pg_temp.assegna(s, op2, c1);

    -- ---- in consegna: il corriere trasmette la posizione ----
    s := pg_temp.nuova('ORD-2026-0012', op1, marco, dora, 'Via Belenzani', 'Via Manci', 1);
    PERFORM pg_temp.assegna(s, op1, c1);
    PERFORM pg_temp.stato(s, 'corriere', c1, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c1, 'in consegna');

    s := pg_temp.nuova('ORD-2026-0013', op2, sport, luca, 'Via Brennero', 'Via Museo', 2);
    PERFORM pg_temp.assegna(s, op2, c2);
    PERFORM pg_temp.stato(s, 'corriere', c2, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c2, 'in consegna');

    -- seconda uscita in consegna dopo un tentativo fallito
    s := pg_temp.nuova('ORD-2026-0014', op1, libreria, elena, 'Corso Rosmini', 'Via Mazzini', 4);
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

    s := pg_temp.nuova('ORD-2026-0016', op2, sport, sara, 'Via Brennero', 'Corso Porta Nuova', 7);
    PERFORM pg_temp.assegna(s, op2, c2);
    PERFORM pg_temp.stato(s, 'corriere', c2, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c2, 'in consegna');
    PERFORM pg_temp.stato(s, 'corriere', c2, 'consegnata');

    -- consegnata al secondo tentativo
    s := pg_temp.nuova('ORD-2026-0017', op1, libreria, dora, 'Corso Rosmini', 'Via Manci', 8);
    PERFORM pg_temp.assegna(s, op1, c4);
    PERFORM pg_temp.stato(s, 'corriere', c4, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c4, 'in consegna');
    PERFORM pg_temp.fallito(s, c4, 'consegna', 'destinatario assente', 'Portineria chiusa');
    PERFORM pg_temp.stato(s, 'operatore', op1, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c4, 'in consegna');
    PERFORM pg_temp.stato(s, 'corriere', c4, 'consegnata');

    -- ---- non consegnate: tre consegne fallite ----
    s := pg_temp.nuova('ORD-2026-0018', op2, marco, paolo, 'Via Belenzani', 'Via Dante', 9);
    PERFORM pg_temp.assegna(s, op2, c3);
    PERFORM pg_temp.stato(s, 'corriere', c3, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c3, 'in consegna');
    PERFORM pg_temp.fallito(s, c3, 'consegna', 'destinatario assente', 'Primo passaggio: nessuno');
    PERFORM pg_temp.stato(s, 'operatore', op2, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c3, 'in consegna');
    PERFORM pg_temp.fallito(s, c3, 'consegna', 'destinatario assente', 'Secondo passaggio: nessuno');
    PERFORM pg_temp.stato(s, 'operatore', op2, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c3, 'in consegna');
    PERFORM pg_temp.fallito(s, c3, 'consegna', 'rifiutata dal destinatario', 'Il destinatario non vuole più il pacco');
    PERFORM pg_temp.stato(s, 'corriere', c3, 'non consegnata');

    -- non consegnata e poi segnata come reso
    s := pg_temp.nuova('ORD-2026-0019', op1, sport, elena, 'Via Brennero', 'Corso Buenos Aires', 10);
    PERFORM pg_temp.assegna(s, op1, c2);
    PERFORM pg_temp.stato(s, 'corriere', c2, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c2, 'in consegna');
    PERFORM pg_temp.fallito(s, c2, 'consegna', 'accesso impossibile', 'Zona a traffico limitato');
    PERFORM pg_temp.stato(s, 'operatore', op1, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c2, 'in consegna');
    PERFORM pg_temp.fallito(s, c2, 'consegna', 'destinatario assente', 'Negozio del destinatario chiuso');
    PERFORM pg_temp.stato(s, 'operatore', op1, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c2, 'in consegna');
    PERFORM pg_temp.fallito(s, c2, 'consegna', 'destinatario assente', 'Terzo tentativo a vuoto');
    PERFORM pg_temp.stato(s, 'corriere', c2, 'non consegnata');
    PERFORM pg_temp.autore('operatore', pg_temp.id_operatore(op1));
    UPDATE spedizioni SET is_reso = TRUE WHERE id = s;

    -- ---- annullate ----
    s := pg_temp.nuova('ORD-2026-0020', op1, marco, luca, 'Via Belenzani', 'Piazza Walther', 5);
    PERFORM pg_temp.chiudi(s, op1, 'annullata', 'richiesta del mittente', 'Il negozio ha annullato l''ordine per telefono');

    -- annullata dopo tre ritiri falliti (i tentativi hanno già i loro motivi)
    s := pg_temp.nuova('ORD-2026-0021', op2, libreria, giulia, 'Corso Rosmini', 'Via Garibaldi', 6);
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

    s := pg_temp.nuova('ORD-2026-0023', op2, marco, paolo, 'Via Belenzani', 'Viale Verona', 8);
    PERFORM pg_temp.assegna(s, op2, c4);
    PERFORM pg_temp.stato(s, 'corriere', c4, 'ritirata');
    PERFORM pg_temp.stato(s, 'corriere', c4, 'in consegna');
    PERFORM pg_temp.chiudi(s, op2, 'persa', 'furto', 'Furgone forzato durante la sosta: denuncia presentata');
END $$;

-- ===== Posizioni dei corrieri in consegna (le più recenti in fondo) =====

INSERT INTO posizioni_corrieri (id_corriere, latitudine, longitudine, rilevata_il)
SELECT pg_temp.id_corriere(p.email), p.lat, p.lon, now() - make_interval(mins => p.minuti_fa)
FROM (VALUES
  ('corriere@veloce24.test',  46.0705, 11.1190, 9),   -- Carlo, a Trento
  ('corriere@veloce24.test',  46.0692, 11.1203, 6),
  ('corriere@veloce24.test',  46.0679, 11.1211, 3),
  ('corriere@veloce24.test',  46.0671, 11.1225, 0),
  ('corriere2@veloce24.test', 46.4930, 11.3480, 8),   -- Chiara, a Bolzano
  ('corriere2@veloce24.test', 46.4962, 11.3521, 4),
  ('corriere2@veloce24.test', 46.4983, 11.3548, 0),
  ('corriere3@veloce24.test', 45.4420, 10.9870, 7),   -- Kevin, a Verona
  ('corriere3@veloce24.test', 45.4399, 10.9902, 3),
  ('corriere3@veloce24.test', 45.4384, 10.9916, 0)
) AS p(email, lat, lon, minuti_fa)
ORDER BY p.minuti_fa DESC;

COMMIT;

-- ===== Riepilogo =====

SELECT stato, count(*) AS spedizioni FROM spedizioni GROUP BY stato ORDER BY stato;

SELECT r.ruolo, u.nome || ' ' || u.cognome AS utente, u.email, r.id AS id_nel_ruolo
FROM (
    SELECT 'operatore' AS ruolo, id, id_utente FROM operatori_logistici
    UNION ALL SELECT 'amministratore', id, id_utente FROM amministratori
    UNION ALL SELECT 'corriere', id, id_utente FROM corrieri
    UNION ALL SELECT 'cliente', id, id_utente FROM clienti
) r
JOIN utenti u ON u.id = r.id_utente
ORDER BY r.ruolo, r.id;
