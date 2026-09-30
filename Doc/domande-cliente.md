# Domande al cliente — \[Nome progetto\] / \[Cliente\]

|  |  |
| :---- | :---- |
| **Team** | *(nome team, componenti)* |
| **Cliente** | *(es. NordFacility S.r.l.)* |
| **Data** | 23/09/2026 |

*Ogni punto della richiesta cliente rimasto ambiguo va scritto qui come domanda, non deciso a intuito. Ciò che resta ambiguo non è un requisito: è una domanda da porre al cliente.*

> **Esempio** (Smart Maintenance): "Se un intervento è assegnato a più tecnici e uno solo lo segna come risolto, si chiude per tutti o resta aperto per gli altri?" — non è specificato nella richiesta, va chiesto.

## Domande aperte

| \# | Domanda | Perché è ambigua | Impatto se cambia la risposta |
| :---- | :---- | :---- | :---- |
| 1 | per integrazioni con sistemi di e-commerce si intende il fatto che il cliente mittente possa poi integrare il portale con il proprio negozio online in modo tale che quando arriva un ordine venga subito creata una spedizione?  | nella traccia si parla in modo generico di rendere possibile l’integrazione con sistemi di ecommerce | se la risposta è si allora bisognerà prevedere la possibilità di integrazione con sistemi esterni con un costo per il cliente maggiore |
| 2 | il cliente vuole che vengano tracciate le operazioni di modifica / inserimento fatte dagli operatori per tenere sotto controllo chi ha fatto cosa? | dato che il sistema è centralizzato e i vari operatori possono modificare i dati delle spedizioni e talvolta possono sbagliarsi può essere utile sapere chi ha causato l’errore | se cambia la risposta il cliente può trovarsi con una feature inutile nel migliore dei casi ma con un costo in più oppure non avere una feature che aveva richiesto |
| 3 | sono gli operatori o è il sistema che in automatico decide a quale corriere assegnare le spedizioni in base al loro peso, volumetria, destinazione? | è ambigua perche la traccia non dice come vengono assegnate le spedizioni | se cambia la risposta il cliente può non avere questa funzionalità che gli farebbe risparmiare costi e tempo da parte degli operatori e gli garantirebbe una maggiore efficenza |
| 4 | dopo quanto tempo da quando una spedizione viene completata questa viene cancellate per il cliente mittente e destinatario | perchè nella traccia cio non viene specificato | se viene cancellata subito il cliente rischia di non sapere se la spedizione è avvenuta o no, se non viene cancellata l’applicazione dovrà mantenere piu dati e avra piu costi |
| 5 | come viene aggiornato lo stato di consegna dei pacchi?tramite scansione qr code? | non viene specificato nella traccia la traccia dice che il corriere deve aggiornare lo stato ma non dice come | se viene aggiornato in questo modo il corriere/magazziniere perde meno tempo a cercare il pacco e ad aggiornare il suo stato però la piattaforma deve poter fornire al mittente le etichette da stampare e mettere sul pacco  |
| 6 | Cosa si intende per simulazione dell'aggiornamento posizione tramite chiamate periodiche.? | perche del pacco non si dovrebbe sapere mai della sua posizione ma si sa solo delle varie fasi che ha raggiunto nel suo ciclo di vita della spedizione | se non è cosi allora va pensato un sistema di tracking in tempo reale per i pacchi |
| 7  | Gli stati essenziali dei pacchi durante la spedizione come cambiano e quando vengono aggiornati?ad es quando un pacco viene messo in non consegnato? | è specificato che lo stato debba cambiare e sono anche specificati quali valori debbano avere ma non è specificato come e quando cambiano questi valori | se la risposta è diversa da quello che ci si aspetta si ha la possibilità di dare stati incoerenti ai clienti e invalidare l’utilità del portale |
| 8 | il sw deve anche integrare una funzionalità per permettere ai corrieri e ai magazzinieri (siccome i pacchi nella maggiorparte dei casi arrivano a centri di smistamento logistico e non direttamente a destinazione) di capire come caricare in modo corretto i pacchi per farcene stare il più possibile? | è una funzionalità importante che hanno le varie aziende di spedizione solo che non è menzionata | se la risposta è si allora bisognerà prevedere la possibilità di integrare la feature con un costo per il cliente maggiore |
| 9 | il ciclo di vita della spedizione è composto dal corriere che ritira la spedizione assegnata e la consegna al cliente finale oppure ci sono dei passaggi intermedi dove consegna i pacchi a dei centri di smistamento e poi da questi altri corrieri consegnano i pacchi caricati dai magazzinieri contrassegnati da loro come in consegna al destinatario finale ? | perche di solito è raro se non per spedizioni a corta distanza che i corrieri portino i pacchi dei ritiri direttamente al destinatario | se la risposta è si allora bisognerà prevedere la possibilità che anche i magazzinieri possano aggiornare lo stato delle spedizioni quando caricano i furgoni dei corrieri per permettere ai corrieri la mattina di avere il furgone già pronto con i pacchi che sono gia messi in consegna |
| 10 | al cliente serve conservare lo storico delle spedizioni? e a quale utente serve, serve anche al mittente o destinatario della spedizione? | viene specificato di conservare uno storico della spedizione ma non si specifica per chi | se al cliente serve allora si dovra prevedere nel db la possibilità di conservare lo storico altrimenti no |
| 11 | chi crea la spedizione paga direttamente sul portale tramite un servizio esterno? | nella traccia cio non viene specificato ma quando si crea un ritiro di solito chi lo crea deve pagare il servizio direttamente sul sito web del fornitore di logistica | a seconda di qual’è la risposta bisogna implementare un collegamento ad servizio esterno di pagamento o un servizio di fatturazione |
| 12 | la spedizione viene creata dal cliente mittente? | nella traccia viene comunicato che è l’operatore logistico che crea la spedizione però nella maggior parte dei servizi di logistica è il cliente mittente che crea la spedizione dal portale  | se la spedizione viene creata dall’operatore non bisogna creare un pagina apposta sul portale per chi vuole spedire dove crea la spedizione ma lasciare soltanto i recapiti telefonici o email per richiedere la spedizione |
| 13 | le note operative interne da chi vengono inserite ? | nella traccia si parla di note operative ad uso interno ma nessuno dice chi le puo inserire | se le possono inserire solo certi tipi di utenti bisogna configurare degli appositi permessi |
| 14 | Che fine fanno i pacchi non consegnati? | nella traccia si cita lo stato nel quale il pacco possa ritrovarsi in non consegnato ma non si dice se ci sono tentativi di riconsegna e che fine debba fare il pacco non consegnato | se la risposta cambia cambiano anche le funzionalità che avrà l’operatore logistico e quando il pacco potrà dirsi non consegnato |
| 15 | Quando il corriere rende disponibile la propria posizione? | la traccia non specifica quando dice che deve rendere nota la sua ultima posizione però di solito nei servizi di logistica i corrieri comunicano la propria posizione solamente quando il pacco è in consegna |  |
| 16 | Che cosa deve poter fare l’amministratore | nella traccia si parla di gestione utenti e permessi però non è chiaro nello specifico quali sono le funzionalità che l’amministratore deve avere |  |

*Aggiungete righe quante servono: tre sono il minimo per considerare la richiesta letta con attenzione, non un limite.*

## Come nascono le domande

*Ogni volta che, scrivendo `requirements.md` o `use-cases.md`, vi accorgete di aver "riempito un vuoto" da soli — una regola non scritta, un caso limite non previsto, un valore non specificato — fermatevi e portate il punto qui prima di proseguire, invece di lasciarlo solo nella vostra testa.*

## Dopo la risposta

*Riguardatele prima di ogni consegna successiva: alcune verranno chiarite dal docente nel ruolo di cliente, altre resteranno aperte finché non deciderete voi una posizione. In quel caso, non cancellate la domanda: aggiungete la decisione presa e la ragione, così chi legge dopo capisce cosa avete assunto e perché.*

| \# | Decisione presa (se non ancora chiarita dal cliente) | Perché |
| :---- | :---- | :---- |
|  |  |  |

