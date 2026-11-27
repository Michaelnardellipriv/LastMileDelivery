-- Dati dimostrativi del DATABASE GLOBALE: più utenti, indirizzi e sedi.
--
-- Da eseguire su un database VUOTO, subito dopo globale/schema.sql; poi, sul database
-- delle spedizioni, spedizioni/schema.sql e spedizioni/seed_demo.sql (che usa gli id
-- creati qui: database appena creato, tutti gli id partono da 1).
--
--   psql -U <utente> -d <database globale>    -f globale/schema.sql -f globale/seed_demo.sql
--   psql -U <utente> -d <database spedizioni> -f spedizioni/schema.sql -f spedizioni/seed_demo.sql
--
-- Tutti gli utenti hanno la password PasswordProva1 (solo sviluppo).
--
-- Due sedi, Trento e Bolzano: ogni sede ha il suo operatore e i suoi corrieri.


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

-- ===== Luoghi, indirizzi e sedi =====

INSERT INTO nazioni (nome) VALUES ('Italia');

INSERT INTO regioni (nome, id_nazione)
SELECT 'Trentino-Alto Adige', id FROM nazioni WHERE nome = 'Italia';

INSERT INTO citta (nome, id_regione)
SELECT c.nome, r.id
FROM (VALUES ('Trento', 1), ('Bolzano', 2)) AS c(nome, ordine)
JOIN regioni r ON r.nome = 'Trentino-Alto Adige'
ORDER BY c.ordine;

INSERT INTO indirizzi (via, civico, cod_postale, id_citta)
SELECT v.via, v.civico, v.cap, c.id
FROM (VALUES
  -- Trento
  ('Via Belenzani',   '1',   '38122', 'Trento',   1),
  ('Via Manci',       '20',  '38122', 'Trento',   2),
  ('Via Brennero',    '150', '38121', 'Trento',   3),
  ('Viale Verona',    '190', '38123', 'Trento',   4),
  ('Via Rosmini',     '45',  '38122', 'Trento',   5),
  ('Via Garibaldi',   '12',  '38122', 'Trento',   6),
  ('Via Mazzini',     '5',   '38122', 'Trento',   7),
  -- Bolzano
  ('Via Cavour',      '3',   '39100', 'Bolzano',  8),
  ('Piazza Walther',  '8',   '39100', 'Bolzano',  9),
  ('Via Museo',       '30',  '39100', 'Bolzano', 10),
  ('Corso Libertà',   '10',  '39100', 'Bolzano', 11),
  ('Via Druso',       '50',  '39100', 'Bolzano', 12),
  ('Via Roma',        '77',  '39100', 'Bolzano', 13)
) AS v(via, civico, cap, citta, ordine)
JOIN citta c ON c.nome = v.citta
ORDER BY v.ordine;

INSERT INTO sedi (nome, id_citta, id_indirizzo)
SELECT v.nome, c.id, i.id
FROM (VALUES ('Veloce24 Trento', 'Trento', 'Via Brennero'), ('Veloce24 Bolzano', 'Bolzano', 'Via Cavour')) AS v(nome, citta, via)
JOIN citta c ON c.nome = v.citta
JOIN indirizzi i ON i.via = v.via AND i.id_citta = c.id
ORDER BY v.nome DESC;

-- ===== Ruoli: Trento ha Oscar, Carlo e Kevin; Bolzano ha Olga, Chiara e Simona =====

INSERT INTO operatori_logistici (id_utente, id_sede)
SELECT u.id, s.id
FROM (VALUES ('operatore@veloce24.test', 'Veloce24 Trento'), ('operatore2@veloce24.test', 'Veloce24 Bolzano')) AS v(email, sede)
JOIN utenti u ON u.email = v.email
JOIN sedi s ON s.nome = v.sede
ORDER BY u.id;

INSERT INTO amministratori (id_utente)
SELECT id FROM utenti WHERE email = 'admin@veloce24.test';

INSERT INTO corrieri (id_utente, id_sede)
SELECT u.id, s.id
FROM (VALUES ('corriere@veloce24.test',  'Veloce24 Trento'),  ('corriere2@veloce24.test', 'Veloce24 Bolzano'),
             ('corriere3@veloce24.test', 'Veloce24 Trento'),  ('corriere4@veloce24.test', 'Veloce24 Bolzano')) AS v(email, sede)
JOIN utenti u ON u.email = v.email
JOIN sedi s ON s.nome = v.sede
ORDER BY u.id;

INSERT INTO clienti (id_utente)
SELECT id FROM utenti
WHERE email IN ('mittente@veloce24.test', 'sport@veloce24.test', 'libreria@veloce24.test', 'destinatario@veloce24.test',
                'giulia.rossi@veloce24.test', 'luca.bianchi@veloce24.test', 'sara.verdi@veloce24.test',
                'paolo.neri@veloce24.test', 'elena.gallo@veloce24.test')
ORDER BY id;

-- i clienti che ricevono spedizioni (spedizioni/seed_demo.sql) possono entrare
UPDATE clienti c SET destinatario = TRUE
FROM utenti u
WHERE u.id = c.id_utente
  AND u.email IN ('destinatario@veloce24.test', 'giulia.rossi@veloce24.test', 'luca.bianchi@veloce24.test',
                  'sara.verdi@veloce24.test', 'paolo.neri@veloce24.test', 'elena.gallo@veloce24.test');

COMMIT;

-- ===== Riepilogo =====

SELECT r.ruolo, u.nome || ' ' || u.cognome AS utente, u.email, r.id AS id_nel_ruolo, se.nome AS sede
FROM (
    SELECT 'operatore' AS ruolo, id, id_utente, id_sede FROM operatori_logistici
    UNION ALL SELECT 'amministratore', id, id_utente, NULL FROM amministratori
    UNION ALL SELECT 'corriere', id, id_utente, id_sede FROM corrieri
    UNION ALL SELECT 'cliente', id, id_utente, NULL FROM clienti
) r
JOIN utenti u ON u.id = r.id_utente
LEFT JOIN sedi se ON se.id = r.id_sede
ORDER BY r.ruolo, r.id;
