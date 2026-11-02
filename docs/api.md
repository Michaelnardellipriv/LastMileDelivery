# API — LastMileDelivery / Veloce24

| | |
|---|---|
| **Team** | GLS |
| **Versione** | v0.2 |
| **Base URL (sviluppo)** | `http://localhost:8080` |
| **Contratto formale** | `openapi.yaml` |

Se questo documento e `openapi.yaml` divergono, vale il contratto: aggiornate entrambi nello stesso commit.

## Convenzioni

- Formato dei body: JSON (`Content-Type: application/json`).
- Date e orari: stringhe ISO 8601, es. `2026-10-21T10:30:00+02:00`.
- Gli errori tornano come **testo semplice** (`text/plain`) con lo status code che indica l'esito.

### Autenticazione (temporanea)

Finché non c'è il login vero (lezione 10), gli endpoint protetti identificano l'utente dall'header:

```
X-Utente-Email: corriere@veloce24.test
```

Il ruolo (cliente, corriere, operatore, amministratore) viene ricavato dal database. Questo meccanismo sarà sostituito da token o sessione senza cambiare i contratti qui sotto.

### Status code usati

| Codice | Significato |
|---|---|
| 200 | OK |
| 201 | Risorsa creata |
| 400 | Dati o parametri non validi |
| 401 | Non autenticato o utente sconosciuto |
| 409 | Conflitto (es. codice ordine già usato) |
| 500 | Errore interno del server |

---

## GET /api/spedizioni

Elenca le spedizioni visibili all'utente che chiama. Il contenuto dipende dal ruolo.

**Autenticazione:** richiesta (`X-Utente-Email`).

### Cosa vede ciascun ruolo

| Ruolo | Spedizioni restituite | Note | `id_corriere` |
|---|---|---|---|
| Corriere | solo quelle assegnate a lui, negli stati `ritirata` e `in consegna` | tutte | omesso |
| Operatore | tutte, in tutti gli stati | tutte | presente |
| Amministratore | tutte, in tutti gli stati | tutte | presente |
| Cliente destinatario | solo quelle di cui è destinatario | solo non interne | omesso |

Risultati ordinati dalla più recente. Il filtro `stato` si applica **dentro** ciò che il ruolo può già vedere: un corriere che chiede `stato=consegnata` riceve una lista vuota.

### Parametri di query (tutti opzionali)

| Nome | Tipo | Default | Descrizione |
|---|---|---|---|
| `stato` | stringa | — | uno tra `creata`, `assegnata`, `ritirata`, `in consegna`, `consegnata`, `non consegnata`, `persa`, `annullata` |
| `page` | intero ≥ 1 | 1 | numero di pagina |
| `pageSize` | intero 1–100 | 20 | elementi per pagina |

### Risposta 200

```json
{
  "dati": [
    {
      "id": 2,
      "cod_ordine": "ORD-2",
      "stato": "ritirata",
      "is_reso": false,
      "creata_il": "2026-10-27T21:00:00Z",
      "dataora_richiesta_spedizione": "2026-10-27T21:00:00Z",
      "indirizzo_ritiro": { "via": "Via Roma", "civico": "1" },
      "indirizzo_spedizione": { "via": "Via Verdi", "civico": "10" },
      "id_corriere": 1,
      "note": [
        {
          "id": 1,
          "nota": "nota INTERNA",
          "is_interna": true,
          "motivo": null,
          "creata_il": "2026-10-27T21:01:00Z"
        }
      ]
    }
  ]
}
```

Se non ci sono spedizioni visibili, `dati` è una lista vuota (`[]`).

### Errori

| Codice | Quando |
|---|---|
| 400 | `stato` non valido, `page` < 1, `pageSize` fuori da 1–100 |
| 401 | header `X-Utente-Email` assente o utente sconosciuto |
| 500 | errore interno |

### Esempi

```
curl -H "X-Utente-Email: operatore@veloce24.test" http://localhost:8080/api/spedizioni
curl -H "X-Utente-Email: corriere@veloce24.test" http://localhost:8080/api/spedizioni
curl -H "X-Utente-Email: destinatario@veloce24.test" http://localhost:8080/api/spedizioni
curl -H "X-Utente-Email: operatore@veloce24.test" "http://localhost:8080/api/spedizioni?stato=ritirata&page=1&pageSize=10"
```

---

## POST /api/spedizioni

Crea una spedizione. Nasce nello stato `creata`. Per ora l'operatore che la crea è un operatore di test fisso (l'autenticazione arriverà più avanti).

### Body (JSON)

Campi sconosciuti vengono rifiutati.

| Campo | Tipo | Obbligatorio | Vincoli |
|---|---|---|---|
| `cod_ordine` | stringa | sì | 1–50 caratteri, non solo spazi, univoco |
| `is_reso` | booleano | no | default `false` |
| `id_cliente_mittente` | intero | sì | ≥ 1, cliente esistente |
| `id_cliente_destinatario` | intero | sì | ≥ 1, cliente esistente |
| `dataora_richiesta_spedizione` | data-ora ISO 8601 | sì | — |
| `id_indirizzo_ritiro` | intero | sì | ≥ 1, indirizzo esistente |
| `id_indirizzo_spedizione` | intero | sì | ≥ 1, indirizzo esistente |

Esempio:

```json
{
  "cod_ordine": "ORD-API-003",
  "is_reso": false,
  "id_cliente_mittente": 1,
  "id_cliente_destinatario": 2,
  "dataora_richiesta_spedizione": "2026-10-21T10:30:00+02:00",
  "id_indirizzo_ritiro": 1,
  "id_indirizzo_spedizione": 2
}
```

### Risposta 201

```json
{ "id": 5 }
```

### Errori

| Codice | Quando |
|---|---|
| 400 | JSON non valido, campo mancante o non valido, cliente o indirizzo inesistente |
| 409 | `cod_ordine` già utilizzato |
| 500 | errore interno |

Esempio:

```
curl -X POST http://localhost:8080/api/spedizioni \
  -H "Content-Type: application/json" \
  -d '{"cod_ordine":"ORD-API-003","id_cliente_mittente":1,"id_cliente_destinatario":2,"dataora_richiesta_spedizione":"2026-10-21T10:30:00+02:00","id_indirizzo_ritiro":1,"id_indirizzo_spedizione":2}'
```

---

## Stati della spedizione

`creata` · `assegnata` · `ritirata` · `in consegna` · `consegnata` · `non consegnata` · `persa` · `annullata`

Una spedizione si può annullare solo se è in stato `creata` o `assegnata` (regola applicata dal database).

## Non ancora implementato

- Autenticazione vera (oggi header `X-Utente-Email`).
- Cambio di stato (`PATCH`), assegnazione ai corrieri, note, annullamento.
- Regola RNF6: il cliente non vede le spedizioni consegnate da più di 30 giorni.
