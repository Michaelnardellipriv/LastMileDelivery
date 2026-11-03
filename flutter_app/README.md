# Veloce24 – app Flutter

Porta Flutter del front end, basata su `db/schema_LastMileDb.sql` e `backend/openapi.yaml`.

    flutter create . --project-name veloce24_app   # genera android/ios/web/ecc. (una volta)
    flutter pub get
    flutter run                                     # demo con dati locali
    flutter run --dart-define=API_BASE=http://localhost:8080   # usa POST /api/spedizioni del backend Go

Emulatore Android: usa `http://10.0.2.2:8080`. Per il web serve CORS sul backend Go.
Struttura: `lib/store.dart` (modello, regole, API), `lib/main.dart` (UI Material 3).
Nel backend esiste solo `POST /api/spedizioni`: le altre operazioni usano lo store in memoria.
