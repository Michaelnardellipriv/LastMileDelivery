// flutter test — posizione simulata del corriere: finché l'area è aperta il telefono
// la manda al backend a ogni intervallo, con coordinate che si spostano; se il backend
// la rifiuta, il suo messaggio compare in fondo alla pagina.
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:veloce24_app/corriere/api.dart';
import 'package:veloce24_app/corriere/app.dart';

http.Response _json(Object corpo, int status) =>
    http.Response.bytes(utf8.encode(jsonEncode(corpo)), status, headers: {'content-type': 'application/json; charset=utf-8'});

class _Backend {
  final inviate = <Map<String, dynamic>>[]; // posizioni ricevute
  bool inConsegna = true;
  int intervallo = 5;

  Future<http.Response> rispondi(http.Request req) async {
    switch ((req.method, req.url.path)) {
      case ('POST', '/api/auth/login'):
        return _json({
          'token': 'tok',
          'tipo': 'Bearer',
          'scade_il': '2026-11-16T23:00:00Z',
          'utente': {'id_utente': 2, 'ruolo': 'corriere', 'id': 1, 'nome': 'Carlo', 'cognome': 'Corriere', 'email': 'c@test.it'},
        }, 200);
      case ('GET', '/api/spedizioni'):
        return _json({'dati': [], 'pagina': 1, 'per_pagina': 20}, 200);
      case ('POST', '/api/corrieri/posizione'):
        if (!inConsegna) {
          return _json({'errore': 'il corriere non ha spedizioni in consegna: la posizione non viene registrata'}, 409);
        }
        inviate.add(jsonDecode(req.body) as Map<String, dynamic>);
        return _json({'intervallo_secondi': intervallo}, 201);
    }
    return _json({'errore': 'percorso sconosciuto'}, 404);
  }
}

Future<void> _apri(WidgetTester tester, _Backend backend) async {
  final api = ApiCorriere(base: 'http://backend:8080', client: MockClient(backend.rispondi));
  await tester.runAsync(() => api.login('c@test.it', 'x'));
  await tester.pumpWidget(MaterialApp(home: HomeCorriere(api: api, alLogout: () {})));
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('la posizione parte subito e poi a ogni intervallo deciso dal backend, con coordinate che si spostano', (tester) async {
    final backend = _Backend();
    await _apri(tester, backend);

    expect(backend.inviate, hasLength(1));
    expect(find.textContaining('Posizione inviata alle'), findsOneWidget);
    expect(find.textContaining('ogni 5 s'), findsOneWidget);

    // prima dell'intervallo non parte nulla; allo scadere sì, e così a ogni giro
    await tester.pump(const Duration(seconds: 4));
    expect(backend.inviate, hasLength(1));
    await tester.pump(const Duration(seconds: 2));
    await tester.pumpAndSettle();
    expect(backend.inviate, hasLength(2));
    await tester.pump(const Duration(seconds: 5));
    await tester.pumpAndSettle();
    expect(backend.inviate, hasLength(3));

    // ogni invio ha coordinate diverse dal precedente: sulla mappa il corriere si muove
    final punti = {for (final p in backend.inviate) '${p['latitudine']},${p['longitudine']}'};
    expect(punti, hasLength(3));
    for (final p in backend.inviate) {
      expect(p['latitudine'], closeTo(46.07, 0.05));
      expect(p['longitudine'], closeTo(11.12, 0.05));
    }

    // l'amministratore cambia l'intervallo: il telefono si adegua dalla risposta successiva
    backend.intervallo = 20;
    await tester.pump(const Duration(seconds: 5));
    await tester.pumpAndSettle();
    expect(backend.inviate, hasLength(4));
    expect(find.textContaining('ogni 20 s'), findsOneWidget);
    await tester.pump(const Duration(seconds: 10));
    expect(backend.inviate, hasLength(4));

    // chiusa l'area (logout) il telefono smette di mandare la posizione
    await tester.pumpWidget(const SizedBox());
    await tester.pump(const Duration(seconds: 60));
    expect(backend.inviate, hasLength(4));
  });

  testWidgets('posizione rifiutata dal backend: il suo messaggio va a schermo e il telefono riprova', (tester) async {
    final backend = _Backend()..inConsegna = false;
    await _apri(tester, backend);

    expect(backend.inviate, isEmpty);
    expect(find.textContaining('Posizione non registrata: il corriere non ha spedizioni in consegna'), findsOneWidget);

    // appena il corriere ha una spedizione in consegna, l'invio successivo viene registrato
    backend.inConsegna = true;
    await tester.pump(const Duration(seconds: 30));
    await tester.pumpAndSettle();
    expect(backend.inviate, hasLength(1));
    expect(find.textContaining('Posizione inviata alle'), findsOneWidget);

    await tester.pumpWidget(const SizedBox());
  });
}
