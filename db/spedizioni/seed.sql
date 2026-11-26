-- Dati di prova del database delle SPEDIZIONI: da eseguire su un database vuoto, subito
-- dopo spedizioni/schema.sql. Gli id di sedi, città, persone e indirizzi sono quelli
-- creati da globale/seed.sql:
--   sede 1 Trento (città 1), sede 2 Bolzano (città 2)
--   operatore 1 e corriere 1 a Trento; clienti 1 (mittente) e 2 (destinatario)
--   indirizzi 2 (Via Roma 1) e 3 (Via Verdi 10) a Trento
BEGIN;

-- impostazioni predefinite delle due sedi
INSERT INTO impostazioni (id_sede) VALUES (1), (2);

-- indirizzi di ritiro e consegna delle due sedi (stessi id di globale/seed.sql, dove 1 e 4
-- sono anche gli indirizzi delle sedi)
INSERT INTO indirizzi (id, id_sede, id_citta, via, civico, cod_postale) OVERRIDING SYSTEM VALUE VALUES
    (1, 1, 1, 'Via Brennero', '1',  '38121'),
    (2, 1, 1, 'Via Roma',     '1',  '38122'),
    (3, 1, 1, 'Via Verdi',    '10', '38122'),
    (4, 2, 2, 'Via Galvani',  '2',  '39100');
SELECT setval('indirizzi_id_seq', 4);

DO $$
DECLARE
    sede_tn CONSTANT BIGINT := 1; tn   CONSTANT BIGINT := 1;
    op      CONSTANT BIGINT := 1; cor  CONSTANT BIGINT := 1;
    mit     CONSTANT BIGINT := 1; des  CONSTANT BIGINT := 2;
    ind1    CONSTANT BIGINT := 2; ind2 CONSTANT BIGINT := 3;
    ass2 BIGINT; ass3 BIGINT;
    sp2 BIGINT;
BEGIN
    -- ORD-1: creata, non assegnata
    INSERT INTO spedizioni (id_sede, id_citta, cod_ordine, stato, id_operatore_ha_creato, id_cliente_mittente, id_cliente_destinatario,
                            dataora_richiesta_spedizione, id_indirizzo_ritiro, id_indirizzo_spedizione)
        VALUES (sede_tn, tn, 'ORD-1', 'creata', op, mit, des, now(), ind1, ind2);

    -- ORD-2: ritirata (serve un'assegnazione)
    INSERT INTO assegnazioni (id_sede, id_operatore, id_corriere) VALUES (sede_tn, op, cor) RETURNING id INTO ass2;
    INSERT INTO spedizioni (id_sede, id_citta, cod_ordine, stato, id_operatore_ha_creato, id_cliente_mittente, id_cliente_destinatario,
                            dataora_richiesta_spedizione, id_indirizzo_ritiro, id_indirizzo_spedizione,
                            id_assegnazione_corrente)
        VALUES (sede_tn, tn, 'ORD-2', 'ritirata', op, mit, des, now(), ind1, ind2, ass2) RETURNING id INTO sp2;

    -- ORD-3: in consegna (altra assegnazione, perché è UNIQUE)
    INSERT INTO assegnazioni (id_sede, id_operatore, id_corriere) VALUES (sede_tn, op, cor) RETURNING id INTO ass3;
    INSERT INTO spedizioni (id_sede, id_citta, cod_ordine, stato, id_operatore_ha_creato, id_cliente_mittente, id_cliente_destinatario,
                            dataora_richiesta_spedizione, id_indirizzo_ritiro, id_indirizzo_spedizione,
                            id_assegnazione_corrente)
        VALUES (sede_tn, tn, 'ORD-3', 'in consegna', op, mit, des, now(), ind1, ind2, ass3);

    -- ORD-4: consegnata (il corriere NON deve vederla)
    INSERT INTO assegnazioni (id_sede, id_operatore, id_corriere) VALUES (sede_tn, op, cor) RETURNING id INTO ass3;
    INSERT INTO spedizioni (id_sede, id_citta, cod_ordine, stato, id_operatore_ha_creato, id_cliente_mittente, id_cliente_destinatario,
                            dataora_richiesta_spedizione, id_indirizzo_ritiro, id_indirizzo_spedizione,
                            id_assegnazione_corrente)
        VALUES (sede_tn, tn, 'ORD-4', 'consegnata', op, mit, des, now(), ind1, ind2, ass3);

    -- Due note su ORD-2: una interna, una pubblica
    INSERT INTO note (id_sede, nota, is_interna, id_operatore_logistico_creata, id_spedizione)
        VALUES (sede_tn, 'nota INTERNA', TRUE,  op, sp2),
               (sede_tn, 'nota pubblica', FALSE, op, sp2);
END $$;

COMMIT;
