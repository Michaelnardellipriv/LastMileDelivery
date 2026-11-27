# Due microservizi, due database

Il backend è diviso in due servizi, ognuno con il suo database:

| Servizio | Programma | Database | Cosa fa |
| :---- | :---- | :---- | :---- |
| **utenti** | `backend/cmd/utenti` | globale (`db/globale/schema.sql`) | login e token, utenti e ruoli, clienti, corrieri e operatori, sedi, indirizzi e geografia |
| **spedizioni** | `backend/cmd/spedizioni` | spedizioni (`db/spedizioni/schema.sql`), uno per gruppo di sedi | spedizioni, assegnazioni, note, tentativi, storico, impostazioni delle sedi, motivi, posizione dei corrieri |

Il servizio utenti autentica: emette il token, che contiene ruolo e id. Se l'utente è
l'amministratore, gli dà accesso alla gestione di utenti e sedi. Il servizio spedizioni
non fa login: verifica lo stesso token (stesso `JWT_SEGRETO`) e lavora sulle spedizioni.

## Perché due

- **Scalano in modo diverso.** Le spedizioni ricevono quasi tutto il traffico: corrieri
  che cambiano stato e mandano la posizione ogni 30 secondi, operatori che assegnano.
  Login e anagrafiche cambiano poco. In Kubernetes ognuno ha il suo Deployment e il suo
  numero di repliche (`kubectl scale deployment spedizioni --replicas=4`).
- **Il database delle spedizioni è quello che cresce.** Storico e note aumentano ogni
  giorno, utenti e sedi quasi mai. Con due database si dimensiona (e si divide per
  gruppi di città, vedi sotto) solo quello che serve.
- **Confini chiari.** Ogni servizio è l'unico che legge e scrive le sue tabelle: un
  errore o una migrazione nelle spedizioni non tocca il login.

## Cosa ci costa

Tra due database non ci sono JOIN, chiavi esterne né transazioni comuni. Quello che prima
faceva PostgreSQL adesso lo fa il codice, con una chiamata all'altro servizio:

| Prima (un database) | Adesso |
| :---- | :---- |
| `JOIN` di spedizioni con clienti, utenti, indirizzi, sedi per l'elenco | il servizio spedizioni legge le sue righe, poi chiede nomi, indirizzi e sede di tutta la pagina con **una** chiamata `POST /interno/risolvi` |
| chiavi esterne verso clienti, corrieri, indirizzi; indirizzi nella città della sede | `controlla` (`internal/spedizioni/repository.go`) chiede i dati prima di scrivere e dà gli stessi errori (400 sul campo, 409) |
| ad ogni richiesta il middleware rilegge se l'utente è attivo e in che sede lavora | ogni token porta la sua **sessione**: il servizio utenti la controlla nel database globale (aperta, utente attivo, sede attuale); il servizio spedizioni controlla nel suo database la tabella `sessioni_revocate`, che il servizio utenti aggiorna (`POST /interno/sessioni/revocate`) **prima** di confermare una disattivazione, un cambio di sede, una password nuova o un logout. Revoca immediata su tutti e due, senza una chiamata di rete per richiesta |
| `EXISTS` sulle spedizioni per sapere se un cliente è destinatario (login, elenchi) | colonna `clienti.destinatario` nel database globale, segnata dal servizio spedizioni quando il cliente riceve una spedizione (`POST /interno/clienti/{id}/destinatario`) |
| conteggio delle spedizioni aperte di un corriere prima di cambiargli sede | il servizio utenti chiede `GET /interno/corrieri/{id}/spedizioni-aperte` |
| riga di `impostazioni` creata insieme alla sede | la sede nasce nel database globale; la sua riga di impostazioni la crea il servizio spedizioni al primo uso, con i valori predefiniti |

Due conseguenze da conoscere:

- **Niente transazione unica.** Se l'amministratore disattiva un corriere nello stesso
  istante in cui un operatore glielo assegna, l'assegnazione può passare: il controllo e
  la scrittura sono in due database. Succede di rado e si corregge riassegnando.
- **Revoca subito, ma servono tutti e due i servizi.** Una disattivazione (o un cambio di
  sede, una password nuova, un logout) si conferma solo dopo che il servizio spedizioni ha
  registrato le sessioni chiuse: se il servizio spedizioni non risponde, l'operazione
  fallisce con 503 e si riprova. Non può succedere che un utente risulti disattivato ma
  possa ancora lavorare sulle spedizioni.
- **Se il servizio utenti è giù, le spedizioni rispondono 503** ("servizio non
  disponibile, riprova") sulle letture: non sanno i nomi da mostrare.

## Il contratto interno

`backend/internal/interno` è l'unico punto in cui i due servizi si toccano:

| Rotta | Servizio | Chi la chiama | Risposta |
| :---- | :---- | :---- | :---- |
| `POST /interno/risolvi` | utenti | spedizioni | sedi (nome, attiva, città, `gruppo`: il database delle spedizioni), indirizzi, persone (nome, email, telefono, attivo, sede) per gli id chiesti |
| `POST /interno/clienti/{id}/destinatario` | utenti | spedizioni | 204: il cliente può entrare a seguire le sue spedizioni |
| `GET /interno/corrieri/{id}/spedizioni-aperte` | spedizioni | utenti | `{"aperte": true/false}` |
| `POST /interno/sessioni/revocate` | spedizioni | utenti | 204: da adesso i token di quelle sessioni qui non valgono (403 se l'utente è stato disattivato, 401 altrimenti); ogni sessione porta ruolo e sede del login, e si scrive nel database che controlla quel token |
| `GET /interno/gruppi` | spedizioni | utenti | `{"codici": [0, 1, ...]}`: i gruppi di sedi che hanno un database delle spedizioni (prima di creare una sede in un gruppo) |

Le rotte `/interno` non sono nell'Ingress (che espone solo `/api` e il frontend) e
rispondono solo con l'intestazione `X-Chiave-Interna` uguale a `CHIAVE_INTERNA`; senza,
sembrano non esistere (404).

L'Ingress manda ogni percorso al servizio che ha i dati, quindi l'app Flutter non sa che
i servizi sono due. La divisione in servizi non ha cambiato l'API pubblica
(`backend/api/openapi.yaml`); la mappa sede → database (0.46.0) sì, poco: `id_sede`
obbligatorio per l'amministratore nell'elenco delle spedizioni e `gruppo_spedizioni`
nelle sedi.

## In sviluppo

`backend/cmd/server` fa girare i due servizi nello stesso processo, ognuno con il suo
database (`DATABASE_URL` e `DATABASE_SPEDIZIONI_URL`, più `DATABASE_SPEDIZIONI_URL_<n>`
per gli altri gruppi di sedi). Smista i percorsi come l'Ingress
(`server.PerSpedizioni`) e le chiamate `/interno` passano in memoria invece che in rete:
stesso codice, senza Kubernetes. Lo usano `docker compose` e i test di integrazione.

## Più database delle spedizioni: la mappa sede → database

Dalla versione 2.4.0 (API 0.46.0) il database delle spedizioni si divide per **gruppi di
sedi**: le città grandi in un database ciascuna, le piccole raggruppate. Il servizio
utenti e il database globale restano uno solo; il servizio spedizioni resta un solo
Deployment, collegato a tutti i database dei gruppi.

### Gruppi e mappa

- Il gruppo di una sede è la colonna `sedi.gruppo_spedizioni` del database globale
  (`SMALLINT`, 0..9006, predefinito 0). Si sceglie alla creazione (`POST /api/sedi`,
  campo facoltativo `gruppo_spedizioni`) e **non si cambia**: il trigger
  `fn_gruppo_immutabile` rifiuta l'`UPDATE` e `PATCH /api/sedi/{id}` con
  `gruppo_spedizioni` risponde 400. Spostare una sede in un altro gruppo è una migrazione
  dei dati (vedi i limiti).
- Il **gruppo 0** è il principale e c'è sempre. Con il solo gruppo 0 tutto funziona come
  prima, con un unico database delle spedizioni.
- Il servizio spedizioni chiede il gruppo di una sede al servizio utenti
  (`/interno/risolvi`, campo `gruppo` della sede) la prima volta che gli serve e lo tiene
  in memoria per sempre: la mappa non cambia.
- Una sede si può creare in un gruppo solo se quel gruppo ha un database configurato: il
  servizio utenti lo chiede a `GET /interno/gruppi` (altrimenti 400 su
  `gruppo_spedizioni`, con l'elenco dei gruppi configurati).

### Configurazione

| Dove | Gruppo 0 | Gruppo n |
| :---- | :---- | :---- |
| servizio spedizioni (`cmd/spedizioni`) | `DATABASE_URL` | `DATABASE_URL_<n>` (chiavi del Secret `spedizioni-secret`) |
| sviluppo (`cmd/server`) | `DATABASE_SPEDIZIONI_URL` | `DATABASE_SPEDIZIONI_URL_<n>` |
| immagine del database (`db-spedizioni`) | `GRUPPO_SPEDIZIONI` vuota o 0 | `GRUPPO_SPEDIZIONI=n`, immagine costruita con `DATI=nessuno` |

Ogni database delle spedizioni dichiara il suo gruppo nella tabella `gruppo_database`
(una riga, di sola lettura per l'utente del backend). All'avvio il servizio spedizioni la
controlla per ogni URL e si ferma se non corrisponde: un URL sbagliato metterebbe le
spedizioni di un gruppo nel database di un altro. Un database nuovo prende il gruppo alla
prima creazione del container: `db/gruppo_spedizioni.sh` (in
`/docker-entrypoint-initdb.d/01-z-gruppo-spedizioni.sh`) gira dopo lo schema e prima dei
dati e chiama `fn_imposta_gruppo(n)`. In Kubernetes l'esempio pronto è
`k8s/esempi/gruppo-spedizioni-1.yaml` (passi in `docs/deploy-kubernetes.md`).

### Id unici tra gruppi

Nel database del gruppo g ogni sequenza (spedizioni, note, tentativi, storico, ...)
parte da **g × 10^12 + 1**. Quindi:

- il gruppo di una spedizione è `id / 10^12` (`database.GruppoDellID`): dato un id, il
  servizio sa in quale database cercarla senza chiedere niente a nessuno;
- gli id non si ripetono tra gruppi diversi;
- con al massimo 9006 gruppi gli id restano sotto 2^53: numeri esatti anche in
  JavaScript (l'app web). Ogni gruppo ha posto per 10^12 righe per tabella.

Il gruppo 0 parte da 1, come prima: un database esistente diventa il gruppo 0 senza
cambiare i suoi id.

### Chi legge quale database

| Operazione | Database |
| :---- | :---- |
| operatore o corriere: elenco, creazione, assegnazione, tentativi, stato, note, posizione | quello della sede del token |
| una spedizione per id (`/api/spedizioni/{id}`, `/note`, `/storico`, ...) | quello del gruppo dell'id (`id / 10^12`); un id di un gruppo che non c'è risponde 404 |
| amministratore, `GET /api/spedizioni` | quello della sede scelta con `id_sede` (**obbligatorio**) |
| amministratore, dettaglio e storico | quello del gruppo dell'id, di qualunque sede |
| cliente, `GET /api/spedizioni` | **tutti** i gruppi, risultati uniti (vedi sotto) |
| `GET`/`PATCH /api/sedi/{id}/impostazioni` | quello della sede `{id}` |
| `GET /api/impostazioni` (intervallo della posizione) | quello della sede del token |
| motivi (`/api/motivi`, `/api/impostazioni/motivi`) | letti e modificati nel principale (gruppo 0), copiati negli altri |
| chiavi di idempotenza | quello della sede di chi chiede; il principale per amministratore e cliente |
| controllo delle sessioni revocate | quello della sede del token; il principale per amministratore e cliente |
| `POST /interno/sessioni/revocate` | per ogni sessione, quello che controlla il suo token: il gruppo della sede del login (operatori e corrieri), il principale (clienti e amministratori); tutti per le sessioni aperte prima della 2.4.0, senza sede |
| `GET /interno/corrieri/{id}/spedizioni-aperte` | **tutti** i gruppi (un corriere già spostato può averne nel gruppo della sede di prima) |
| `GET /interno/gruppi` | nessuno: i gruppi configurati |
| `/ready` | solo il principale |

### La regola dell'amministratore

L'amministratore guarda l'elenco delle spedizioni **una sede alla volta**: in
`GET /api/spedizioni` `id_sede` è obbligatorio (senza: 400 sul campo `id_sede`,
"obbligatorio per l'amministratore: scegliere una sede"; una sede che non esiste: 404
"sede non trovata"). Non esiste più la vista "tutte le sedi": unire tutti i database per
l'elenco dell'amministratore sarebbe la query più pesante del sistema, e con una sede
scelta se ne legge uno solo. Dettaglio e storico per id restano liberi, perché l'id dice
già il database. Per operatori e corrieri non cambia niente.

### L'elenco del cliente

Il cliente non ha una sede e riceve spedizioni da sedi di gruppi diversi, quindi il suo
elenco legge tutti i gruppi: da ognuno le prime `pagina × per_pagina` righe, poi le unisce
con lo stesso ordine (`creata_il` decrescente, poi `id` decrescente) e prende la pagina
chiesta; `totale_elementi` è la somma dei conteggi dei gruppi. Con un solo gruppo è la
query di sempre.

### Dati copiati in ogni gruppo

- **Motivi.** La copia di riferimento è nel principale: l'amministratore li cambia lì e
  subito dopo il servizio li copia negli altri gruppi (stessi id e descrizioni; un motivo
  tolto viene disattivato, non cancellato). La copia si ripete anche ogni minuto
  (`SincronizzazionePeriodica`): un gruppo che era giù durante la modifica si riallinea
  da solo, e per quel poco vede i motivi di prima.
- **Sessioni revocate** (non sono copiate: ognuna va in un solo database). Il token di un
  operatore o di un corriere si controlla nel gruppo della sede del login, quello di un
  cliente o di un amministratore nel principale; la sessione ricorda la sede
  (`sessioni.id_sede`) e il servizio utenti la manda con la revoca. Se quel database non
  risponde la chiamata fallisce e l'operazione del servizio utenti (disattivazione, cambio
  di sede, password, logout) fallisce con lei: la revoca vale o si riprova. Un gruppo giù
  blocca solo le revoche dei suoi utenti.

### Se un database di gruppo è giù

`/ready` controlla solo il principale: se il database di un altro gruppo non risponde,
ricevono 503 solo le richieste delle sue sedi (e quelle che leggono tutti i gruppi:
elenco del cliente, spedizioni aperte di un corriere), mentre le altre sedi continuano a
lavorare. Un `/ready` fallito toglierebbe invece il servizio a tutte. Anche all'avvio: il
gruppo 0 deve rispondere, un altro gruppo che non risponde non ferma il servizio (resta da
controllare e si controlla al primo uso; intanto le sue sedi hanno 503). Un database che
risponde ma dichiara un altro gruppo (URL sbagliato, database non migrato) invece ferma
l'avvio: è un errore di configurazione, non un guasto passeggero.

### Limiti

- `cod_ordine` è unico solo **dentro un gruppo**: due sedi di gruppi diversi possono avere
  lo stesso codice ordine (il vincolo è del database).
- **Spostare una sede in un altro gruppo** è una migrazione dei dati fatta a mano: copiare
  le sue spedizioni (con tutte le tabelle collegate, mantenendo gli id) nel database del
  nuovo gruppo, toglierle dal vecchio e cambiare `sedi.gruppo_spedizioni` disattivando il
  trigger, a servizi fermi (il servizio spedizioni tiene la mappa in memoria). Non c'è uno
  strumento per farlo.
- L'**elenco del cliente** con più gruppi legge `pagina × per_pagina` righe da ogni gruppo:
  va bene per le prime pagine (il caso normale), costa di più andando avanti.
- Le **migrazioni** dei database delle spedizioni vanno eseguite su ogni gruppo, una per
  una (`docs/deploy-kubernetes.md`): non c'è uno strumento che le applichi a tutti.

## Indirizzi e città: in quale database

Dalla versione 2.5.0 gli **indirizzi di ritiro e consegna** stanno nel database delle
spedizioni del gruppo della loro sede (tabella `indirizzi`, con `id_sede` e la Row-Level
Security come le altre tabelle), e `GET/POST /api/indirizzi` li serve il servizio
spedizioni. Il motivo:

- li scrivono solo gli operatori, sempre nella città della loro sede, crescono con il
  numero delle spedizioni (fino a due nuovi per spedizione) e non si modificano;
- li legge quasi solo il servizio spedizioni: ora con una join, non più con
  `/interno/risolvi`;
- la regola "ritiro e consegna sono indirizzi della sede della spedizione" è una chiave
  esterna (`fk_spedizioni_ritiro_stessa_sede`, `fk_spedizioni_consegna_stessa_sede`);
- il database globale non cresce più con le spedizioni.

Le chiamate di rete non calano: nomi di clienti e corrieri e dati della sede vengono
comunque dal servizio utenti. Gli id restano unici tra i gruppi, perché ogni gruppo numera
da `gruppo × 10^12`.

**Città, regioni e nazioni restano nel globale**, con l'indirizzo di ogni sede: sono poche
righe (una città per sede), nascono con `POST /api/sedi` e non cambiano. Il servizio
spedizioni usa solo l'id della città della sede; i nomi (città, regione, nazione) gli
arrivano con la sede da `/interno/risolvi` e valgono per tutti gli indirizzi della sede.
Copiarli in ogni gruppo non toglierebbe nessuna chiamata e aggiungerebbe una copia da
tenere allineata. La validazione dei campi di un indirizzo è una sola, comune ai due
servizi (`backend/internal/indirizzo`).

Gli indirizzi delle spedizioni creati prima della 2.5.0 restano anche nel database
globale (non servono più, ma toglierli non è necessario); la migrazione li copia nei
database dei gruppi con lo stesso id (`docs/deploy-kubernetes.md`).
