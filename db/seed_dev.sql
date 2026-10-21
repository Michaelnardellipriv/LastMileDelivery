
BEGIN;

INSERT INTO utenti (nome, cognome, prefisso, nr_telefono, email) VALUES
  ('Oscar', 'Operatore',    '+39', '3330000002', 'operatore@veloce24.test'),
  ('Carlo', 'Corriere',     '+39', '3330000003', 'corriere@veloce24.test'),
  ('Marco', 'Mittente',     '+39', '3330000005', 'mittente@veloce24.test'),
  ('Dora',  'Destinataria', '+39', '3330000006', 'destinatario@veloce24.test');


INSERT INTO operatori_logistici (pwd_hash, id_utente)
SELECT 'da-sostituire', id FROM utenti WHERE email = 'operatore@veloce24.test';

INSERT INTO corrieri (pwd_hash, id_utente)
SELECT 'da-sostituire', id FROM utenti WHERE email = 'corriere@veloce24.test';

INSERT INTO clienti (id_utente) SELECT id FROM utenti WHERE email = 'mittente@veloce24.test';
INSERT INTO clienti (id_utente) SELECT id FROM utenti WHERE email = 'destinatario@veloce24.test';

INSERT INTO nazioni (nome) VALUES ('Italia');
INSERT INTO regioni (nome, id_nazione) SELECT 'Trentino-Alto Adige', id FROM nazioni WHERE nome = 'Italia';
INSERT INTO citta (nome, cod_postale, id_regione)
SELECT c.nome, c.cap, r.id
FROM regioni r, (VALUES ('Trento', '38121', 1), ('Bolzano', '39100', 2)) AS c(nome, cap, ordine)
WHERE r.nome = 'Trentino-Alto Adige'
ORDER BY c.ordine;

COMMIT;
