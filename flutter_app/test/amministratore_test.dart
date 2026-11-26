// flutter test — area amministratori con un backend finto (MockClient): le pagine
// mandano ciò che è a schermo senza controllarlo, mostrano gli errori del backend e
// il dettaglio delle spedizioni è in sola lettura. Sedi: creazione, modifica,
// disattivazione, parametri per sede e sede degli utenti; gruppo del database delle
// spedizioni scelto solo alla creazione. Spedizioni: l'amministratore sceglie sempre una
// sede, nessuna richiesta parte senza id_sede.
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:veloce24_app/amministratore/api.dart';
import 'package:veloce24_app/amministratore/app.dart';
import 'package:veloce24_app/amministratore/configurazione.dart';
import 'package:veloce24_app/amministratore/sedi.dart';
import 'package:veloce24_app/amministratore/spedizioni.dart';
import 'package:veloce24_app/amministratore/utenti.dart';
import 'package:veloce24_app/comune/spedizioni.dart';

final _richieste = <http.Request>[];

http.Response _json(Object corpo, int status) =>
    http.Response.bytes(utf8.encode(jsonEncode(corpo)), status, headers: {'content-type': 'application/json; charset=utf-8'});

http.Response _nonValido(Map<String, String> campi) => _json({
      'errore': 'dati non validi',
      'dettagli': [for (final e in campi.entries) {'campo': e.key, 'messaggio': e.value}],
    }, 400);

Map<String, Object?> _sede(int id, String nome, String citta, {bool attiva = true, int gruppo = 0}) => {
      'id': id,
      'nome': nome,
      'attiva': attiva,
      'gruppo_spedizioni': gruppo,
      'creata_il': '2026-11-01T09:00:00Z',
      'indirizzo': {'id': id, 'via': 'Via Roma', 'civico': '1', 'citta': citta, 'cap': '38121', 'regione': 'Trentino-Alto Adige', 'nazione': 'Italia'},
    };

final _sedi = [
  _sede(10, 'Veloce24 Trento', 'Trento'),
  _sede(30, 'Veloce24 Rovereto', 'Rovereto', gruppo: 1),
  _sede(20, 'Veloce24 Bolzano', 'Bolzano', attiva: false),
];

/// Sedi restituite da GET /api/sedi: i test le cambiano, setUp rimette quelle sopra.
var _elencoSedi = _sedi;

/// Con true GET /api/sedi risponde con un errore.
var _sediGuaste = false;

/// Totale delle sedi oltre la pagina (molte città): null = quelle di _elencoSedi. Con q
/// (ricerca) GET /api/sedi cerca in _sediCercabili.
int? _totaleSedi;
var _sediCercabili = <Map<String, Object?>>[];

const _carlo = {
  'id': 12,
  'ruolo': 'corriere',
  'id_ruolo': 3,
  'id_sede': 10,
  'sede': 'Veloce24 Trento',
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
  'id_sede': null,
  'sede': null,
};

Future<http.Response> _backend(http.Request req) async {
  _richieste.add(req);
  final corpo = req.body.isEmpty ? null : jsonDecode(req.body) as Map<String, dynamic>;
  switch ((req.method, req.url.path)) {
    case ('GET', '/api/spedizioni'):
      return _json({'dati': [], 'pagina': 1, 'per_pagina': 20, 'totale_elementi': 0, 'totale_pagine': 0}, 200);
    case ('GET', '/api/sedi'):
      if (_sediGuaste) return _json({'errore': 'servizio non disponibile'}, 503);
      final q = req.url.queryParameters['q'];
      if (q != null) {
        final trovate = [for (final s in _sediCercabili) if ((s['nome'] as String).contains(q)) s];
        return _json({'dati': trovate, 'pagina': 1, 'per_pagina': 20, 'totale_elementi': trovate.length, 'totale_pagine': 1}, 200);
      }
      return _json({
        'dati': _elencoSedi, 'pagina': 1, 'per_pagina': 100,
        'totale_elementi': _totaleSedi ?? _elencoSedi.length, 'totale_pagine': 1,
      }, 200);
    case ('POST', '/api/sedi'):
      return _nonValido({
        'nome': 'obbligatorio',
        'indirizzo.via': 'obbligatoria',
        if (corpo!.containsKey('gruppo_spedizioni'))
          'gruppo_spedizioni': 'nessun database delle spedizioni per il gruppo ${corpo['gruppo_spedizioni']}: gruppi configurati 0, 1',
      });
    case ('PATCH', '/api/sedi/10'):
      return _json({..._sedi[0], ...corpo!}, 200);
    case ('GET', '/api/sedi/10/impostazioni'):
      return _json({
        'id_sede': 10,
        'giorni_visibilita_cliente': 30,
        'max_tentativi_ritiro': 3,
        'max_tentativi_consegna': 3,
        'intervallo_posizione_sec': 30,
        'modificate_il': '2026-11-18T09:00:00Z',
        'id_amministratore_modifica': 1,
      }, 200);
    case ('GET', '/api/sedi/30/impostazioni'):
      return _json({
        'id_sede': 30,
        'giorni_visibilita_cliente': 7,
        'max_tentativi_ritiro': 2,
        'max_tentativi_consegna': 2,
        'intervallo_posizione_sec': 60,
      }, 200);
    case ('PATCH', '/api/sedi/10/impostazioni'):
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

/// Richieste dell'elenco delle spedizioni, in ordine.
Iterable<http.Request> get _elenchi => _richieste.where((r) => r.method == 'GET' && r.url.path == '/api/spedizioni');

/// Elenco delle spedizioni dell'amministratore (con la scelta della sede).
SpedizioniPage _spedizioniAmministratore(ApiAmministratore api, {int versioneSedi = 0}) => SpedizioniPage(
    api: api, filtroSede: true, versioneSedi: versioneSedi, dettaglio: (id, _) => DettaglioSolaLettura(api: api, id: id));

void main() {
  setUp(() {
    _elencoSedi = _sedi;
    _sediGuaste = false;
    _totaleSedi = null;
    _sediCercabili = [];
  });

  testWidgets('configurazione: partono solo i parametri cambiati, così come sono scritti; motivi', (tester) async {
    await _apri(tester, (api) => ConfigurazionePage(api: api));
    expect(find.text('Annulla modifiche'), findsNothing);

    await tester.enterText(find.widgetWithText(TextField, 'Giorni di visibilità per il cliente'), 'tanti');
    await tester.enterText(find.widgetWithText(TextField, 'Intervallo della posizione (secondi)'), '10');
    await tester.tap(find.text('Salva modifiche'));
    await tester.pumpAndSettle();
    // i parametri sono della sede scelta (la prima)
    expect(_corpo('PATCH', '/api/sedi/10/impostazioni'), {'giorni_visibilita_cliente': 'tanti', 'intervallo_posizione_sec': 10});
    expect(find.text('deve essere tra 1 e 365'), findsOneWidget); // errore del backend sotto il campo

    // un'altra sede: si leggono i suoi parametri
    await tester.tap(find.byType(DropdownButtonFormField<int>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Veloce24 Rovereto · Rovereto').last);
    await tester.pumpAndSettle();
    expect(_richieste.last.url.path, '/api/sedi/30/impostazioni');
    expect(find.text('Parametri delle spedizioni · Veloce24 Rovereto'), findsOneWidget);
    expect(find.widgetWithText(TextField, '60'), findsOneWidget);

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
    expect(find.textContaining('sede Veloce24 Trento'), findsOneWidget); // i clienti non hanno sede

    // filtro per sede
    await tester.tap(find.widgetWithText(DropdownButtonFormField<int?>, 'Tutte'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Veloce24 Rovereto').last);
    await tester.pumpAndSettle();
    expect(_richieste.last.url.queryParameters['id_sede'], '30');

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

    // cambio di sede: parte solo id_sede
    await tester.tap(find.byTooltip('Modifica dati').first);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Veloce24 Trento · Trento'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Veloce24 Rovereto · Rovereto').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Salva modifiche'));
    await tester.pumpAndSettle();
    expect(_corpo('PATCH', '/api/utenti/12'), {'id_sede': 30});

    // il cliente non ha una sede da scegliere
    await tester.tap(find.byTooltip('Modifica dati').last);
    await tester.pumpAndSettle();
    expect(find.descendant(of: find.byType(AlertDialog), matching: find.text('Sede')), findsNothing);
    await tester.tap(find.text('Annulla'));
    await tester.pumpAndSettle();

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

  testWidgets('sedi: elenco, nuova sede senza controlli, modifica del solo nome, disattivazione', (tester) async {
    await _apri(tester, (api) => SediPage(api: api));
    expect(find.text('Veloce24 Trento'), findsOneWidget);
    expect(find.text('disattivata'), findsOneWidget); // Bolzano
    expect(find.widgetWithText(TextButton, 'Riattiva'), findsOneWidget);

    // nuova sede con i campi vuoti: gli errori del backend vanno sotto i campi
    await tester.tap(find.text('Nuova sede'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Crea sede'));
    await tester.pumpAndSettle();
    expect(_corpo('POST', '/api/sedi'), {
      'nome': '',
      'indirizzo': {'via': '', 'civico': '', 'citta': '', 'cap': '', 'regione': '', 'nazione': 'Italia'},
    });
    expect(find.text('obbligatorio'), findsOneWidget);
    expect(find.text('obbligatoria'), findsOneWidget); // indirizzo.via, sotto il campo Via

    // gruppo del database delle spedizioni: parte solo se scritto, l'errore va sotto il campo
    expect(find.textContaining('0 = principale. Non si cambia dopo la creazione.'), findsOneWidget);
    await tester.enterText(find.widgetWithText(TextField, 'Gruppo database spedizioni'), '5');
    await tester.tap(find.text('Crea sede'));
    await tester.pumpAndSettle();
    expect(_corpo('POST', '/api/sedi')['gruppo_spedizioni'], 5);
    const erroreGruppo = 'nessun database delle spedizioni per il gruppo 5: gruppi configurati 0, 1';
    final gruppo = tester.widget<TextField>(find.widgetWithText(TextField, 'Gruppo database spedizioni'));
    expect(gruppo.decoration!.errorText, erroreGruppo);
    expect(find.text(erroreGruppo), findsOneWidget); // solo sotto il campo, non anche in fondo
    // svuotato: non parte più
    await tester.enterText(find.widgetWithText(TextField, 'Gruppo database spedizioni'), '');
    await tester.tap(find.text('Crea sede'));
    await tester.pumpAndSettle();
    expect(_corpo('POST', '/api/sedi').containsKey('gruppo_spedizioni'), isFalse);
    await tester.tap(find.text('Annulla'));
    await tester.pumpAndSettle();

    // il gruppo è in elenco
    expect(find.textContaining('Database spedizioni: gruppo 0'), findsNWidgets(2));
    expect(find.textContaining('Database spedizioni: gruppo 1'), findsOneWidget); // Rovereto

    // modifica: la città non si cambia, parte solo il nome
    await tester.tap(find.byTooltip('Modifica sede').first);
    await tester.pumpAndSettle();
    // il gruppo si vede ma non si cambia
    expect(find.widgetWithText(TextField, 'Gruppo database spedizioni'), findsNothing);
    expect(find.text('Database spedizioni: gruppo 0 (non si cambia)'), findsOneWidget);
    final citta = tester.widget<TextField>(find.widgetWithText(TextField, 'Città'));
    expect(citta.readOnly, isTrue);
    await tester.enterText(find.widgetWithText(TextField, 'Nome della sede'), 'Trento Centro');
    await tester.tap(find.text('Salva modifiche'));
    await tester.pumpAndSettle();
    expect(_corpo('PATCH', '/api/sedi/10'), {'nome': 'Trento Centro'});

    // disattivazione con conferma
    await tester.tap(find.widgetWithText(TextButton, 'Disattiva').first);
    await tester.pumpAndSettle();
    expect(find.textContaining('non accetterà più nuove spedizioni'), findsOneWidget);
    await tester.tap(find.widgetWithText(FilledButton, 'Disattiva'));
    await tester.pumpAndSettle();
    expect(_corpo('PATCH', '/api/sedi/10'), {'attiva': false});
  });

  testWidgets('home: spedizioni, utenti, sedi e configurazione; filtro per sede nelle spedizioni', (tester) async {
    await _apri(
        tester,
        (api) => HomeAmministratore(
            api: api..utente = UtenteSessione(1, 1, 'amministratore', 'Anna', 'Admin', 'anna@test.it'), alLogout: () {}));
    for (final voce in ['Spedizioni', 'Utenti', 'Sedi', 'Configurazione']) {
      expect(find.text(voce), findsOneWidget);
    }
    // spedizioni: la sede è già scelta (la prima attiva) e la prima richiesta la porta
    expect(find.widgetWithText(DropdownButtonFormField<int>, 'Sede'), findsOneWidget);
    expect(_elenchi, isNotEmpty);
    expect(_elenchi.map((r) => r.url.queryParameters['id_sede']), everyElement('10'));
  });

  testWidgets('spedizioni (amministratore): sede obbligatoria, la prima attiva, mai una richiesta senza id_sede',
      (tester) async {
    _elencoSedi = [_sedi[2], _sedi[1], _sedi[0]]; // Bolzano (disattivata), Rovereto, Trento
    late StateSetter aggiorna;
    var versione = 0;
    await _apri(
        tester,
        (api) => StatefulBuilder(builder: (_, set) {
              aggiorna = set;
              return _spedizioniAmministratore(api, versioneSedi: versione);
            }));
    // prima le sedi, poi le spedizioni della prima sede attiva (Rovereto)
    final primaSedi = _richieste.indexWhere((r) => r.url.path == '/api/sedi');
    final primoElenco = _richieste.indexWhere((r) => r.url.path == '/api/spedizioni');
    expect(primaSedi, greaterThanOrEqualTo(0));
    expect(primoElenco, greaterThan(primaSedi));
    expect(_elenchi.single.url.queryParameters['id_sede'], '30');

    // nel menu solo sedi, nessuna voce "Tutte"
    await tester.tap(find.byType(DropdownButtonFormField<int>));
    await tester.pumpAndSettle();
    expect(find.text('Tutte'), findsNothing);
    expect(find.text('Veloce24 Bolzano (disattivata)'), findsOneWidget);

    // un'altra sede: si rilegge con il suo id_sede
    await tester.tap(find.text('Veloce24 Trento').last);
    await tester.pumpAndSettle();
    expect(_elenchi.last.url.queryParameters['id_sede'], '10');

    // "Azzera filtri" tiene la sede
    await tester.tap(find.text('Azzera filtri'));
    await tester.pumpAndSettle();
    expect(_elenchi.last.url.queryParameters['id_sede'], '10');

    // una sede rinominata (versioneSedi cambia): si rileggono le sedi e resta quella scelta
    _elencoSedi = [_sedi[2], _sedi[1], {..._sedi[0], 'nome': 'Trento Centro'}];
    aggiorna(() => versione++);
    await tester.pumpAndSettle();
    expect(_richieste.where((r) => r.url.path == '/api/sedi').length, 2);
    expect(find.text('Trento Centro'), findsOneWidget);
    expect(_elenchi.last.url.queryParameters['id_sede'], '10');

    // la sede scelta non c'è più: si passa alla prima attiva
    _elencoSedi = [_sedi[1]];
    aggiorna(() => versione++);
    await tester.pumpAndSettle();
    expect(_elenchi.last.url.queryParameters['id_sede'], '30');

    // nessuna richiesta senza id_sede
    expect(_elenchi.every((r) => r.url.queryParameters['id_sede'] != null), isTrue);
  });

  testWidgets('spedizioni (amministratore): con più sedi di quante ne stanno nella tendina si cerca la sede', (tester) async {
    _elencoSedi = [_sedi[0]];
    _totaleSedi = 250; // molte città: la tendina ha solo la prima pagina
    _sediCercabili = [_sede(77, 'Veloce24 Merano', 'Merano', gruppo: 1)];
    await _apri(tester, _spedizioniAmministratore);
    expect(_elenchi.single.url.queryParameters['id_sede'], '10');

    await tester.tap(find.byType(DropdownButtonFormField<int>));
    await tester.pumpAndSettle();
    await tester.tap(find.text("Cerca un'altra sede…").last);
    await tester.pumpAndSettle();
    await tester.enterText(find.descendant(of: find.byType(AlertDialog), matching: find.byType(TextField)), 'Merano');
    await tester.pump(const Duration(milliseconds: 400));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Veloce24 Merano'));
    await tester.pumpAndSettle();

    // la sede trovata diventa quella scelta, ed entra nella tendina
    expect(_elenchi.last.url.queryParameters['id_sede'], '77');
    expect(find.widgetWithText(DropdownButtonFormField<int>, 'Veloce24 Merano'), findsOneWidget);
    expect(_elenchi.every((r) => r.url.queryParameters['id_sede'] != null), isTrue);
  });

  testWidgets('spedizioni (amministratore): sedi non lette, errore con Riprova e nessun elenco senza sede', (tester) async {
    _sediGuaste = true;
    await _apri(tester, _spedizioniAmministratore);
    expect(find.text('Il server ha avuto un problema. Riprova tra poco.'), findsOneWidget);
    expect(find.text('Riprova'), findsOneWidget);
    expect(_elenchi, isEmpty);

    // riprova con le sedi tornate: parte l'elenco della prima sede attiva
    _sediGuaste = false;
    await tester.tap(find.text('Riprova'));
    await tester.pumpAndSettle();
    expect(_elenchi.single.url.queryParameters['id_sede'], '10');
  });

  testWidgets('spedizioni (amministratore): nessuna sede, il messaggio e nessuna richiesta', (tester) async {
    _elencoSedi = [];
    await _apri(tester, _spedizioniAmministratore);
    expect(find.text('Nessuna sede: creane una dalla sezione Sedi.'), findsOneWidget);
    expect(_elenchi, isEmpty);
  });

  testWidgets('spedizioni (operatore): nessuna scelta della sede, nessun id_sede', (tester) async {
    await _apri(tester, (api) => SpedizioniPage(api: api, dettaglio: (id, _) => const SizedBox()));
    expect(find.widgetWithText(DropdownButtonFormField<int>, 'Sede'), findsNothing);
    expect(_richieste.where((r) => r.url.path == '/api/sedi'), isEmpty);
    expect(_elenchi.single.url.queryParameters.containsKey('id_sede'), isFalse);
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
