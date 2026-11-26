# Security review — LastMileDelivery

Lezione 10. Analisi difensiva dell'architettura e della configurazione: cosa è esposto,
chi può fare cosa, dove vivono le credenziali, cosa resta nei log. Revisione del
25/11/2026 sul ramo `feature-deploy` (API 0.45.0, servizi 2.3.0), aggiornata per la mappa
sede → database delle spedizioni (API 0.46.0, servizi 2.4.0) e per gli indirizzi nel database
del gruppo della sede (API 0.47.0, servizi 2.5.0).

Confini di fiducia e componenti: [architecture-v1.md](architecture-v1.md#confini-di-fiducia).

## 1. Superficie d'attacco

Tutto ciò che arriva da fuori passa da **un solo punto**, l'Ingress (`https://lastmile.test`
in TEST, il dominio del cliente in PROD), e da lì solo verso tre Service:

| Ingresso | Chi lo raggiunge | Protezione |
| :---- | :---- | :---- |
| `/` frontend (file statici, nginx non root) | chiunque | nessun dato: solo l'app |
| `POST /api/auth/login`, `/recupero-password`, `/reimposta-password` | chiunque (**pubblici di proposito**) | bcrypt; stessa risposta per email inesistente e password sbagliata; recupero limitato a 3 richieste l'ora, token di recupero usa-e-getta di 30 minuti salvato solo come hash |
| `POST /api/auth/rinnova`, `POST /api/auth/esci` | chi ha un token (anche appena scaduto) | fuori dal middleware di identità, ma verificano da sé firma e sessione (il rinnovo anche utente attivo e sede); entro `DURATA_SESSIONE` dal login |
| tutti gli altri `/api/...` | utenti con un token | firma, scadenza 15 min, **sessione aperta nel database a ogni richiesta**, ruolo, sede |
| `/interno/...` (tra i due servizi) | **nessuno da fuori**: non sono nell'Ingress | chiave `CHIAVE_INTERNA`; senza, 404 |
| database | solo i Pod del cluster (ClusterIP) | utente non superutente con privilegi minimi; nel database delle spedizioni anche Row-Level Security sulle sedi |
| `/health`, `/ready` | solo il cluster (probe) | non sono sotto `/api`: l'Ingress non li espone |

Non c'è upload di file. Dipendenze di terze parti: vedi il punto 6.

## 2. Chi può fare cosa (matrice dei permessi)

Ogni riga è un controllo **lato server** (`backend/internal/*/servizio.go`), con 403 se il
ruolo non basta; il frontend nasconde i pulsanti solo per comodità.

| Endpoint | Operatore | Corriere | Cliente | Amministratore |
| :---- | :----: | :----: | :----: | :----: |
| `GET /api/spedizioni` | sua sede | solo le sue | solo quelle a lui destinate (di qualunque sede) | la sede scelta (`id_sede` obbligatorio; sola lettura) |
| `GET /api/spedizioni/{id}` | sua sede | solo le sue | solo quelle a lui destinate | tutte, per id (sola lettura) |
| `POST /api/spedizioni`, `PATCH /api/spedizioni/{id}`, `POST .../assegnazioni` | ✓ sua sede | | | |
| `POST .../tentativi` | | ✓ le sue | | |
| `PATCH .../stato`, `POST .../note` | ✓ sua sede | ✓ le sue | | |
| `GET .../storico` | ✓ sua sede | | | ✓ |
| `GET /api/clienti`, `/api/corrieri` | ✓ | | | ✓ |
| `GET /api/indirizzi` | sua sede | | | la sede scelta (`id_sede` obbligatorio) |
| `POST /api/clienti`, `POST /api/indirizzi` | ✓ | | | |
| `POST /api/corrieri/posizione` | | ✓ (solo se in consegna) | | |
| `GET /api/corrieri/posizioni` | ✓ sua sede | | | |
| `GET /api/sedi`, `/api/sedi/{id}`, `/api/sedi/{id}/impostazioni` | sua sede | sua sede | | ✓ |
| `GET /api/impostazioni/intervallo-posizione` | sua sede | sua sede | | |
| `GET /api/motivi` (solo i motivi attivi, per i menu) | ✓ | ✓ | ✓ | ✓ |
| `POST/PATCH /api/sedi` (il gruppo delle spedizioni solo alla creazione), `PATCH .../impostazioni`, `GET/POST/PATCH /api/impostazioni/motivi` | | | | ✓ |
| `GET/POST /api/utenti` | | | | ✓ |
| `PATCH /api/utenti/{id}` | dati dei clienti, email compresa (no password, no stato) | | | ✓ (mai il ruolo) |
| `POST /api/auth/cambio-password`, `/esci` | ✓ | ✓ | ✓ | ✓ |

La sede di operatori e corrieri si controlla nel codice (ogni query filtra per sede) e, nel
database delle spedizioni, anche con Row-Level Security (`db/spedizioni/schema.sql`). Le
anagrafiche del database globale (sedi, corrieri, operatori) sono filtrate solo
nel codice (`anagrafiche.limitaAllaSede`).

Con più gruppi di sedi (un database delle spedizioni per gruppo, `docs/microservizi.md`) la
separazione è anche fisica: il servizio spedizioni apre per operatori e corrieri solo il
database del gruppo della sede del token, e una spedizione di un altro gruppo non è
nemmeno in quel database (404). Il gruppo di una sede non si cambia (trigger
`fn_gruppo_immutabile`, 400 dall'API) e ogni database dichiara il suo gruppo
(`gruppo_database`, di sola lettura per l'utente del backend): un URL sbagliato ferma il
servizio all'avvio invece di mescolare i dati.

**Revoche con più gruppi.** Un token si controlla in un solo database delle spedizioni:
quello del gruppo della sede del login per operatori e corrieri (la sede del token, salvata
anche in `sessioni.id_sede`), il principale per clienti e amministratori. Il servizio
spedizioni scrive ogni revoca proprio lì, prima di rispondere al servizio utenti; se quel
database non risponde la disattivazione (o il cambio di sede, la password nuova, il logout)
fallisce con 503 e si riprova, quindi un token non resta valido dopo una revoca
confermata. Un gruppo giù blocca solo le revoche dei suoi utenti, non quelle di tutti. Le
sessioni aperte prima della 2.4.0 (senza sede salvata) si revocano in tutti i gruppi.

## 3. Checklist minima (slide 24)

| | Controllo | Esito | Dove |
| :----: | :---- | :---- | :---- |
| ✅ | Ogni endpoint richiede autenticazione, salvo quelli volutamente pubblici | Pubblici solo login, recupero e reimpostazione password, `/health`, `/ready` (non esposti dall'Ingress). Rinnovo e logout sono fuori dal middleware perché accettano un token appena scaduto, ma verificano da sé firma e sessione. Gli header `X-Id-*` di sviluppo sono spenti in TEST e PROD (`IDENTITA_HEADER_SVILUPPO=disattiva`). | `identita/middleware.go`, `k8s/base/configmap.yaml` |
| ✅ | Ogni operazione sensibile controlla il ruolo, lato server | Matrice al punto 2; test di integrazione su 403 per ogni ruolo. Il ruolo non si cambia (400) e un trigger lo impedisce anche nel database. | `*/servizio.go`, `db/globale/schema.sql` (`fn_ruolo_immutabile`) |
| ✅ | Ogni input esterno è validato lato server | JSON con campi sconosciuti rifiutato (400), limite di dimensione del corpo, errori per campo; vincoli e CHECK nel database. Il frontend manda quello che c'è a schermo. | `httpx/json.go`, `*/servizio.go` |
| ✅ | Nessun secret nel codice o nella cronologia | `config/dev.env`, `config/test.env` e `config/prod.env` non sono più versionati (`.gitignore`): `dev.env` si copia da `config/dev.env.example`, i valori di TEST e PROD sono nelle ConfigMap di `k8s/`. `k8s/secret.yaml` è stato tolto (l'esempio con valori finti è `k8s/esempi/secret-example.yaml`). `.gitignore` per i file di credenziali; nessun blocco automatico (punto 5). | `docs/configuration.md` |
| 🟡 | Dipendenze aggiornate e controllate prima del rilascio | Poche dipendenze (Go: `pgx`, `x/crypto`; Flutter: `http`, `flutter_map`, `latlong2`, `web`), lockfile versionati. **Il controllo automatico non è ancora fatto**: vedi il punto 6. | `go.mod`, `pubspec.lock` |
| ✅ | I log tracciano le operazioni significative, senza dati sensibili | Righe `audit` con chi, cosa e su cosa: login (anche falliti), accessi negati, sessioni chiuse, modifiche di utenti (solo i nomi dei campi), spedizioni create, assegnate, cambi di stato. Niente password, token, email. | `internal/audit`, punto 7 |

## 4. Criticità trovate e corrette

| # | Criticità | Rischio | Correzione |
| :---- | :---- | :---- | :---- |
| 1 | **Revoca non immediata.** Il servizio spedizioni si fidava del token fino alla scadenza: un utente a cui l'amministratore negava l'accesso lavorava ancora fino a 15 minuti, e un token rubato restava valido dopo il logout. | accesso dopo la revoca | **Sessioni nel database**: ogni token porta la sua sessione (`sid`), controllata a ogni richiesta su tutti e due i servizi. Logout, disattivazione, cambio di sede e password nuova la chiudono subito; il servizio spedizioni riceve le sessioni chiuse prima del commit (se non risponde, l'operazione fallisce). `POST /api/auth/esci` per il logout. |
| 2 | **Ruolo modificabile nel database.** Un `UPDATE`/`DELETE` sulle tabelle dei ruoli (o un bug futuro nell'API) poteva dare a un corriere i permessi di amministratore. | escalation di privilegi | Trigger `fn_ruolo_immutabile` su clienti, corrieri, operatori e amministratori; l'API risponde 400 con errore sul campo `ruolo`. |
| 3 | **Utente del database con troppi privilegi.** `DELETE` su tutte le tabelle, `UPDATE` sullo storico. | un backend compromesso cancella i dati | `db/privilegi_applicazione.sql`: `DELETE` solo su chiavi di idempotenza e sessioni scadute; storico, note e tentativi in sola aggiunta. Test `TestPrivilegiDelBackend`. |
| 4 | **Password nel log.** Senza SMTP, il testo delle mail (con la password di un nuovo account o il link di recupero) finiva nel log anche in TEST/PROD; un errore di invio scriveva l'email dell'utente. | credenziali e dati personali nei log | Il testo si scrive solo in sviluppo (header `X-Id-*` attivi); nel resto solo l'oggetto. Nessuna email nei log. |
| 5 | **Operazioni non tracciate.** Login falliti, accessi negati e modifiche di utenti non lasciavano traccia. | impossibile ricostruire un incidente | Log di audit (punto 7). |
| 6 | **Token senza scadenza assoluta** (versione 2.2.0, rinnovo scorrevole). | token rubato rinnovabile per sempre | Sessione con fine assoluta (`DURATA_SESSIONE`, 12 ore) nel database; nessun token vale oltre la fine della sessione. |
| 7 | **Login e revoca in contemporanea.** Un login che leggeva l'account un attimo prima della disattivazione apriva una sessione che la revoca non vedeva. | accesso dopo la revoca | La sessione si apre bloccando la riga dell'utente e ricontrollando attivo, password e sede: o nasce dopo la revoca e viene rifiutata, o prima e la revoca la chiude. |
| 8 | **Logout con token scaduto** non chiudeva la sessione: il token copiato restava rinnovabile fino a 12 ore. | token rubato ancora usabile | Il logout accetta un token appena scaduto, come il rinnovo. L'app, ricevendo un 403, verifica con il rinnovo e chiude la sessione se l'account è disattivato (anche l'invio della posizione si ferma). |
| 9 | **Errori SMTP nel log** con il testo del server, che può contenere l'indirizzo del destinatario. | dati personali nei log | Nel log solo il codice SMTP. |

Tutte le correzioni hanno test: unitari (`identita`, `autenticazione`) e di integrazione
(`sicurezza_endpoint_test.go`, `autenticazione_endpoint_test.go`,
`amministrazione_endpoint_test.go`, `sedi_endpoint_test.go`).

## 5. Rischi aperti (non corretti, con il motivo)

| Rischio | Perché resta | Cosa si farà |
| :---- | :---- | :---- |
| Il controllo automatico delle dipendenze (`govulncheck`, `flutter pub outdated`) non è ancora nel processo | lo strumento va scaricato e non è stato possibile farlo in questa revisione | eseguire prima di ogni rilascio (punto 6) e metterlo nella pipeline CI (lezione 11) |
| Nessun controllo automatico blocca una credenziale nel commit: `.gitignore` non ferma `git add -f` né una password scritta in un file versionato | il controllo prima del commit è stato tolto dal team | controllo dei segreti nella pipeline CI (lezione 11); fino ad allora `git diff --cached` prima di ogni commit |
| L'operatore può cambiare l'email di un cliente che usa l'app: mettendoci la sua e chiedendo il recupero della password potrebbe entrare al posto del cliente | scelta del cliente: l'operatore gestisce tutti i dati dei clienti tranne la password | a ogni cambio di email i link di recupero già mandati scadono, e il cambio resta nel log di audit (`utente_modificato`, `campi=email`, con chi l'ha fatto); si può aggiungere un avviso al vecchio indirizzo |
| La password di un nuovo account viaggia per mail | è il flusso chiesto dal cliente per i destinatari | sostituirla con un link "imposta la tua password" (come il recupero) |
| Nessun limite ai tentativi di login | bcrypt rallenta, ma non blocca un attacco distribuito | limite per IP sull'Ingress (`nginx.ingress.kubernetes.io/limit-rps`) e blocco temporaneo dopo N tentativi falliti |
| Certificato auto-firmato in TEST | è un ambiente di laboratorio | in PROD Let's Encrypt (già nell'overlay `k8s/prod`) |
| Le chiamate `/interno` tra i servizi sono in HTTP dentro il cluster | il traffico non esce dal cluster; la chiave le autentica | NetworkPolicy che permetta solo ai Pod dei servizi di parlarsi (lezione 11) |

## 6. Dipendenze

Da eseguire prima di ogni rilascio (checklist di `docs/deployment.md`):

```bash
cd backend && go run golang.org/x/vuln/cmd/govulncheck@latest ./...   # vulnerabilità note nel codice Go usato
cd flutter_app && flutter pub outdated                                  # versioni vecchie delle librerie Flutter
```

Immagini: `golang:1.26-alpine` → finale `distroless/static:nonroot` (niente shell, utente
non root); `postgres:16`; `nginx-unprivileged:1.27-alpine`. Si aggiornano cambiando il tag.

## 7. Log di audit

Formato: `audit utente=<ruolo:id> azione=<azione> <campo>=<valore> ...`, una riga per
evento, valori senza spazi.

```
audit utente=- azione=login_fallito id_utente=13 motivo=password
audit utente=operatore:5 azione=login
audit utente=amministratore:3 azione=utente_modificato id_utente=32 ruolo=corriere campi=attivo attivo=false
audit utente=amministratore:3 azione=sessioni_chiuse id_utente=32 numero=1 motivo=utente_disattivato
audit utente=corriere:15 azione=accesso_rifiutato percorso=/api/spedizioni motivo=account_disattivato...
audit utente=amministratore:3 azione=cambio_ruolo_rifiutato id_utente=21
audit utente=corriere:10 azione=spedizione_stato spedizione=42 a=ritirata
audit utente=cliente:7 azione=accesso_negato metodo=POST percorso=/api/spedizioni
```

Azioni: `login`, `login_fallito`, `logout`, `rinnovo_rifiutato`, `accesso_rifiutato`
(sessione chiusa o utente disattivato), `accesso_negato` (403), `sessioni_chiuse`,
`password_cambiata`, `password_reimpostata`, `utente_modificato`, `cambio_ruolo_rifiutato`,
`spedizione_creata`, `spedizione_modificata`, `spedizione_assegnata`, `spedizione_stato`,
`spedizione_tentativo`, `spedizione_nota`. Lo storico delle spedizioni resta comunque nel
database (`cambiamenti_spedizioni`, in sola aggiunta). Mai: password, token, segreti, email.

```bash
kubectl logs deploy/utenti | grep 'audit '        # in Kubernetes
```

## 8. Review incrociata

Da compilare dal team che fa la review incrociata (output della lezione): per ogni voce
della checklist del punto 3, conferma o criticità trovata, e cosa è stato corretto.

| Voce | Esito del revisore | Criticità | Corretta? |
| :---- | :---- | :---- | :---- |
| | | | |
