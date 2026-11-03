# LastMileDelivery

|---|---|
| **Cliente** | *veloce24* |
| **Team** | *GLS* |
| **Membri** | *Dennis Pasquali*,  *Francesco Plateroti*, *Michael Nardelli*, *Gianluca Vivaldi*, *Amine Reddad*, *Christian Pellegrini*
| **Data** | *07/10/2026* |
| **Versione** | *v0.1* |

## Cosa fa questo progetto

*- Centralizzare le spedizione
- e il suo ciclo di vita di spedizione *

## Stack

```
Stack: Go (net/http) + pgx
Frontend: Flutter (web e mobile)
Database: PostgreSQL 16
Deploy: Docker, MicroK8s
```

## Come si esegue

Ambiente DEV, database + backend in container (dettagli e altri ambienti in
`docs/configuration.md`):

```
docker compose --env-file config/dev.env up --build
curl -H "X-Id-Operatore: 1" http://localhost:8080/api/spedizioni
```

Frontend collegato al backend:

```
cd flutter_app
flutter run -d chrome --web-port 5173 --dart-define=API_BASE=http://localhost:8080
```

## Struttura del repository

```
.
├── README.md             questo file
├── CONTRIBUTING.md       come contribuire: branch, commit, Definition of Done
├── .env.example          tutte le variabili, senza valori veri
├── docker-compose.yml    ambiente DEV: database + backend
├── config/               dev.env, test.env (configurazione per ambiente)
├── backend/              API in Go (cmd/, internal/, api/openapi.yaml, tests/)
├── db/                   schema SQL, seed, Dockerfile del database
├── flutter_app/          frontend Flutter
├── k8s/                  manifest per MicroK8s (ambiente TEST)
└── docs/                 requisiti, casi d'uso, architettura, modello dati,
                          API, configurazione
```

## Stato del progetto

*in progettazione *

> Esempio: "Lezione 2: repository creato, branch strategy concordata, backlog
> trasferito in issue. Non esiste ancora codice applicativo."

## Come contribuire

Branch strategy, convenzioni di commit e Definition of Done sono in
[`CONTRIBUTING.md`](./CONTRIBUTING.md) — leggetelo prima di aprire il primo branch.
