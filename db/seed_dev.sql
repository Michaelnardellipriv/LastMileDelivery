-- Tutti gli utenti di prova hanno la password PasswordProva1 (solo sviluppo).

BEGIN;

INSERT INTO utenti (nome, cognome, prefisso, nr_telefono, email, pwd_hash) VALUES
  ('Oscar', 'Operatore',    '+39', '3330000002', 'operatore@veloce24.test', '$2a$10$SNjhWOVP8LM0v7WcZuAjROVxOjm3QadNZirnsdusfZvt3FjLAl7vG'),
  ('Carlo', 'Corriere',     '+39', '3330000003', 'corriere@veloce24.test', '$2a$10$SNjhWOVP8LM0v7WcZuAjROVxOjm3QadNZirnsdusfZvt3FjLAl7vG'),
  ('Marco', 'Mittente',     '+39', '3330000005', 'mittente@veloce24.test', '$2a$10$SNjhWOVP8LM0v7WcZuAjROVxOjm3QadNZirnsdusfZvt3FjLAl7vG'),
  ('Dora',  'Destinataria', '+39', '3330000006', 'destinatario@veloce24.test', '$2a$10$SNjhWOVP8LM0v7WcZuAjROVxOjm3QadNZirnsdusfZvt3FjLAl7vG');


INSERT INTO operatori_logistici (id_utente)
SELECT id FROM utenti WHERE email = 'operatore@veloce24.test';

INSERT INTO corrieri (id_utente)
SELECT id FROM utenti WHERE email = 'corriere@veloce24.test';

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
