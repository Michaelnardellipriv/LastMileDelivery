# Configurazione dell'applicazione

Lezione 6. Lo stesso codice e la stessa immagine girano in ambienti diversi:
cambia solo la configurazione che gli passiamo all'avvio.

> Regola del team: se un valore cambia tra un ambiente e l'altro, non è codice,
> è configurazione. Va in una variabile d'ambiente e va documentato qui.

## 1. Le variabili

Il backend è diviso in due microservizi, `cmd/utenti` (database globale) e
`cmd/spedizioni` (database delle spedizioni); `cmd/server`, per lo sviluppo, li fa girare
nello stesso processo. Leggono le variabili **una volta sola, all'avvio**
(`backend/internal/avvio`). Se le cambiate, il processo va riavviato (in Kubernetes:
`kubectl rollout restart deploy/utenti deploy/spedizioni`).

| Variabile | Chi la legge | Obbligatoria | Default | Cosa fa |
|---|---|---|---|---|
| `DATABASE_URL` | utenti, spedizioni, server | **sì** | — | Il database del servizio (per `cmd/server` il globale), `postgres://utente:password@host:5432/db?sslmode=disable`. Contiene una password: è un **secret**. |
| `DATABASE_SPEDIZIONI_URL` | server | **sì** per `cmd/server` | — | Il database delle spedizioni (gruppo di sedi 0), quando i due servizi girano insieme. È un **secret**. |
| `DATABASE_URL_<n>` | spedizioni | no | — | Il database delle spedizioni del gruppo di sedi `n` (1..9006), es. `DATABASE_URL_1`; `DATABASE_URL` è il gruppo 0. Senza variabili `_<n>` c'è il solo gruppo 0, come un unico database. All'avvio il servizio controlla che ogni database dichiari il suo gruppo (tabella `gruppo_database`): si ferma se il gruppo 0 non risponde o se un URL porta al database di un altro gruppo; un altro gruppo che non risponde non lo ferma (avviso nel log, le sue sedi hanno 503 finché non torna). Mappa sede → database: `docs/microservizi.md`. È un **secret** (chiave in `spedizioni-secret`). |
| `DATABASE_SPEDIZIONI_URL_<n>` | server | no | — | Come `DATABASE_URL_<n>`, per `cmd/server` (es. `DATABASE_SPEDIZIONI_URL_1`). Vuota = ignorata. È un **secret**. |
| `GRUPPO_SPEDIZIONI` | immagine db (spedizioni) | no | vuota = 0 | Gruppo di sedi di un database delle spedizioni nuovo: letta **solo alla prima creazione** del container (`db/gruppo_spedizioni.sh`), fa partire gli id da `n × 10^12 + 1`. Per un gruppo diverso da 0 l'immagine va costruita con `DATI=nessuno`. Cambiarla dopo non fa niente: il gruppo di un database non si cambia. |
| `URL_SERVIZIO_SPEDIZIONI` | utenti | **sì** | — | Dove trovare il servizio spedizioni per le chiamate `/interno`, es. `http://spedizioni-svc:8080`. |
| `URL_SERVIZIO_UTENTI` | spedizioni | **sì** | — | Dove trovare il servizio utenti per le chiamate `/interno`, es. `http://utenti-svc:8080`. |
| `CHIAVE_INTERNA` | utenti, spedizioni | **sì** (casuale con `cmd/server`) | — | Almeno 32 caratteri, **uguale** per i due servizi: senza, le rotte `/interno` rispondono 404. È un **secret**. |
| `JWT_SEGRETO` | utenti, spedizioni | consigliata | casuale | Almeno 32 caratteri, **uguale** per i due servizi: il servizio utenti firma i token, quello spedizioni li verifica. È un **secret**. |
| `DURATA_TOKEN` | utenti | no | `15m` | Durata del token (almeno `1m`). Breve, così un token copiato vale poco; l'app lo rinnova da sola con `POST /api/auth/rinnova`. La revoca (disattivazione, logout…) non aspetta la scadenza: chiude la sessione nel database. |
| `DURATA_SESSIONE` | utenti | no | `12h` | Quanto dura una sessione dal login (non meno di `DURATA_TOKEN`). Oltre, si rifà il login: un token rubato non si allunga all'infinito. |
| `CORS_ORIGIN` | backend | **sì** | — | Origine del frontend che il browser può usare per chiamare l'API, es. `http://localhost:5173`. Più origini separate da virgola, `*` = tutte (solo DEV). Senza percorso né barra finale. |
| `LISTEN_ADDR` | backend | no | `127.0.0.1:8080` | Indirizzo e porta su cui ascolta. Dentro un container **deve** essere `0.0.0.0:8080`. Accetta anche il vecchio nome `HTTP_ADDR`. |
| `LINK_RECUPERO_PASSWORD` | backend | no | `http://localhost:5173/` | Pagina del **frontend** a cui punta il link della mail di recupero password. Il backend vi aggiunge `?token=<token>` (non scriverlo qui): l'app legge il token e mostra "nuova password". In TEST/PROD è l'URL pubblico del frontend. |
| `POSTGRES_USER` | immagine db | sì | — | Utente creato al primo avvio del database. |
| `POSTGRES_PASSWORD` | immagine db | sì | — | Password di quell'utente. È un **secret**. |
| `POSTGRES_DB` | immagine db | sì | — | Nome del database creato al primo avvio. |
| `DB_PORT` | docker compose | no | `5432` | Porta del PC su cui viene pubblicato il database globale. |
| `DB_SPEDIZIONI_PORT` | docker compose | no | `5433` | Porta del PC su cui viene pubblicato il database delle spedizioni. |
| `DB_SPEDIZIONI_1_PORT` | docker compose | no | `5434` | Porta del database delle spedizioni del gruppo 1 (solo con il profilo `gruppo1`). |
| `API_BASE` | app Flutter | no | vuota = solo dati demo | URL del backend, passato con `--dart-define=API_BASE=...` alla compilazione. |

**Parametri dell'amministratore.** Tentativi massimi di ritiro e di consegna,
giorni di visibilità del cliente, intervallo della posizione e motivi **non** sono
variabili d'ambiente: stanno nelle tabelle `impostazioni` e `motivi` del database delle
spedizioni e li modifica l'amministratore con `/api/impostazioni` (vedi `docs/api.md`),
senza riavviare il backend. `MAX_TENTATIVI` e `GIORNI_VISIBILITA_CLIENTE`, se ancora
impostate, vengono ignorate con un avviso nel log. Su un database già esistente va
eseguita una volta `db/migrazione_configurazione_amministratore.sql`.

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
| `config/dev.env.example` | sì | Valori DEV senza password: si copia in `config/dev.env`. |
| `config/dev.env` | **no** (`.gitignore`) | La vostra copia DEV, con le password dei database usa-e-getta sul portatile (dati finti: `db/globale/seed.sql`, `db/spedizioni/seed.sql`). |
| `.env` | **no** (`.gitignore`) | La vostra copia locale compilata, se vi serve. |
| `k8s/base/configmap.yaml` | sì | Valori TEST **non sensibili**: è la ConfigMap `backend-config`, letta dai due microservizi. |
| `k8s/prod/configmap-patch.yaml` | sì | Valori PROD **non sensibili** (dominio, durata dei token): modifica la ConfigMap per PROD. |
| `k8s/esempi/secret-example.yaml` | sì, **solo valori finti** | Esempio dei Secret `db-secret`, `backend-comune-secret`, `utenti-secret` e `spedizioni-secret`. I valori veri li crea solo chi ha accesso al cluster di TEST. |

Le credenziali di TEST e PROD **non stanno mai in un file del repository** e la
password di PROD non deve mai girare su un portatile.

### Credenziali: cosa impedisce di committarle

- **`.gitignore`** esclude:
  - `.env` e `.env.*` (tranne `.env.example`) e `*.secret.env`;
  - chiavi e certificati (`*.key`, `*.pem`, `*.crt`, `*.p12`…), come il certificato TLS di `lastmile.test`;
  - i manifest dei Secret veri (`secret*.yaml`, `*-secret.yaml`; l'esempio in `k8s/esempi/` resta versionato);
  - `kubeconfig*`;
  - in `config/`, ogni file diverso dagli esempi `*.example` (quindi `dev.env`, `test.env`, `prod.env`).

`.gitignore` non ferma un `git add -f`, né una credenziale scritta dentro un file
versionato (per esempio una password vera in `k8s/prod/configmap-patch.yaml`): prima di ogni commit
guardate `git status` e `git diff --cached`. Valori ammessi nei file versionati: vuoti o
segnaposto (`<password>`, `${VAR}`).

Se una credenziale vera è già finita in un commit, toglierla dal file non basta: resta
nella storia di git. Va **cambiata** (password nuova, `JWT_SEGRETO` nuovo).

## 3. Valori per ambiente

| Variabile | DEV (portatile) | TEST (Kubernetes) | PROD |
|---|---|---|---|
| `DATABASE_URL` | `...@localhost:5432/lastmile` con `go run`, `...@db-globale:5432/lastmile` nel compose | Secret `utenti-secret` (host `db-globale-svc`) e `spedizioni-secret` (host `db-spedizioni-svc`) | gli stessi Secret, nel namespace `lastmile-prod`, con password nuove |
| `DATABASE_SPEDIZIONI_URL` | `...@localhost:5433/lastmile` con `go run`, `...@db-spedizioni:5432/lastmile` nel compose | non serve (i servizi sono separati) | non serve |
| `DATABASE_SPEDIZIONI_URL_<n>` | non impostata (solo gruppo 0); per provare un secondo gruppo `..._1=...@localhost:5434/lastmile` con `go run`, `...@db-spedizioni-1:5432/lastmile` nel compose (profilo `gruppo1`) | non serve | non serve |
| `DATABASE_URL_<n>` (spedizioni) | non serve (`cmd/server`) | non impostata; per un gruppo in più chiave `DATABASE_URL_1` in `spedizioni-secret` (host `db-spedizioni-1-svc`, `k8s/esempi/gruppo-spedizioni-1.yaml`) | come TEST, nel namespace `lastmile-prod`, una chiave per ogni gruppo oltre lo 0 |
| `GRUPPO_SPEDIZIONI` | vuota; `1` nel container `db-spedizioni-1` del compose | vuota; `"1"` nel Deployment `db-spedizioni-1` dell'esempio | come TEST, un valore per ogni database di gruppo |
| `URL_SERVIZIO_*` | non servono (`cmd/server`) | `http://utenti-svc:8080`, `http://spedizioni-svc:8080` | uguali (stessi nomi, nel namespace `lastmile-prod`) |
| `CORS_ORIGIN` | `http://localhost:5173` | `https://lastmile.test` | il dominio del cliente (oggi segnaposto `https://lastmile.veloce24.example`) |
| `LISTEN_ADDR` | `127.0.0.1:8080` con `go run`, `0.0.0.0:8080` nel compose | `0.0.0.0:8080` | `0.0.0.0:8080` |
| `IDENTITA_HEADER_SVILUPPO` | `attiva` | `disattiva` | `disattiva` |
| Dati | finti, `db/globale/seed.sql` e `db/spedizioni/seed.sql` | di prova (stesse immagini) | reali: database senza dati di prova (immagini `-prod`), backup ogni notte |

## 4. Come si avvia in ogni ambiente

**DEV, tutto in container** (i due database + il backend di sviluppo):

```bash
cp config/dev.env.example config/dev.env    # la prima volta, poi scegliete le password
docker compose --env-file config/dev.env up --build
curl -H "X-Id-Operatore: 1" http://localhost:8080/api/spedizioni
```

`--env-file` serve al compose per sostituire `${POSTGRES_USER}` ecc. nel file.
Il backend parte solo quando i database rispondono al healthcheck: senza, la
prima connessione fallisce con `connection refused` solo perché il backend è
partito troppo presto.

**DEV, backend con `go run`** (database nel compose):

```bash
docker compose --env-file config/dev.env up -d db-globale db-spedizioni
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

**TEST su Kubernetes** (passi completi in `docs/deploy-kubernetes.md`):

```bash
# una volta: ingress-nginx, file hosts, certificato e Secret (valori veri mai nel repository)
# la ConfigMap backend-config è k8s/base/configmap.yaml
kubectl apply -k k8s/test
```

**PROD su Kubernetes** (passi completi in `docs/deployment.md`, "PROD"):

```bash
# una volta: ingress-nginx, cert-manager, DNS, namespace e Secret di PROD
kubectl apply -k k8s/prod
# una volta: il primo amministratore (database senza dati di prova)
kubectl -n lastmile-prod exec -i deploy/db-globale -- \
  sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f -' < db/globale/primo_amministratore.sql
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
