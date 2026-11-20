// flutter test — pagina di tracking del cliente destinatario, con un backend finto.
import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:veloce24_app/api.dart';
import 'package:veloce24_app/tracking_page.dart';

const _voce = {
  'id': 7, 'cod_ordine': 'ORD-7', 'stato': 'in consegna', 'is_reso': false,
  'creata_il': '2026-11-03T09:00:00Z', 'dataora_richiesta_spedizione': '2026-11-04T09:00:00Z',
  // come il backend vero: al cliente non arriva id_cliente_mittente
  'id_cliente_destinatario': 2,
  'indirizzo_ritiro': {'via': 'Via Roma', 'civico': '1'},
  'indirizzo_spedizione': {'via': 'Via Verdi', 'civico': '10'},
  'conclusa_il': null, 'note': [],
};

// Il dettaglio contiene di proposito dati che il backend al cliente non manda
// (corriere, testo della nota, nota interna): la pagina non deve mostrarli comunque.
final _dettaglio = {
  ..._voce,
  'id_corriere': 9,
  'annullata_il': null,
  'tentativi_ritiro_falliti': 0,
  'tentativi_consegna_falliti': 1,
  'note': [
    {'id': 1, 'nota': 'Citofono rotto', 'is_interna': false, 'motivo': 'destinatario assente', 'creata_il': '2026-11-04T10:00:00Z'},
    {'id': 2, 'nota': 'Cliente scortese', 'is_interna': true, 'motivo': null, 'creata_il': '2026-11-04T10:05:00Z'},
  ],
};

void main() {
  late List<Uri> richieste;

  Widget app() {
    richieste = [];
    final api = ApiClient(
      base: 'http://backend:8080',
      ruolo: RuoloApi.cliente,
      idUtente: 2,
      client: MockClient((req) async {
        richieste.add(req.url);
        if (req.url.path == '/api/spedizioni/7') return http.Response(jsonEncode(_dettaglio), 200);
        final q = req.url.queryParameters;
        final dati = (q['stato'] == null || q['stato'] == 'in consegna') && !(q['cod_ordine'] ?? '').contains('X') ? [_voce] : [];
        return http.Response(jsonEncode({'dati': dati, 'pagina': 1, 'per_pagina': 20}), 200);
      }),
    );
    return MaterialApp(home: TrackingClientePage(api: api));
  }

  testWidgets('elenca i pacchi del cliente', (t) async {
    await t.pumpWidget(app());
    await t.pumpAndSettle();
    expect(find.text('ORD-7'), findsOneWidget);
    expect(find.text('in consegna'), findsOneWidget);
    expect(richieste.single.queryParameters['pagina'], '1');
  });

  testWidgets('filtra per stato', (t) async {
    await t.pumpWidget(app());
    await t.pumpAndSettle();
    await t.tap(find.text('Tutti'));
    await t.pumpAndSettle();
    await t.tap(find.text('consegnata').last);
    await t.pumpAndSettle();
    expect(richieste.last.queryParameters['stato'], 'consegnata');
    expect(find.text('Nessun pacco corrisponde ai filtri.'), findsOneWidget);

    await t.tap(find.text('Azzera filtri'));
    await t.pumpAndSettle();
    expect(richieste.last.queryParameters.containsKey('stato'), isFalse);
    expect(find.text('ORD-7'), findsOneWidget);
    expect(find.text('Tutti'), findsOneWidget, reason: 'il menu dello stato torna a "Tutti"');
  });

  testWidgets('filtra per numero ordine dopo una breve pausa', (t) async {
    await t.pumpWidget(app());
    await t.pumpAndSettle();
    await t.enterText(find.byKey(const Key('filtro-codice')), 'ORD-X');
    await t.pump(const Duration(milliseconds: 500));
    await t.pumpAndSettle();
    expect(richieste.last.queryParameters['cod_ordine'], 'ORD-X');
    expect(find.text('Nessun pacco corrisponde ai filtri.'), findsOneWidget);
  });

  testWidgets('dettaglio: indirizzi e motivi, niente corriere né testo delle note', (t) async {
    await t.pumpWidget(app());
    await t.pumpAndSettle();
    await t.tap(find.text('ORD-7'));
    await t.pumpAndSettle();

    expect(richieste.last.path, '/api/spedizioni/7');
    expect(find.text('Mittente'), findsOneWidget);
    expect(find.text('Via Roma 1'), findsOneWidget);
    expect(find.text('Destinatario'), findsOneWidget);
    expect(find.text('Via Verdi 10'), findsOneWidget);
    expect(find.text('destinatario assente'), findsOneWidget);
    expect(find.text('Tentativi di consegna non riusciti'), findsOneWidget);

    expect(find.textContaining('Citofono'), findsNothing);
    expect(find.textContaining('scortese'), findsNothing);
    expect(find.textContaining('Corriere'), findsNothing);
  });

  testWidgets('dettaglio non visibile: messaggio 404', (t) async {
    final api = ApiClient(
      base: 'http://backend:8080',
      ruolo: RuoloApi.cliente,
      idUtente: 2,
      client: MockClient((_) async => http.Response('{"errore":"spedizione non trovata"}', 404)),
    );
    await t.pumpWidget(MaterialApp(home: TrackingDettaglioPage(api: api, id: 99, codOrdine: 'ORD-99')));
    await t.pumpAndSettle();
    expect(find.text('Spedizione non trovata.'), findsOneWidget);
  });
}
