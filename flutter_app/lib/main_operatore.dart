// Stessa app di main.dart (un solo login, poi la pagina del ruolo), ma se API_BASE non è
// passato parte già collegata al backend locale invece che in modalità demo:
//   flutter run -d chrome --web-port 5173 -t lib/main_operatore.dart --dart-define=API_BASE=http://localhost:8080
// È lo stesso ingresso di main_corriere.dart: la pagina che si apre dipende dal ruolo di chi fa il login.
import 'main.dart' show avvia;

Future<void> main() => avvia(apiBaseDiDefault: 'http://localhost:8080');
