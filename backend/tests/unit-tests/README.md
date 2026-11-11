# Unit test

Gli unit test **non stanno in questa cartella**: in Go un test può usare le
funzioni non esportate (`decidiStato`, `validaCambio`, ...) solo se sta nella
stessa cartella del package che testa. Per questo restano accanto al codice,
divisi in un file per feature.

| Feature | File |
|---|---|
| Creazione spedizione | `internal/spedizioni/creazione_test.go` |
| Modifica dati e reso | `internal/spedizioni/modifica_dati_test.go` |
| Assegnazione | `internal/spedizioni/assegnazione_test.go` |
| Cambio stato | `internal/spedizioni/cambio_stato_test.go` |
| Tentativi falliti | `internal/spedizioni/tentativi_test.go` |
| Note | `internal/spedizioni/note_test.go` |
| Elenco e dettaglio | `internal/spedizioni/elenco_dettaglio_test.go` |
| Storico | `internal/spedizioni/storico_test.go` |
| Permessi in sola lettura | `internal/spedizioni/permessi_test.go` |
| Posizione corriere | `internal/posizioni/posizione_test.go` |
| Server (401, 404/405, 503) | `internal/server/server_test.go` |

Le funzioni di supporto comuni (`servizioTest`, `inStato`, `tipoErrore`,
l'archivio finto `archivioElenco`, ...) sono in
`internal/spedizioni/helpers_test.go`.

Non usano rete né database. Si lanciano dalla cartella `backend/`:

```
go test ./...
```

Gli integration test (API vera + database reale) sono in
`tests/integration-tests/`, uno per feature con gli stessi nomi di file.
Dalla cartella `backend/`, con il server avviato e `TEST_DATABASE_URL` impostata:

```
go test -tags=integration -count=1 -v ./tests/integration-tests
```

oppure `tests/integration-tests/avvia_test.ps1`.
