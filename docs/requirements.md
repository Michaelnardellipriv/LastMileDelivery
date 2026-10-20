# Requisiti — Last mile delivery / Veloce 24 S.r.l.

|  |  |
| :---- | :---- |
| **Team** | *(GLS, componenti)* |
| **Cliente** | *Last mile delivery S.r.l.* |
| **Data** | 23/09/2026 |
| **Versione** | v2 |

## Problema

*In tre righe: cosa non funziona oggi per il cliente, per chi, con quale conseguenza concreta. Non descrivete ancora una soluzione.*

> Per l’azienda Veloce24 il problema riguarda il fatto che le spedizioni e il loro ciclo di vita viene tenuto tramite fogli condivisi e telefonate rendendo molto difficile all’azienda avere una visione d’insieme su uno o più spedizioni che sia univoca per tutti gli operatori, inoltre si ha un problema anche per quanto riguarda possibile perdita di dati se gli operatori con le telefonate non scrivono poi nei loro fogli condivisi la nuova spedizione o i dati associati a essa  
>   
>   
> Obiettivi del progetto  
         *Elenco breve, dal punto di vista del cliente: cosa vuole ottenere, non come.*

- Centralizzazione delle informazioni sulle spedizioni e del loro ciclo di vita in modo tale da avere una vista univoca sulla situazione delle spedizioni e completa  
- Fornire quindi agli operatori e clienti una vista sempre aggiornata univoca e corretta sullo stato e le informazioni delle spedizioni  
- Permettere ai dipendenti di poter cambiare lo stato ed effettuare tutte le operazioni necessarie dal portale in maniera centralizzata e autorizzata  
- Mantenere uno storico delle spedizioni interno  
- Mantenere storico delle operazioni effettuate dai dipendenti sul portale su ciascuna spedizione visibile ad amministratore e operatori  
- Rendere il sistema adatto ad essere integrato con massimo 20h di lavoro extra ad un servizio di notifica esterno e ad un qualsiasi gestionale o e-commerce  
- Garantire sicurezza in modo tale che solo gli utenti autorizzati vedano determinate informazioni e possano solo eseguire determinate operazioni  
- Consentire agli operatori di poter effettuare operazioni sul portale tramite i loro smartphones

## Requisiti funzionali

*Frasi su cui si può rispondere sì o no. Numerate, così potete richiamarle nei casi d'uso e nel backlog.*

| \# | Requisito |
| :---- | :---- |
| RF1 | l’operatore deve creare le spedizioni ricevute via mail e pagate dal cliente mettendogli come riferimento il numero d’ordine e poi può assegnarle ai corrieri o annullarle prima che siano ritirate |
| RF2 | il cliente destinatario visualizza solo le spedizioni in corso con il loro stato e le spedizioni effettuate da meno di x giorni  |
| RF3 | gli operatori devono poter modificare le spedizioni create finché non sono in ritirate  |
| RF4 | gli operatori devono poter riassegnare le spedizioni ad altri corrieri |
| RF5 | L' operatore deve poter contrassegnare la spedizione come persa e da in consegna a ritirata  |
| RF6 | i corrieri devono visualizzare sui loro smartphone le consegne che hanno da fare senza il loro ordine di consegna/ritiro |
| RF7 | I corrieri devono poter modificare la spedizione da in consegna a consegnata, non consegnata, o da ritirata a in consegna |
|  |    |
| RF9 | operatori e corrieri devono poter scrivere note operative interne |
| RF 10 | tutti gli utenti si devono poter registrare e accedere per poter effettuare delle operazioni nel portale e vedere solo i dati a loro riservati |
| RF11 | il ciclo di vita della spedizione deve essere tenuto nello storico accessibile da operatori e amministratore |
| RF12 | ogni cambiamento di stato e modifica fatta deve essere tracciata e visibile a operatori e amministratore |
| RF13 | l’amministratore deve poter vedere lo storico e le spedizioni ma non modificarle e puo gestire utenti con ruoli e permessi ma non puo eliminare utenti e lo storico e gestisce anche le configurazioni |
| RF14 | Gestione di un workflow composto da: creata, assegnata, ritirata, in consegna, consegnata, consegna non riuscita annullata e persa  |
| RF15 | Impedimento dei passaggi di stato palesemente incoerenti. |
| RF16 | Aggiornamento simulato dell'ultima posizione nota del corriere o del mezzo visibile solo agli operatori. |
| RF17 | Ricerca per stato, corriere, data e riferimento spedizione. |
| RF18 | utilizzo AI per riassumere note interne |
| RF 19 | utilizzo AI per dare una label di categoria alle problematiche interne |

## 

## Requisiti non funzionali

*Anche questi verificabili, non aggettivi. Se non riuscite a dire come lo misurereste, non è ancora un requisito.*

| \# | Requisito |
| :---- | :---- |
| RNF1 | *ogni utente deve essere autenticato e ha dei permessi e deve poter modificare e vedere solo cio che lo riguarda*  |
| RNF2 | Il sistema deve mantenere consistenti gli stati delle spedizioni |
| RNF3 | Le API devono essere progettate per consentire future integrazioni con sistemi di notifica |
| RNF4 | La soluzione deve essere scalabile e poter gestire anche spedizioni in piu citta di grandi dimensioni |
| RNF5 | Le informazioni visibili al cliente finale devono essere separate dalle note operative interne. |
| RNF6 | cliente mittente e destinatario non devono visualizzare dopo 30 giorni dalla consegna le spedizioni già effettuate |
| RNF7 | lo sviluppo del portale richiede che gli errori vengano gestiti nel modo corretto cosi come lo stato dei componenti principali per permettere un debugging efficace nel caso di problemi cosi come procedure di deploy e aggiornamento documentate |
| RFN8 | gli output dati dall AI sono solo dei suggerimenti che possono essere accettati o meno dagli operatori |

*✗ "Il sistema deve essere veloce" — ✓ "La lista degli interventi aperti compare in meno di 2 secondi con 5.000 interventi a archivio"*

## Vincoli dichiarati dal cliente

*Copiateli dalla richiesta cliente: cosa il cliente esclude o impone esplicitamente (stack libero salvo diversa indicazione, ambiente di esecuzione, dati di test, ecc.).*

- il cliente non impone uno specifico stack tecnologico  
- il cliente impone di dimostrare il fatto che il progetto funzioni con più utenti che lo utilizzano per gestire più spedizioni  
- l’integrazione con sistemi di notifica e gestionali / ecommerce non è richiesta ma il progetto deve essere predisposto alla loro integrazione  
- il pacco deve essere geolocalizzato ma le coordinate ce le possiamo anche inventare  
- Una spedizione può attraversare il workflow definito senza perdere lo storico.  
- Un corriere può cambiare stato solo alle spedizioni di propria competenza.  
- Il cliente finale vede esclusivamente le informazioni autorizzate.  
- I passaggi di stato non validi vengono respinti.  
- Il sistema resta utilizzabile dopo riavvio e conserva i dati.  
- il sistema deve garantire un certo livello di sicurezza e di autorizzazioni

## Glossario

*Solo se nella richiesta cliente ci sono termini di dominio che userete spesso e che non sono ovvi fuori da questo progetto.*

| Termine | Significato |
| :---- | :---- |
| cliente mittente | colui che crea la spedizione sul portale |
| cliente destinatario | colui che riceve la spedizione a casa |
| operatori | coloro che fanno parte della centrale operativa dell’azienda che si occupano di monitorare le spedizioni |

