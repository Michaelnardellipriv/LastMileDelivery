-- Dati di prova per testare GET /api/spedizioni
-- Da eseguire su un database con lo schema appena caricato (database vuoto).
DO $$
DECLARE
    u_op  INT; u_cor INT; u_mit INT; u_des INT; u_adm INT;
    op INT; cor INT; mit INT; des INT;
    ind1 INT; ind2 INT;
    ass2 INT; ass3 INT;
    sp2 INT;
BEGIN
    INSERT INTO utenti (nome, cognome, prefisso, nr_telefono, email)
        VALUES ('Op', 'Erator', '+39', '3330000001', 'operatore@veloce24.test') RETURNING id INTO u_op;
    INSERT INTO utenti (nome, cognome, prefisso, nr_telefono, email)
        VALUES ('Cor', 'Riere', '+39', '3330000002', 'corriere@veloce24.test') RETURNING id INTO u_cor;
    INSERT INTO utenti (nome, cognome, prefisso, nr_telefono, email)
        VALUES ('Mit', 'Tente', '+39', '3330000003', 'mittente@veloce24.test') RETURNING id INTO u_mit;
    INSERT INTO utenti (nome, cognome, prefisso, nr_telefono, email)
        VALUES ('Des', 'Tinatario', '+39', '3330000004', 'destinatario@veloce24.test') RETURNING id INTO u_des;
    INSERT INTO utenti (nome, cognome, prefisso, nr_telefono, email)
        VALUES ('Ad', 'Min', '+39', '3330000005', 'admin@veloce24.test') RETURNING id INTO u_adm;

    INSERT INTO operatori_logistici (pwd_hash, id_utente) VALUES ('x', u_op)  RETURNING id INTO op;
    INSERT INTO corrieri            (pwd_hash, id_utente) VALUES ('x', u_cor) RETURNING id INTO cor;
    INSERT INTO amministratori      (pwd_hash, id_utente) VALUES ('x', u_adm);
    INSERT INTO clienti (id_utente) VALUES (u_mit) RETURNING id INTO mit;
    INSERT INTO clienti (id_utente) VALUES (u_des) RETURNING id INTO des;

    INSERT INTO indirizzi (via, civico) VALUES ('Via Roma', '1')   RETURNING id INTO ind1;
    INSERT INTO indirizzi (via, civico) VALUES ('Via Verdi', '10') RETURNING id INTO ind2;

    -- ORD-1: creata, non assegnata
    INSERT INTO spedizioni (cod_ordine, stato, id_operatore_ha_creato, id_cliente_mittente, id_cliente_destinatario,
                            dataora_richiesta_spedizione, id_indirizzo_ritiro, id_indirizzo_spedizione)
        VALUES ('ORD-1', 'creata', op, mit, des, now(), ind1, ind2);

    -- ORD-2: ritirata (serve un'assegnazione)
    INSERT INTO assegnazioni (id_operatore, id_corriere) VALUES (op, cor) RETURNING id INTO ass2;
    INSERT INTO spedizioni (cod_ordine, stato, id_operatore_ha_creato, id_cliente_mittente, id_cliente_destinatario,
                            dataora_richiesta_spedizione, id_indirizzo_ritiro, id_indirizzo_spedizione,
                            id_assegnazione_corrente)
        VALUES ('ORD-2', 'ritirata', op, mit, des, now(), ind1, ind2, ass2) RETURNING id INTO sp2;

    -- ORD-3: in consegna (altra assegnazione, perché è UNIQUE)
    INSERT INTO assegnazioni (id_operatore, id_corriere) VALUES (op, cor) RETURNING id INTO ass3;
    INSERT INTO spedizioni (cod_ordine, stato, id_operatore_ha_creato, id_cliente_mittente, id_cliente_destinatario,
                            dataora_richiesta_spedizione, id_indirizzo_ritiro, id_indirizzo_spedizione,
                            id_assegnazione_corrente)
        VALUES ('ORD-3', 'in consegna', op, mit, des, now(), ind1, ind2, ass3);

    -- ORD-4: consegnata (il corriere NON deve vederla)
    INSERT INTO assegnazioni (id_operatore, id_corriere) VALUES (op, cor) RETURNING id INTO ass3;
    INSERT INTO spedizioni (cod_ordine, stato, id_operatore_ha_creato, id_cliente_mittente, id_cliente_destinatario,
                            dataora_richiesta_spedizione, id_indirizzo_ritiro, id_indirizzo_spedizione,
                            id_assegnazione_corrente)
        VALUES ('ORD-4', 'consegnata', op, mit, des, now(), ind1, ind2, ass3);

    -- Due note su ORD-2: una interna, una pubblica
    INSERT INTO note (nota, is_interna, id_operatore_logistico_creata, id_spedizione)
        VALUES ('nota INTERNA', TRUE,  op, sp2),
               ('nota pubblica', FALSE, op, sp2);
END $$;