# Modello dati — Gestione spedizioni

|  |  |
| :---- | :---- |
| **Team** | GLS |
| **Cliente** | Veloce24 |
| **Data** | 19/10/2026 |
| **Versione** | v2 — lezione 3 (BOZZA) |

*Questo documento resta agnostico rispetto al database che usate: descrive entità, attributi, relazioni e vincoli, non un dialetto SQL o un ORM specifico. Dove serve un esempio concreto, è segnalato come "esempio" ed è solo un modo possibile di realizzare la stessa cosa.*

## 1\. Entità

**Le nostre entità:**

| Entità | Descrizione | Attributi essenziali (tipo logico) |
| :---- | :---- | :---- |
| Utente | chiunque acceda al sistema; i dati anagrafici e di accesso stanno qui una volta sola, i ruoli sono entità separate | id (identificativo), nome (testo), cognome (testo), prefisso (testo, es. "+39"), nr\_telefono (testo), email (testo), pwd\_hash (testo) |
| Cliente | ruolo di un Utente: può essere mittente o destinatario di una spedizione | id, utente (riferimento a Utente) |
| Corriere | ruolo di un Utente: ritira e consegna fisicamente il pacco | id, utente (riferimento a Utente) |
| Operatore logistico | ruolo di un Utente: crea, assegna e annulla le spedizioni, gestisce disdette, riprogrammazioni e resi | id, utente (riferimento a Utente) |
| Amministratore | ruolo di un Utente: gestisce il sistema | id, utente (riferimento a Utente) |
| Spedizione | un pacco da ritirare dal mittente e consegnare al destinatario | id, cod\_ordine (testo), stato (enumerazione: creata / assegnata / ritirata / in consegna / consegnata / non consegnata / persa / annullata), is\_reso (booleano), creata\_il (data/ora), dataora\_richiesta\_spedizione (data/ora), annullata\_ il (data/ora, opzionale) |
| Assegnazione | l'affidamento di una spedizione a un corriere, deciso da un operatore | id, creata\_il (data/ora) |
| Tentativo | un tentativo di ritiro o di consegna | id, tipo (enumerazione: ritiro / consegna), effettuato\_il (data/ora) |
| Nota | un commento su una spedizione, scritto da un operatore o da un corriere | id, nota (testo), is\_interna (booleano), motivo (testo, opzionale), creata\_il (data/ora) |
| CambiamentoSpedizione | una riga di storico: ogni operazione fatta su una spedizione, con chi e quando | id, operazione (enumerazione: creazione / modifica / annullamento), valore\_vecchio (oggetto chiave → valore, opzionale), valore\_nuovo (oggetto chiave → valore), modificato\_il (data/ora) |
| Indirizzo | via e civico, entità a parte per evitare ridondanza | id, via (testo), civico (testo) |
| Città |  | id, nome (testo), cod\_postale (testo) |
| Regione |  | id, nome (testo) |
| Nazione |  | id, nome (testo) |

*Nota: lo storico delle spedizioni è persistente (slide 12), per questo è un'entità e non solo un attributo. Rispetto alla v1, "CambioStato" è diventato "CambiamentoSpedizione": non registra solo i cambi di stato, ma la creazione, ogni modifica di qualsiasi campo e gli annullamenti di spedizione. Nelle modifiche, valore\_vecchio e valore\_nuovo contengono solo i campi cambiati, ad esempio* `{"stato":"assegnata"}` \-\> `{"stato":"in consegna"}`.

*Nota: Cliente mittente e Cliente destinatario della v1 sono diventati un'unica entità Cliente. Uno stesso cliente può essere mittente di una spedizione e destinatario di un'altra; il ruolo lo dà la relazione con la Spedizione.*

## 2\. Relazioni

**Le nostre relazioni:**

- Cliente / Corriere / Operatore logistico / Amministratore → Utente (1 a 1: ogni Cliente, Corriere, ecc è al massimo un solo Utente e un Utente può essere uno tra i Cliente Corriere ecc)  
- Spedizione → Cliente, come **mittente** (molte spedizioni per un cliente)  
- Spedizione → Cliente, come **destinatario** (molte spedizioni per un cliente; il destinatario può monitorarle)  
- Spedizione → Operatore logistico, **ha creato** (molte spedizioni create da un operatore)  
- Spedizione → Operatore logistico, **ha annullato** (opzionale: solo per le spedizioni annullate)  
- Spedizione → Indirizzo, di **ritiro** (esattamente 1 per spedizione)  
- Spedizione → Indirizzo, di **consegna** (esattamente 1 per spedizione)  
- Spedizione → Assegnazione **corrente** (opzionale: nessuna finché la spedizione è non assegnata ; un'assegnazione è corrente per al massimo una spedizione)  
- Assegnazione → Operatore logistico (chi ha assegnato) e → Corriere (a chi è assegnata)  
- Nota → Spedizione (molte note per una spedizione)  
- Nota → Operatore logistico **oppure** Corriere (l'autore: esattamente uno dei due)  
- Tentativo → Spedizione (molti tentativi per una spedizione di ritiro o di consegna)  
- Tentativo → Nota (ogni tentativo ha una nota, es. il motivo di un fallimento e ogni nota in tentativo deve riguardare la stessa spedizione del tentativo)  
- CambiamentoSpedizione → Spedizione (molte righe di storico per una spedizione)  
- CambiamentoSpedizione → Operatore logistico **oppure** Corriere (l'autore: esattamente uno dei due)  
- Indirizzo ↔ Città (associazione molti a molti, tramite l'entità IndirizzoInCittà)  
- Città → Regione → Nazione (molte città per regione, molte regioni per nazione)

## 3\. Vincoli

**I nostri vincoli:**

*Garantiti dal database:*

- Ogni entità ha un id univoco.  
- Ogni Spedizione deve riferire un Cliente mittente, un Cliente destinatario, l'Operatore che l'ha creata, un indirizzo di ritiro e un indirizzo di consegna esistenti.  
- cod\_ordine di Spedizione è univoco.  
- stato può avere solo i valori dell'elenco definito sopra; il valore di default alla creazione è "creata".  
- is\_reso è falso di default.  
- annullata\_il e operatore che ha annullato sono presenti entrambi o assenti entrambi; annullata\_il non può essere precedente a creata\_il.  
- email di Utente è univoca e ha un formato valido; la coppia (prefisso, nr\_telefono) è univoca.  
- prefisso ha il formato "+" seguito da 1–4 cifre; nr\_telefono ha da 4 a 15 cifre.  
- Nota e CambiamentoSpedizione hanno esattamente un autore: un operatore oppure un corriere, mai tutti e due, mai nessuno.  
- Creazione e annullamento di una spedizione le può fare solo un Operatore logistico; le modifiche possono farle sia operatori che corrieri.  
- Ogni spedizione all’inizio ha una riga di storico di tipo "creazione".  
- Nello storico, valore\_vecchio è assente per la creazione e presente (e diverso da valore\_nuovo) per modifica e annullamento; entrambi sono oggetti chiave → valore.  
- Lo storico è in sola scrittura: le righe non si possono modificare né cancellare.  
- Il nome di una Regione è univoco all'interno della sua Nazione; la terna (nome, cod\_postale, regione) di una Città è univoca.  
- Da “assegnata” in poi (assegnata, ritirata, in consegna, consegnata, non consegnata, persa) la spedizione deve avere un’assegnazione corrente, e quindi un corriere.   
- La nota collegata a un tentativo deve riferire la stessa spedizione del tentativo.   
- Una spedizione può essere annullata solo se è in stato “creata” o “assegnata”   
  


  

## 4\. Rappresentazione dello schema

BEGIN;

DROP TABLE IF EXISTS cambiamenti\_spedizioni CASCADE;  
DROP TABLE IF EXISTS tentativi              CASCADE;  
DROP TABLE IF EXISTS note                   CASCADE;  
DROP TABLE IF EXISTS spedizioni             CASCADE;  
DROP TABLE IF EXISTS assegnazioni           CASCADE;  
DROP TABLE IF EXISTS indirizzi\_in\_citta     CASCADE;  
DROP TABLE IF EXISTS indirizzi              CASCADE;  
DROP TABLE IF EXISTS citta                  CASCADE;  
DROP TABLE IF EXISTS regioni                CASCADE;  
DROP TABLE IF EXISTS nazioni                CASCADE;  
DROP TABLE IF EXISTS amministratori         CASCADE;  
DROP TABLE IF EXISTS operatori\_logistici    CASCADE;  
DROP TABLE IF EXISTS corrieri               CASCADE;  
DROP TABLE IF EXISTS clienti                CASCADE;  
DROP TABLE IF EXISTS utenti                 CASCADE;

DROP FUNCTION IF EXISTS fn\_traccia\_cambiamenti\_spedizioni() CASCADE;  
DROP FUNCTION IF EXISTS fn\_blocca\_modifica\_storico()        CASCADE;  
DROP FUNCTION IF EXISTS fn\_controlla\_annullamento()         CASCADE;

DROP TYPE IF EXISTS tipo\_operazione\_spedizione CASCADE;  
DROP TYPE IF EXISTS stato\_spedizione           CASCADE;  
DROP TYPE IF EXISTS tipo\_tentativo             CASCADE;

CREATE TYPE tipo\_tentativo AS ENUM ('consegna', 'ritiro');

CREATE TYPE stato\_spedizione AS ENUM (  
    'creata',  
    'assegnata',  
    'ritirata',  
    'persa',  
    'in consegna',  
    'consegnata',  
    'non consegnata',  
    'annullata'  
);

CREATE TYPE tipo\_operazione\_spedizione AS ENUM (  
    'creazione',  
    'modifica',  
    'annullamento'  
);

CREATE TABLE utenti (  
    id           INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,  
    nome         VARCHAR(100) NOT NULL,  
    cognome      VARCHAR(100) NOT NULL,  
    prefisso     VARCHAR(5)   NOT NULL,          \-- es. '+39'  
    nr\_telefono  VARCHAR(20)  NOT NULL,  
    pwd\_hash     VARCHAR(255) NOT NULL,  
    email        VARCHAR(254) NOT NULL,  
    CONSTRAINT uq\_utenti\_email    UNIQUE (email),  
    CONSTRAINT uq\_utenti\_telefono UNIQUE (prefisso, nr\_telefono),  
    CONSTRAINT ck\_utenti\_prefisso CHECK (prefisso \~ '^\\+\[0-9\]{1,4}\$'),  
    CONSTRAINT ck\_utenti\_telefono CHECK (nr\_telefono \~ '^\[0-9\]{4,15}\$'),  
    CONSTRAINT ck\_utenti\_email    CHECK (email \~\* '^\[^@\\s\]+@\[^@\\s\]+\\.\[^@\\s\]+\$')  
);

CREATE TABLE clienti (  
    id         INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,  
    id\_utente  INTEGER NOT NULL UNIQUE  
               REFERENCES utenti (id) ON UPDATE CASCADE ON DELETE RESTRICT  
);

CREATE TABLE corrieri (  
    id         INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,  
    id\_utente  INTEGER NOT NULL UNIQUE  
               REFERENCES utenti (id) ON UPDATE CASCADE ON DELETE RESTRICT  
);

CREATE TABLE operatori\_logistici (  
    id         INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,  
    id\_utente  INTEGER NOT NULL UNIQUE  
               REFERENCES utenti (id) ON UPDATE CASCADE ON DELETE RESTRICT  
);

CREATE TABLE amministratori (  
    id         INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,  
    id\_utente  INTEGER NOT NULL UNIQUE  
               REFERENCES utenti (id) ON UPDATE CASCADE ON DELETE RESTRICT  
);

CREATE TABLE nazioni (  
    id    INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,  
    nome  VARCHAR(100) NOT NULL UNIQUE  
);

CREATE TABLE regioni (  
    id          INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,  
    nome        VARCHAR(100) NOT NULL,  
    id\_nazione  INTEGER NOT NULL  
                REFERENCES nazioni (id) ON UPDATE CASCADE ON DELETE RESTRICT,  
    CONSTRAINT uq\_regioni\_nome\_nazione UNIQUE (nome, id\_nazione)  
);

CREATE TABLE citta (  
    id           INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,  
    nome         VARCHAR(100) NOT NULL,  
    cod\_postale  VARCHAR(10)  NOT NULL,  
    id\_regione   INTEGER NOT NULL  
                 REFERENCES regioni (id) ON UPDATE CASCADE ON DELETE RESTRICT,  
    CONSTRAINT uq\_citta UNIQUE (nome, cod\_postale, id\_regione)  
);

CREATE TABLE indirizzi (  
    id      INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,  
    via     VARCHAR(150) NOT NULL,  
    civico  VARCHAR(10)  NOT NULL  
);

CREATE TABLE indirizzi\_in\_citta (  
    id            INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,  
    id\_citta      INTEGER NOT NULL  
                  REFERENCES citta (id)     ON UPDATE CASCADE ON DELETE RESTRICT,  
    id\_indirizzo  INTEGER NOT NULL  
                  REFERENCES indirizzi (id) ON UPDATE CASCADE ON DELETE RESTRICT,  
    CONSTRAINT uq\_indirizzi\_in\_citta UNIQUE (id\_citta, id\_indirizzo)  
);

CREATE TABLE assegnazioni (  
    id           INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,  
    id\_operatore INTEGER NOT NULL  
                 REFERENCES operatori\_logistici (id) ON UPDATE CASCADE ON DELETE RESTRICT,  
    id\_corriere  INTEGER NOT NULL  
                 REFERENCES corrieri (id)            ON UPDATE CASCADE ON DELETE RESTRICT,  
    creata\_il    TIMESTAMPTZ NOT NULL DEFAULT now()  
);

CREATE TABLE spedizioni (  
    id                            INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,  
    stato                         stato\_spedizione NOT NULL DEFAULT 'creata',  
    cod\_ordine                    VARCHAR(50) NOT NULL,  
    is\_reso                       BOOLEAN     NOT NULL DEFAULT FALSE,  
    id\_operatore\_ha\_annullato     INTEGER NULL  
                                  REFERENCES operatori\_logistici (id) ON UPDATE CASCADE ON DELETE RESTRICT,  
    annullata\_il                  TIMESTAMPTZ NULL,  
    id\_operatore\_ha\_creato        INTEGER NOT NULL  
                                  REFERENCES operatori\_logistici (id) ON UPDATE CASCADE ON DELETE RESTRICT,  
    creata\_il                     TIMESTAMPTZ NOT NULL DEFAULT now(),  
    id\_cliente\_mittente           INTEGER NOT NULL  
                                  REFERENCES clienti (id)   ON UPDATE CASCADE ON DELETE RESTRICT,  
    id\_cliente\_destinatario       INTEGER NOT NULL  
                                  REFERENCES clienti (id)   ON UPDATE CASCADE ON DELETE RESTRICT,  
    dataora\_richiesta\_spedizione  TIMESTAMPTZ NOT NULL,  
    id\_indirizzo\_ritiro           INTEGER NOT NULL  
                                  REFERENCES indirizzi (id) ON UPDATE CASCADE ON DELETE RESTRICT,  
    id\_indirizzo\_spedizione       INTEGER NOT NULL  
                                  REFERENCES indirizzi (id) ON UPDATE CASCADE ON DELETE RESTRICT,  
    id\_assegnazione\_corrente      INTEGER NULL  
                                  REFERENCES assegnazioni (id) ON UPDATE CASCADE ON DELETE SET NULL,

    CONSTRAINT uq\_spedizioni\_cod\_ordine          UNIQUE (cod\_ordine),  
    CONSTRAINT uq\_spedizioni\_assegnazione\_corr   UNIQUE (id\_assegnazione\_corrente),

    \-- annullamento: operatore e data presenti entrambi o assenti entrambi  
    CONSTRAINT ck\_spedizioni\_annullamento        CHECK (  
        (id\_operatore\_ha\_annullato IS NULL) \= (annullata\_il IS NULL)  
    ),  
    \-- la data di annullamento c'è se e solo se lo stato è 'annullata'  
    CONSTRAINT ck\_spedizioni\_annullamento\_stato  CHECK (  
        (stato \= 'annullata') \= (annullata\_il IS NOT NULL)  
    ),  
    CONSTRAINT ck\_spedizioni\_date\_annullamento   CHECK (annullata\_il IS NULL OR annullata\_il \>= creata\_il),  
    \-- da 'assegnata' in poi serve l'assegnazione (e quindi il corriere);  
    \-- possono non averla solo le spedizioni 'creata' e 'annullata'  
    CONSTRAINT ck\_spedizioni\_assegnazione\_stato  CHECK (  
        stato IN ('creata', 'annullata') OR id\_assegnazione\_corrente IS NOT NULL  
    )  
);

CREATE TABLE note (  
    id                             INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,  
    nota                           TEXT        NOT NULL,  
    is\_interna                     BOOLEAN     NOT NULL DEFAULT FALSE,  
    motivo                         VARCHAR(100) NULL,  
    creata\_il                      TIMESTAMPTZ NOT NULL DEFAULT now(),  
    id\_operatore\_logistico\_creata  INTEGER NULL  
                                   REFERENCES operatori\_logistici (id) ON UPDATE CASCADE ON DELETE RESTRICT,  
    id\_corriere\_creata             INTEGER NULL  
                                   REFERENCES corrieri (id)            ON UPDATE CASCADE ON DELETE RESTRICT,  
    id\_spedizione  INTEGER NOT NULL  
                                   REFERENCES spedizioni (id) ON UPDATE CASCADE ON DELETE CASCADE,  
    \-- esattamente uno tra operatore e corriere  
    CONSTRAINT ck\_note\_autore CHECK (  
        num\_nonnulls(id\_operatore\_logistico\_creata, id\_corriere\_creata) \= 1  
    ),  
    \-- serve alla chiave esterna composta di tentativi (nota \+ spedizione)  
    CONSTRAINT uq\_note\_id\_spedizione UNIQUE (id, id\_spedizione)  
);

CREATE TABLE tentativi (  
    id             INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,  
    tipo           tipo\_tentativo NOT NULL,       \-- era "isConsegna"  
    effettuato\_il  TIMESTAMPTZ    NOT NULL DEFAULT now(),  
    id\_spedizione  INTEGER NOT NULL  
                   REFERENCES spedizioni (id) ON UPDATE CASCADE ON DELETE RESTRICT,  
    id\_nota        INTEGER NOT NULL,  
    \-- la nota del tentativo deve essere della stessa spedizione del tentativo  
    CONSTRAINT fk\_tentativi\_nota\_stessa\_spedizione  
        FOREIGN KEY (id\_nota, id\_spedizione)  
        REFERENCES note (id, id\_spedizione) ON UPDATE CASCADE ON DELETE RESTRICT  
);

CREATE TABLE cambiamenti\_spedizioni (  
    id                       INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,  
    operazione               tipo\_operazione\_spedizione NOT NULL,  
    valore\_vecchio           JSONB NULL,  
    valore\_nuovo             JSONB NOT NULL,  
    modificato\_il            TIMESTAMPTZ NOT NULL DEFAULT now(),  
    id\_corriere              INTEGER NULL  
                             REFERENCES corrieri (id)            ON UPDATE CASCADE ON DELETE RESTRICT,  
    id\_operatore             INTEGER NULL  
                             REFERENCES operatori\_logistici (id) ON UPDATE CASCADE ON DELETE RESTRICT,  
    id\_spedizione            INTEGER NOT NULL  
                             REFERENCES spedizioni (id)          ON UPDATE CASCADE ON DELETE RESTRICT,

    CONSTRAINT ck\_cambiamenti\_autore CHECK (  
        num\_nonnulls(id\_corriere, id\_operatore) \= 1  
    ),  
    \--check su annullamento e creazione che non le può fare un corriere  
    CONSTRAINT ck\_cambiamenti\_autore\_operazione CHECK (  
        operazione \= 'modifica' OR id\_operatore IS NOT NULL  
    ),  
    \-- mi assicuro che ci siano json chiave \-\> valore ad es. stato-\> consegnata  
    CONSTRAINT ck\_cambiamenti\_json\_oggetti CHECK (  
        jsonb\_typeof(valore\_nuovo) \= 'object'  
        AND (valore\_vecchio IS NULL OR jsonb\_typeof(valore\_vecchio) \= 'object')  
    ),  
    CONSTRAINT ck\_cambiamenti\_valori CHECK (  
        CASE operazione  
            WHEN 'creazione' THEN valore\_vecchio IS NULL  
            ELSE valore\_vecchio IS NOT NULL AND valore\_vecchio \<\> valore\_nuovo  
        END  
    )  
);

\-- controllo se non ci siano piu stesse spedizioni con stato creazione  
CREATE UNIQUE INDEX uq\_cambiamenti\_creazione  
    ON cambiamenti\_spedizioni (id\_spedizione)  
    WHERE operazione \= 'creazione';

CREATE FUNCTION fn\_traccia\_cambiamenti\_spedizioni()  
RETURNS trigger \--funzione che si può collegare solo ad un trigger  
LANGUAGE plpgsql \--definiscon il linguaggio che voglio usare per la funzione questo è il linguaggio procedurale di postgres  
AS \$\$ \--i dollari delimitano il corpo della funzione  
\--dichiaro delle variabili  
DECLARE  
    v\_operazione   tipo\_operazione\_spedizione;  
    v\_id\_corriere  INTEGER;  
    v\_id\_operatore INTEGER;  
    v\_vecchio      JSONB;  
    v\_nuovo        JSONB;  
    \--new e old contengono i valori vecchi e nuovi dell operazione  
BEGIN  
    IF TG\_OP \= 'INSERT' THEN  
        v\_operazione   :\= 'creazione';  
        v\_id\_operatore :\= NEW.id\_operatore\_ha\_creato;  
        v\_nuovo        :\= to\_jsonb(NEW);  
    ELSE  
        \--jsonb\_each(to\_jsonb(OLD)) trasforma la riga vecchia in una tabella con una riga per campo  
        \--creo una tabella con solo le modifiche che ha subito la spezione attributo valore nuovo  
        \-- e attributo valore vecchio visualizzati come due oggetti json e poi salvo i valori nuovo e vecchio  
        \-- in due variabili v\_vecchio e v\_nuovo  
        SELECT jsonb\_object\_agg(o.key, o.value),  
               jsonb\_object\_agg(o.key, n.value)  
          INTO v\_vecchio, v\_nuovo  
          FROM jsonb\_each(to\_jsonb(OLD)) AS o  
          JOIN jsonb\_each(to\_jsonb(NEW)) AS n ON n.key \= o.key  
         WHERE o.value IS DISTINCT FROM n.value;

        \--se non ci sono nuove modifiche ma rimodifico con gli stessi valori gli attributi esco  
        IF v\_nuovo IS NULL THEN  
            RETURN NEW;  
        END IF;

        \--salvo nello storico  
        IF OLD.annullata\_il IS NULL AND NEW.annullata\_il IS NOT NULL THEN  
            v\_operazione   :\= 'annullamento';  
            v\_id\_operatore :\= NEW.id\_operatore\_ha\_annullato;  
        ELSE  
            \--setto le variabili per contenere il corriere o l'operatore corretto  
            v\_operazione   :\= 'modifica';  
            v\_id\_corriere  :\= NULLIF(current\_setting('app.id\_corriere',  true), '')::INTEGER;  
            v\_id\_operatore :\= NULLIF(current\_setting('app.id\_operatore', true), '')::INTEGER;  
            \--num\_nonnulls(…) conta quanti argomenti non sono NULL  
            IF num\_nonnulls(v\_id\_corriere, v\_id\_operatore) \<\> 1 THEN  
                RAISE EXCEPTION 'Autore della modifica non valido per la spedizione %', NEW.id  
                    USING HINT \= 'Impostare uno solo tra app.id\_operatore e app.id\_corriere '  
                                 'con SET LOCAL prima dell''UPDATE.';  
            END IF;  
        END IF;  
    END IF;

    INSERT INTO cambiamenti\_spedizioni  
        (operazione, valore\_vecchio, valore\_nuovo,  
         id\_corriere, id\_operatore, id\_spedizione)  
    VALUES  
        (v\_operazione, v\_vecchio, v\_nuovo,  
         v\_id\_corriere, v\_id\_operatore, NEW.id);

    RETURN NEW;  
END;  
\$\$;

CREATE TRIGGER trg\_spedizioni\_storico\_insert  
AFTER INSERT ON spedizioni  
FOR EACH ROW  
EXECUTE FUNCTION fn\_traccia\_cambiamenti\_spedizioni();

CREATE TRIGGER trg\_spedizioni\_storico\_update  
AFTER UPDATE ON spedizioni  
FOR EACH ROW  
WHEN (OLD.\* IS DISTINCT FROM NEW.\*)  
EXECUTE FUNCTION fn\_traccia\_cambiamenti\_spedizioni();

\-- Una spedizione si può annullare solo se è ancora 'creata' o 'assegnata'.  
\-- Serve un trigger perché un CHECK vede solo la riga nuova, non lo stato precedente.  
CREATE FUNCTION fn\_controlla\_annullamento()  
RETURNS trigger  
LANGUAGE plpgsql  
AS \$\$  
BEGIN  
    IF OLD.stato NOT IN ('creata', 'assegnata') THEN  
        RAISE EXCEPTION 'La spedizione % non può essere annullata: è nello stato "%"', NEW.id, OLD.stato  
            USING HINT \= 'Si possono annullare solo spedizioni in stato "creata" o "assegnata".';  
    END IF;  
    RETURN NEW;  
END;  
\$\$;

CREATE TRIGGER trg\_spedizioni\_controlla\_annullamento  
BEFORE UPDATE OF stato ON spedizioni  
FOR EACH ROW  
WHEN (NEW.stato \= 'annullata' AND OLD.stato \<\> 'annullata')  
EXECUTE FUNCTION fn\_controlla\_annullamento();

\-- Lo storico è in sola scrittura: le righe non si possono modificare né cancellare.  
CREATE FUNCTION fn\_blocca\_modifica\_storico()  
RETURNS trigger  
LANGUAGE plpgsql  
AS \$\$  
BEGIN  
    RAISE EXCEPTION 'Lo storico delle spedizioni non può essere modificato o cancellato';  
END;  
\$\$;

CREATE TRIGGER trg\_cambiamenti\_spedizioni\_immutabile  
BEFORE UPDATE OR DELETE ON cambiamenti\_spedizioni  
FOR EACH ROW  
EXECUTE FUNCTION fn\_blocca\_modifica\_storico();

COMMIT;

## 5\. Evoluzioni

| Data | Cosa è cambiato | Perché |
| :---- | :---- | :---- |
| \[14/10/2026\] | v1: prima bozza dal diagramma di flusso | punto di partenza |
| \[19/10/2026\] | v2: introdotto Utente con i ruoli Cliente / Corriere / Operatore logistico / Amministratore | anagrafica e credenziali in un posto solo, senza ripeterle per ogni ruolo |
| \[19/10/2026\] | v2: Cliente mittente e Cliente destinatario unificati in Cliente | uno stesso cliente può avere entrambi i ruoli in spedizioni diverse |
| \[19/10/2026\] | v2: Indirizzo scomposto in Indirizzo / Città / Regione / Nazione; due indirizzi per spedizione (ritiro e consegna) | evitare ridondanza; il pacco va ritirato in un posto e consegnato in un altro |
| \[19/10/2026\] | v2: aggiunte Assegnazione e Nota | tracciare chi assegna a quale corriere; commenti e motivi dei tentativi falliti |
| \[19/10/2026\] | v2: CambioStato sostituito da CambiamentoSpedizione | lo storico deve registrare ogni operazione (creazione, modifica di qualsiasi campo, archiviazione) con chi e quando, non solo i cambi di stato |
| \[19/10/2026\] | v2: stati "non assegnata" → "creata" e "disdetta" → "annullata"; aggiunta l'archiviazione | allineamento con lo schema fisico |

## Punti da confermare nel team

