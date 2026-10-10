# Backlog iniziale — [Last Mile Delivery] / [Veloce24]

| | |
|---|---|
| **Team** | *(GLS, Francesco Platenotti, Dennis Pasquali, Gianluca Vivaldi, Micheal Nandelli, Mohammad Amine Reddad, Pellegrini Christian)* |
| **Cliente** | *Veloce24* |
| **Data** | 23/09/2026 |

*8-10 voci, ordinate per valore e rischio: prima le cose che fanno paura,
perché sono quelle che possono far cambiare l'architettura. Ogni voce deve
avere un "Fatto quando" verificabile — se non sapete come si verifica, la
voce non è pronta.*

## Formato

```
Come <attore> voglio <azione> per <beneficio>

Fatto quando:
- ...
- ...
```

> **Esempio**
> Come tecnico voglio vedere solo gli interventi assegnati a me, per non
> perdere tempo tra le segnalazioni altrui.
> **Fatto quando:** la lista mostra solo i miei interventi · un tecnico non
> vede quelli di un collega nemmeno chiamando l'API a mano · la lista è
> ordinata per priorità.

## Voci

### 1.Transizione controllata degli stati del workflow

Come operatore logistico o corriere voglio che il sistema imponga il rispetto sequenziale delle fasi del ciclo di spedizione (Creata -> Assegnata -> Ritirata -> In consegna -> Consegnata / Consegna non riuscita) per evitare modifiche di stato incoerenti o non valide nel database.   

**Fatto quando:**

-Un tentativo di passaggio di stato non valido (es. da Creata direttamente a Consegnata) viene rifiutato dal backend con un errore esplicito.   

-Viene mantenuto e registrato l'intero storico con i timestamp di ciascuna transizione di stato avvenuta con successo.

-L'esito e le transizioni ammesse sono verificati tramite test automatizzati sulle API.

### 2.Segregazione accessi e riservatezza del cliente destinatario

Come cliente destinatario voglio consultare lo stato e lo storico unicamente della mia spedizione per non visualizzare per errore note operative riservate o dati riguardanti altri clienti.   

**Fatto quando:**

-La vista del destinatario mostra esclusivamente le informazioni pubbliche della spedizione collegata al proprio codice/link univoco.

-Tenta di accedere o modificare altre spedizioni tramite chiamate dirette alle API e riceve un codice di errore HTTP 403/404.

-Le note interne e i dettagli operativi inseriti dai corrieri non sono inclusi nella risposta JSON/HTML per il cliente finale.

### 3.Autenticazione e autorizzazione basata sui ruoli (RBAC)

Come amministratore del sistema voglio che le chiamate API richiedano autenticazione e siano autorizzate in base al ruolo dell'utente per garantire che ciascun attore operi esclusivamente sulle risorse di sua competenza.   

**Fatto quando:**

-Un corriere authenticated può consultare e aggiornare la gestione dello stato solo per le spedizioni a lui assegnate.  

-Un operatore logistico può accedere alle funzionalità di creazione, assegnazione e ricerca su tutte le spedizioni.  

-Le chiamate alle API senza token valido o con ruolo insufficiente restituiscono rispettivamente errore HTTP 401 Unauthorized o 403 Forbidden.

### 4.Creazione e assegnazione spedizione

Come operatore logistico voglio creare una nuova spedizione con i dati essenziali di consegna e assegnarla a un corriere della flotta per avviare tempestivamente il processo operativo di recapito.

**Fatto quando:**

-Compilando il modulo con i dati essenziali, la spedizione viene creata con lo stato Creata.

-Selezionando un corriere disponibile, lo stato della spedizione cambia in Assegnata ed essa compare immediatamente nella lista delle consegne di quel corriere.

-Non è possibile assegnare la spedizione a un utente che non possiede il ruolo di corriere. 

### 5.Registrazione dell'esito di consegna e note sui problemi

Come corriere voglio registrare l'esito finale della consegna e inserire note dettagliate in caso di mancato recapito per informare la centrale operativa sulla motivazione dell'eventuale problema.

**Fatto quando:**

-Il corriere può impostare lo stato a Consegnata oppure a Consegna non riuscita.

-In caso di Consegna non riuscita, il campo "note operative" è obbligatorio per completare il salvataggio.

-L'operatore logistico visualizza istantaneamente la nota inserita nella dashboard della centrale operativa. 

### 6.Aggiornamento periodico della posizione (simulata)

Come corriere voglio inviare periodicamente la posizione aggiornata del mio mezzo per consentire alla centrale operativa di tracciare l'avanzamento sulla mappa/dashboard.

**Fatto quando:**

-L'applicazione/API riceve le coordinate geografiche simulate tramite chiamate periodiche.

-L'ultima posizione nota viene salvata nel sistema e associata al corriere e alla spedizione in corso.

-In assenza di coordinate reali, il sistema valida ed elabora correttamente le coordinate simulate senza generare errori.

### 7.Ricerca avanzata e filtraggio spedizioni

Come operatore logistico voglio filtrare le spedizioni per stato, corriere assegnato, data o codice di riferimento per individuare velocemente le consegne in ritardo o le criticità operative.

**Fatto quando:**

-La ricerca restituisce esclusivamente i risultati coerenti con i filtri applicati (es. filtri combinati per stato In consegna e per specifico corriere).

-La risposta della ricerca avviene in tempi idonei ed è navigabile/impaginata anche in presenza di un volume elevato di spedizioni di prova.

### 8.Persistenza dei dati e stabilità al riavvio

Come operatore logistico o amministratore voglio che lo stato delle spedizioni, lo storico e gli utenti rimangano salvati in modo permanente per non perdere le informazioni operative in seguito a un riavvio dei servizi backend.   

**Fatto quando:**

-In seguito all'arresto e al successivo riavvio dell'applicazione o del server, l'intero storico delle spedizioni e degli utenti risulta inalterato.

-Viene eseguito un test di riavvio controllato confermando l'integrità dei dati salvati prima del riavvio.

*Aggiungete la 9 e la 10 se il tempo lo permette. Meglio 8 voci solide che 10
generiche.*
