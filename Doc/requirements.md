# Requisiti — \[Nome progetto\] / Last mile delivery S.r.l.

|  |  |
| :---- | :---- |
| **Team** | *(, componenti)* |
| **Cliente** | *(Last mile delivery S.r.l.)* |
| **Data** | 23/09/2026 |
| **Versione** | v1 |

## Problema

*In tre righe: cosa non funziona oggi per il cliente, per chi, con quale conseguenza concreta. Non descrivete ancora una soluzione.*

> Per l’azienda last mile delivery il problema riguarda il fatto che le spedizioni e il loro ciclo di vita viene tenuto tramite fogli condivisi e telefonate rendendo molto difficile all’azienda avere una visione d’insieme su uno o più spedizioni che sia univoca per tutti gli operatori, inoltre si ha un problema anche per quanto riguarda possibile perdita di dati se gli operatori con le telefonate non scrivono poi nei loro fogli condivisi la nuova spedizione o i dati associati a essa  
>   
>   
> Obiettivi del progetto  
         *Elenco breve, dal punto di vista del cliente: cosa vuole ottenere, non come.*

- Centralizzazione delle informazioni sulle spedizioni e del loro ciclo di vita in modo tale da avere una vista univoca sulla situazione delle spedizioni e completa  
- Fornire quindi agli operatori e clienti una vista sempre aggiornata univoca e corretta sullo stato e le informazioni delle spedizioni  
- Permettere agli utenti di poter aprire ticket di assistenza  
- Permettere ai dipendenti di poter cambiare lo stato ed effettuare tutte le operazioni necessarie dal portale in maniera centralizzata  
- Mantenere uno storico delle spedizioni interno  
- Mantenere storico delle operazioni effettuate dai dipendenti sul portale su ciascuna spedizione fino a tot giorni ?  
- Poter fare in modo che i destinatari della spedizione e chi spedisce possa costantemente tenere traccia dello stato della spedizione senza dover telefonare ogni volta  
- Rendere il sistema adatto ad essere integrato con massimo 20h di lavoro extra ad un servizio di notifica esterno e ad un qualsiasi gestionale o e-commerce  
- Garantire sicurezza in modo tale che solo gli utenti autorizzati vedano determinate informazioni e facciano determinate operazioni  
- Consentire agli operatori di poter effettuare operazioni sul portale tramite i loro smartphones

## Requisiti funzionali

*Frasi su cui si può rispondere sì o no. Numerate, così potete richiamarle nei casi d'uso e nel backlog.*

| \# | Requisito |
| :---- | :---- |
| RF1 | *il cliente mittente solo se registrato e dopo aver indicato il punto ritiro deve poter creare la spedizione sulla piattaforma e pagarla per poterla effettivamente creare*  |
| RF2 | il cliente mittente e destinatario visualizzano solo le spedizioni in corso e le spedizioni effettuate da meno di 30 giorni |
| RF3 | i corrieri devono visualizzare sui loro smartphone le consegne e i ritiri che hanno da fare con il loro ordine di consegna/ritiro |
| RF4 | il sistema o gli operatori devono assegnare ai corrieri più adatti i pacchi da consegnare e i ritiri (quindi in base al camion che utilizzano alla destinazione dei pacchi al loro peso e volumetria ) in modo tale che questi nel loro orario di lavoro riescano a consegnare più pacchi possibili? |
| RF5 | il sistema deve consentire ai magazzinieri o ai corrieri che ritirano i pacchi di stiparli nel modo più efficente possibile per risparmiare spazio e poter estrarli facilmente dal camion/furgone in base alle spedizioni che devono essere effettuate ? |
| RF6 | il sistema deve consentire (ai magazzinieri) , ai corrieri, e operatori di poter notificare ciascuno al cliente destinatario lo stato della spedizione tramite il proprio smartphone o pc sul portale scansionando il qr code del pacco o ricercando il pacco sul portale e questi devono poter aggiungere delle note operative interne? (qr code) |
| RF7 | il sistema deve generare le etichette da attaccare ai pacchi da parte dei magazzinieri o clienti mittenti ?(qr code) |
| RF8 | il sistema o gli operatori devono poter visualizzare i pacchi da assegnare e assegnarl ai corrieri? |
| RF9 | il cliente in caso di problemi deve poter aprire un ticket assistenza per 1 o più spedizioni sulla piattaforma con il quale verrà assegnato ad un operatore  |
| RF 10 | tutti gli utenti si devono poter registrare per poter effettuare delle operazioni nel portale e vedere solo i dati a loro riservati |
| RF11 | il ciclo di vita della spedizione fino alla sua fine deve essere tracciato e documentato e continuamente aggiornato e poi salvato nello storico? |
| RF12 | l’applicativo deve conservare uno storico delle spedizioni ad uso interno? |
| RF13 | l’amministratore deve poter vedere tutti i dati che vuole e fare tutte le operazioni che vuole |
| RF14 | Gestione di un workflow almeno composto da: creata, assegnata, ritirata, in consegna, consegnata, consegna non riuscita.  |
| RF15 | Impedimento dei passaggi di stato palesemente incoerenti. |
| RF16  | Inserimento di note relative a problemi o tentativi di consegna. |
| RF17  | Aggiornamento simulato dell'ultima posizione nota del corriere o del mezzo. |
| RF18 | Ricerca per stato, corriere, data e riferimento spedizione. |
| RF19 | utilizzo AI per riassumere note interne |
| RF 20 | utilizzo AI per dare una label di categoria alle problematiche interne |

## 

## Requisiti non funzionali

*Anche questi verificabili, non aggettivi. Se non riuscite a dire come lo misurereste, non è ancora un requisito.*

| \# | Requisito |
| :---- | :---- |
| RNF1 | *ogni utente deve essere autenticato e ha dei permessi e deve poter modificare e vedere solo cio che lo riguarda*  |
| RNF2 | Il sistema deve mantenere consistenti gli stati delle spedizioni |
| RNF3 | Le API devono essere progettate per consentire future integrazioni con sistemi di notifica |
| RNF4 | La soluzione deve essere scalabile e poter gestire anche spedizioni internazionali |
| RNF5 | Le informazioni visibili al cliente finale devono essere separate dalle note operative interne. |
| RNF6 | *cliente mittente e destinatario non devono visualizzare dopo 30 giorni dalla consegna le spedizioni già effettuate* |
| RNF7 | *lo sviluppo del portale richiede che gli errori vengano gestiti nel modo corretto cosi come lo stato dei componenti principali per permettere un debugging efficace nel caso di problemi cosi come procedure di deploy e aggiornamento documentate* |
| RFN8 | *gli output dati dall AI sono solo dei suggerimenti che possono essere accettati o meno dagli operatori* |

*✗ "Il sistema deve essere veloce" — ✓ "La lista degli interventi aperti compare in meno di 2 secondi con 5.000 interventi a archivio"*

## Vincoli dichiarati dal cliente

*Copiateli dalla richiesta cliente: cosa il cliente esclude o impone esplicitamente (stack libero salvo diversa indicazione, ambiente di esecuzione, dati di test, ecc.).*

- il cliente non impone uno specifico stack tecnologico  
- il cliente impone di dimostrare il fatto che il progetto funzioni con più utenti che lo utilizzano per gestire più spedizioni  
- l’integrazione con sistemi di notifica e gestionali / ecommerce non è richiesta ma il progetto deve essere predisposto alla loro integrazione  
- il pacco deve essere geolocalizzato ma le coordinate ce le possiamo anche inventare  
- Una spedizione può attraversare il workflow definito senza perdere lo storico.  
- Un corriere può modificare solo le spedizioni di propria competenza.  
- Il cliente finale vede esclusivamente le informazioni autorizzate.  
- I passaggi di stato non validi vengono respinti.  
- Il sistema resta utilizzabile dopo riavvio e conserva i dati.  
- il sistema deve essere predisposto ad essere scalabile  
- il sistema deve garantire un certo livello di sicurezza e di autorizzazioni

## Glossario

*Solo se nella richiesta cliente ci sono termini di dominio che userete spesso e che non sono ovvi fuori da questo progetto.*

| Termine | Significato |
| :---- | :---- |
| cliente mittente | colui che crea la spedizione sul portale |
| cliente destinatario | colui che riceve la spedizione a casa |
| operatori | coloro che fanno parte della centrale operativa dell’azienda che si occupano di monitorare le spedizioni |

