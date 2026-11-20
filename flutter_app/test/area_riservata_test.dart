// flutter test — login unico dell'area riservata con un backend finto (MockClient):
// ogni ruolo entra con la stessa pagina e arriva alla sua area; chi non può entrare lo
// decide il backend.
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:veloce24_app/area_riservata.dart';

http.Response _json(Object corpo, int status) =>
    http.Response.bytes(utf8.encode(jsonEncode(corpo)), status, headers: {'content-type': 'application/json; charset=utf-8'});

final _richieste = <http.Request>[];

/// Backend finto: il ruolo dipende dall'email; mittente@ è un cliente solo mittente.
Future<http.Response> _backend(http.Request req) async {
  _richieste.add(req);
  switch (req.url.path) {
    case '/api/auth/login':
      final corpo = jsonDecode(req.body) as Map<String, dynamic>;
      final email = corpo['email'] as String;
      if (corpo['password'] == '') {
        return _json({
          'errore': 'dati non validi',
          'dettagli': [
            {'campo': 'password', 'messaggio': 'obbligatoria'}
          ]
        }, 400);
      }
      if (corpo['password'] != 'PasswordProva1') return _json({'errore': 'email o password non corrette'}, 401);
      if (email.startsWith('mittente')) {
        return _json({'errore': 'accesso riservato ai clienti destinatari: non risulta nessuna spedizione destinata a te'}, 403);
      }
      final ruolo = email.split('@').first; // operatore@, corriere@, amministratore@, cliente@
      return _json({
        'token': 'tok-$ruolo',
        'tipo': 'Bearer',
        'scade_il': '2026-11-16T23:00:00Z',
        'utente': {'id_utente': 1, 'ruolo': ruolo, 'id': 1, 'nome': 'Utente', 'cognome': 'Prova', 'email': email},
      }, 200);
    case '/api/spedizioni':
      return _json({'dati': [], 'pagina': 1, 'per_pagina': 20}, 200);
    case '/api/corrieri/posizioni':
      return _json({'intervallo_secondi': 30, 'corrieri': []}, 200);
    case '/api/corrieri/posizione':
      return _json({'intervallo_secondi': 30}, 201);
    case '/api/clienti':
      return _json({'dati': [], 'pagina': 1, 'per_pagina': 20, 'totale_elementi': 0, 'totale_pagine': 0}, 200);
    case '/api/utenti':
      return _json({'dati': [], 'pagina': 1, 'per_pagina': 20, 'totale_elementi': 0, 'totale_pagine': 0}, 200);
    case '/api/impostazioni':
      return _json({
        'giorni_visibilita_cliente': 30,
        'max_tentativi_ritiro': 3,
        'max_tentativi_consegna': 3,
        'intervallo_posizione_sec': 30,
        'modificate_il': '2026-11-18T09:00:00Z',
        'id_amministratore_modifica': null,
      }, 200);
    case '/api/impostazioni/motivi':
      return _json({'dati': []}, 200);
  }
  return _json({'errore': 'percorso sconosciuto'}, 404);
}

Future<void> _login(WidgetTester tester, String email, {String password = 'PasswordProva1'}) async {
  await tester.pumpWidget(MaterialApp(home: AreaRiservata(apiBase: 'http://backend:8080', client: MockClient(_backend))));
  await tester.enterText(find.widgetWithText(TextField, 'Email'), email);
  await tester.enterText(find.widgetWithText(TextField, 'Password'), password);
  await tester.tap(find.text('Entra'));
  await tester.pumpAndSettle();
}

/// Smonta le pagine a fine test, così i timer di aggiornamento si fermano.
Future<void> _chiudi(WidgetTester tester) => tester.pumpWidget(const SizedBox());

void main() {
  testWidgets('operatore: dopo il login si apre l\'area operatori', (tester) async {
    await _login(tester, 'operatore@test.it');
    expect(find.text('Nuova spedizione'), findsOneWidget);
    expect(find.text('Corrieri'), findsOneWidget);
    expect(find.text('Clienti'), findsOneWidget);
    await _chiudi(tester);
  });

  testWidgets('corriere: dopo il login si apre l\'elenco delle sue spedizioni', (tester) async {
    await _login(tester, 'corriere@test.it');
    expect(find.text('Le mie spedizioni'), findsOneWidget);
    expect(find.text('Numero ordine'), findsOneWidget);
    await _chiudi(tester);
  });

  testWidgets("amministratore: dopo il login si apre l'area amministratori", (tester) async {
    await _login(tester, 'amministratore@test.it');
    expect(find.text('Spedizioni'), findsOneWidget);
    expect(find.text('Utenti'), findsOneWidget);
    expect(find.text('Configurazione'), findsOneWidget);
    await _chiudi(tester);
  });

  testWidgets('cliente destinatario: dopo il login si apre il tracking dei suoi pacchi', (tester) async {
    _richieste.clear();
    await _login(tester, 'cliente@test.it');
    expect(find.text('I miei pacchi'), findsOneWidget);
    // l'elenco arriva dal backend con il token del login, non con l'header di sviluppo
    final elenco = _richieste.lastWhere((r) => r.url.path == '/api/spedizioni');
    expect(elenco.headers['Authorization'], 'Bearer tok-cliente');
    expect(elenco.headers.containsKey('X-Id-Cliente'), isFalse);

    // "Esci" riporta al login
    await tester.tap(find.byTooltip('Esci'));
    await tester.pumpAndSettle();
    expect(find.text('Entra'), findsOneWidget);
  });

  testWidgets('cliente solo mittente: il backend rifiuta il login e la pagina mostra il suo messaggio', (tester) async {
    await _login(tester, 'mittente@test.it');
    expect(find.text('accesso riservato ai clienti destinatari: non risulta nessuna spedizione destinata a te'), findsOneWidget);
    expect(find.text('Entra'), findsOneWidget);
  });

  testWidgets('credenziali sbagliate o vuote: nessun controllo nella pagina, risponde il backend', (tester) async {
    await _login(tester, 'operatore@test.it', password: '');
    expect(find.text('obbligatoria'), findsOneWidget); // l'errore del backend sotto il campo password
    await _login(tester, 'operatore@test.it', password: 'Sbagliata1');
    expect(find.text('Sessione scaduta o credenziali non valide: accedi di nuovo.'), findsOneWidget);
    expect(find.text('Entra'), findsOneWidget);
  });
}
