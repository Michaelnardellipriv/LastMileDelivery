// Token di recupero password che arriva da un link (`LINK_RECUPERO_PASSWORD?token=<token>`).
//
// `tokenRecuperoDaLink` è l'unico punto in cui il token entra nell'app: `App` (main.dart) lo
// osserva e, appena è valorizzato, mostra la pagina "nuova password" al posto di qualunque altra
// schermata (login, area di un ruolo, demo), chiudendo prima le schermate aperte sopra la home.
// Così il link funziona sia con l'app chiusa (web: il token è già nell'URL all'avvio) sia con
// l'app già aperta e un utente già loggato (nuovo URL / link profondo ricevuto mentre gira).
import 'package:flutter/foundation.dart';

/// Estrae `token` da un URI: nella query (`https://app/?token=abc`) oppure, con la strategia di URL
/// a "hash" di Flutter web, dentro il frammento (`https://app/#/?token=abc`). Spazi e valori vuoti → null.
String? tokenRecuperoDaUri(Uri uri) {
  String? t = uri.queryParameters['token'];
  if (t == null || t.trim().isEmpty) {
    final f = uri.fragment;
    final i = f.indexOf('?');
    if (i >= 0) {
      try {
        t = Uri.splitQueryString(f.substring(i + 1))['token'];
      } on FormatException {
        t = null; // frammento con una query non valida: nessun token
      }
    }
  }
  t = t?.trim();
  return (t == null || t.isEmpty) ? null : t;
}

/// Token in attesa di essere usato; null = nessun recupero in corso. Letto una volta all'avvio dall'URL
/// e riempito di nuovo da `App` quando arriva un nuovo link. Chi ha finito (reset riuscito o
/// annullato) lo rimette a null.
final tokenRecuperoDaLink = ValueNotifier<String?>(tokenRecuperoDaUri(Uri.base));
