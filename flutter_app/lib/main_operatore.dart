// Pagina web degli operatori logistici, collegata al backend.
//   flutter run -d chrome --web-port 5173 -t lib/main_operatore.dart --dart-define=API_BASE=http://localhost:8080
import 'package:flutter/material.dart';

import 'operatore/app.dart';
import 'store.dart' show apiBase;

void main() => runApp(OperatoreApp(apiBase: apiBase.isEmpty ? 'http://localhost:8080' : apiBase));
