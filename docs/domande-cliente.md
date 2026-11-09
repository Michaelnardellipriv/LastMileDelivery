# Domande al cliente — Lastmile Delivery / Veloce24 s.r.l.

|  |  |
| :---- | :---- |
| **Team** | *(GLS, Dennis Pasquali,Micheal Nardelli,Gianluca, Vivaldi,Mohammed Amine Reddad,Francesco Plateroti,Cristian Pellegrini)* |
| **Cliente** | *Veloce24 S.r.l.* |
| **Data** | 3/11/2026 |

*Ogni punto della richiesta cliente rimasto ambiguo va scritto qui come domanda, non deciso a intuito. Ciò che resta ambiguo non è un requisito: è una domanda da porre al cliente.*

> **Esempio** (Smart Maintenance): "Se un intervento è assegnato a più tecnici e uno solo lo segna come risolto, si chiude per tutti o resta aperto per gli altri?" — non è specificato nella richiesta, va chiesto.

## Domande aperte

| \# | Domanda | Perché è ambigua | Esito |
| :---- | :---- | :---- | :---- |
| 1 | per integrazioni con sistemi di e-commerce si intende il fatto che il cliente mittente possa poi integrare il portale con il proprio negozio online in modo tale che quando arriva un ordine venga subito creata una spedizione?  | nella traccia si parla in modo generico di rendere possibile l’integrazione con sistemi di ecommerce | connettori esclusi; API pronte per creare spedizioni da sistemi esterni |
| 2 | il cliente vuole che vengano tracciate le operazioni di modifica / inserimento fatte dagli operatori per tenere sotto controllo chi ha fatto cosa? | dato che il sistema è centralizzato e i vari operatori possono modificare i dati delle spedizioni e talvolta possono sbagliarsi può essere utile sapere chi ha causato l’errore | SI |
| 3 | sono gli operatori o è il sistema che in automatico decide a quale corriere assegnare le spedizioni in base al loro peso, volumetria, destinazione? | è ambigua perche la traccia non dice come vengono assegnate le spedizioni | sono gli operatori |
| 4 | dopo quanto tempo da quando una spedizione viene completata questa viene cancellate per il cliente mittente e destinatario | perchè nella traccia cio non viene specificato | default 30 giorni poi lo decide l’amministratore |
| 5 | come viene aggiornato lo stato di consegna dei pacchi?tramite scansione qr code? | non viene specificato nella traccia la traccia dice che il corriere deve aggiornare lo stato ma non dice come | niente qr code lo aggiornano l’operatore e corriere |
| 6 | Cosa si intende per simulazione dell'aggiornamento posizione tramite chiamate periodiche.? | perche del pacco non si dovrebbe sapere mai della sua posizione ma si sa solo delle varie fasi che ha raggiunto nel suo ciclo di vita della spedizione | si intende posizione del corriere, coordinate inventate inviate periodicamente |
| 7  | Gli stati essenziali dei pacchi durante la spedizione come cambiano e quando vengono aggiornati?ad es quando un pacco viene messo in non consegnato? | è specificato che lo stato debba cambiare e sono anche specificati quali valori debbano avere ma non è specificato come e quando cambiano questi valori | definite (RF7, RF14, RF15) |
| 8 | il sw deve anche integrare una funzionalità per permettere ai corrieri e ai magazzinieri (siccome i pacchi nella maggiorparte dei casi arrivano a centri di smistamento logistico e non direttamente a destinazione) di capire come caricare in modo corretto i pacchi per farcene stare il più possibile? | è una funzionalità importante che hanno le varie aziende di spedizione solo che non è menzionata | no |
| 9 | il ciclo di vita della spedizione è composto dal corriere che ritira la spedizione assegnata e la consegna al cliente finale oppure ci sono dei passaggi intermedi dove consegna i pacchi a dei centri di smistamento e poi da questi altri corrieri consegnano i pacchi caricati dai magazzinieri contrassegnati da loro come in consegna al destinatario finale ? | perche di solito è raro se non per spedizioni a corta distanza che i corrieri portino i pacchi dei ritiri direttamente al destinatario | no niente centri di smistamento solo consegne urbane  |
| 10 | al cliente serve conservare lo storico delle spedizioni? e a quale utente serve, serve anche al mittente o destinatario della spedizione? | viene specificato di conservare uno storico della spedizione ma non si specifica per chi | operatori e amministratore; destinatario fino a x giorni solo le sue spedizioni |
| 11 | chi crea la spedizione paga direttamente sul portale tramite un servizio esterno? | nella traccia cio non viene specificato ma quando si crea un ritiro di solito chi lo crea deve pagare il servizio direttamente sul sito web del fornitore di logistica | no |
| 12 | la spedizione viene creata dal cliente mittente? | nella traccia viene comunicato che è l’operatore logistico che crea la spedizione però nella maggior parte dei servizi di logistica è il cliente mittente che crea la spedizione dal portale  | no dall’operatore |
| 13 | le note operative interne da chi vengono inserite ? | nella traccia si parla di note operative ad uso interno ma nessuno dice chi le puo inserire | da corrieri e operatori  |
| 14 | Che fine fanno i pacchi non consegnati? | nella traccia si cita lo stato nel quale il pacco possa ritrovarsi in non consegnato ma non si dice se ci sono tentativi di riconsegna e che fine debba fare il pacco non consegnato | vengono contrassegnati con is reso a true per essere restituiti |
| 15 | Quando il corriere rende disponibile la propria posizione? | la traccia non specifica quando dice che deve rendere nota la sua ultima posizione però di solito nei servizi di logistica i corrieri comunicano la propria posizione solamente quando il pacco è in consegna | quando la spedizione è in ritirata oppure in consegna a intervalli regolari definiti dall amministratore  |
| 16 | Che cosa deve poter fare l’amministratore | nella traccia si parla di gestione utenti e permessi però non è chiaro nello specifico quali sono le funzionalità che l’amministratore deve avere | gestire configurazioni utenti e storico  |
| 17 | Le spedizioni vengono archiviate nello storico automaticamente dopo tot giorni o immediatamente? o le archivia l’operatore logistico? | nella traccia è specificato solo che le spedizioni concluse debbano essere nello storico  | lo storico si forma automaticamente |
| 18 | se chi spedisce non da la spedizione al corriere o il corriere non riesce a ritirare cosa succede ? | nella traccia ciò non viene specificato | la spedizione viene annullata dopo 3 tentativi di ritiro |
| 19 | cosa succede se il corriere perde la spedizione? | nella traccia ciò non viene specificato | viene contrassegnata come persa e viene impostato is\_reso \=true |
| 20 | cosa succede se il cliente vuole il reso di una spedizione che l’operatore logistico ha già creato oppure vuole modificarla? | nella traccia ciò non viene specificato | puo modificarla finche non è in ritirata e non puo fare il reso se è gia in ritirata  |

*Aggiungete righe quante servono: tre sono il minimo per considerare la richiesta letta con attenzione, non un limite.*

## Come nascono le domande

*Ogni volta che, scrivendo `requirements.md` o `use-cases.md`, vi accorgete di aver "riempito un vuoto" da soli — una regola non scritta, un caso limite non previsto, un valore non specificato — fermatevi e portate il punto qui prima di proseguire, invece di lasciarlo solo nella vostra testa.*

## Dopo la risposta

*Riguardatele prima di ogni consegna successiva: alcune verranno chiarite dal docente nel ruolo di cliente, altre resteranno aperte finché non deciderete voi una posizione. In quel caso, non cancellate la domanda: aggiungete la decisione presa e la ragione, così chi legge dopo capisce cosa avete assunto e perché.*

| \# | Decisione presa (se non ancora chiarita dal cliente) | Perché |
| :---- | :---- | :---- |
| fino a che stato l’operatore può modificare la spedizione assegnata? | finché non è in ritirata | perché altrimenti il corriere deve continuamente cambiare strada perdendo tempo |
| come viene creato l’utente destinatario che deve accedere alle spedizioni? | l’operatore lo crea inserendolo e dandogli una pwd di default che gli viene comunicata via email | In questo modo l’utente entrerà solo con le sue credenziali e basta e nessun altro porta entrare illecitamente conoscendo la sua mail o nr d’ordine  |
| l’utente mittente deve essere registrato nel db? per far capire subito agli operatori la spedizione a quale cliente mittente è collegata | si e viene inserito dagli operatori al momento della creazione della spedizione insieme al destinatario | in questo modo l’operatore sara facilitato nelle eventuali operazioni di reso |
| se il corriere non riesce a consegnare la spedizione questa ritorna a ritirata o assegnata? | ritorna a ritirata da parte dell’operatore  | perche la spedizione era gia stata ritirata prima e può essere in questo stato riassegnata ad un altro operatore  |
| per quali stati devono essere impostati i motivi e le note ? | vengono impostati per ogni tentativo di spedizione e ritiro e per le spedizioni perse non consegnate e annullate si deve mettere nota \+ motivo e solo il motivo è pubblico | perche ad ogni tentiativo fallito il cliente è interessato a capire perche il pacco non è stato ritirato o consegnato |
| Operatore e corriere possono impostare note pubbliche oppure no? | sia l’operatore che il corriere possono aggiungere quante note interne vogliono, i motivi pubblici rimangono limitati ai possibili stati della spedizione e ai tentativi | in questo modo corriere ed operatore non aggiungono troppi motivi pubblici al cliente  |
| il corriere manda la propria posizione quando la spedizione è in ritirata o in consegna ? | quando è in consegna  | si è deciso di utilizzare in consegna perché è in quello stato che il corriere decide di consegnare il pacco e recapitarlo mentre un pacco potrebbe essere ritirato oggi e consegnato domani  |
| il reso viene fatto anche per le spedizioni annullate oltre che per le spedizioni perse e non consegnate ? | si  | perché in quelle annullate ci sono quelle nelle quali il cliente ha chiesto il reso non solo quelle nelle quali non è stato possibile effettuare il ritiro |

