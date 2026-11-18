# Bug report — 01


| Progetto | Team / cliente | Data | Versione | Autore |
|---|---|---|---|---|
| LastMileDelivery | GLS / Veloce24 S.r.l. | 18/11/2026 | v0.1 | frunz276 |


 Difetto **introdotto di proposito** per l'esercitazione della lezione 8
 (filtro per corriere rimosso dalla query dell'elenco), poi trattato come un bug reale.
## Sintomo


Il corriere B vede nell'elenco e apre nel dettaglio anche le spedizioni assegnate al
corriere A, comprese note interne e indirizzi.


## Passi per riprodurre


l'operatore crea una spedizione e la assegna al corriere 1 (POST /api/spedizioni/{id}/assegnazioni)
GET /api/spedizioni con header X-Id-Corriere: 2
la spedizione del corriere 1 è nella lista; GET /api/spedizioni/{id} come corriere 2 risponde 200 invece di 404
## Isolamento e ipotesi


Unit test (go test ./...) verdi, falliscono solo gli integration test
  TestElencoSpedizioni/corriere_vede_solo_le_sue_non_concluse e
  TestDettaglioSpedizione/corriere_solo_le_sue_aperte: la logica di visibilità in
  elenco.go passa correttamente IDCorriere, il problema è nella query SQL.
Ipotesi: "la query del repository non filtra per il corriere che chiama".
Verifica: in backend/internal/spedizioni/repository.go la condizione era
  a.id_corriere IS NOT NULL invece di a.id_corriere = $N.


## Causa


In backend/internal/spedizioni/repository.go, nella costruzione delle condizioni della
query di elenco e dettaglio, il filtro sul corriere controllava solo che la spedizione
fosse assegnata a qualcuno (a.id_corriere IS NOT NULL), non al corriere richiedente.
Elenco e dettaglio condividono la query, quindi il difetto colpiva entrambi.


## Correzione


Ripristinata la condizione "a.id_corriere = "+arg(*v.IDCorriere) + test di regressione
TestElencoFiltratoPerCorriere.


## Test aggiunto


| Test | Percorso | Verifica |
|---|---|---|
| TestElencoFiltratoPerCorriere | backend/tests/integration-tests/elenco_dettaglio_test.go | il corriere 2 non vede né apre una spedizione assegnata al corriere 1; il corriere 1 la vede |


Verificato: **fallisce** con il difetto presente
(il corriere 2 non deve vedere la spedizione 62 assegnata al corriere 1) e **passa** con la correzione.


## Verifica di regressione


Suite completa eseguita il 18/11/2026: unit test (spedizioni, posizioni, server) e
integration test (tests/integration-tests) tutti passati, incluso il test nuovo.