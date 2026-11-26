# Deployment: rilascio e rollback

Lezione 9. Come si pubblica LastMileDelivery nell'ambiente di TEST, come si aggiorna e
come si torna indietro. Chiunque nel team deve poterlo fare seguendo questa pagina, senza
chiedere a chi l'ha fatto l'ultima volta.

Come è fatto il cluster (oggetti, immagini, diagnosi): [deploy-kubernetes.md](deploy-kubernetes.md).
Perché i servizi sono due: [microservizi.md](microservizi.md).

## Dal browser al Pod

```
browser ── https://lastmile.test ──► DNS (file hosts) ──► Ingress controller (ingress-nginx)
                                                             │  termina il TLS (Secret lastmile-tls)
                                                             │  http:// → https://
                                                             │  instrada per path:
   /api/spedizioni, /api/corrieri/posizion*,                 ├──► spedizioni-svc ──► Pod spedizioni ──► db-spedizioni (+ db-spedizioni-<n>, un database per gruppo di sedi)
   /api/impostazioni, /api/motivi, /api/sedi/{id}/impostazioni
   /api (login, rinnovo, utenti, clienti, sedi, indirizzi)   ├──► utenti-svc     ──► Pod utenti     ──► db-globale
   /                                                         └──► frontend-svc   ──► Pod frontend (nginx, Flutter web)
```

- **Un solo punto d'ingresso.** Il cliente conosce solo `https://lastmile.test`: porte,
  Service e Pod restano dentro il cluster. Il reverse proxy è l'Ingress controller; le sue
  regole sono `k8s/base/ingress.yaml`.
- **Path relativi.** L'app Flutter chiama l'API sullo stesso indirizzo della pagina
  (`/api/...`), senza indirizzi assoluti: se cambia il nome, non si ricompila niente.
- **TLS terminato sull'Ingress.** Dopo l'Ingress il traffico verso i Service resta HTTP,
  dentro il cluster. Le rotte `/interno` tra i due servizi non sono nell'Ingress e
  rispondono solo con `CHIAVE_INTERNA`.

## Una volta sola, per preparare l'ambiente

Chi lo fa: chi ha accesso al cluster e il permesso di amministratore sul PC di prova.

1. **Ingress controller.** Senza controller le regole dell'Ingress non fanno nulla (su
   MicroK8s: `microk8s enable ingress`).

   ```bash
   kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.12.1/deploy/static/provider/cloud/deploy.yaml
   kubectl -n ingress-nginx get pods        # controller Running
   ```

2. **Nome host.** In laboratorio non c'è un DNS pubblico: lo si simula con il file hosts
   (su Windows `C:\Windows\System32\drivers\etc\hosts`, aperto come amministratore).
   Docker Desktop espone il controller su `localhost`; su MicroK8s e minikube si usa
   l'indirizzo del nodo.

   ```
   127.0.0.1  lastmile.test
   ```

3. **Certificato.** In TEST è auto-firmato: funziona, ma il browser avvisa (una volta si
   accetta l'eccezione). In produzione serve un certificato di una CA riconosciuta (es.
   Let's Encrypt con cert-manager), con lo stesso nome del Secret. Il certificato è legato
   all'hostname e scade dopo un anno: va rinnovato prima, non dopo.

   ```bash
   openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
     -keyout lastmile.key -out lastmile.crt \
     -subj "/CN=lastmile.test" -addext "subjectAltName=DNS:lastmile.test"
   kubectl create secret tls lastmile-tls --cert=lastmile.crt --key=lastmile.key
   # la chiave non va nel repository: si cancella dal PC dopo averla caricata
   ```

4. **Secret dell'applicazione** (valori veri, mai nel repository): i comandi sono in
   `k8s/esempi/secret-example.yaml` (`db-secret`, `backend-comune-secret`,
   `utenti-secret`, `spedizioni-secret`).

5. **Primo apply e verifica**: `kubectl apply -k k8s/test`, poi la parte "Dopo" della
   checklist.

## Rilascio di una versione nuova

Si rilascia **un componente alla volta**, ognuno con il suo tag (mai `latest`). Esempio:
servizio spedizioni dalla 2.2.0 alla 2.3.0.

```bash
# 1. build: l'artefatto ha una versione
docker build --build-arg SERVIZIO=spedizioni -t localhost:32000/lastmile/spedizioni:2.3.0 backend

# 2. pubblica nel cluster
docker push localhost:32000/lastmile/spedizioni:2.3.0                  # MicroK8s (registry)
docker save localhost:32000/lastmile/spedizioni:2.3.0 | \
  docker exec -i desktop-control-plane ctr -n k8s.io images import -   # Docker Desktop

# 3. applica: si cambia il tag nel manifest (il file resta la fonte di verità) e si annota il perché
#    k8s/base/deployment-spedizioni.yaml: image: localhost:32000/lastmile/spedizioni:2.3.0
kubectl apply -f k8s/base/deployment-spedizioni.yaml
kubectl annotate deploy/spedizioni kubernetes.io/change-cause="spedizioni 2.3.0: <cosa cambia>" --overwrite
kubectl rollout status deployment/spedizioni      # aspetta che tutti i Pod nuovi siano pronti

# 4. verifica (vedi "Dopo" nella checklist)
```

Il Deployment sostituisce i Pod **uno alla volta** e toglie un Pod vecchio solo quando il
nuovo risponde a `/ready`: se la versione nuova non parte, quelle vecchie continuano a
servire.

**Ordine quando cambiano più parti:**
1. migrazioni del database (se ci sono, vedi `deploy-kubernetes.md`), su ogni database
   delle spedizioni se ci sono più gruppi di sedi;
2. il servizio che fornisce una rotta nuova (es. utenti);
3. chi la usa (spedizioni, poi frontend).

Il contratto `/interno` tra i due servizi cambia solo in modo compatibile.

## Rollback

Se la verifica dopo il rilascio non passa **si torna alla versione precedente**, non si
corregge in produzione:

```bash
kubectl rollout undo deployment/spedizioni        # torna alla revisione precedente
kubectl rollout status deployment/spedizioni
kubectl rollout history deployment/spedizioni     # revisioni, con la causa annotata
```

Poi si rimette nel manifest il tag della versione buona (il rollback cambia il Deployment
nel cluster ma non il file: al prossimo `apply` tornerebbe la versione rotta). Un rollback
di un servizio non tocca il database: se il rilascio aveva una migrazione, la versione
precedente deve funzionare anche con lo schema nuovo (per questo le migrazioni aggiungono
colonne con un valore predefinito e non ne tolgono).

### Prova fatta il 25/11/2026 (Docker Desktop)

Rilascio sbagliato simulato del servizio spedizioni, con un'immagine che non esiste:

```
$ kubectl set image deployment/spedizioni spedizioni=localhost:32000/lastmile/spedizioni:2.3.0
$ kubectl rollout status deployment/spedizioni --timeout=40s
Waiting for deployment "spedizioni" rollout to finish: 1 out of 2 new replicas have been updated...
error: timed out waiting for the condition
$ kubectl get pods -l app=spedizioni
spedizioni-549659867c-gth4v   1/1     Running            0   84s
spedizioni-549659867c-nrvx6   1/1     Running            0   71s
spedizioni-6f48868bd8-prkzf   0/1     ImagePullBackOff   0   42s
$ curl http://<spedizioni-svc>/ready                    → 200: le repliche vecchie servono ancora
$ kubectl rollout undo deployment/spedizioni
deployment "spedizioni" successfully rolled out          → immagine in esecuzione: spedizioni:2.2.0
$ kubectl rollout history deployment/spedizioni
5   spedizioni 2.3.0: rilascio di prova (immagine inesistente)
6   spedizioni 2.2.0: rinnovo dei token e limite di sessione
```

### Verifica dall'esterno fatta il 25/11/2026 (Docker Desktop, ingress-nginx 1.12.1)

Con il controller installato e `127.0.0.1 lastmile.test` nel file hosts:

| Prova | Risultato |
| :---- | :---- |
| `https://lastmile.test/` | 200, la pagina dell'app (frontend) |
| `http://lastmile.test/` | 308 → `https://lastmile.test/` |
| certificato presentato | `CN=lastmile.test`, scade il 25/11/2027 |
| `/api/motivi` senza token | 401 JSON del backend (non la pagina 404 di nginx) |
| con token: `/api/spedizioni`, `/api/motivi`, `/api/corrieri/posizioni`, `/api/impostazioni/intervallo-posizione`, `/api/sedi/1/impostazioni` | 200 dal servizio **spedizioni** |
| con token: `/api/sedi`, `/api/sedi/1`, `/api/clienti`, `/api/corrieri`, `/api/utenti`, `/api/indirizzi`, `POST /api/auth/rinnova` | 200 dal servizio **utenti** |
| `/interno/risolvi` da fuori | non arriva ai servizi: risponde il frontend (pagina dell'app, 405 sulla POST) |

Configurazioni sbagliate, provate senza toccare il deploy:

| Prova | Risultato |
| :---- | :---- |
| nome senza riga in hosts (`https://app.lastmile.test`) | il nome non si risolve (curl: errore 6), nessuna risposta |
| host diverso dalla regola (`https://localhost/api/motivi`) | 404 della pagina di nginx; con `Host: lastmile.test` lo stesso indirizzo dà 401 del backend |
| nome senza certificato (SNI `altro.test`) | il controller presenta "Kubernetes Ingress Controller Fake Certificate" |
| client che verifica il certificato (come il browser) | rifiutato (curl: errore 60): auto-firmato, nessuna CA lo riconosce. In TEST si accetta l'eccezione, in produzione serve Let's Encrypt |

## Checklist di rilascio

```markdown
# Checklist di rilascio -- LastMileDelivery (TEST)

## Prima
- [ ] test superati: `go test ./...`, test di integrazione, `flutter test`
- [ ] dipendenze controllate: `govulncheck ./...` (backend), `flutter pub outdated` (vedi docs/security-review.md)
- [ ] immagine con tag di versione nuovo (non `latest`), pubblicata nel cluster
- [ ] tag aggiornato nel manifest in k8s/, nello stesso commit del codice
- [ ] configurazione dell'ambiente pronta: ConfigMap backend-config, Secret, certificato non scaduto
- [ ] migrazione del database pronta e provata, se serve
- [ ] elenco dei database delle spedizioni (un gruppo di sedi ciascuno: `kubectl get deploy -l 'app in (db-spedizioni,db-spedizioni-1)'`, chiavi `DATABASE_URL_<n>` di spedizioni-secret)

## Durante
- [ ] migrazione del database (se c'è), poi un componente alla volta
- [ ] migrazione eseguita su **ogni** gruppo delle spedizioni (db-spedizioni, db-spedizioni-1, ...), non solo sul gruppo 0
- [ ] `kubectl apply -f k8s/<componente>.yaml` e annotazione change-cause
- [ ] osserva `kubectl rollout status deployment/<componente>`

## Dopo
- [ ] `https://lastmile.test` apre il login (certificato per lastmile.test)
- [ ] `http://lastmile.test` rimanda a https
- [ ] login come operatore@veloce24.test: l'elenco spedizioni si carica (servizio spedizioni)
- [ ] l'amministratore vede Utenti e Sedi (servizio utenti) e, scelta una sede, le sue spedizioni
- [ ] con più gruppi: `kubectl logs deploy/spedizioni | grep gruppi` dice il numero giusto di gruppi
- [ ] routing: `curl -k https://lastmile.test/api/motivi` → 401 JSON del backend, non 404 di nginx
- [ ] `kubectl get pods`: tutti READY 1/1, nessun riavvio nuovo
- [ ] `kubectl logs deploy/<componente> --since=5m`: nessun errore nuovo

## Se qualcosa non va
- [ ] `kubectl rollout undo deployment/<componente>`
- [ ] tag della versione buona rimesso nel manifest
- [ ] annotare in docs/deployment.md cosa è successo
```

## Configurazioni sbagliate: cosa si vede

| Cosa manca o è sbagliato | Sintomo | Verifica |
| :---- | :---- | :---- |
| Riga in hosts assente | il browser non trova `lastmile.test` ("non risponde", nessun errore del server) | `ping lastmile.test` non risolve |
| Ingress controller non installato | connessione rifiutata sulla 80/443; `kubectl get ingress` senza ADDRESS | `kubectl -n ingress-nginx get pods` vuoto |
| Host diverso da quello della regola (es. `https://localhost`) | 404 della pagina di nginx, non del backend | `curl -k -H "Host: lastmile.test" https://localhost/api/motivi` → 401 del backend |
| Secret `lastmile-tls` mancante | il controller usa il suo certificato finto ("Kubernetes Ingress Controller Fake Certificate") | `kubectl get secret lastmile-tls` |
| Certificato scaduto o di un altro nome | il browser blocca con un avviso di sicurezza | `openssl x509 -in lastmile.crt -noout -enddate -ext subjectAltName` |
| `use-regex` tolto dall'Ingress | `/api/sedi/1/impostazioni` va al servizio utenti: 404 | `kubectl describe ingress lastmile-ingress` |
| `CORS_ORIGIN` con `http://` invece di `https://` | con l'app sullo stesso host non cambia nulla; un frontend su un altro host viene bloccato dal browser | `kubectl get configmap backend-config -o yaml` |
| Immagine con un tag inesistente | Pod nuovo in `ImagePullBackOff`, i vecchi restano in servizio | `kubectl rollout status` va in timeout → rollback |

## PROD

Stessi manifest di TEST (`k8s/base/`), con l'overlay `k8s/prod/` che cambia solo quello
che in produzione deve essere diverso:

| | TEST (`k8s/test`) | PROD (`k8s/prod`) |
| :---- | :---- | :---- |
| Namespace | `default` | `lastmile-prod` |
| Immagini | `localhost:32000/...`, caricate nel nodo | dal registry di produzione (`images` in `kustomization.yaml`) |
| Database | con i dati di prova (utenti con password nota) | **solo lo schema** (immagini `-prod`, `DATI=nessuno`); il primo amministratore si crea a mano |
| Nome e certificato | `lastmile.test`, auto-firmato | dominio vero, certificato Let's Encrypt rinnovato da cert-manager |
| Configurazione | `k8s/base/configmap.yaml` | `k8s/prod/configmap-patch.yaml` |
| Repliche spedizioni | 2 | 3 |
| Volumi | 1 Gi | 5 Gi globale, 20 Gi spedizioni, 50 Gi backup |
| Backup | no | ogni notte alle 2:30, 14 giorni, tutti i database di `GRUPPI_DB` (`k8s/prod/backup.yaml`) |

### Da decidere prima del primo rilascio

Due valori sono ancora segnaposto con il dominio riservato `.example`:

- il **dominio** del cliente, in `k8s/prod/ingress-patch.yaml`,
  `k8s/prod/configmap-patch.yaml`;
- il **registry** delle immagini, in `k8s/prod/kustomization.yaml`.

E poi:

- l'email per gli avvisi di Let's Encrypt, in `k8s/prod/letsencrypt.yaml`;
- dove copiare i backup fuori dal cluster.

### Una volta sola (chi gestisce il cluster di produzione)

```bash
# 1. ingress-nginx e cert-manager
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.12.1/deploy/static/provider/cloud/deploy.yaml
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.2/cert-manager.yaml
kubectl apply -f k8s/prod/letsencrypt.yaml

# 2. DNS: un record A (o CNAME) del dominio verso l'indirizzo pubblico del controller
kubectl -n ingress-nginx get svc ingress-nginx-controller      # EXTERNAL-IP

# 3. namespace e Secret, con valori nuovi generati qui (mai quelli di TEST, mai in un file)
kubectl create namespace lastmile-prod
#    gli stessi 4 comandi di k8s/esempi/secret-example.yaml, con -n lastmile-prod

# 4. immagini: le stesse versioni provate in TEST; i database nella variante senza dati
docker build --build-arg PARTE=globale    --build-arg DATI=nessuno -t <registry>/lastmile/db-globale:2.4.0-prod    db
docker build --build-arg PARTE=spedizioni --build-arg DATI=nessuno -t <registry>/lastmile/db-spedizioni:2.4.0-prod db
docker tag localhost:32000/lastmile/utenti:2.5.0 <registry>/lastmile/utenti:2.5.0       # e così spedizioni, frontend
docker push <registry>/lastmile/...                                                    # tutte e cinque

# 5. primo rilascio
kubectl apply -k k8s/prod
kubectl -n lastmile-prod get pods,certificate      # Pod READY, certificato Ready=True

# 6. primo amministratore (chiede nome, email, telefono e password a terminale)
kubectl -n lastmile-prod exec -i deploy/db-globale -- \
  sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f -' < db/globale/primo_amministratore.sql
```

Da lì l'amministratore entra dall'app, crea le sedi, gli operatori e i corrieri. I motivi
dei tentativi falliti sono già nello schema.

Si parte con il solo gruppo di sedi 0 (un database delle spedizioni). Un gruppo in più
(database delle spedizioni per una città grande o per un insieme di città piccole) si
aggiunge quando serve, prima di creare le sue sedi: `docs/deploy-kubernetes.md`,
"Aggiungere un gruppo di sedi", con `-n lastmile-prod` e l'immagine del registry.

Rilasci e rollback come in TEST, con `-n lastmile-prod` e la stessa checklist: si cambia il
tag in `k8s/prod/kustomization.yaml` (sezione `images`) dopo averlo provato in TEST, poi
`kubectl apply -k k8s/prod`.

### Backup e ripristino

Il CronJob `backup-database` scrive ogni notte un dump per ogni database della variabile
`GRUPPI_DB` (predefinita `globale spedizioni`): `globale-AAAA-MM-GG.dump`,
`spedizioni-AAAA-MM-GG.dump` e, con più gruppi di sedi, `spedizioni-1-AAAA-MM-GG.dump`, ...
nel volume `backup-pvc`; cancella quelli di più di 14 giorni. **Ogni gruppo nuovo va
aggiunto a `GRUPPI_DB`** in `k8s/prod/backup.yaml` (es. `globale spedizioni spedizioni-1`,
Service `db-spedizioni-1-svc`), altrimenti il suo database non ha backup.

I dump dei database sono indipendenti: si ripristina solo quello che serve. Il gruppo di
un database delle spedizioni è nel dump (tabella `gruppo_database` e sequenze), quindi un
dump va ripristinato solo nel database dello **stesso** gruppo: altrimenti il servizio
spedizioni non parte ("porta al database del gruppo ..."). Ripristinare un database a una
data diversa dagli altri è possibile, ma le spedizioni create dopo il dump in quel gruppo
si perdono, e i riferimenti del database globale (clienti, indirizzi) restano quelli di
oggi.

```bash
# backup subito (es. prima di un rilascio)
kubectl -n lastmile-prod create job backup-adesso --from=cronjob/backup-database
kubectl -n lastmile-prod logs job/backup-adesso

# ripristino di un database (es. spedizioni) da un dump, con il servizio fermo
kubectl -n lastmile-prod scale deploy/spedizioni --replicas=0
kubectl -n lastmile-prod cp <dump> <pod-db-spedizioni>:/tmp/ripristino.dump
kubectl -n lastmile-prod exec deploy/db-spedizioni -- sh -c \
  'pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --clean --if-exists --no-owner /tmp/ripristino.dump'
kubectl -n lastmile-prod scale deploy/spedizioni --replicas=3
# un altro gruppo: stessi comandi con deploy/db-spedizioni-1 e il dump spedizioni-1-<giorno>.dump
# (il servizio spedizioni va fermato lo stesso: è uno solo per tutti i gruppi)
```

Prove fatte il 25/11/2026:
- il comando del CronJob, lanciato come Job sui database di TEST, ha scritto i due dump (40 K e 44 K);
- un dump del database globale "vuoto" di PROD, ripristinato in un database nuovo, ha dato gli stessi utenti e la stessa sede.

### Prova del database di PROD (25/11/2026, in locale)

- Con le immagini `-prod` i database partono senza utenti né spedizioni, con i 15 motivi dello schema.
- `primo_amministratore.sql`:
  - rifiuta una password troppo corta;
  - crea l'amministratore con un hash bcrypt (`$2a$10$`) che il backend accetta;
  - la password non compare nei log del database.
- Con il backend in configurazione di PROD:
  - l'amministratore entra;
  - gli utenti di prova non esistono (401);
  - gli header `X-Id-*` sono rifiutati (401);
  - la prima sede si crea dall'API e le sue impostazioni nascono al primo uso.
