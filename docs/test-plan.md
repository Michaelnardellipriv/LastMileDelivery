# Test plan — LastMileDelivery

| Progetto | Team / cliente | Data | Versione |
|---|---|---|---|
| LastMileDelivery | GLS / Veloce24 S.r.l. | 11/11/2026 | v0.1 |

## Caso d'uso

> UC3–UC5 — Il corriere assegnato aggiorna lo stato della spedizione
> (ritirata → in consegna → consegnata); l'operatore logistico può annullarla
> prima del ritiro o segnarla persa dopo. Il cliente destinatario vede il nuovo stato.

Endpoint: `PATCH /api/spedizioni/{id}/stato` (contratto in `backend/api/openapi.yaml`).

## Matrice requisito → caso di test → esito

| # | Requisito / vincolo | Caso di test | Input / precondizioni | Risultato atteso | Tipo | Test | Esito |
|---|---|---|---|---|---|---|---|
| 1 | il corriere assegnato fa avanzare la spedizione | happy path | corriere1 assegnato, `assegnata → ritirata → in consegna → consegnata` | 200, nuovo stato salvato e scritto nello storico | unit + integration | `TestCambioStato/ritira`, `/in_consegna`, `/consegna` (unit) · `TestCambioStato/ciclo_completo_fino_a_consegnata` (integration) | da eseguire |
| 2 | non si salta uno stato | transizione non valida | `assegnata → consegnata` | 409, stato invariato | unit + integration | `TestCambioStato/salta_stati` (unit) · `TestCambioStato/transizioni_non_ammesse_409` (integration) | da eseguire |
| 3 | solo il corriere assegnato modifica | permesso mancante | corriere2 su spedizione assegnata a corriere1 | 403 | unit + integration | `TestCambioStato/corriere_non_assegnato` (unit) · **da scrivere** (integration) | da eseguire |
| 4 | ogni stato ha il suo ruolo | ruolo sbagliato | il corriere annulla, l'operatore ritira | 403 | unit | `TestCambioStato/corriere_annulla`, `/operatore_ritira` | da eseguire |
| 5 | la risorsa deve esistere | id inesistente | id 999999999 | 404 | integration | `TestCambioStato/spedizione_inesistente_404_e_id_non_valido_400` | da eseguire |
| 6 | dati minimi obbligatori | body malformato | campo `stato` assente | 400 | integration | `TestCambioStato/stati_che_si_impostano_altrove_400` | da eseguire |
| 7 | annullata e persa richiedono motivo e nota | dati mancanti | `annullata` senza `motivo` né `nota` | 400 | unit | `TestCambioStato/annulla_senza_motivo_né_nota`, `TestMotivoObbligatorio` | da eseguire |
| 8 | si annulla solo prima del ritiro | transizione non valida | `ritirata → annullata` | 409 | unit + integration | `TestCambioStato/annulla_dopo_il_ritiro` (unit) · `TestCambioStato/annullamento_solo_prima_del_ritiro` (integration) | da eseguire |
| 9 | una spedizione chiusa non cambia più | stato finale | `consegnata → in consegna` | 409 | unit + integration | `TestCambioStato/stato_finale` (unit) · **da scrivere** (integration) | da eseguire |
| 10 | dopo 3 ritiri falliti non si ritira più | limite tentativi | 3 tentativi di ritiro falliti, poi `ritirata` | 409 | unit + integration | `TestTentativiDiRitiro` (unit) · `TestTentativi/tre_ritiri_falliti_poi_annullata_dall_operatore` (integration) | da eseguire |
| 11 | ripetere lo stesso stato non cambia niente | idempotenza | `ritirata → ritirata` | 200, nessuna nuova riga nello storico | unit + integration | `TestRipetereLoStessoStato` (unit) · `TestCambioStato/ripetere_lo_stesso_stato_non_cambia_niente` (integration) | da eseguire |

Unit test in `backend/internal/spedizioni/cambio_stato_test.go` e `tentativi_test.go`;
integration test in `backend/tests/integration-tests/cambio_stato_test.go` e `tentativi_test.go`.

## Note

- Per una transizione non ammessa l'API risponde **409 (conflitto con lo stato attuale)**,
  non 400 come nell'esempio della slide: 400 è riservato ai dati non validi. La scelta è
  documentata in `openapi.yaml`.
- Una spedizione che il richiedente non può vedere risponde 404 come se non esistesse:
  da tenere presente quando si scrive il test della riga 3 a livello integration.