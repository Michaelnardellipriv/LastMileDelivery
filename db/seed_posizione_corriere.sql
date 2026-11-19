-- Tutti gli utenti di prova hanno la password PasswordProva1 (solo sviluppo).
-- Dati di prova per la posizione dei corrieri (solo sviluppo).
-- Da eseguire DOPO schema_LastMileDb.sql e seed_dev.sql.
--
-- Crea:
--   - un amministratore            (admin@veloce24.test)
--   - un secondo corriere          (corriere2@veloce24.test) con una spedizione solo "ritirata"
--   - due indirizzi a Trento
--   - una spedizione "in consegna" assegnata a corriere@veloce24.test
--   - una spedizione "ritirata"    assegnata a corriere2@veloce24.test
--
-- Così si possono provare tutti i casi: corriere attivo (può inviare),
-- corriere con pacco solo ritirato (rifiutato), operatore (vede), amministratore (intervallo).

BEGIN;

INSERT INTO utenti (nome, cognome, prefisso, nr_telefono, email, pwd_hash) VALUES
  ('Anna',  'Admin',     '+39', '3330000001', 'admin@veloce24.test', '$2a$10$SNjhWOVP8LM0v7WcZuAjROVxOjm3QadNZirnsdusfZvt3FjLAl7vG'),
  ('Paolo', 'Corriere2', '+39', '3330000004', 'corriere2@veloce24.test', '$2a$10$SNjhWOVP8LM0v7WcZuAjROVxOjm3QadNZirnsdusfZvt3FjLAl7vG');

INSERT INTO amministratori (id_utente)
SELECT id FROM utenti WHERE email = 'admin@veloce24.test';

INSERT INTO corrieri (id_utente)
SELECT id FROM utenti WHERE email = 'corriere2@veloce24.test';

INSERT INTO indirizzi (via, civico) VALUES
  ('Via Belenzani', '1'),
  ('Via Manci',     '20');

INSERT INTO indirizzi_in_citta (id_citta, id_indirizzo)
SELECT c.id, i.id
FROM citta c, indirizzi i
WHERE c.nome = 'Trento' AND i.via IN ('Via Belenzani', 'Via Manci');

-- assegnazione: l'operatore affida il lavoro a corriere@veloce24.test
INSERT INTO assegnazioni (id_operatore, id_corriere)
SELECT ol.id, co.id
FROM operatori_logistici ol
JOIN utenti uo ON uo.id = ol.id_utente AND uo.email = 'operatore@veloce24.test'
CROSS JOIN corrieri co
JOIN utenti uc ON uc.id = co.id_utente AND uc.email = 'corriere@veloce24.test';

-- spedizione già "in consegna": il corriere può trasmettere la posizione
INSERT INTO spedizioni (
    stato, cod_ordine, id_operatore_ha_creato,
    id_cliente_mittente, id_cliente_destinatario, dataora_richiesta_spedizione, ritirata_il,
    id_indirizzo_ritiro, id_indirizzo_spedizione, id_assegnazione_corrente
)
SELECT 'in consegna', 'SEED-POS-001', a.id_operatore,
       (SELECT cl.id FROM clienti cl JOIN utenti u ON u.id = cl.id_utente WHERE u.email = 'mittente@veloce24.test'),
       (SELECT cl.id FROM clienti cl JOIN utenti u ON u.id = cl.id_utente WHERE u.email = 'destinatario@veloce24.test'),
       now(), now(),
       (SELECT id FROM indirizzi WHERE via = 'Via Belenzani'),
       (SELECT id FROM indirizzi WHERE via = 'Via Manci'),
       a.id
FROM assegnazioni a
ORDER BY a.id DESC
LIMIT 1;

-- assegnazione a corriere2: pacco ritirato ma NON ancora in consegna,
-- quindi la sua posizione deve essere rifiutata
INSERT INTO assegnazioni (id_operatore, id_corriere)
SELECT ol.id, co.id
FROM operatori_logistici ol
JOIN utenti uo ON uo.id = ol.id_utente AND uo.email = 'operatore@veloce24.test'
CROSS JOIN corrieri co
JOIN utenti uc ON uc.id = co.id_utente AND uc.email = 'corriere2@veloce24.test';

INSERT INTO spedizioni (
    stato, cod_ordine, id_operatore_ha_creato,
    id_cliente_mittente, id_cliente_destinatario, dataora_richiesta_spedizione, ritirata_il,
    id_indirizzo_ritiro, id_indirizzo_spedizione, id_assegnazione_corrente
)
SELECT 'ritirata', 'SEED-POS-002', a.id_operatore,
       (SELECT cl.id FROM clienti cl JOIN utenti u ON u.id = cl.id_utente WHERE u.email = 'mittente@veloce24.test'),
       (SELECT cl.id FROM clienti cl JOIN utenti u ON u.id = cl.id_utente WHERE u.email = 'destinatario@veloce24.test'),
       now(), now(),
       (SELECT id FROM indirizzi WHERE via = 'Via Belenzani'),
       (SELECT id FROM indirizzi WHERE via = 'Via Manci'),
       a.id
FROM assegnazioni a
ORDER BY a.id DESC
LIMIT 1;

COMMIT;

-- Gli id da usare negli header delle richieste:
SELECT 'X-Id-Corriere (attivo)'     AS header, co.id FROM corrieri co JOIN utenti u ON u.id = co.id_utente WHERE u.email = 'corriere@veloce24.test'
UNION ALL
SELECT 'X-Id-Corriere (solo ritirata)', co.id FROM corrieri co JOIN utenti u ON u.id = co.id_utente WHERE u.email = 'corriere2@veloce24.test'
UNION ALL
SELECT 'X-Id-Operatore',             ol.id FROM operatori_logistici ol JOIN utenti u ON u.id = ol.id_utente WHERE u.email = 'operatore@veloce24.test'
UNION ALL
SELECT 'X-Id-Amministratore',        am.id FROM amministratori am JOIN utenti u ON u.id = am.id_utente WHERE u.email = 'admin@veloce24.test';
