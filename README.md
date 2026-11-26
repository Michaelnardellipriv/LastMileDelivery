# LastMileDelivery

| | |
|---|---|
| **Cliente** | Veloce24 S.r.l. |
| **Team** | GLS |
| **Membri** | Dennis Pasquali, Francesco Plateroti, Michael Nardelli, Gianluca Vivaldi, Mohammed Amine Reddad, Christian Pellegrini |
| **Data** | 03/11/2026 |
| **Versione** | v0.3 |

## Cosa fa questo progetto

Centralizza le spedizioni urbane di Veloce24 e il loro ciclo di vita, oggi gestiti con fogli condivisi e telefonate:

- l'operatore crea le spedizioni, le assegna ai corrieri e le monitora;
- il corriere ritira, consegna e registra i tentativi non riusciti;
- il cliente destinatario segue lo stato delle sue spedizioni;
- l'amministratore consulta spedizioni e storico.

Ogni cambio di stato e ogni modifica restano nello storico, con autore, data, valore precedente e valore nuovo.

## Stack

```
Backend:   Go 1.26 (net/http, pgx v5), due microservizi: utenti e spedizioni
Frontend:  Flutter web (operatori, corrieri, amministratore, destinatari)
Database:  PostgreSQL 16: globale, e spedizioni (uno per gruppo di sedi)
Deploy:    Docker, Kubernetes
```

Il servizio **utenti** (database globale) fa il login e gestisce utenti, clienti, sedi e
indirizzi; il servizio **spedizioni** (database delle spedizioni) gestisce spedizioni,
note, storico, impostazioni delle sedi e posizione dei corrieri. Si parlano solo con le
rotte `/interno`. Le sedi possono essere divise in gruppi, ognuno con il suo database delle
spedizioni (mappa sede → database); con il solo gruppo 0 c'è un unico database delle
spedizioni. Perché e come: [docs/microservizi.md](docs/microservizi.md).

## Come si esegue

Servono Docker e Go 1.26.

Il modo più rapido è tutto in container (i due database e il backend di sviluppo):

```bash
cp config/dev.env.example config/dev.env   # la prima volta: scegliete le due password (file ignorato da git)
docker compose --env-file config/dev.env up --build
```

Oppure, passo per passo:

1. Database. Avviate i due container: globale sulla porta 5432, spedizioni sulla 5433. Alla prima creazione caricano schema e dati di prova (`db/globale/` e `db/spedizioni/`, con due sedi: Trento e Bolzano) e creano l'utente normale del backend (`db/ruolo_applicazione.sh`):

   ```bash
   docker compose --env-file config/dev.env up -d --build db-globale db-spedizioni
   ```

2. Backend. In sviluppo `cmd/server` fa girare i due microservizi nello stesso processo, sulla stessa porta. Impostate i due URL con l'utente del backend (`APP_DB_USER`, non `POSTGRES_USER`, che è superutente e ignorerebbe la Row-Level Security sulle sedi), poi avviatelo (i valori di sviluppo sono in `config/dev.env`; in PowerShell `$env:DATABASE_URL = "..."`):

   ```bash
   export DATABASE_URL="postgres://lastmile_app:<APP_DB_PASSWORD>@localhost:5432/lastmile?sslmode=disable"
   export DATABASE_SPEDIZIONI_URL="postgres://lastmile_app:<APP_DB_PASSWORD>@localhost:5433/lastmile?sslmode=disable"
   ```
   ```bash
   cd backend && go run ./cmd/server
   ```

   Il server ascolta su `http://127.0.0.1:8080`. Variabile facoltativa: `LISTEN_ADDR`. Tentativi massimi, giorni di visibilità del cliente, intervallo della posizione e motivi li imposta l'amministratore con `/api/impostazioni` (vedi [docs/api.md](docs/api.md#amministrazione)).

   Per i dati dimostrativi (più utenti e spedizioni in tutti gli stati) caricate su database vuoti `db/globale/seed_demo.sql` e `db/spedizioni/seed_demo.sql`.

3. Prova:

   ```bash
   curl -H "X-Id-Operatore: 1" http://localhost:8080/api/spedizioni
   ```

Gli endpoint sono descritti in [`docs/api.md`](./docs/api.md) e nel contratto [`backend/api/openapi.yaml`](./backend/api/openapi.yaml).

### Un secondo gruppo di sedi (facoltativo)

Le sedi sono divise in gruppi, ognuno con il suo database delle spedizioni (mappa sede → database, [docs/microservizi.md](docs/microservizi.md)). Di solito in sviluppo basta il gruppo 0. Per provarne un secondo c'è il profilo `gruppo1` del compose: un database delle spedizioni vuoto (`DATI=nessuno`) con `GRUPPO_SPEDIZIONI=1`, sulla porta 5434.

```bash
# tutto in container
DATABASE_SPEDIZIONI_URL_1="postgres://lastmile_app:<APP_DB_PASSWORD>@db-spedizioni-1:5432/lastmile?sslmode=disable" \
  docker compose --env-file config/dev.env --profile gruppo1 up --build

# oppure con go run: il database nel compose, il backend sul PC
docker compose --env-file config/dev.env --profile gruppo1 up -d --build db-spedizioni-1
export DATABASE_SPEDIZIONI_URL_1="postgres://lastmile_app:<APP_DB_PASSWORD>@localhost:5434/lastmile?sslmode=disable"
cd backend && go run ./cmd/server      # nel log: "database delle spedizioni: 2 gruppi"
```

Poi l'amministratore crea una sede del gruppo 1 (`POST /api/sedi` con `"gruppo_spedizioni": 1`): le sue spedizioni hanno id da 1000000000001 in su. Nell'elenco delle spedizioni l'amministratore indica sempre la sede (`?id_sede=...`).

### Test

- Unit test, senza database:

  ```bash
  cd backend && go test ./...
  ```

- Test di integrazione: richiedono il server di sviluppo avviato (`go run ./cmd/server`) e i due database di prova. Il contenuto dei database viene modificato, quindi non usate quelli di produzione.

  ```bash
  cd backend && go test -tags=integration -count=1 ./tests
  ```

  Variabili: `TEST_API_URL` (predefinito `http://localhost:8080`), `TEST_DATABASE_URL` (database globale) e `TEST_DATABASE_SPEDIZIONI_URL` (obbligatorie). Su Windows c'è `backend/tests/avvia_test.ps1`.

### Kubernetes

I manifest sono in `k8s/`: i due database, i due microservizi e il frontend (2 repliche per servizio e per il frontend), i Service e l'Ingress su `lastmile.test` (i percorsi `/api` divisi tra utenti e spedizioni, `/` al frontend). Le immagini si costruiscono da `db/Dockerfile` (`PARTE=globale|spedizioni`), `backend/Dockerfile` (`SERVIZIO=utenti|spedizioni`) e `flutter_app/Dockerfile`. Procedura completa (MicroK8s, Docker Desktop o minikube, Secret, verifica): [docs/deploy-kubernetes.md](docs/deploy-kubernetes.md). Rilascio, rollback, HTTPS, checklist e ambiente PROD: [docs/deployment.md](docs/deployment.md).

Tre ambienti: **DEV** (`docker compose`, `config/dev.env`), **TEST** (`kubectl apply -k k8s/test`, `k8s/base/configmap.yaml`) e **PROD** (`kubectl apply -k k8s/prod`, `k8s/prod/configmap-patch.yaml`, database senza dati di prova).

## Struttura del repository

```
.
├── README.md                questo file
├── CONTRIBUTING.md          branch, commit, review e Definition of Done
├── backend/
│   ├── api/openapi.yaml     contratto delle API
│   ├── cmd/utenti/          microservizio utenti (database globale)
│   ├── cmd/spedizioni/      microservizio spedizioni (database delle spedizioni)
│   ├── cmd/server/          i due servizi insieme, per lo sviluppo e i test
│   ├── internal/
│   │   ├── spedizioni/      handler, regole (servizio) e SQL (repository) delle spedizioni
│   │   ├── anagrafiche/     utenti, clienti, sedi, indirizzi
│   │   ├── interno/         contratto tra i due servizi (rotte /interno)
│   │   ├── identita/        chi chiama e con quale ruolo
│   │   ├── idempotenza/     gestione di Idempotency-Key
│   │   ├── httpx/           rotte, JSON, errori
│   │   ├── errori/          tipi di errore del dominio
│   │   ├── database/        connessione a PostgreSQL
│   │   ├── avvio/           configurazione e avvio dei servizi
│   │   └── server/          collegamento dei componenti, smistamento dei percorsi
│   ├── tests/               test di integrazione
│   └── Dockerfile
├── db/
│   ├── globale/             schema e dati di prova del database globale
│   ├── spedizioni/          schema (vincoli, trigger dello storico, RLS) e dati delle spedizioni
│   ├── Dockerfile           immagine di uno dei due database (PARTE=globale|spedizioni)
│   └── docker-compose.yaml
├── docs/
│   ├── requirements.md      requisiti
│   ├── use-cases.md         attori e casi d'uso
│   ├── architecture-v1.md   componenti e diagramma
│   ├── microservizi.md      i due servizi, i due database e il contratto interno
│   ├── deploy-kubernetes.md deploy nel cluster
│   ├── deployment.md        rilascio, rollback, HTTPS e checklist
│   ├── data-model.md        modello dati
│   ├── domande-cliente.md   domande, risposte del cliente e decisioni
│   ├── backlog.md           backlog
│   └── api.md               riassunto delle API
└── k8s/                     manifest Kubernetes: base/ comune, test/ e prod/ per ambiente
```

## Stato del progetto

- **Fatto:** backend delle spedizioni, cioè creazione, modifica, assegnazione, tentativi, cambi di stato, note, elenco con filtri, dettaglio e storico. Login, gestione utenti, configurazioni dell'amministratore, posizione del corriere (RF16). **Sedi per città** (v0.40): ogni sede ha i suoi operatori, corrieri, spedizioni e impostazioni; Row-Level Security nel database; idempotenza salvata nel database; chiavi BIGINT. App Flutter per operatori, corrieri e amministratore. Test unitari e di integrazione. Manifest Kubernetes. **Due microservizi con due database** (utenti/globale e spedizioni), deployati su Kubernetes. **Mappa sede → database** (v0.46): le sedi divise in gruppi, ognuno con il suo database delle spedizioni; l'amministratore vede l'elenco delle spedizioni una sede alla volta.
- **Da fare:**
  - suggerimenti AI (RF18, RF19);
  - area del cliente destinatario nell'app.

## Come contribuire

Branch strategy, convenzioni di commit e Definition of Done sono in [`CONTRIBUTING.md`](./CONTRIBUTING.md): leggetelo prima di aprire il primo branch.
