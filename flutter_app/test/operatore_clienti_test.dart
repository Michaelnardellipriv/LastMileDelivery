// flutter test — sezione Clienti dell'operatore con un backend finto (MockClient):
// l'operatore modifica solo i dati, partono solo i campi cambiati e gli errori del
// backend vanno sotto i campi.
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:veloce24_app/operatore/api.dart';
import 'package:veloce24_app/operatore/clienti.dart';

final _richieste = <http.Request>[];

http.Response _json(Object corpo, int status) =>
    http.Response.bytes(utf8.encode(jsonEncode(corpo)), status, headers: {'content-type': 'application/json; charset=utf-8'});

const _dora = {
  'id': 7,
  'id_utente': 21,
  'nome': 'Dora',
  'cognome': 'Destinataria',
  'email': 'dora@test.it',
  'prefisso': '+39',
  'nr_telefono': '3330000006',
  'attivo': false,
  'destinatario': true,
};

const _nuovo = {
  'id': 8,
  'id_utente': 22,
  'nome': 'Nino',
  'cognome': 'Nuovo',
  'email': 'nino@test.it',
  'prefisso': '+39',
  'nr_telefono': '3330000008',
  'attivo': true,
  'destinatario': false,
};

Future<http.Response> _backend(http.Request req) async {
  _richieste.add(req);
  switch ((req.method, req.url.path)) {
    case ('GET', '/api/clienti'):
      return _json({'dati': [_dora, _nuovo], 'pagina': 1, 'per_pagina': 20, 'totale_elementi': 2, 'totale_pagine': 1}, 200);
    case ('PATCH', '/api/utenti/21'):
      final corpo = jsonDecode(req.body) as Map<String, dynamic>;
      if (corpo['cognome'] == '') {
        return _json({
          'errore': 'dati non validi',
          'dettagli': [
            {'campo': 'cognome', 'messaggio': 'obbligatorio'}
          ]
        }, 400);
      }
      return _json({..._dora, 'ruolo': 'cliente', 'id': 21, 'id_ruolo': 7, 'destinatario': true, ...corpo}, 200);
  }
  return _json({'errore': 'percorso sconosciuto'}, 404);
}

void main() {
  testWidgets('clienti: l\'operatore modifica solo i dati, il backend valida', (tester) async {
    tester.view.physicalSize = const Size(1200, 1400);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    final api = ApiOperatore(base: 'http://backend:8080', client: MockClient(_backend));
    await tester.pumpWidget(MaterialApp(home: Scaffold(body: ClientiPage(api: api))));
    await tester.pumpAndSettle();

    expect(find.text('Dora Destinataria'), findsOneWidget);
    expect(find.text('disattivato'), findsOneWidget);
    // chi ha ricevuto una spedizione è destinatario, gli altri (anche appena creati) mittenti
    expect(find.text('Destinatario'), findsOneWidget);
    expect(find.text('Mittente'), findsOneWidget);
    expect(find.text('Disattiva'), findsNothing); // la disattivazione spetta all'amministratore

    await tester.tap(find.byTooltip('Modifica dati').first);
    await tester.pumpAndSettle();
    expect(find.text('Reimposta la password'), findsNothing);

    // l'operatore cambia anche l'email di chi usa l'app (non la password)
    expect(tester.widget<TextField>(find.widgetWithText(TextField, 'Email')).readOnly, isFalse);

    // cognome vuoto: parte vuoto e l'errore del backend va sotto il campo
    await tester.enterText(find.widgetWithText(TextField, 'Cognome'), '');
    await tester.tap(find.text('Salva modifiche'));
    await tester.pumpAndSettle();
    expect(jsonDecode(_richieste.last.body), {'cognome': ''});
    expect(find.text('obbligatorio'), findsOneWidget);

    // solo i campi cambiati, verso PATCH /api/utenti/{id_utente}
    await tester.enterText(find.widgetWithText(TextField, 'Cognome'), 'Rossi');
    await tester.tap(find.text('Salva modifiche'));
    await tester.pumpAndSettle();
    final patch = _richieste.lastWhere((r) => r.method == 'PATCH');
    expect(patch.url.path, '/api/utenti/21');
    expect(jsonDecode(patch.body), {'cognome': 'Rossi'});
    expect(find.text('Dati di Dora Rossi aggiornati'), findsOneWidget);
  });
}
