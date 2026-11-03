# Configurazione dell'applicazione

Lezione 6. Lo stesso codice e la stessa immagine girano in ambienti diversi:
cambia solo la configurazione che gli passiamo all'avvio.

> Regola del team: se un valore cambia tra un ambiente e l'altro, non è codice,
> è configurazione. Va in una variabile d'ambiente e va documentato qui.

## 1. Le variabili

Il backend le legge **una volta sola, all'avvio** (`backend/cmd/server/main.go`).
Se le cambiate, il processo va riavviato (in Kubernetes:
`microk8s kubectl rollout restart deploy/backend`).

| Variabile | Chi la legge | Obbligatoria | Default | Cosa fa |
|---|---|---|---|---|
| `DATABASE_URL` | backend | **sì** | — | Connessione a PostgreSQL, `postgres://utente:password@host:5432/db?sslmode=disable`. Contiene una password: è un **secret**. |
| `CORS_ORIGIN` | backend | **sì** | — | Origine del frontend che il browser può usare per chiamare l'API, es. `http://localhost:5173`. Più origini separate da virgola, `*` = tutte (solo DEV). Senza percorso né barra finale. |
| `LISTEN_ADDR` | backend | no | `127.0.0.1:8080` | Indirizzo e porta su cui ascolta. Dentro un container **deve** essere `0.0.0.0:8080`. Accetta anche il vecchio nome `HTTP_ADDR`. |
| `MAX_TENTATIVI` | backend | no | `3` | Tentativi di consegna/ritiro falliti oltre i quali la spedizione non si riassegna. |
| `GIORNI_VISIBILITA_CLIENTE` | backend | no | `30` | Per quanti giorni il cliente vede una spedizione conclusa. |
| `POSTGRES_USER` | immagine db | sì | — | Utente creato al primo avvio del database. |
| `POSTGRES_PASSWORD` | immagine db | sì | — | Password di quell'utente. È un **secret**. |
| `POSTGRES_DB` | immagine db | sì | — | Nome del database creato al primo avvio. |
| `DB_PORT` | docker compose | no | `5432` | Porta del PC su cui viene pubblicato il database. |
| `API_BASE` | app Flutter | no | vuota = solo dati demo | URL del backend, passato con `--dart-define=API_BASE=...` alla compilazione. |

**Fallire presto.** Se `DATABASE_URL` o `CORS_ORIGIN` mancano, o se un valore
non è valido, il backend si ferma subito con un messaggio chiaro invece di
partire e rispondere male alla prima richiesta:

```
$ env -u CORS_ORIGIN ./server
avvio fallito: CORS_ORIGIN non impostata: indicare l'origine del frontend, es. http://localhost:5173

$ CORS_ORIGIN=http://localhost:5173/ ./server
avvio fallito: CORS_ORIGIN: "http://localhost:5173/" non deve avere percorso né barra finale, usare http://localhost:5173
```

`LOG_LEVEL`, che stava nella vecchia ConfigMap, non è letto da nessuna parte
del codice: l'abbiamo tolto per non documentare una variabile che non fa nulla.

## 2. Dove vive ogni valore

| File | Nel repository? | Cosa contiene |
|---|---|---|
| `.env.example` | sì | Tutte le variabili, **senza valori veri**. È l'elenco da copiare. |
| `config/dev.env` | sì | Valori DEV. Contiene una password, ma vale solo per il database usa-e-getta sul portatile con dati finti (`db/seed.sql`). |
| `config/test.env` | sì | Valori TEST **non sensibili**. Diventa la ConfigMap `backend-config`. |
| `.env` | **no** (`.gitignore`) | La vostra copia locale compilata, se vi serve. |
| `k8s/configmap.yaml` | sì | Gli stessi valori di `config/test.env`. Se cambiate uno, cambiate l'altro nello stesso commit. |
| `k8s/secret.yaml` | sì, **solo valori finti** | Esempio del Secret `db-secret`. I valori veri li crea solo chi ha accesso al cluster di TEST. |

Le credenziali di TEST e PROD **non stanno mai in un file del repository** e la
password di PROD non deve mai girare su un portatile.

## 3. Valori per ambiente

| Variabile | DEV (portatile) | TEST (MicroK8s) | PROD |
|---|---|---|---|
| `DATABASE_URL` | `...@localhost:5432/lastmile` con `go run`, `...@db:5432/lastmile` nel compose | dal Secret `db-secret`, host `db-svc` | dal secret di PROD (da definire) |
| `CORS_ORIGIN` | `http://localhost:5173` | `http://lastmile.test` | dominio del cliente (da definire) |
| `LISTEN_ADDR` | `127.0.0.1:8080` con `go run`, `0.0.0.0:8080` nel compose | `0.0.0.0:8080` | `0.0.0.0:8080` |
| `MAX_TENTATIVI` | `3` | `3` | `3` |
| `GIORNI_VISIBILITA_CLIENTE` | `30` | `30` | `30` |
| Dati | finti, `db/seed.sql` | di prova | reali, da proteggere |

## 4. Come si avvia in ogni ambiente

**DEV, tutto in container** (database + backend):

```bash
docker compose --env-file config/dev.env up --build
curl -H "X-Id-Operatore: 1" http://localhost:8080/api/spedizioni
```

`--env-file` serve al compose per sostituire `${POSTGRES_USER}` ecc. nel file.
Il backend parte solo quando il database risponde al healthcheck: senza, la
prima connessione fallisce con `connection refused` solo perché il backend è
partito troppo presto.

**DEV, backend con `go run`** (database nel compose):

```bash
docker compose --env-file config/dev.env up -d db
set -a; source config/dev.env; set +a        # bash / macOS
cd backend && go run ./cmd/server
```

```powershell
# PowerShell
Get-Content config\dev.env | Where-Object { $_ -match '^[A-Z_]+=' } |
  ForEach-Object { $k, $v = $_ -split '=', 2; Set-Item "env:$k" $v }
cd backend; go run ./cmd/server
```

**Frontend Flutter web in DEV**: la porta deve essere quella di `CORS_ORIGIN`.

```bash
cd flutter_app
flutter run -d chrome --web-port 5173 --dart-define=API_BASE=http://localhost:8080
```

**TEST su MicroK8s**:

```bash
microk8s kubectl create configmap backend-config --from-env-file=config/test.env \
  --dry-run=client -o yaml | microk8s kubectl apply -f -
# il Secret con i valori veri lo crea chi ha accesso al cluster, non dal repository
microk8s kubectl apply -f k8s/
```

## 5. Guasto introdotto e diagnosticato

Metodo della lezione 6: sintomo → ipotesi → verifica → causa → correzione,
**una modifica alla volta**.

**Guasto introdotto.** In DEV abbiamo avviato l'app Flutter web senza
`--web-port`: Flutter sceglie una porta a caso (nel nostro caso 51234),
mentre `CORS_ORIGIN` sul backend è `http://localhost:5173`.

**1. Sintomo.** L'elenco spedizioni dell'app resta vuoto con
"Backend non raggiungibile". Con `curl` dal terminale invece l'API risponde
`200` con i dati, quindi backend e database funzionano. Nella console del
browser (F12):

```
Access to fetch at 'http://localhost:8080/api/spedizioni' from origin
'http://localhost:51234' has been blocked by CORS policy: Response to
preflight request doesn't pass access control check: It does not have HTTP ok status.
```

**2. Ipotesi.** La richiesta parte dal browser (origine `localhost:51234`) verso
un host diverso (`localhost:8080`), quindi passa dal controllo CORS. Il
backend non riconosce l'origine del frontend.

**3. Verifica.** Un pezzo alla volta.

Quale valore ha davvero il backend? (log di avvio, oppure dentro il container
`docker compose exec backend env | grep CORS_ORIGIN`)

```
CORS ammesso per: http://localhost:5173
```

Riproduciamo la richiesta di preflight del browser con `curl`, con l'origine
sbagliata e con quella giusta:

```
$ curl -i -X OPTIONS -H "Origin: http://localhost:51234" \
    -H "Access-Control-Request-Method: GET" \
    -H "Access-Control-Request-Headers: x-id-operatore" \
    http://localhost:8080/api/spedizioni
HTTP/1.1 403 Forbidden
{"errore":"origine non ammessa: http://localhost:51234"}

$ curl -i -X OPTIONS -H "Origin: http://localhost:5173" ... (stessa richiesta)
HTTP/1.1 204 No Content
Access-Control-Allow-Headers: Content-Type, Idempotency-Key, X-Id-Operatore, X-Id-Corriere, X-Id-Cliente, X-Id-Amministratore
Access-Control-Allow-Methods: GET, POST, PATCH, OPTIONS
Access-Control-Allow-Origin: http://localhost:5173
```

Il log del backend lo conferma:

```
CORS: origine "http://localhost:51234" non ammessa (CORS_ORIGIN=http://localhost:5173)
```

**4. Causa.** L'origine del frontend (`http://localhost:51234`) non coincide
con `CORS_ORIGIN` (`http://localhost:5173`). Il codice di frontend e backend
non c'entra: è configurazione.

**5. Correzione.** Una sola modifica: riavviato il frontend con
`--web-port 5173` (la porta documentata in `config/dev.env`), senza toccare il
backend. Riverificato lo stesso sintomo: l'elenco mostra le spedizioni e la
console non ha più errori CORS.

In alternativa si poteva aggiungere l'origine a `CORS_ORIGIN` e riavviare il
backend, ma avremmo cambiato la configurazione per inseguire una porta casuale:
fissare la porta è più stabile.

**Cosa ci portiamo via.** Anche `http://localhost:5173/` (con la barra finale)
avrebbe dato lo stesso errore, perché il browser confronta le stringhe esatte:
per questo il backend ora rifiuta all'avvio un `CORS_ORIGIN` con la barra
finale invece di partire e rispondere male.
