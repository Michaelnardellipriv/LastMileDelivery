# Architettura v1 — Lastmile Delivery / Veloce24 s.r.l

|  |  |
| :---- | :---- |
| **Team** | *(GLS, componenti)* |
| **Cliente** | *Veloce24 S.r.l.* |
| **Data** | 23/09/2026 |
| **Versione** | v1 — provvisoria per definizione, la confronterete con la v2 a dicembre |

*Questo template è agnostico rispetto al linguaggio: ogni team scelto il proprio stack (linguaggio, framework, database), qui si descrivono componenti e responsabilità, non implementazioni.*

## Componenti

*Un componente è un pezzo che potreste rilasciare, sostituire o spegnere da solo. Per ciascuno: una riga di responsabilità, una di non-responsabilità. Se per descriverne uno vi serve una "e" tra due responsabilità diverse, sono probabilmente due componenti.*

| Componente | Risponde di | Non risponde di |
| :---- | :---- | :---- |
| *Esempio: API backend* | *regole, validazione, transizioni di stato, autorizzazione* | *come i dati appaiono a schermo* |
| Login-frontend | come appaiono i dati a schermo, raccogliere i dati corretti, consentire il login | *regole, validazione, transizioni di stato, autorizzazione* |
| Cambia-pwd-frontend | come appaiono i dati a schermo, raccogliere i dati corretti, consentire di cambiare password del proprio account | *regole, validazione, transizioni di stato, autorizzazione* |
| Recupera-pwd-frontend | come appaiono i dati a schermo, raccogliere i dati corretti, consentire di recuperare password del proprio account | *regole, validazione, transizioni di stato, autorizzazione* |
| Registrati-frontend | come appaiono i dati a schermo, raccogliere i dati corretti, consentire la registrazione | *regole, validazione, transizioni di stato, autorizzazione* |
| Tracking-frontend | come appaiono i dati a schermo, raccogliere i dati corretti, consentire all' operatore di monitorare le spedizioni in consegna | *regole, validazione, transizioni di stato, autorizzazione* |
| creaspedizione-frontend | come appaiono i dati a schermo, raccogliere i dati corretti, consentire al cliente mittente di monitorare le sue spedizioni | *regole, validazione, transizioni di stato, autorizzazione* |
| pacchiDaVisualizzare/storico-frontend | come appaiono i dati a schermo, raccogliere i dati corretti, consentire agli utenti di avere tutte le spedizioni che gli interessano tranne all’amministratore che vede tutti i pacchi compreso lo storico | *regole, validazione, transizioni di stato, autorizzazione* |
| gestione-utenti-fontend | come appaiono i dati a schermo | *regole, validazione, transizioni di stato, autorizzazione* |
| login-api-backend | Verificare credenziali e stato dell’account; creare una sessione di accesso e restituire l’esito.  | *Mostrare la pagina di login, registrare utenti o attribuire nuovi permessi.*  |
| cambia-pwd-api-backend | Verificare permessi per cambiare password dell’utente |  |
| logout-api-backend | Far disconnettere l’utente |  |
| registrati-api-backend | Validare i dati, verificare eventuali duplicati e creare l’account con il ruolo consentito, proteggendo la password.  | *Mostrare il modulo o consentire all’utente di attribuirsi privilegi amministrativi.*  |
| recupera-pwd-api-backend | Validare i dati, e consentire la reimpostazione di una nuova pwd |  |
| creasped-api-backend | Verificare i permessi del richiedente, validare i dati e creare la spedizione nello stato «creata», registrando l’evento.  | *Mostrare il modulo o decidere autonomamente percorsi e corriere assegnato.*  |
| modifica-sped-api-backend | Permette all'operatore logistico di modificare i dati della sua spedizione se questa non è ancora stata ritirata |  |
| elimina-sped-api-backend | Permette al operatore logistico di poter eliminare la spedizione se questa non è in ritirata se il cliente non vuole più spedire |  |
| assegna-sped-api-backend | Verificare i permessi dell’operatore, controllare spedizione e corriere che siano disponibili, registrare l’assegnazione e aggiornare il nuovo stato e storico del workflow.  | *Mostrare la schermata di assegnazione o ottimizzare automaticamente i percorsi.*  |
| modifica-stato-api-backend | Verificare che l’utente possa intervenire sulla spedizione, controllare la transizione e registrare nuovo stato se coerente, autore e data/ora.  | *Mostrare lo stato o di cambiamenti stato fatti nel db direttamente* |
| recupera-stato-api-backend | Restituire stato e storico autorizzato della spedizione, escludendo le note interne dalla risposta al destinatario.  | *Modificare lo stato o decidere l’aspetto delle pagine in cui compare.* |
| recupera-spedizioni-api-backend | Restituire le spedizioni accessibili all’utente in base al ruolo, applicando eventuali filtri per stato, corriere, data e riferimento.  | *Cambiare stati o assegnazioni; disegnare le tabelle a schermo.*  |
| disattiva-utente-api-backend | Verificare i permessi amministrativi e disattivare l’account secondo le regole concordate, preservando la tracciabilità.  | *Cancellare da db altri dati che non hanno a che fare con gli utenti e non risponde nemmeno di cancellazione dell’utente con i dati a esso associati direttamente da db* |
| aggiungi-utente-api-backend | Verificare i permessi dell’amministratore, validare i dati e creare un account con un ruolo ammesso.  | *Mostrare il modulo o consentire creazioni amministrative direttamente da db* |
| manda-coordinate-posizione-api-backend | Mandare all’operatore la posizione del corriere non appena mette la spedizione in consegna | *Mostrare le coordinate graficamente* |
|  |  |  |
| modifica-permessi-api-backend | Verificare chi può modificare i ruoli e applicare esclusivamente le modifiche richieste.  |  |
| modifica-conf-api-backend | Gestire le configurazioni per il sistema |  |
| inserisci-note-api-backend | far inserire le note agli operatori corretti |  |
| inserisci-problematiche-api-backend | far inserire le problematiche agli operatori corretti |  |
| Database | salvare i dati che servono, ridondanza,eliminazioni di dati che servono | *regole, validazione, transizioni di stato, autorizzazione e come appaiono i dati a schermo* |

## Diagramma

*Scatole \= componenti, frecce \= "chiama"/"dipende da" con sopra cosa passa (es. HTTPS/JSON, SQL, file). Segnate cosa è dentro il vostro perimetro e cosa è fuori (servizi di terzi, sistemi del cliente). Ciò che è previsto ma non ancora realizzato si disegna tratteggiato. Va bene un blocco Mermaid come questo, un disegno fotografato, o qualunque notazione capiate a colpo d'occhio come team — l'importante è che le frecce siano etichettate.*

flowchart LR

  Utente \--\>|HTTPS| Frontend

  Frontend \--\>|HTTPS/JSON| API\[API backend\]

  API \--\>|SQL| DB\[(Database)\]

  API \-.-\>|previsto: notifiche| Notifiche\[Servizio notifiche\]

## Dipendenze

*Elenco esplicito: chi dipende da chi, e cosa succede se il componente da cui si dipende si ferma o risponde male.*

| Componente | Dipende da | Se si ferma |
| :---- | :---- | :---- |
| *Esempio: Frontend* | *API backend* | *l'utente vede un errore, nessun dato scritto due volte* |
| Login-frontend | Registrati-frontend,registrati-api-backend | l’utente non riuscirà mai a creare un account |
| tracking-frontend | Login-frontend,login-api-backend,pacchiDaVisualizzare/storico-frontend,recupera-spedizioni-api-backend | l’utente non potra mai accedere alla pagina |
| creaspedizione-frontend | Login-frontend,login-api-backend, | l’utente non potra mai accedere alla pagina |
| pacchiDaVisualizzare/storico-frontend | Login-frontend,login-api-backend,recupera-spedizioni-api-backend | l’utente non potra mai accedere alla pagina, oppure vede un errore con nessun dato,oppure se amministratore eliminare lo strorico |
| gestione-utenti-fontend | Login-frontend,login-api-backend,aggiungi-utente-api-backend,disattiva-utente-api-backend,modifica-permessi-api-backend | l’utente non potra mai accedere alla pagina,oppure disattivare aggiungere un utente o modificare i ruoli di un utente |
| Cambia-pwd-frontend | Login-frontend,cambia-pwd-backend | L’utente non può più cambiare la sua pwd  |
| Recupera-pwd-frontend | Cambia-pwd-frontend,recupera-pwd-backend,cambia-pwd-backend | L’utente se si dimentica la password non potrà più accedere |
| api backend | db | le api non posso più restituire i dati e restituiranno degli errori che verranno visualizzati nel front-end |

## Fuori dal perimetro

*Cosa esiste ma non lo costruite voi: sistemi del cliente, servizi esterni, integrazioni future dichiarate nella richiesta.*

- Esempio: integrazione con sistemi di notifica aziendali (futura, non in v1)  
- Integrazione con sistema di mailing per avvisi al cliente  
- Integrazione con sistemi gestionali  
- Integrazione con sistemi di e-commerce 