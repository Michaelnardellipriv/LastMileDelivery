// Area riservata collegata al backend (login unico: operatori, corrieri, ...).
//   flutter run -d chrome --web-port 5173 -t lib/main_operatore.dart --dart-define=API_BASE=http://localhost:8080
// È lo stesso ingresso di main_corriere.dart: la pagina che si apre dipende dal ruolo
// di chi fa il login.
import 'package:flutter/material.dart';

import 'area_riservata.dart';
import 'store.dart' show apiBase;

void main() => runApp(AreaRiservataApp(apiBase: apiBase.isEmpty ? 'http://localhost:8080' : apiBase));
