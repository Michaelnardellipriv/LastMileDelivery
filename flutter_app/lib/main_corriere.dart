// Area riservata collegata al backend (login unico: operatori, corrieri, ...).
//   nel browser:  flutter run -d chrome --web-port 5173 -t lib/main_corriere.dart --dart-define=API_BASE=http://localhost:8080
//   su emulatore Android:  flutter run -t lib/main_corriere.dart --dart-define=API_BASE=http://10.0.2.2:8080
// È lo stesso ingresso di main_operatore.dart: la pagina che si apre dipende dal ruolo
// di chi fa il login.
import 'package:flutter/material.dart';

import 'area_riservata.dart';
import 'store.dart' show apiBase;

void main() => runApp(AreaRiservataApp(apiBase: apiBase.isEmpty ? 'http://localhost:8080' : apiBase));
