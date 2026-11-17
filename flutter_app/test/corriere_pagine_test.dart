// flutter test — pagine dell'area corrieri su uno schermo da telefono, con un backend
// finto (MockClient) che risponde come quello vero: elenco e ricerca, dettaglio,
// tentativo non riuscito, cambio di stato e nota interna.
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:veloce24_app/corriere/api.dart';
import 'package:veloce24_app/corriere/dettaglio.dart';
import 'package:veloce24_app/corriere/spedizioni.dart';

/// Backend finto con una sola spedizione del corriere.
class _Backend {
  String stato = 'assegnata';
  int ritiri = 0, consegne = 0;
  final note = <Map<String, Object?>>[];
  final richieste = <http.Request>[];
  static const _motivi = ['mittente assente', 'destinatario assente', 'altro'];

  http.Request get ultima => richieste.last;
  Map<String, dynamic> get ultimoCorpo => jsonDecode(ultima.body) as Map<String, dynamic>;

  // come il backend vero: il corriere vede le sue spedizioni solo in questi stati
  bool get _visibile => const ['assegnata', 'ritirata', 'in consegna'].contains(stato);

  Map<String, Object?> get _piatta => {
        'id': 42,
        'cod_ordine': 'ORD-42',
        'stato': stato,
        'is_reso': false,
        'creata_il': '2026-11-10T09:00:00Z',
        'dataora_richiesta_spedizione': '2026-11-17T08:00:00Z',
        'ritirata_il': null,
        'id_cliente_mittente': 3,
        'id_cliente_destinatario': 7,
        'id_indirizzo_ritiro': 1,
        'id_indirizzo_spedizione': 2,
        'id_corriere': 1,
        'annullata_il': null,
        'tentativi_ritiro_falliti': ritiri,
        'tentativi_consegna_falliti': consegne,
      };

  Map<String, Object?> get _dettaglio => {
        ..._piatta,
        'indirizzo_ritiro': {'via': 'Via Roma', 'civico': '1', 'citta': 'Trento', 'cap': '38121', 'regione': 'Trentino-Alto Adige', 'nazione': 'Italia'},
        'indirizzo_spedizione': {'via': 'Via Verdi', 'civico': '10', 'citta': 'Rovereto', 'cap': '38068', 'regione': 'Trentino-Alto Adige', 'nazione': 'Italia'},
        'mittente': {'id': 3, 'nome': 'Marco', 'cognome': 'Mittente', 'email': 'm@test.it', 'telefono': '+39 3330000005'},
        'destinatario': {'id': 7, 'nome': 'Dora', 'cognome': 'Destinataria', 'email': 'd@test.it', 'telefono': '+39 3330000006'},
        'conclusa_il': null,
        'note': note,
      };

  static http.Response _json(Object corpo, int status) =>
      http.Response.bytes(utf8.encode(jsonEncode(corpo)), status, headers: {'content-type': 'application/json; charset=utf-8'});

  static http.Response _errore(int status, String messaggio, [Map<String, String> campi = const {}]) => _json({
        'errore': messaggio,
        if (campi.isNotEmpty)
          'dettagli': [
            for (final c in campi.entries) {'campo': c.key, 'messaggio': c.value}
          ],
      }, status);

  Future<http.Response> rispondi(http.Request req) async {
    richieste.add(req);
    final percorso = req.url.path;
    final corpo = req.body.isEmpty ? const <String, dynamic>{} : jsonDecode(req.body) as Map<String, dynamic>;
    switch ((req.method, percorso)) {
      case ('GET', '/api/spedizioni'):
        final filtro = req.url.queryParameters['stato'];
        return _json({
          'dati': [if (_visibile && (filtro == null || filtro == stato)) _dettaglio],
          'pagina': 1,
          'per_pagina': 20
        }, 200);
      case ('GET', '/api/spedizioni/42'):
        return _visibile ? _json(_dettaglio, 200) : _errore(404, 'spedizione non trovata');
      case ('GET', '/api/motivi'):
        return _json({'dati': _motivi}, 200);
      case ('POST', '/api/spedizioni/42/tentativi'):
        final mancanti = {
          if (!tipiTentativo.contains(corpo['tipo'])) 'tipo': 'valore non previsto: ammessi ritiro, consegna',
          if (corpo['nota'] == null)
            'nota': 'obbligatoria per ogni tentativo non riuscito'
          else if ((corpo['nota'] as String).trim().isEmpty)
            'nota': 'non può essere vuota',
          if (corpo['motivo'] == null)
            'motivo': 'obbligatorio per ogni tentativo non riuscito'
          else if (!_motivi.contains(corpo['motivo']))
            'motivo': 'valore non previsto: usare uno dei motivi di GET /api/motivi',
        };
        if (mancanti.isNotEmpty) return _errore(400, 'dati non validi', mancanti);
        if (corpo['tipo'] == 'ritiro') {
          ritiri++;
        } else if (++consegne >= 3) {
          stato = 'non consegnata';
        }
        note.add({
          'id': note.length + 1,
          'nota': corpo['nota'],
          'is_interna': false,
          'motivo': corpo['motivo'],
          'creata_il': '2026-11-16T10:00:00Z',
          'autore': {'ruolo': 'corriere', 'id': 1, 'nome': 'Carlo', 'cognome': 'Corriere'},
        });
        return _json(_piatta, 201);
      case ('PATCH', '/api/spedizioni/42/stato'):
        const ammessi = {'assegnata': 'ritirata', 'ritirata': 'in consegna', 'in consegna': 'consegnata'};
        final nuovo = corpo['stato'] as String? ?? '';
        if (!statiSpedizione.contains(nuovo)) return _errore(400, 'dati non validi', {'stato': 'valore non previsto'});
        if (nuovo == 'annullata' || nuovo == 'persa') return _errore(403, 'solo un operatore può portare la spedizione in "$nuovo"');
        if (ammessi[stato] != nuovo) return _errore(409, 'da "$stato" non si può passare a "$nuovo"');
        stato = nuovo;
        return _json(_piatta, 200);
      case ('POST', '/api/spedizioni/42/note'):
        final testo = (corpo['nota'] as String? ?? '').trim();
        if (testo.isEmpty) return _errore(400, 'dati non validi', {'nota': 'non può essere vuota'});
        if (stato == 'assegnata') return _errore(409, 'il corriere può aggiungere note solo su spedizioni ritirate o in consegna');
        note.add({
          'id': note.length + 1,
          'nota': testo,
          'is_interna': true,
          'motivo': null,
          'creata_il': '2026-11-16T10:00:00Z',
          'autore': {'ruolo': 'corriere', 'id': 1, 'nome': 'Carlo', 'cognome': 'Corriere'},
        });
        return _json(note.last, 201);
    }
    return _errore(404, 'percorso sconosciuto');
  }
}

void _telefono(WidgetTester tester) {
  tester.view.physicalSize = const Size(375, 812);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
}

Future<void> _apriElenco(WidgetTester tester, _Backend backend) async {
  _telefono(tester);
  final api = ApiCorriere(base: 'http://backend:8080', client: MockClient(backend.rispondi));
  await tester.pumpWidget(MaterialApp(home: Scaffold(body: SpedizioniCorrierePage(api: api))));
  await tester.pumpAndSettle();
}

Future<void> _apriDettaglio(WidgetTester tester, _Backend backend) async {
  _telefono(tester);
  final api = ApiCorriere(base: 'http://backend:8080', client: MockClient(backend.rispondi));
  await tester.pumpWidget(MaterialApp(home: DettaglioCorrierePage(api: api, id: 42, codOrdine: 'ORD-42')));
  await tester.pumpAndSettle();
}

Future<void> _mostra(WidgetTester tester, Finder cosa) async {
  await tester.scrollUntilVisible(cosa, 200, scrollable: find.descendant(of: find.byType(ListView), matching: find.byType(Scrollable)).first);
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('elenco: mostra le spedizioni del corriere e le cerca per numero ordine e stato', (tester) async {
    final backend = _Backend();
    await _apriElenco(tester, backend);

    expect(find.text('ORD-42'), findsOneWidget);
    expect(find.text('assegnata'), findsOneWidget);
    expect(find.textContaining('Marco Mittente · Via Roma 1'), findsOneWidget);
    expect(find.textContaining('Dora Destinataria · Via Verdi 10'), findsOneWidget);

    await tester.enterText(find.widgetWithText(TextField, 'Numero ordine'), 'ORD-42');
    await tester.testTextInput.receiveAction(TextInputAction.search);
    await tester.pumpAndSettle();
    expect(backend.ultima.url.queryParameters['cod_ordine'], 'ORD-42');

    await tester.tap(find.byType(DropdownButtonFormField<String?>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('in consegna').last);
    await tester.pumpAndSettle();
    expect(backend.ultima.url.queryParameters, containsPair('stato', 'in consegna'));
    expect(find.text('Nessuna spedizione con questi filtri'), findsOneWidget);

    // nel menu ci sono tutti gli stati, non solo quelli che il corriere vede oggi
    await tester.tap(find.byType(DropdownButtonFormField<String?>));
    await tester.pumpAndSettle();
    for (final s in statiSpedizione) {
      expect(find.text(s), findsWidgets, reason: 'stato $s nel filtro');
    }
  });

  testWidgets('dettaglio: toccando una spedizione si vedono mittente, destinatario, indirizzi e tentativi', (tester) async {
    final backend = _Backend()..ritiri = 2;
    await _apriElenco(tester, backend);

    await tester.tap(find.text('ORD-42'));
    await tester.pumpAndSettle();

    expect(find.text('Marco Mittente'), findsOneWidget);
    expect(find.text('+39 3330000005'), findsOneWidget);
    expect(find.textContaining('Via Roma 1, 38121 Trento'), findsOneWidget);
    expect(find.widgetWithText(Row, 'Ritiri falliti'), findsWidgets);
    expect(find.text('2'), findsOneWidget);
    // la sezione della consegna è più in basso: si scorre come sul telefono
    await _mostra(tester, find.text('Dora Destinataria'));
    expect(find.text('+39 3330000006'), findsOneWidget);
    expect(find.textContaining('Via Verdi 10, 38068 Rovereto'), findsOneWidget);
  });

  testWidgets('tentativo non riuscito: gli errori del backend vanno sotto i campi, poi il tentativo viene registrato', (tester) async {
    final backend = _Backend();
    await _apriDettaglio(tester, backend);

    await tester.tap(find.text('Tentativo non riuscito'));
    await tester.pumpAndSettle();

    // Nessun controllo nel frontend: nota vuota e motivo vuoto partono e risponde il backend.
    await tester.tap(find.text('Registra'));
    await tester.pumpAndSettle();
    expect(backend.ultimoCorpo, {'tipo': '', 'motivo': '', 'nota': ''});
    expect(find.text('valore non previsto: ammessi ritiro, consegna'), findsOneWidget);
    expect(find.text('valore non previsto: usare uno dei motivi di GET /api/motivi'), findsOneWidget);
    expect(find.text('non può essere vuota'), findsOneWidget);
    final chiaveRifiutata = backend.ultima.headers['Idempotency-Key'];

    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('ritiro').last);
    await tester.pumpAndSettle();
    await tester.tap(find.byType(DropdownButtonFormField<String?>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('mittente assente').last);
    await tester.pumpAndSettle();
    await tester.enterText(find.widgetWithText(TextField, 'Nota interna'), 'negozio chiuso');
    await tester.tap(find.text('Registra'));
    await tester.pumpAndSettle();

    final invio = backend.richieste.lastWhere((r) => r.method == 'POST');
    expect(jsonDecode(invio.body), {'tipo': 'ritiro', 'motivo': 'mittente assente', 'nota': 'negozio chiuso'});
    // dopo un rifiuto, i dati corretti sono una richiesta nuova con una chiave nuova
    expect(invio.headers['Idempotency-Key'], isNot(chiaveRifiutata));
    expect(find.text('Registra'), findsNothing);
    expect(find.textContaining('Tentativo registrato: stato "assegnata", ritiri falliti 1'), findsOneWidget);
    expect(backend.ritiri, 1);

    // la nota pubblica compare nel dettaglio con motivo e autore
    await _mostra(tester, find.text('negozio chiuso'));
    expect(find.textContaining('il cliente vede il motivo "mittente assente"'), findsOneWidget);
    expect(find.textContaining('Carlo Corriere (corriere · id 1)'), findsOneWidget);
  });

  testWidgets('ultima consegna fallita: la spedizione diventa non consegnata e si torna all\'elenco', (tester) async {
    final backend = _Backend()
      ..stato = 'in consegna'
      ..consegne = 2;
    await _apriElenco(tester, backend);
    await tester.tap(find.text('ORD-42'));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Tentativo non riuscito'));
    await tester.pumpAndSettle();
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('consegna').last);
    await tester.pumpAndSettle();
    await tester.tap(find.byType(DropdownButtonFormField<String?>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('destinatario assente').last);
    await tester.pumpAndSettle();
    await tester.enterText(find.widgetWithText(TextField, 'Nota interna'), 'terzo passaggio');
    await tester.tap(find.text('Registra'));
    await tester.pumpAndSettle();

    expect(backend.stato, 'non consegnata');
    // il backend non mostra più la spedizione al corriere (404): si torna all'elenco con l'esito
    expect(find.textContaining('ORD-42: Tentativo registrato: stato "non consegnata"'), findsOneWidget);
    expect(find.text('Nessuna spedizione'), findsOneWidget);
  });

  testWidgets('cambio di stato: tutti gli stati in elenco, il rifiuto del backend va a schermo, il passaggio ammesso aggiorna il dettaglio',
      (tester) async {
    final backend = _Backend();
    await _apriDettaglio(tester, backend);

    await tester.tap(find.text('Cambia stato'));
    await tester.pumpAndSettle();
    // nessun filtro: si può scegliere qualunque stato, anche quelli che il corriere non può impostare
    for (final s in statiSpedizione) {
      expect(find.widgetWithText(RadioListTile<String>, s), findsOneWidget, reason: 'stato $s nel menu');
    }
    await tester.tap(find.widgetWithText(RadioListTile<String>, 'annullata'));
    await tester.tap(find.text('Conferma'));
    await tester.pumpAndSettle();
    expect(backend.ultimoCorpo, {'stato': 'annullata'});
    expect(find.text('solo un operatore può portare la spedizione in "annullata"'), findsOneWidget);

    await tester.tap(find.widgetWithText(RadioListTile<String>, 'consegnata'));
    await tester.tap(find.text('Conferma'));
    await tester.pumpAndSettle();
    expect(find.text('da "assegnata" non si può passare a "consegnata"'), findsOneWidget);
    expect(backend.stato, 'assegnata');

    await tester.tap(find.widgetWithText(RadioListTile<String>, 'ritirata'));
    await tester.tap(find.text('Conferma'));
    await tester.pumpAndSettle();
    expect(backend.stato, 'ritirata');
    expect(find.text('Conferma'), findsNothing);
    expect(find.text('Stato aggiornato: ritirata'), findsOneWidget);
    expect(find.widgetWithText(Chip, 'ritirata'), findsOneWidget);
  });

  testWidgets('nota interna: l\'errore del backend va sotto il campo, la nota valida compare tra le note', (tester) async {
    final backend = _Backend()..stato = 'ritirata';
    await _apriDettaglio(tester, backend);

    await _mostra(tester, find.text('Aggiungi nota interna'));
    await tester.tap(find.text('Aggiungi nota interna'));
    await tester.pumpAndSettle();
    expect(backend.ultimoCorpo, {'nota': ''});
    expect(find.text('non può essere vuota'), findsOneWidget);

    await tester.enterText(find.widgetWithText(TextField, 'Nuova nota interna'), 'pacco caricato');
    await tester.tap(find.text('Aggiungi nota interna'));
    await tester.pumpAndSettle();
    expect(find.text('Nota interna aggiunta'), findsOneWidget);
    await _mostra(tester, find.text('pacco caricato'));
    expect(find.textContaining('interna · '), findsOneWidget);
  });

  testWidgets('nota interna rifiutata per lo stato: il messaggio del backend arriva al corriere', (tester) async {
    final backend = _Backend(); // assegnata: il backend non ammette note del corriere
    await _apriDettaglio(tester, backend);

    await _mostra(tester, find.text('Aggiungi nota interna'));
    await tester.enterText(find.widgetWithText(TextField, 'Nuova nota interna'), 'prova');
    await tester.tap(find.text('Aggiungi nota interna'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    expect(find.text('il corriere può aggiungere note solo su spedizioni ritirate o in consegna'), findsOneWidget);
    expect(backend.note, isEmpty);
  });
}
