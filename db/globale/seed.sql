-- Tutti gli utenti di prova hanno la password PasswordProva1 (solo sviluppo).
-- Dati di prova del DATABASE GLOBALE: da eseguire su un database vuoto, subito dopo
-- globale/schema.sql. Le spedizioni sono in spedizioni/seed.sql, che usa gli id creati
-- qui (database appena creato: tutti gli id partono da 1).
--
-- Due sedi: Trento (id 1, con operatore 1, corriere 1 e 4 spedizioni) e Bolzano (id 2,
-- con operatore 2 e corriere 2), per provare che ogni sede vede solo i suoi dati.
-- Clienti: Mit Tente (1, solo mittente) e Des Tinatario (2, destinatario: può entrare).
DO $$
DECLARE
    u_op  BIGINT; u_cor BIGINT; u_mit BIGINT; u_des BIGINT; u_adm BIGINT;
    u_op_bz BIGINT; u_cor_bz BIGINT;
    op BIGINT; cor BIGINT; mit BIGINT; des BIGINT;
    naz BIGINT; reg BIGINT; tn BIGINT; bz BIGINT;
    ind1 BIGINT; ind2 BIGINT; ind_sede_tn BIGINT; ind_sede_bz BIGINT;
    sede_tn BIGINT; sede_bz BIGINT;
BEGIN
    -- geografia e sedi
    INSERT INTO nazioni (nome) VALUES ('Italia') RETURNING id INTO naz;
    INSERT INTO regioni (nome, id_nazione) VALUES ('Trentino-Alto Adige', naz) RETURNING id INTO reg;
    INSERT INTO citta (nome, id_regione) VALUES ('Trento', reg)  RETURNING id INTO tn;
    INSERT INTO citta (nome, id_regione) VALUES ('Bolzano', reg) RETURNING id INTO bz;

    INSERT INTO indirizzi (via, civico, cod_postale, id_citta) VALUES ('Via Brennero', '1', '38121', tn) RETURNING id INTO ind_sede_tn;
    INSERT INTO indirizzi (via, civico, cod_postale, id_citta) VALUES ('Via Roma', '1', '38122', tn)     RETURNING id INTO ind1;
    INSERT INTO indirizzi (via, civico, cod_postale, id_citta) VALUES ('Via Verdi', '10', '38122', tn)   RETURNING id INTO ind2;
    INSERT INTO indirizzi (via, civico, cod_postale, id_citta) VALUES ('Via Galvani', '2', '39100', bz)  RETURNING id INTO ind_sede_bz;

    INSERT INTO sedi (nome, id_citta, id_indirizzo) VALUES ('Veloce24 Trento', tn, ind_sede_tn)  RETURNING id INTO sede_tn;
    INSERT INTO sedi (nome, id_citta, id_indirizzo) VALUES ('Veloce24 Bolzano', bz, ind_sede_bz) RETURNING id INTO sede_bz;

    -- utenti
    INSERT INTO utenti (nome, cognome, prefisso, nr_telefono, email, pwd_hash)
        VALUES ('Op', 'Erator', '+39', '3330000001', 'operatore@veloce24.test', '$2a$10$SNjhWOVP8LM0v7WcZuAjROVxOjm3QadNZirnsdusfZvt3FjLAl7vG') RETURNING id INTO u_op;
    INSERT INTO utenti (nome, cognome, prefisso, nr_telefono, email, pwd_hash)
        VALUES ('Cor', 'Riere', '+39', '3330000002', 'corriere@veloce24.test', '$2a$10$SNjhWOVP8LM0v7WcZuAjROVxOjm3QadNZirnsdusfZvt3FjLAl7vG') RETURNING id INTO u_cor;
    INSERT INTO utenti (nome, cognome, prefisso, nr_telefono, email, pwd_hash)
        VALUES ('Mit', 'Tente', '+39', '3330000003', 'mittente@veloce24.test', '$2a$10$SNjhWOVP8LM0v7WcZuAjROVxOjm3QadNZirnsdusfZvt3FjLAl7vG') RETURNING id INTO u_mit;
    INSERT INTO utenti (nome, cognome, prefisso, nr_telefono, email, pwd_hash)
        VALUES ('Des', 'Tinatario', '+39', '3330000004', 'destinatario@veloce24.test', '$2a$10$SNjhWOVP8LM0v7WcZuAjROVxOjm3QadNZirnsdusfZvt3FjLAl7vG') RETURNING id INTO u_des;
    INSERT INTO utenti (nome, cognome, prefisso, nr_telefono, email, pwd_hash)
        VALUES ('Ad', 'Min', '+39', '3330000005', 'admin@veloce24.test', '$2a$10$SNjhWOVP8LM0v7WcZuAjROVxOjm3QadNZirnsdusfZvt3FjLAl7vG') RETURNING id INTO u_adm;
    INSERT INTO utenti (nome, cognome, prefisso, nr_telefono, email, pwd_hash)
        VALUES ('Op', 'Bolzano', '+39', '3330000006', 'operatore.bz@veloce24.test', '$2a$10$SNjhWOVP8LM0v7WcZuAjROVxOjm3QadNZirnsdusfZvt3FjLAl7vG') RETURNING id INTO u_op_bz;
    INSERT INTO utenti (nome, cognome, prefisso, nr_telefono, email, pwd_hash)
        VALUES ('Cor', 'Bolzano', '+39', '3330000007', 'corriere.bz@veloce24.test', '$2a$10$SNjhWOVP8LM0v7WcZuAjROVxOjm3QadNZirnsdusfZvt3FjLAl7vG') RETURNING id INTO u_cor_bz;

    INSERT INTO operatori_logistici (id_utente, id_sede) VALUES (u_op, sede_tn)  RETURNING id INTO op;
    INSERT INTO corrieri            (id_utente, id_sede) VALUES (u_cor, sede_tn) RETURNING id INTO cor;
    INSERT INTO operatori_logistici (id_utente, id_sede) VALUES (u_op_bz, sede_bz);
    INSERT INTO corrieri            (id_utente, id_sede) VALUES (u_cor_bz, sede_bz);
    INSERT INTO amministratori      (id_utente) VALUES (u_adm);
    INSERT INTO clienti (id_utente) VALUES (u_mit) RETURNING id INTO mit;
    INSERT INTO clienti (id_utente) VALUES (u_des) RETURNING id INTO des;

    UPDATE clienti SET destinatario = TRUE WHERE id = des;
END $$;
