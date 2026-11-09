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
Backend:   Go 1.26 (net/http, pgx v5)
Frontend:  Flutter (app per tutti i ruoli, non ancora nel repository)
Database:  PostgreSQL 16
Deploy:    Docker, Kubernetes
```

## Come si esegue

Servono Docker e Go 1.26.

1. Database. Create `db/.env` con `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` e `DB_PORT` (il file non va committato), poi avviate il container. Alla prima creazione carica schema e dati di prova (`db/schema_LastMileDb.sql`, `db/seed.sql`):

   ```bash
   docker compose -f db/docker-compose.yaml up -d --build
   ```

2. Backend. Impostate `DATABASE_URL` con gli stessi dati di `db/.env`, poi avviate il server (in PowerShell: `$env:DATABASE_URL = "..."`):

   ```bash
   export DATABASE_URL="postgres://UTENTE:PASSWORD@localhost:5432/NOME_DB?sslmode=disable"
   ```
   ```bash
   cd backend && go run ./cmd/server
   ```

   Il server ascolta su `http://127.0.0.1:8080`. Variabili facoltative: `LISTEN_ADDR`, `MAX_TENTATIVI` (3), `GIORNI_VISIBILITA_CLIENTE` (30).

3. Prova:

   ```bash
   curl -H "X-Id-Operatore: 1" http://localhost:8080/api/spedizioni
   ```

Gli endpoint sono descritti in [`docs/api.md`](./docs/api.md) e nel contratto [`backend/api/openapi.yaml`](./backend/api/openapi.yaml).

### Test

- Unit test, senza database:

  ```bash
  cd backend && go test ./...
  ```

- Test di integrazione: richiedono il server avviato e un database di prova. Il contenuto del database viene modificato, quindi non usate quello di produzione.

  ```bash
  cd backend && go test -tags=integration -count=1 ./tests
  ```

  Variabili: `TEST_API_URL` (predefinito `http://localhost:8080`) e `TEST_DATABASE_URL` (obbligatoria). Su Windows c'è `backend/tests/avvia_test.ps1`.

### Kubernetes

I manifest sono in `k8s/`: database, backend con 2 repliche, servizi e ingress su `lastmile.test`. L'immagine del backend si costruisce da `backend/Dockerfile`.

## Struttura del repository

```
.
├── README.md                questo file
├── CONTRIBUTING.md          branch, commit, review e Definition of Done
├── backend/
│   ├── api/openapi.yaml     contratto delle API
│   ├── cmd/server/          avvio del server
│   ├── internal/
│   │   ├── spedizioni/      handler, regole (servizio) e SQL (repository) delle spedizioni
│   │   ├── identita/        chi chiama e con quale ruolo
│   │   ├── idempotenza/     gestione di Idempotency-Key
│   │   ├── httpx/           rotte, JSON, errori
│   │   ├── errori/          tipi di errore del dominio
│   │   ├── database/        connessione a PostgreSQL
│   │   └── server/          collegamento dei componenti
│   ├── tests/               test di integrazione
│   └── Dockerfile
├── db/
│   ├── schema_LastMileDb.sql  schema, vincoli e trigger dello storico
│   ├── seed.sql, seed_dev.sql dati di prova
│   ├── Dockerfile
│   └── docker-compose.yaml
├── docs/
│   ├── requirements.md      requisiti
│   ├── use-cases.md         attori e casi d'uso
│   ├── architecture-v1.md   componenti e diagramma
│   ├── data-model.md        modello dati
│   ├── domande-cliente.md   domande, risposte del cliente e decisioni
│   ├── backlog.md           backlog
│   └── api.md               riassunto delle API
└── k8s/                     manifest Kubernetes
```

## Stato del progetto

- **Fatto:** backend delle spedizioni, cioè creazione, modifica, assegnazione, tentativi, cambi di stato, note, elenco con filtri, dettaglio e storico. Schema del database con storico automatico. Test unitari e di integrazione. Manifest Kubernetes.
- **Da fare:**
  - login e gestione utenti;
  - configurazioni dell'amministratore;
  - posizione del corriere (RF16);
  - suggerimenti AI (RF18, RF19);
  - app Flutter.

## Come contribuire

Branch strategy, convenzioni di commit e Definition of Done sono in [`CONTRIBUTING.md`](./CONTRIBUTING.md): leggetelo prima di aprire il primo branch.
