// flutter test — area amministratori con un backend finto (MockClient): le pagine
// mandano ciò che è a schermo senza controllarlo, mostrano gli errori del backend e
// il dettaglio delle spedizioni è in sola lettura.
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:veloce24_app/amministratore/api.dart';
import 'package:veloce24_app/amministratore/configurazione.dart';
import 'package:veloce24_app/amministratore/spedizioni.dart';
import 'package:veloce24_app/amministratore/utenti.dart';

final _richieste = <http.Request>[];

http.Response _json(Object corpo, int status) =>
    http.Response.bytes(utf8.encode(jsonEncode(corpo)), status, headers: {'content-type': 'application/json; charset=utf-8'});

http.Response _nonValido(Map<String, String> campi) => _json({
      'errore': 'dati non validi',
      'dettagli': [for (final e in campi.entries) {'campo': e.key, 'messaggio': e.value}],
    }, 400);

const _carlo = {
  'id': 12,
  'ruolo': 'corriere',
  'id_ruolo': 3,
  'nome': 'Carlo',
  'cognome': 'Corriere',
  'email': 'carlo@test.it',
  'prefisso': '+39',
  'nr_telefono': '3330000001',
  'attivo': true,
};

const _mittente = {
  'id': 40,
  'ruolo': 'cliente',
  'id_ruolo': 9,
  'nome': 'Marco',
  'cognome': 'Mittente',
  'email': 'marco@test.it',
  'prefisso': '+39',
  'nr_telefono': '3330000009',
  'attivo': true,
  'destinatario': false,
};

Future<http.Response> _backend(http.Request req) async {
  _richieste.add(req);
  final corpo = req.body.isEmpty ? null : jsonDecode(req.body) as Map<String, dynamic>;
  switch ((req.method, req.url.path)) {
    case ('GET', '/api/impostazioni'):
      return _json({
        'giorni_visibilita_cliente': 30,
        'max_tentativi_ritiro': 3,
        'max_tentativi_consegna': 3,
        'intervallo_posizione_sec': 30,
        'modificate_il': '2026-11-18T09:00:00Z',
        'id_amministratore_modifica': 1,
      }, 200);
    case ('PATCH', '/api/impostazioni'):
      return _nonValido({if (corpo!['giorni_visibilita_cliente'] is! int) 'giorni_visibilita_cliente': 'deve essere tra 1 e 365'});
    case ('GET', '/api/impostazioni/motivi'):
      return _json({
        'dati': [
          {'id': 1, 'descrizione': 'destinatario assente', 'attivo': true, 'creato_il': '2026-11-01T09:00:00Z'},
          {'id': 2, 'descrizione': 'furto', 'attivo': true, 'creato_il': '2026-11-01T09:00:00Z'},
        ]
      }, 200);
    case ('POST', '/api/impostazioni/motivi'):
      return _nonValido({'descrizione': 'obbligatoria'});
    case ('PATCH', '/api/impostazioni/motivi/2'):
      return _json({'id': 2, 'descrizione': 'furto', 'attivo': false, 'creato_il': '2026-11-01T09:00:00Z'}, 200);
    case ('GET', '/api/utenti'):
      return _json({
        'dati': [_carlo, _mittente],
        'pagina': 1,
        'per_pagina': 20,
        'totale_elementi': 2,
        'totale_pagine': 1
      }, 200);
    case ('PATCH', '/api/utenti/40'):
      return _json({'errore': "un cliente solo mittente non accede all'app: si possono modificare solo i dati"}, 409);
    case ('POST', '/api/utenti'):
      return _nonValido({if (!corpo!.containsKey('ruolo')) 'ruolo': 'obbligatorio: corriere oppure operatore', 'nome': 'obbligatorio'});
    case ('PATCH', '/api/utenti/12'):
      return _json({..._carlo, ...corpo!}, 200);
    case ('GET', '/api/spedizioni/42'):
      return _json({
        'id': 42,
        'cod_ordine': 'ORD-42',
        'stato': 'in consegna',
        'is_reso': false,
        'creata_il': '2026-11-10T09:00:00Z',
        'dataora_richiesta_spedizione': '2026-11-11T09:00:00Z',
        'ritirata_il': '2026-11-11T10:00:00Z',
        'indirizzo_ritiro': {'via': 'Via Roma', 'civico': '1', 'citta': 'Trento', 'cap': '38121', 'regione': 'Trentino-Alto Adige', 'nazione': 'Italia'},
        'indirizzo_spedizione': {'via': 'Via Verdi', 'civico': '10', 'citta': 'Trento', 'cap': '38121', 'regione': 'Trentino-Alto Adige', 'nazione': 'Italia'},
        'mittente': {'id': 3, 'nome': 'Marco', 'cognome': 'Mittente'},
        'destinatario': {'id': 7, 'nome': 'Dora', 'cognome': 'Destinataria'},
        'corriere': {'id': 3, 'nome': 'Carlo', 'cognome': 'Corriere'},
        'note': [
          {'id': 1, 'nota': 'pacco caricato', 'is_interna': true, 'motivo': null, 'creata_il': '2026-11-11T10:05:00Z'}
        ],
      }, 200);
    case ('GET', '/api/spedizioni/42/storico'):
      return _json({
        'dati': [
          {
            'id': 1,
            'operazione': 'modifica',
            'valore_vecchio': {'stato': 'ritirata'},
            'valore_nuovo': {'stato': 'in consegna'},
            'modificato_il': '2026-11-11T10:30:00Z',
            'autore': {'ruolo': 'corriere', 'id': 3, 'nome': 'Carlo', 'cognome': 'Corriere'},
          }
        ]
      }, 200);
  }
  return _json({'errore': 'percorso sconosciuto'}, 404);
}

Future<void> _apri(WidgetTester tester, Widget Function(ApiAmministratore) pagina) async {
  _richieste.clear();
  tester.view.physicalSize = const Size(1200, 1600);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  final api = ApiAmministratore(base: 'http://backend:8080', client: MockClient(_backend));
  await tester.pumpWidget(MaterialApp(home: Scaffold(body: pagina(api))));
  await tester.pumpAndSettle();
}

Map<String, dynamic> _corpo(String metodo, String percorso) =>
    jsonDecode(_richieste.lastWhere((r) => r.method == metodo && r.url.path == percorso).body) as Map<String, dynamic>;

void main() {
  testWidgets('configurazione: partono solo i parametri cambiati, così come sono scritti; motivi', (tester) async {
    await _apri(tester, (api) => ConfigurazionePage(api: api));
    expect(find.text('Annulla modifiche'), findsNothing);

    await tester.enterText(find.widgetWithText(TextField, 'Giorni di visibilità per il cliente'), 'tanti');
    await tester.enterText(find.widgetWithText(TextField, 'Intervallo della posizione (secondi)'), '10');
    await tester.tap(find.text('Salva modifiche'));
    await tester.pumpAndSettle();
    expect(_corpo('PATCH', '/api/impostazioni'), {'giorni_visibilita_cliente': 'tanti', 'intervallo_posizione_sec': 10});
    expect(find.text('deve essere tra 1 e 365'), findsOneWidget); // errore del backend sotto il campo

    // disattivare un motivo
    await tester.tap(find.widgetWithText(SwitchListTile, 'furto'));
    await tester.pumpAndSettle();
    expect(_corpo('PATCH', '/api/impostazioni/motivi/2'), {'attivo': false});

    // nuovo motivo vuoto: parte vuoto, risponde il backend
    await tester.tap(find.text('Aggiungi'));
    await tester.pumpAndSettle();
    expect(_corpo('POST', '/api/impostazioni/motivi'), {'descrizione': ''});
    expect(find.text('obbligatoria'), findsOneWidget);
  });

  testWidgets('utenti: creazione senza controlli, modifica dei soli campi cambiati, disattivazione', (tester) async {
    await _apri(tester, (api) => UtentiPage(api: api));
    expect(find.text('Carlo Corriere'), findsOneWidget);
    expect(find.text('Cliente mittente'), findsOneWidget); // anche i clienti sono in elenco

    // creazione con i campi vuoti e senza ruolo: gli errori del backend vanno sotto i campi
    await tester.tap(find.text('Nuovo utente'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Crea utente'));
    await tester.pumpAndSettle();
    expect(_corpo('POST', '/api/utenti'),
        {'nome': '', 'cognome': '', 'prefisso': '+39', 'nr_telefono': '', 'email': '', 'password': ''});
    expect(find.text('obbligatorio: corriere oppure operatore'), findsOneWidget);
    expect(find.text('obbligatorio'), findsOneWidget);
    await tester.tap(find.text('Annulla'));
    await tester.pumpAndSettle();

    // modifica: solo il cognome cambiato
    await tester.tap(find.byTooltip('Modifica dati').first);
    await tester.pumpAndSettle();
    expect(find.textContaining('Il ruolo non si cambia'), findsOneWidget);
    await tester.enterText(find.widgetWithText(TextField, 'Cognome'), 'Rossi');
    await tester.tap(find.text('Salva modifiche'));
    await tester.pumpAndSettle();
    expect(_corpo('PATCH', '/api/utenti/12'), {'cognome': 'Rossi'});

    // disattivazione con conferma
    await tester.tap(find.widgetWithText(TextButton, 'Disattiva').first);
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Disattiva'));
    await tester.pumpAndSettle();
    expect(_corpo('PATCH', '/api/utenti/12'), {'attivo': false});

    // il cliente solo mittente: la richiesta parte, il rifiuto del backend va a schermo
    tester.state<ScaffoldMessengerState>(find.byType(ScaffoldMessenger)).clearSnackBars(); // via gli avvisi precedenti
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(TextButton, 'Disattiva').last);
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Disattiva'));
    await tester.pumpAndSettle();
    expect(_corpo('PATCH', '/api/utenti/40'), {'attivo': false});
    expect(find.textContaining('un cliente solo mittente non accede'), findsOneWidget);
  });

  testWidgets('spedizione: dettaglio, note e storico in sola lettura, senza azioni', (tester) async {
    await _apri(tester, (api) => DettaglioSolaLettura(api: api, id: 42));

    expect(find.text('Sola lettura'), findsOneWidget);
    expect(find.text('pacco caricato'), findsOneWidget);
    expect(find.text('stato: ritirata → in consegna'), findsOneWidget);
    expect(find.textContaining('Carlo Corriere (corriere · id 3)'), findsOneWidget);
    for (final azione in ['Cambia stato', 'Assegna corriere', 'Riassegna corriere', 'Segna come reso', 'Aggiungi']) {
      expect(find.text(azione), findsNothing);
    }
    expect(find.byType(TextField), findsNothing);
    // solo letture: nessuna richiesta che modifica
    expect(_richieste.where((r) => r.method != 'GET'), isEmpty);
  });
}
