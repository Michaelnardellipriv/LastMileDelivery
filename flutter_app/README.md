# Veloce24 – app Flutter

    flutter create . --project-name veloce24_app   # genera android/ios/web/ecc. (una volta)
    flutter pub get
    flutter test                                    # test del client API (senza backend)
    flutter run                                     # demo con dati locali

## Collegata al backend vero (DEV)

Backend avviato con `config/dev.env` (vedi `docs/configuration.md`), poi:

    flutter run -d chrome --web-port 5173 --dart-define=API_BASE=http://localhost:8080

Nella pagina iniziale compare "Dati reali dal backend": si sceglie ruolo e ID
(con `db/seed.sql`: operatore 1, corriere 1, clienti 1 e 2) e l'elenco arriva da
`GET /api/spedizioni`, con l'header `X-Id-<Ruolo>` che il backend richiede.

- `--web-port 5173` è obbligatoria: deve coincidere con `CORS_ORIGIN` del backend,
  altrimenti il browser blocca le chiamate (guasto descritto in `docs/configuration.md`).
- Emulatore Android: `API_BASE=http://10.0.2.2:8080` (niente CORS fuori dal browser).

## Struttura

- `lib/api.dart` — client HTTP verso il backend (header di identità, errori per status)
- `lib/backend_page.dart` — accesso con ruolo/ID ed elenco spedizioni dal backend
- `lib/store.dart`, `lib/main.dart` — demo con dati in memoria (non ancora collegata)
- `test/api_test.dart` — test del client con un server finto
