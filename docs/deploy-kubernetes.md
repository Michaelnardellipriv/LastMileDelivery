# Deploy su Kubernetes

Lezione 5: tutta l'applicazione gira nel cluster, con configurazione esterna alle immagini
e un solo punto d'ingresso. Il backend è diviso in **due microservizi, ognuno con il suo
database** (perché: `docs/microservizi.md`).

```
                       Ingress lastmile.test
                       /api/spedizioni, /api/corrieri/posizion*,
  browser ──HTTP──►    /api/impostazioni, /api/motivi,           ──► spedizioni-svc ──► Deployment spedizioni (2 Pod) ──SQL──► db-spedizioni-svc ──► db-spedizioni (volume)
                       /api/sedi/{id}/impostazioni                          │  ▲
                                                                   /interno │  │ /interno
                       /api (tutto il resto: login, utenti,                 ▼  │
                       clienti, sedi, indirizzi)                 ──► utenti-svc ────► Deployment utenti (2 Pod)    ──SQL──► db-globale-svc    ──► db-globale (volume)
                       /                                         ──► frontend-svc ──► Deployment frontend (2 Pod, nginx + Flutter web)
```

Il disegno mostra il solo gruppo di sedi 0. Con più gruppi (mappa sede → database,
`docs/microservizi.md`) il Deployment `spedizioni` si collega anche a `db-spedizioni-1-svc`,
`db-spedizioni-2-svc`, ...: un database (con il suo volume) per gruppo, e sempre un solo
servizio spedizioni.

| File | Cosa crea |
| :---- | :---- |
| `k8s/base/deployment-db-globale.yaml` | volume `db-globale-pvc` e Deployment `db-globale` (1 replica, `Recreate`): utenti, ruoli, sedi, indirizzi |
| `k8s/base/deployment-db-spedizioni.yaml` | volume `db-spedizioni-pvc` e Deployment `db-spedizioni`: spedizioni, storico, impostazioni, posizioni |
| `k8s/base/service-db-globale.yaml`, `k8s/base/service-db-spedizioni.yaml` | `db-globale-svc` e `db-spedizioni-svc`, ClusterIP, porta 5432 |
| `k8s/base/deployment-utenti.yaml` | Deployment `utenti`, 2 repliche, aspetta il database globale, probe `/ready` e `/health` |
| `k8s/base/deployment-spedizioni.yaml` | Deployment `spedizioni`, 2 repliche, aspetta il database delle spedizioni, stesse probe |
| `k8s/base/service-utenti.yaml`, `k8s/base/service-spedizioni.yaml` | `utenti-svc` e `spedizioni-svc`, ClusterIP, porta 8080 |
| `k8s/base/deployment-frontend.yaml` | Deployment `frontend`, 2 repliche, nginx non root sulla porta 8080 |
| `k8s/base/service-frontend.yaml` | `frontend-svc`, ClusterIP, porta 80 → 8080 |
| `k8s/base/configmap.yaml` | `backend-config`: valori TEST non sensibili, letta dai due microservizi |
| `k8s/base/ingress.yaml` | host `lastmile.test` in HTTPS (Secret `lastmile-tls`): i percorsi `/api` divisi tra i due servizi, `/` → frontend |
| `k8s/esempi/secret-example.yaml` | **solo esempio**: le chiavi dei quattro Secret, con valori finti |
| `k8s/esempi/gruppo-spedizioni-1.yaml` | **solo esempio**, fuori dagli ambienti: volume, Deployment e Service del database delle spedizioni del gruppo di sedi 1 (vedi "Aggiungere un gruppo di sedi") |

I manifest stanno in `k8s/base/`; ogni ambiente è un overlay kustomize che li prende e
cambia solo quello che deve: `k8s/test/` (questo cluster, `kubectl apply -k k8s/test`) e
`k8s/prod/` (vedi [deployment.md](deployment.md), "PROD"). Il Secret d'esempio sta in
`k8s/esempi/`, fuori da tutti e due: non sovrascrive mai i Secret veri con i valori finti.

I due servizi si parlano solo con le rotte `/interno` (`backend/internal/interno`): non
sono nell'Ingress, quindi da fuori non si raggiungono, e rispondono solo con la chiave
`CHIAVE_INTERNA`. Il servizio spedizioni non tocca mai il database globale, e viceversa.

## Immagini

Cinque immagini, con tag versionato (mai `latest`):

| Immagine | Dockerfile | Note |
| :---- | :---- | :---- |
| `lastmile/db-globale:2.4.0` | `db/Dockerfile`, `PARTE=globale` | PostgreSQL 16 con `db/globale/schema.sql`, dati di prova e utente del backend |
| `lastmile/db-spedizioni:2.4.0` | `db/Dockerfile`, `PARTE=spedizioni` | PostgreSQL 16 con `db/spedizioni/schema.sql`, dati di prova e utente del backend (`2.4.0-prod` con `DATI=nessuno`: solo lo schema, serve anche in TEST per i gruppi di sedi oltre lo 0) |
| `lastmile/utenti:2.5.0` | `backend/Dockerfile`, `SERVIZIO=utenti` | build in due fasi, immagine finale distroless, utente non root |
| `lastmile/spedizioni:2.5.0` | `backend/Dockerfile`, `SERVIZIO=spedizioni` | come sopra: stesso codice, un altro programma (`cmd/spedizioni`) |
| `lastmile/frontend:1.6.0` | `flutter_app/Dockerfile` | build in due fasi: Flutter 3.27 compila per il web, nginx non root serve i file |

Il frontend non ha configurazione: senza `API_BASE`, nel browser l'app chiama l'API sullo
stesso indirizzo della pagina. Dietro l'Ingress `/` e `/api` sono sullo stesso host, quindi
la stessa immagine va bene in TEST e in PROD, e non sa nemmeno che i servizi sono due.

```bash
docker build --build-arg PARTE=globale       -t localhost:32000/lastmile/db-globale:2.4.0    db
docker build --build-arg PARTE=spedizioni    -t localhost:32000/lastmile/db-spedizioni:2.4.0 db
docker build --build-arg SERVIZIO=utenti     -t localhost:32000/lastmile/utenti:2.5.0        backend
docker build --build-arg SERVIZIO=spedizioni -t localhost:32000/lastmile/spedizioni:2.5.0    backend
docker build -t localhost:32000/lastmile/frontend:1.6.0 flutter_app
# solo se si aggiunge un gruppo di sedi (k8s/esempi/gruppo-spedizioni-1.yaml): senza dati di prova
docker build --build-arg PARTE=spedizioni --build-arg DATI=nessuno -t localhost:32000/lastmile/db-spedizioni:2.4.0-prod db
```

## Portare le immagini nel cluster

Kubernetes non usa le immagini di `docker build`: le scarica (pull) dal registry scritto in
`image:`. Senza, il Pod resta in `ImagePullBackOff`.

**MicroK8s (Linux, come in aula)**: c'è il registry integrato su `localhost:32000`.

```bash
microk8s enable dns storage ingress registry
for i in db-globale:2.4.0 db-spedizioni:2.4.0 utenti:2.5.0 spedizioni:2.5.0 frontend:1.6.0; do
  docker push localhost:32000/lastmile/$i
done
```

**Kubernetes di Docker Desktop (Windows)**: il cluster ("kind") gira in un container
nascosto, `desktop-control-plane`, e non vede le immagini di Docker. Si caricano nel nodo
con lo stesso nome usato nei manifest (`imagePullPolicy` predefinita per un tag fisso:
`IfNotPresent`, quindi non prova a scaricarle):

```powershell
# un file, non una pipe: Windows PowerShell 5.1 passa la pipe tra due programmi come testo
# e rovina l'archivio. Percorso relativo: "C:\..." per docker cp sarebbe un container.
# Nel nodo si copia nella radice: /tmp è montata a parte e docker cp non ci scrive.
docker save -o lastmile.tar localhost:32000/lastmile/db-globale:2.4.0 localhost:32000/lastmile/db-spedizioni:2.4.0 `
  localhost:32000/lastmile/utenti:2.5.0 localhost:32000/lastmile/spedizioni:2.5.0 localhost:32000/lastmile/frontend:1.6.0
docker cp lastmile.tar desktop-control-plane:/lastmile.tar
docker exec desktop-control-plane ctr -n k8s.io images import /lastmile.tar
docker exec desktop-control-plane rm /lastmile.tar; Remove-Item lastmile.tar
docker exec desktop-control-plane crictl images | Select-String lastmile
```

Con un gruppo di sedi in più si carica allo stesso modo anche `db-spedizioni:2.4.0-prod`.

**minikube**: `minikube image load localhost:32000/lastmile/<immagine>` per ognuna.

## Ingress controller

L'oggetto Ingress contiene solo le regole: serve il controller che le applica.
Su MicroK8s è l'addon `ingress`; su Docker Desktop e minikube si installa ingress-nginx:

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.12.1/deploy/static/provider/cloud/deploy.yaml
kubectl -n ingress-nginx get pods,svc     # controller Running; il Service è di tipo LoadBalancer
```

(su minikube: `minikube addons enable ingress`).

## Passi comuni

1. **Secret** (valori veri, mai nel repository): i comandi sono in
   `k8s/esempi/secret-example.yaml`. `JWT_SEGRETO` e `CHIAVE_INTERNA` stanno in
   `backend-comune-secret`, letto da tutti e due i servizi: il servizio utenti firma i
   token, quello spedizioni li verifica, quindi il segreto deve essere lo stesso.
2. **Tutto il resto**, in modo dichiarativo:

   ```bash
   kubectl apply -k k8s/test
   kubectl get pods            # 8 Pod Running e READY 1/1: 2 database, 2+2 servizi, 2 frontend
   kubectl get svc
   kubectl get ingress
   ```

3. **Nome host.** Il PC deve risolvere `lastmile.test`: una riga nel file hosts (su
   Windows `C:\Windows\System32\drivers\etc\hosts`, va aperto come amministratore).
   Con Docker Desktop il controller ascolta su `localhost`:

   ```
   127.0.0.1  lastmile.test
   ```

   Su MicroK8s o minikube l'indirizzo è quello del nodo (`kubectl get ingress`, `minikube ip`).
   Se il Service del controller resta senza `EXTERNAL-IP`, basta un port-forward:
   `kubectl -n ingress-nginx port-forward svc/ingress-nginx-controller 8443:443` e poi
   `https://lastmile.test:8443`.

4. **HTTPS**: il certificato sta nel Secret `lastmile-tls` (in TEST auto-firmato: i comandi
   sono in [deployment.md](deployment.md)); l'Ingress termina il TLS e rimanda `http://` a
   `https://`.

5. **Verifica**: `https://lastmile.test` apre il login (utente di prova
   `operatore@veloce24.test`, password `PasswordProva1`); `https://lastmile.test/api/motivi`
   senza token risponde 401 (in TEST gli header `X-Id-*` sono spenti). Procedura di
   rilascio, rollback e checklist: [deployment.md](deployment.md).

## Verificare i due servizi senza Ingress

Con un port-forward per servizio si vede che ognuno fa la sua parte:

```bash
kubectl port-forward svc/utenti-svc 18181:8080 &
kubectl port-forward svc/spedizioni-svc 18182:8080 &

# il login lo fa il servizio utenti
TOKEN=$(curl -s -X POST localhost:18181/api/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"operatore@veloce24.test","password":"PasswordProva1"}' | jq -r .token)

# il servizio spedizioni accetta lo stesso token e completa le spedizioni con nomi,
# sede e indirizzi chiesti al servizio utenti (/interno/risolvi)
curl -s localhost:18182/api/spedizioni -H "Authorization: Bearer $TOKEN"

# le rotte interne senza chiave non esistono
curl -i -X POST localhost:18181/interno/risolvi -d '{}'        # 404
```

## Scalare

I due servizi si scalano separatamente: le spedizioni (corrieri, posizioni GPS) ricevono
molte più richieste del login.

```bash
kubectl scale deployment spedizioni --replicas=4   # poi riportare il numero nel manifest
```

I database invece hanno un solo Pod ciascuno (volume `ReadWriteOnce`): per crescere ancora
si dividono le spedizioni in più database, uno per gruppo di sedi (vedi "Aggiungere un
gruppo di sedi" e `docs/microservizi.md`).

## Aggiungere un gruppo di sedi

Un gruppo di sedi è un database delle spedizioni in più (le città grandi da sole, le
piccole insieme). L'esempio pronto è `k8s/esempi/gruppo-spedizioni-1.yaml`: non fa parte
di `k8s/test` né di `k8s/prod`, si applica a mano quando serve. Passi (in PROD con
`-n lastmile-prod` e l'immagine del registry, vedi l'intestazione del file):

```bash
# 1. immagine del database senza dati di prova (anche in TEST), caricata nel cluster
docker build --build-arg PARTE=spedizioni --build-arg DATI=nessuno \
  -t localhost:32000/lastmile/db-spedizioni:2.4.0-prod db

# 2. volume, database (GRUPPO_SPEDIZIONI=1: id da 10^12 + 1) e Service db-spedizioni-1-svc
kubectl apply -f k8s/esempi/gruppo-spedizioni-1.yaml
kubectl rollout status deploy/db-spedizioni-1

# 3. URL del nuovo database nel Secret del servizio spedizioni (le altre chiavi restano)
kubectl patch secret spedizioni-secret --type merge -p \
  '{"stringData":{"DATABASE_URL_1":"postgres://<APP_DB_USER>:<APP_DB_PASSWORD>@db-spedizioni-1-svc:5432/<POSTGRES_DB>?sslmode=disable"}}'

# 4. il servizio spedizioni legge le variabili all'avvio: riavvio
kubectl rollout restart deploy/spedizioni
kubectl rollout status deploy/spedizioni
kubectl logs deploy/spedizioni | grep gruppi      # database delle spedizioni: 2 gruppi
```

5. L'amministratore crea le sedi del gruppo con `POST /api/sedi` e `"gruppo_spedizioni": 1`
   (prima del passo 4 risponde 400: il gruppo non ha un database). Una sede esistente non
   si sposta.
6. In PROD: `spedizioni-1` in `GRUPPI_DB` di `k8s/prod/backup.yaml`, così il backup
   notturno salva anche il nuovo database.

Se il servizio spedizioni non parte dopo il riavvio, `kubectl logs deploy/spedizioni`:
"l'URL del gruppo 1 porta al database del gruppo 0" vuol dire che `DATABASE_URL_1` punta
al database sbagliato (o che il database è stato creato senza `GRUPPO_SPEDIZIONI`: va
ricreato cancellando il suo volume, finché è vuoto).

## Aggiornare

Si cambia il tag nel manifest e si rifà `apply`: il Deployment sostituisce i Pod uno alla
volta. Se il nuovo Pod non parte: `kubectl rollout undo deploy/spedizioni`. Dopo una
modifica alla ConfigMap: `kubectl rollout restart deploy/utenti deploy/spedizioni`.

I database tengono i dati nei volumi: schema, dati di prova e utente del backend si creano
**solo alla prima** partenza. In TEST, con dati di prova, si riparte da zero cancellando
anche i volumi: `kubectl delete -f k8s/base/deployment-db-globale.yaml -f k8s/base/deployment-db-spedizioni.yaml`
e di nuovo `apply`. Un volume della versione 1 (un solo database, `db:1.1.0`) non si
riusa: i dati vanno divisi tra i due database.

Da `db-globale:2.0.0` a `2.1.0` (rinnovo dei token: data del cambio password) un database
globale già esistente va migrato una volta, prima di aggiornare il servizio utenti:

```bash
kubectl exec -i deploy/db-globale -- sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"' < db/globale/migrazione_sessione.sql
```

Da `2.1.0` a `2.2.0` (sessioni nel database, ruolo non modificabile, privilegi minimi),
prima di aggiornare i due servizi alla 2.3.0, i due database esistenti:

```bash
for db in spedizioni globale; do
  kubectl exec -i deploy/db-$db -- sh -c \
    'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v app="$APP_DB_USER" -f -' < db/$db/migrazione_sessioni.sql
  kubectl exec -i deploy/db-$db -- sh -c \
    'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v app="$APP_DB_USER" -f -' < db/privilegi_applicazione.sql
done
```

Dopo l'aggiornamento i token emessi dalla 2.2.0 non hanno una sessione: tutti rifanno il
login una volta.

Da `2.3.0` a `2.4.0` (mappa sede → database: colonne `sedi.gruppo_spedizioni` e
`sessioni.id_sede`, tabella `gruppo_database`), **prima** di aggiornare le immagini, i due database esistenti, con
l'utente proprietario delle tabelle, poi di nuovo i permessi (`gruppo_database` di sola
lettura per l'utente del backend):

```bash
for db in globale spedizioni; do
  kubectl exec -i deploy/db-$db -- sh -c \
    'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f -' < db/$db/migrazione_gruppi.sql
  kubectl exec -i deploy/db-$db -- sh -c \
    'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v app="$APP_DB_USER" -f -' < db/privilegi_applicazione.sql
done
```

Poi le immagini nuove (`kubectl apply -k k8s/test`, in PROD `-k k8s/prod`). Il database
delle spedizioni esistente diventa il gruppo 0 e i suoi id non cambiano; le sedi
esistenti restano nel gruppo 0. Senza la migrazione il servizio spedizioni 2.4.0 non parte
("lettura di gruppo_database"). Da qui in poi ogni migrazione di `db/spedizioni/` va
eseguita su **ogni** database di gruppo (`deploy/db-spedizioni`, `deploy/db-spedizioni-1`,
...). Nell'app l'amministratore deve scegliere una sede per vedere l'elenco delle
spedizioni.

Da `2.4.0` a `2.5.0` (gli indirizzi di ritiro e consegna passano dal database globale a
quello delle spedizioni del gruppo della loro sede, `docs/microservizi.md`), in due fasi,
per **ogni** database delle spedizioni (qui il gruppo 0, `deploy/db-spedizioni`; per gli
altri `deploy/db-spedizioni-<n>` e `sh db/migra_indirizzi.sh <n> db-spedizioni-<n>`):

```bash
# 1. prima di aggiornare i servizi: tabella, permessi, copia degli indirizzi
kubectl exec -i deploy/db-spedizioni -- sh -c \
  'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f -' < db/spedizioni/migrazione_indirizzi.sql
kubectl exec -i deploy/db-spedizioni -- sh -c \
  'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v app="$APP_DB_USER" -f -' < db/privilegi_applicazione.sql
kubectl exec -i deploy/db-globale -- sh -c \
  'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v app="$APP_DB_USER" -f -' < db/privilegi_applicazione.sql
sh db/migra_indirizzi.sh 0

# 2. immagini nuove (utenti e spedizioni 2.5.0, database 2.4.0)
kubectl apply -k k8s/test

# 3. dopo l'aggiornamento: di nuovo la copia (gli indirizzi creati dai Pod vecchi
#    durante l'aggiornamento), poi le chiavi esterne
sh db/migra_indirizzi.sh 0
kubectl exec -i deploy/db-spedizioni -- sh -c \
  'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f -' < db/spedizioni/migrazione_indirizzi_vincoli.sql
```

La copia si può rieseguire (non duplica). Nel gruppo 0 gli id dei nuovi indirizzi partono
un milione sopra il più alto copiato, così non si scontrano con quelli che i Pod vecchi
creano ancora nel database globale durante l'aggiornamento. La fase 3 si ferma se una
spedizione cita un indirizzo che non c'è, e dice quale. Rollback dei servizi alla 2.4.0:
possibile finché non si è fatta la fase 3 (i servizi 2.4.0 leggono gli indirizzi dal
globale, che restano lì); gli indirizzi creati dalla 2.5.0 però sono solo nel database
delle spedizioni.

## Diagnosi

| Stato | Prima verifica |
| :---- | :---- |
| `ImagePullBackOff` / `ErrImageNeverPull` | l'immagine c'è nel nodo con quel tag? (`docker push` su MicroK8s, `crictl images` su Docker Desktop, `minikube image ls`) |
| `Init:0/1` su un servizio | il suo database non è pronto: `kubectl logs deploy/db-globale` o `deploy/db-spedizioni` |
| `CrashLoopBackOff` su spedizioni dopo un aggiornamento o un gruppo nuovo | `kubectl logs deploy/spedizioni`: "gruppo_database" = database non migrato (`db/spedizioni/migrazione_gruppi.sql`); "porta al database del gruppo" = `DATABASE_URL_<n>` sbagliato (un gruppo che non risponde invece non ferma il servizio: nel log c'è "non risponde" e solo le sue sedi hanno 503; `kubectl get pods -l app=db-spedizioni-1`) |
| 503 solo per le sedi di un gruppo | il database di quel gruppo non risponde (`/ready` guarda solo il gruppo 0) |
| 400 `gruppo_spedizioni` creando una sede | il gruppo non ha un database: manca `DATABASE_URL_<n>` in `spedizioni-secret`, o il servizio spedizioni non è stato riavviato |
| `CrashLoopBackOff` | `kubectl logs deploy/utenti`: di solito una variabile mancante (`DATABASE_URL`, `CHIAVE_INTERNA`, `URL_SERVIZIO_*`) |
| Running ma `READY 0/1` | `/ready` risponde 503: il servizio non raggiunge il suo database |
| 503 "servizio non disponibile" sulle spedizioni | il servizio spedizioni non raggiunge quello utenti: `kubectl get endpoints utenti-svc`, `URL_SERVIZIO_UTENTI`, stessa `CHIAVE_INTERNA` |
| 401 sulle spedizioni con un token appena preso | `JWT_SEGRETO` diverso tra i due servizi |
| 404 su `/api/sedi/1/impostazioni` | l'Ingress non ha `use-regex`: la richiesta va al servizio utenti |
| 503 dall'Ingress | `kubectl get endpoints utenti-svc spedizioni-svc frontend-svc`: lista vuota = selector sbagliato |
