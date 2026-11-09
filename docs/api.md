# API — LastMileDelivery / Veloce24

|  |  |
| :---- | :---- |
| **Team** | GLS |
| **Versione** | v0.16 (allineata a `backend/api/openapi.yaml` 0.16.0) |
| **Data** | 03/11/2026 |
| **Base URL (sviluppo)** | `http://localhost:8080` |
| **Contratto formale** | [`backend/api/openapi.yaml`](../backend/api/openapi.yaml) |

Questo documento riassume il contratto per chi sviluppa l'app Flutter o prova le API a mano. Se questo documento e `openapi.yaml` divergono vale `openapi.yaml`: si aggiornano entrambi nello stesso commit.

## Endpoint

| Metodo | Percorso | Chi | Cosa fa |
| :---- | :---- | :---- | :---- |
| `GET` | `/api/spedizioni` | tutti i ruoli | elenco delle spedizioni visibili, con filtri ed eventualmente storico |
| `POST` | `/api/spedizioni` | operatore | crea una spedizione (`creata`) |
| `GET` | `/api/spedizioni/{id}` | tutti i ruoli | dettaglio di una spedizione |
| `PATCH` | `/api/spedizioni/{id}` | operatore | modifica i dati o imposta `is_reso` |
| `POST` | `/api/spedizioni/{id}/assegnazioni` | operatore | assegna o riassegna un corriere |
| `POST` | `/api/spedizioni/{id}/tentativi` | corriere assegnato | registra un ritiro o una consegna non riusciti |
| `PATCH` | `/api/spedizioni/{id}/stato` | operatore, corriere assegnato | cambia stato |
| `POST` | `/api/spedizioni/{id}/note` | operatore, corriere assegnato | aggiunge una nota interna |

`POST` per gli endpoint che creano qualcosa (spedizione, assegnazione, tentativo, nota), `PATCH` per quelli che modificano la spedizione. Non esistono endpoint di eliminazione: le spedizioni e lo storico non si cancellano.

## Convenzioni

- Corpo delle richieste e delle risposte in JSON. Le richieste con corpo devono avere `Content-Type: application/json`, altrimenti 415\.  
- I campi sconosciuti nel corpo vengono rifiutati con 400\.  
- Date e orari in ISO 8601 con fuso orario, es. `2026-11-03T09:00:00+01:00`. Le risposte sono in UTC (`...Z`).  
- I filtri per data (`dal`, `al`) sono date `AAAA-MM-GG` e vanno intese in ora italiana.

### Identificazione (provvisoria)

Finché il login non è integrato, chi chiama si indica con **uno solo** di questi header, con l'id nella tabella del proprio ruolo:

```
X-Id-Operatore: 1
X-Id-Corriere: 1
X-Id-Cliente: 7
X-Id-Amministratore: 1
```

Nessun header, o un id che non esiste: 401\. Due header insieme: 400\. Non è autenticazione, serve solo per lo sviluppo: sarà sostituita dal token del login senza cambiare il resto del contratto.

### Idempotency-Key

Tutti i `POST` e i `PATCH` accettano l'header facoltativo `Idempotency-Key` (da 1 a 255 caratteri visibili), generato dal client, per esempio un UUID per ogni operazione. Se la stessa richiesta viene ripetuta con la stessa chiave, per esempio dopo un timeout di rete, il server non la esegue di nuovo e restituisce la risposta della prima, con l'header `Idempotent-Replayed: true`.

- Stessa chiave con un corpo diverso: 409\.  
- Stessa chiave mentre la prima richiesta è ancora in corso: 409\.  
- Per una nuova operazione serve una chiave nuova.

### Errori

Tutti gli errori hanno lo stesso formato:

```json
{
  "errore": "dati non validi",
  "dettagli": [
    { "campo": "motivo", "messaggio": "obbligatorio per ogni tentativo non riuscito" }
  ]
}
```

`dettagli` c'è solo quando l'errore riguarda campi precisi.

| Codice | Quando |
| :---- | :---- |
| 400 | dati o parametri non validi, id nel percorso non numerico, due identità insieme |
| 401 | nessuna identità, o id inesistente |
| 403 | il ruolo non può fare l'operazione, o la spedizione non è assegnata al corriere che chiama |
| 404 | spedizione inesistente, o non visibile a chi chiama; percorso inesistente |
| 405 | metodo non previsto sul percorso (l'header `Allow` dice quelli ammessi) |
| 409 | operazione non ammessa nello stato attuale della spedizione; `cod_ordine` già usato; `Idempotency-Key` in conflitto |
| 415 | corpo non JSON |
| 503 | database non raggiungibile (con `Retry-After: 5`) |

---

## Stati e ciclo di vita

Stati della spedizione (tipo `stato_spedizione` del database): `creata`, `assegnata`, `ritirata`, `in consegna`, `consegnata`, `non consegnata`, `persa`, `annullata`. Gli ultimi quattro sono finali.

| Da | A | Chi | Endpoint |
| :---- | :---- | :---- | :---- |
| creata | assegnata | operatore | `POST /assegnazioni` |
| assegnata, ritirata | stesso stato, altro corriere | operatore | `POST /assegnazioni` |
| assegnata | assegnata (ritiro fallito) | corriere assegnato | `POST /tentativi` con `tipo: ritiro`  con `motivo` e `nota` |
| assegnata | ritirata | corriere assegnato | `PATCH /stato` |
| creata, assegnata | annullata | operatore | `PATCH /stato` con `motivo` e `nota` |
| ritirata | in consegna | corriere assegnato | `PATCH /stato` |
| in consegna | consegnata | corriere assegnato | `PATCH /stato` |
| in consegna | in consegna (consegna fallita, tentativi 1 e 2\) | corriere assegnato | `POST /tentativi` con `tipo: consegna`  con `motivo` e `nota` |
| in consegna | non consegnata (consegna fallita, 3° tentativo) | corriere assegnato | `POST /tentativi` con `tipo: consegna e` con `motivo` e `nota` |
| in consegna | ritirata | operatore | `PATCH /stato` |
| ritirata, in consegna | persa | operatore | `PATCH /stato` con `motivo` e `nota` |

Ogni altro passaggio viene respinto (409, o 403 se lo chiede il ruolo sbagliato).

**Tentativi** (massimo `MAX_TENTATIVI`, predefinito 3):

- **Ritiro.** La spedizione resta `assegnata`. Dopo il 3° fallimento il corriere non può più registrare tentativi né ritirarla, non si può riassegnare e l'operatore la annulla.  
- **Consegna.** Ogni uscita in consegna ammette un solo tentativo. Dopo i tentativi 1 e 2 l'operatore riporta la spedizione in `ritirata` (ed eventualmente la riassegna), poi il corriere la rimette `in consegna`. Al 3° la spedizione passa da sola a `non consegnata`.

**Reso.** `is_reso` lo imposta l'operatore con `PATCH /api/spedizioni/{id}`, solo su spedizioni `annullata`, `persa` o `non consegnata`. Alla creazione vale sempre `false`.

**Dati.** Si modificano con `PATCH /api/spedizioni/{id}` solo finché la spedizione non è ritirata (stati `creata` e `assegnata`).

## Note e motivi

Le note sono di due tipi:

| Tipo | Come nasce | Cosa vede il cliente |
| :---- | :---- | :---- |
| **pubblica** (`is_interna: false`) | con ogni tentativo (`POST /tentativi`) e con il passaggio in `annullata` o `persa` (`PATCH /stato`). `motivo` e `nota` sono entrambi obbligatori | solo il `motivo` e la data; mai il testo né l'autore |
| **interna** (`is_interna: true`) | solo con `POST /api/spedizioni/{id}/note dove viene messa solo la nota niente motivo` | niente |

Gli altri cambi di stato non accettano né `nota` né `motivo` (400).

I motivi sono valori fissi:

| Elenco | Valori |
| :---- | :---- |
| ritiro | `mittente assente`, `pacco non pronto`, `indirizzo di ritiro errato`, `accesso impossibile`, `altro` |
| consegna | `destinatario assente`, `indirizzo errato`, `rifiutata dal destinatario`, `accesso impossibile`, `altro` |
| annullamento | `richiesta del mittente`, `richiesta del destinatario`, `dati della spedizione errati`, `ritiro non riuscito`, `altro` |
| perdita | `smarrita in deposito`, `smarrita durante il trasporto`, `furto`, `altro` |

## Chi vede cosa

| Ruolo | Spedizioni | Note | `id_corriere` |
| :---- | :---- | :---- | :---- |
| Operatore | tutte, in tutti gli statipiu lo storico | tutte, con testo e autore | sì |
| Amministratore | tutte, in tutti gli stati, più lo storico; non modifica nulla | tutte, con testo e autore | sì |
| Corriere | solo quelle assegnate a lui in `assegnata`, `ritirata`, `in consegna` | tutte, con testo e autore | no |
| Cliente destinatario | solo le sue: quelle in corso sempre, quelle chiuse fino a `GIORNI_VISIBILITA_CLIENTE` giorni (predefinito 30\) dalla chiusura | solo il `motivo` delle note pubbliche | no |
| Cliente mittente | nessuna (il cliente non ha previsto l'accesso del mittente in questa fase) | — | — |

Il dettaglio risponde 404 per una spedizione che chi chiama non può vedere, come se non esistesse.

---

## GET /api/spedizioni

Elenco delle spedizioni visibili a chi chiama, dalla più recente.

| Parametro | Tipo | Descrizione |
| :---- | :---- | :---- |
| `stato` | stringa | uno degli 8 stati |
| `id_corriere` | intero | solo spedizioni di quel corriere; solo operatore e amministratore (altrimenti 403\) |
| `dal`, `al` | data `AAAA-MM-GG` | data di creazione, estremi inclusi; `al` non prima di `dal` |
| `cod_ordine` | stringa 1–50 | riferimento: trova i codici che lo contengono, senza distinguere maiuscole e minuscole |
| `pagina` | intero ≥ 1 | predefinito 1 |
| `per_pagina` | intero 1–100 | predefinito 20 |

I filtri si combinano e si applicano dentro ciò che il ruolo può già vedere: un corriere che chiede `stato=consegnata` riceve una lista vuota.

Risposta 200:

```json
{
  "dati": [
    {
      "id": 42,
      "cod_ordine": "ORD-2026-0001",
      "stato": "in consegna",
      "is_reso": false,
      "creata_il": "2026-11-01T09:00:00Z",
      "dataora_richiesta_spedizione": "2026-11-02T09:00:00Z",
      "id_cliente_mittente": 3,
      "id_cliente_destinatario": 5,
      "indirizzo_ritiro": { "via": "Via Roma", "civico": "10" },
      "indirizzo_spedizione": { "via": "Corso Italia", "civico": "5B" },
      "id_corriere": 1,
      "conclusa_il": null,
      "note": [
        {
          "id": 7,
          "nota": "Citofono rotto, nessuno risponde",
          "is_interna": false,
          "motivo": "destinatario assente",
          "creata_il": "2026-11-02T10:00:00Z",
          "autore": { "ruolo": "corriere", "id": 1 }
        }
      ]
    }
  ],
  "pagina": 1,
  "per_pagina": 20
}
```

`conclusa_il` è la data in cui la spedizione è entrata nello stato finale (`null` se è ancora in corso). Per il cliente mancano `id_corriere`, il testo (`nota`) e l'`autore` delle note.

```
curl -H "X-Id-Operatore: 1" "http://localhost:8080/api/spedizioni?stato=assegnata&dal=2026-11-01&cod_ordine=ORD"
```

## POST /api/spedizioni

Solo l'operatore. La spedizione nasce `creata`, senza corriere, con `is_reso: false`. Può usarla anche un sistema esterno (per esempio un e-commerce), passando il proprio codice d'ordine in `cod_ordine`.

| Campo | Tipo | Vincoli |
| :---- | :---- | :---- |
| `cod_ordine` | stringa | obbligatorio, 1–50 caratteri, lettere, cifre e `. _ / -`, univoco |
| `id_cliente_mittente` | intero | obbligatorio, cliente esistente |
| `id_cliente_destinatario` | intero | obbligatorio, cliente esistente |
| `id_indirizzo_ritiro` | intero | obbligatorio, indirizzo esistente |
| `id_indirizzo_spedizione` | intero | obbligatorio, indirizzo esistente, diverso da quello di ritiro |
| `dataora_richiesta_spedizione` | data-ora | obbligatoria, non nel passato |

```json
{
  "cod_ordine": "ORD-2026-0001",
  "id_cliente_mittente": 3,
  "id_cliente_destinatario": 5,
  "id_indirizzo_ritiro": 10,
  "id_indirizzo_spedizione": 11,
  "dataora_richiesta_spedizione": "2026-11-04T09:00:00+01:00"
}
```

Risposta 201: la spedizione creata (stessi campi della risposta di `PATCH` qui sotto). Errori: 400 (dati non validi, cliente o indirizzo inesistente), 403, 409 (`cod_ordine` già usato).

## GET /api/spedizioni/{id}

Dettaglio di una spedizione, con le stesse regole di visibilità dell'elenco. Contiene gli stessi campi di una voce dell'elenco più:

| Campo | Descrizione |
| :---- | :---- |
| `annullata_il` | data di annullamento, o `null` |
| `tentativi_ritiro_falliti` | numero di ritiri falliti |
| `tentativi_consegna_falliti` | numero di consegne fallite |

Errori: 400 (id non valido), 404 (inesistente o non visibile).

## PATCH /api/spedizioni/{id}

Solo l'operatore. Si mandano solo i campi da cambiare (almeno uno).

| Campo | Quando |
| :---- | :---- |
| `is_reso` | solo se la spedizione è `annullata`, `persa` o `non consegnata` |
| `id_cliente_mittente`, `id_cliente_destinatario`, `id_indirizzo_ritiro`, `id_indirizzo_spedizione`, `dataora_richiesta_spedizione` | solo in `creata` e `assegnata` |

`cod_ordine`, lo stato e il corriere non si cambiano qui. Inviare gli stessi valori non scrive nello storico.

Risposta 200:

```json
{
  "id": 42,
  "cod_ordine": "ORD-2026-0001",
  "stato": "assegnata",
  "is_reso": false,
  "creata_il": "2026-11-01T09:00:00Z",
  "dataora_richiesta_spedizione": "2026-11-04T08:00:00Z",
  "id_cliente_mittente": 3,
  "id_cliente_destinatario": 5,
  "id_indirizzo_ritiro": 10,
  "id_indirizzo_spedizione": 11,
  "id_corriere": 1,
  "annullata_il": null,
  "tentativi_ritiro_falliti": 0,
  "tentativi_consegna_falliti": 0
}
```

Errori: 400, 403, 404, 409 (stato che non ammette la modifica).

## POST /api/spedizioni/{id}/assegnazioni

Solo l'operatore. Corpo: `{ "id_corriere": 2 }`.

- `creata` → `assegnata`;  
- `assegnata` o `ritirata`: cambia il corriere e lo stato resta invariato;  
- negli altri stati, o in `assegnata` dopo 3 ritiri falliti: 409\.

Risposta 201 con la spedizione se il corriere cambia, 200 se è già quello indicato (niente di nuovo nello storico). Errori: 400 (corriere mancante o inesistente), 403, 404, 409\.

## POST /api/spedizioni/{id}/tentativi

Solo il corriere assegnato. `tipo`, `motivo` e `nota` sono obbligatori; il motivo deve essere dell'elenco del tipo di tentativo.

```json
{ "tipo": "consegna", "motivo": "destinatario assente", "nota": "Citofono rotto, nessuno risponde" }
```

- `ritiro`: solo su spedizione `assegnata`, al massimo 3;  
- `consegna`: solo su spedizione `in consegna`, uno per uscita; al 3° la spedizione diventa `non consegnata`.

Risposta 201 con la spedizione aggiornata. Errori: 400, 403, 404, 409 (stato sbagliato, ritiri esauriti, tentativo già registrato in questa uscita).

## PATCH /api/spedizioni/{id}/stato

Operatore o corriere assegnato, secondo la tabella del ciclo di vita.

```json
{ "stato": "ritirata" }
```

```json
{ "stato": "persa", "motivo": "furto", "nota": "Furgone forzato in via Verdi" }
```

`annullata` e `persa` richiedono `motivo` e `nota`; gli altri stati non li accettano. `assegnata` e `non consegnata` non si impostano qui (si usano `/assegnazioni` e `/tentativi`), `creata` mai. Chiedere lo stato in cui la spedizione è già non cambia niente e risponde 200\.

Risposta 200 con la spedizione. Errori: 400, 403, 404, 409\.

## POST /api/spedizioni/{id}/note

Aggiunge una nota interna, senza cambiare la spedizione e senza scrivere nello storico. Corpo: `{ "nota": "Pacco fragile" }` (1–2000 caratteri, unico campo ammesso).

- operatore: su qualsiasi spedizione, in qualsiasi stato;  
- corriere: solo sulle spedizioni assegnate a lui in `ritirata` o `in consegna` (403 se non è sua, 409 negli altri stati).

Risposta 201:

```json
{
  "id": 8,
  "nota": "Pacco fragile",
  "is_interna": true,
  "motivo": null,
  "creata_il": "2026-11-03T11:00:00Z",
  "autore": { "ruolo": "operatore", "id": 1 }
}
```

## Configurazione del server

| Variabile | Predefinito | Significato |
| :---- | :---- | :---- |
| `DATABASE_URL` | — (obbligatoria) | stringa di connessione a PostgreSQL |
| `LISTEN_ADDR` | `127.0.0.1:8080` | indirizzo di ascolto (in Kubernetes `0.0.0.0:8080`) |
| `MAX_TENTATIVI` | 3 | tentativi massimi di ritiro e di consegna |
| `GIORNI_VISIBILITA_CLIENTE` | 30 | giorni in cui il cliente vede una spedizione chiusa |

Il cliente ha chiesto che questi valori, l'intervallo della posizione del corriere e l'elenco dei motivi li imposti l'amministratore: per ora sono variabili d'ambiente o costanti nel codice.

## Non ancora implementato

- Login e gestione utenti (oggi gli header `X-Id-*`).  
- RF16: posizione del corriere, inviata periodicamente da un simulatore solo mentre il corriere ha spedizioni `ritirata` o `in consegna`, e visibile solo agli operatori. Mancano tabella, endpoint e simulatore.  
- Configurazioni gestite dall'amministratore (giorni di consultazione, tentativi, intervallo della posizione, motivi).  
- Elenco dell'operatore filtrato di default sulle spedizioni aperte, come chiesto dal cliente: oggi mostra tutte le spedizioni, e quelle aperte si ottengono col filtro `stato`.  
- Riassunto e categorie delle note con l'AI (RF18, RF19).