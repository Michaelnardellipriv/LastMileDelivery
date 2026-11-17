// flutter test — cambio di stato nel dettaglio dell'operatore, con un backend finto
// (MockClient): la pagina manda ciò che è a schermo, anche motivo e nota vuoti, e mostra
// gli errori del backend.
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:veloce24_app/operatore/api.dart';
import 'package:veloce24_app/operatore/dettaglio.dart';

final _richieste = <http.Request>[];

http.Response _json(Object corpo, int status) =>
    http.Response.bytes(utf8.encode(jsonEncode(corpo)), status, headers: {'content-type': 'application/json; charset=utf-8'});

const _spedizione = {
  'id': 42,
  'cod_ordine': 'ORD-42',
  'stato': 'assegnata',
  'is_reso': false,
  'creata_il': '2026-11-10T09:00:00Z',
  'dataora_richiesta_spedizione': '2026-11-11T09:00:00Z',
  'ritirata_il': null,
  'id_cliente_mittente': 3,
  'id_cliente_destinatario': 7,
  'indirizzo_ritiro': {'via': 'Via Roma', 'civico': '1', 'citta': 'Trento', 'cap': '38121', 'regione': 'Trentino-Alto Adige', 'nazione': 'Italia'},
  'indirizzo_spedizione': {'via': 'Via Verdi', 'civico': '10', 'citta': 'Trento', 'cap': '38121', 'regione': 'Trentino-Alto Adige', 'nazione': 'Italia'},
  'mittente': {'id': 3, 'nome': 'Marco', 'cognome': 'Mittente', 'email': 'm@test.it', 'telefono': '+39 3330000005'},
  'destinatario': {'id': 7, 'nome': 'Dora', 'cognome': 'Destinataria', 'email': 'd@test.it', 'telefono': '+39 3330000006'},
  'id_corriere': 1,
  'corriere': {'id': 1, 'nome': 'Carlo', 'cognome': 'Corriere', 'telefono': '+39 3330000003'},
  'conclusa_il': null,
  'annullata_il': null,
  'tentativi_ritiro_falliti': 0,
  'tentativi_consegna_falliti': 0,
  'note': [],
};

Future<http.Response> _backend(http.Request req) async {
  _richieste.add(req);
  switch ((req.method, req.url.path)) {
    case ('GET', '/api/spedizioni/42'):
      return _json(_spedizione, 200);
    case ('GET', '/api/spedizioni/42/storico'):
      return _json({'dati': []}, 200);
    case ('GET', '/api/motivi'):
      return _json({
        'dati': ['richiesta del mittente', 'furto', 'altro']
      }, 200);
    case ('PATCH', '/api/spedizioni/42'):
      // come il backend: il reso solo su annullata, persa o non consegnata
      return _json({'errore': 'is_reso si può impostare solo su una spedizione annullata, persa o non consegnata (stato attuale: "assegnata")'}, 409);
    case ('PATCH', '/api/spedizioni/42/stato'):
      final corpo = jsonDecode(req.body) as Map<String, dynamic>;
      if (corpo['stato'] == 'annullata') {
        return _json({
          'errore': 'dati non validi',
          'dettagli': [
            if (corpo['motivo'] == null)
              {'campo': 'motivo', 'messaggio': 'obbligatorio per annullare una spedizione'}
            else if (corpo['motivo'] == '')
              {'campo': 'motivo', 'messaggio': 'valore non previsto: usare uno dei motivi di GET /api/motivi'},
            if (corpo['nota'] == null)
              {'campo': 'nota', 'messaggio': 'obbligatoria per annullare una spedizione'}
            else if (corpo['nota'] == '')
              {'campo': 'nota', 'messaggio': 'non può essere vuota'},
          ]
        }, 400);
      }
      return _json({'errore': 'solo un corriere può portare la spedizione in "${corpo['stato']}"'}, 403);
  }
  return _json({'errore': 'percorso sconosciuto'}, 404);
}

Future<void> _scegliStato(WidgetTester tester, String stato) async {
  await tester.tap(find.byType(DropdownButtonFormField<String>));
  await tester.pumpAndSettle();
  await tester.tap(find.text(stato).last);
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('cambio di stato: senza nota pubblica parte solo lo stato; con la nota partono anche motivo e nota vuoti', (tester) async {
    _richieste.clear();
    tester.view.physicalSize = const Size(1000, 1400);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    final api = ApiOperatore(base: 'http://backend:8080', client: MockClient(_backend));
    await tester.pumpWidget(MaterialApp(home: Scaffold(body: DettaglioSpedizione(api: api, id: 42))));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Cambia stato'));
    await tester.pumpAndSettle();

    // 1. solo lo stato: il rifiuto del backend va a schermo
    await _scegliStato(tester, 'ritirata');
    await tester.tap(find.text('Conferma'));
    await tester.pumpAndSettle();
    expect(jsonDecode(_richieste.last.body), {'stato': 'ritirata'});
    expect(find.text('solo un corriere può portare la spedizione in "ritirata"'), findsOneWidget);

    // 2. annullata senza motivo e nota: gli errori del backend si vedono anche se i campi non sono a schermo
    await _scegliStato(tester, 'annullata');
    await tester.tap(find.text('Conferma'));
    await tester.pumpAndSettle();
    expect(jsonDecode(_richieste.last.body), {'stato': 'annullata'});
    expect(find.text('motivo: obbligatorio per annullare una spedizione'), findsOneWidget);
    expect(find.text('nota: obbligatoria per annullare una spedizione'), findsOneWidget);

    // 3. con la nota pubblica ma senza compilarla: motivo vuoto e nota vuota arrivano al backend
    await tester.tap(find.text('Aggiungi motivo e nota'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Conferma'));
    await tester.pumpAndSettle();
    expect(jsonDecode(_richieste.last.body), {'stato': 'annullata', 'motivo': '', 'nota': ''});
    expect(find.text('valore non previsto: usare uno dei motivi di GET /api/motivi'), findsOneWidget);
    expect(find.text('non può essere vuota'), findsOneWidget);

    // 4. tutti i motivi nel menu, senza filtri per stato
    await tester.tap(find.byType(DropdownButtonFormField<String?>));
    await tester.pumpAndSettle();
    for (final m in ['richiesta del mittente', 'furto', 'altro']) {
      expect(find.text(m), findsWidgets);
    }
  });

  testWidgets("reso: l'operatore lo chiede con PATCH is_reso e vede il rifiuto del backend", (tester) async {
    _richieste.clear();
    tester.view.physicalSize = const Size(1000, 1400);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    final api = ApiOperatore(base: 'http://backend:8080', client: MockClient(_backend));
    await tester.pumpWidget(MaterialApp(home: Scaffold(body: DettaglioSpedizione(api: api, id: 42))));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Segna come reso'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Conferma'));
    await tester.pumpAndSettle();

    final patch = _richieste.lastWhere((r) => r.method == 'PATCH');
    expect(patch.url.path, '/api/spedizioni/42');
    expect(jsonDecode(patch.body), {'is_reso': true});
    expect(find.textContaining('is_reso si può impostare solo su una spedizione annullata'), findsOneWidget);
  });
}
