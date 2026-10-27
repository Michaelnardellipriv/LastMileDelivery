INSERT INTO utenti (nome, cognome, prefisso, nr_telefono, email) VALUES
 ('Op','Erator','+39','3330000001','operatore@veloce24.test'),
 ('Cor','Riere','+39','3330000002','corriere@veloce24.test'),
 ('Mit','Tente','+39','3330000003','mittente@veloce24.test'),
 ('Des','Tinatario','+39','3330000004','destinatario@veloce24.test'),
 ('Ad','Min','+39','3330000005','admin@veloce24.test');

INSERT INTO operatori_logistici (pwd_hash,id_utente) SELECT 'x',id FROM utenti WHERE email='operatore@veloce24.test';
INSERT INTO corrieri (pwd_hash,id_utente) SELECT 'x',id FROM utenti WHERE email='corriere@veloce24.test';
INSERT INTO amministratori (pwd_hash,id_utente) SELECT 'x',id FROM utenti WHERE email='admin@veloce24.test';
INSERT INTO clienti (id_utente) SELECT id FROM utenti WHERE email IN ('mittente@veloce24.test','destinatario@veloce24.test');

INSERT INTO indirizzi (via,civico) VALUES ('Via Roma','1'),('Via Verdi','10');

INSERT INTO assegnazioni (id_operatore,id_corriere)
 SELECT (SELECT id FROM operatori_logistici LIMIT 1),(SELECT id FROM corrieri LIMIT 1);

-- 1: creata (non assegnata), 2: ritirata, 3: in consegna
INSERT INTO spedizioni (cod_ordine,stato,id_operatore_ha_creato,id_cliente_mittente,id_cliente_destinatario,
  dataora_richiesta_spedizione,id_indirizzo_ritiro,id_indirizzo_spedizione,id_assegnazione_corrente)
SELECT 'ORD-1','creata',
 (SELECT id FROM operatori_logistici LIMIT 1),
 (SELECT c.id FROM clienti c JOIN utenti u ON u.id=c.id_utente WHERE u.email='mittente@veloce24.test'),
 (SELECT c.id FROM clienti c JOIN utenti u ON u.id=c.id_utente WHERE u.email='destinatario@veloce24.test'),
 now(),(SELECT min(id) FROM indirizzi),(SELECT max(id) FROM indirizzi),NULL
UNION ALL SELECT 'ORD-2','ritirata',
 (SELECT id FROM operatori_logistici LIMIT 1),
 (SELECT c.id FROM clienti c JOIN utenti u ON u.id=c.id_utente WHERE u.email='mittente@veloce24.test'),
 (SELECT c.id FROM clienti c JOIN utenti u ON u.id=c.id_utente WHERE u.email='destinatario@veloce24.test'),
 now(),(SELECT min(id) FROM indirizzi),(SELECT max(id) FROM indirizzi),NULL
UNION ALL SELECT 'ORD-3','in consegna',
 (SELECT id FROM operatori_logistici LIMIT 1),
 (SELECT c.id FROM clienti c JOIN utenti u ON u.id=c.id_utente WHERE u.email='mittente@veloce24.test'),
 (SELECT c.id FROM clienti c JOIN utenti u ON u.id=c.id_utente WHERE u.email='destinatario@veloce24.test'),
 now(),(SELECT min(id) FROM indirizzi),(SELECT max(id) FROM indirizzi),NULL;

-- ORD-2 e ORD-3 devono avere l'assegnazione (vincolo del DB)
UPDATE spedizioni SET id_assegnazione_corrente = NULL WHERE FALSE; -- placeholder, vedi nota sotto