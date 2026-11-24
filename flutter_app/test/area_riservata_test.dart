// flutter test — flusso unico: LoginPage (auth/login_page.dart) → AreaRiservata instrada per ruolo.
// Backend finto (MockClient): ogni ruolo entra dalla stessa pagina e arriva alla sua area; chi può
// entrare lo decide il backend, che valida anche le credenziali (il client non blocca nulla).
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:veloce24_app/auth/recupero_link.dart';
import 'package:veloce24_app/main.dart';
import 'package:veloce24_app/store.dart';
import 'package:veloce24_app/auth/session.dart';

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
        'scade_il': '2099-01-01T00:00:00Z',
        'utente': {'id_utente': 1, 'ruolo': ruolo, 'id': 1, 'nome': 'Utente', 'cognome': 'Prova', 'email': email},
      }, 200);
    case '/api/auth/reimposta-password':
      return http.Response('', 204);
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

/// Aspetta un tempo limitato (risposte del finto backend, animazioni, snackbar). Non si usa
/// `pumpAndSettle`: le aree dei ruoli hanno indicatori e aggiornamenti periodici, quindi lo
/// schermo non si ferma mai e il test andrebbe in timeout.
Future<void> _assesta(WidgetTester tester) async {
  for (var i = 0; i < 15; i++) {
    await tester.pump(const Duration(milliseconds: 100));
  }
}

Future<void> _avvia(WidgetTester tester) async {
  await tester.pumpWidget(App(client: MockClient(_backend)));
  await _assesta(tester);
}

Future<void> _login(WidgetTester tester, String email, {String password = 'PasswordProva1'}) async {
  await _avvia(tester);
  await tester.enterText(find.widgetWithText(TextField, 'Email'), email);
  await tester.enterText(find.widgetWithText(TextField, 'Password'), password);
  await tester.tap(find.text('Accedi'));
  await _assesta(tester);
}

/// Schermo largo: il profilo mostra "Cambia password" insieme ai dati (sotto 840 px usa due schede).
void _schermoLargo(WidgetTester tester) {
  tester.view.physicalSize = const Size(1200, 900);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
}

/// Smonta le pagine a fine test, così i timer di aggiornamento si fermano.
Future<void> _chiudi(WidgetTester tester) => tester.pumpWidget(const SizedBox());

void main() {
  setUp(() async {
    store.usaStorage(MemoriaSessioneStorage());
    apiBase = 'http://backend:8080'; // con API_BASE vuoto l'app parte in demo
    tokenRecuperoDaLink.value = null;
    _richieste.clear();
    await store.esci();
  });

  testWidgets("all'avvio si vede il nuovo login, non la vecchia pagina con gli utenti di prova", (tester) async {
    await _avvia(tester);
    expect(find.text('Accedi'), findsOneWidget);
    expect(find.text('Password dimenticata?'), findsOneWidget);
    expect(find.textContaining('non ha ancora un endpoint di login'), findsNothing);
    expect(find.text('Area riservata'), findsNothing);
  });

  testWidgets("operatore: dopo il login si apre l'area operatori", (tester) async {
    await _login(tester, 'operatore@test.it');
    expect(find.text('Nuova spedizione'), findsWidgets);
    expect(find.text('Corrieri'), findsWidgets);
    expect(find.text('Clienti'), findsWidgets);
    await _chiudi(tester);
  });

  testWidgets("corriere: dopo il login si apre l'elenco delle sue spedizioni", (tester) async {
    await _login(tester, 'corriere@test.it');
    expect(find.text('Le mie spedizioni'), findsOneWidget);
    expect(find.text('Numero ordine'), findsWidgets);
    await _chiudi(tester);
  });

  testWidgets("amministratore: dopo il login si apre l'area amministratori", (tester) async {
    await _login(tester, 'amministratore@test.it');
    expect(find.text('Spedizioni'), findsWidgets);
    expect(find.text('Utenti'), findsWidgets);
    expect(find.text('Configurazione'), findsWidgets);
    await _chiudi(tester);
  });

  testWidgets('cliente destinatario: dopo il login si apre il tracking dei suoi pacchi', (tester) async {
    await _login(tester, 'cliente@test.it');
    expect(find.text('I miei pacchi'), findsOneWidget);
    // l'elenco arriva dal backend con il token del login, non con l'header di sviluppo
    final elenco = _richieste.lastWhere((r) => r.url.path == '/api/spedizioni');
    expect(elenco.headers['Authorization'], 'Bearer tok-cliente');
    expect(elenco.headers.containsKey('X-Id-Cliente'), isFalse);

    // "Esci" riporta al login
    await tester.tap(find.byTooltip('Esci'));
    await _assesta(tester);
    expect(find.text('Accedi'), findsOneWidget);
    expect(store.sessione, isNull);
  });

  testWidgets('il profilo è raggiungibile da ogni area (qui: operatore) e si può uscire da lì', (tester) async {
    _schermoLargo(tester);
    await _login(tester, 'operatore@test.it');
    await tester.tap(find.byTooltip('Il mio profilo'));
    await _assesta(tester);
    expect(find.text('Cambia password'), findsOneWidget);

    await tester.tap(find.text('Esci').first);
    await _assesta(tester);
    expect(find.text('Accedi'), findsOneWidget);
    await _chiudi(tester);
  });

  testWidgets('cliente solo mittente: il backend rifiuta il login e la pagina mostra il suo messaggio', (tester) async {
    await _login(tester, 'mittente@test.it');
    expect(find.text('accesso riservato ai clienti destinatari: non risulta nessuna spedizione destinata a te'), findsOneWidget);
    expect(find.text('Accedi'), findsOneWidget);
  });

  testWidgets('credenziali vuote o sbagliate: nessun controllo nel client, risponde il backend', (tester) async {
    await _login(tester, 'operatore@test.it', password: '');
    expect(_richieste.where((r) => r.url.path == '/api/auth/login'), hasLength(1), reason: 'la richiesta deve partire');
    expect(find.text('obbligatoria'), findsOneWidget); // l'errore 400 del backend

    await _login(tester, 'non-una-email', password: 'Sbagliata1'); // formato email non controllato dal client
    expect(find.text('email o password non corrette'), findsOneWidget);
    expect(find.text('Accedi'), findsOneWidget);
  });

  testWidgets('link di recupero con sessione già aperta: appare "nuova password" e poi si torna alla propria area', (tester) async {
    await _login(tester, 'operatore@test.it');
    expect(find.text('Nuova spedizione'), findsOneWidget);

    tokenRecuperoDaLink.value = 'tok-reset'; // link ricevuto mentre l'app è già in uso
    await _assesta(tester);
    expect(find.text('Scegli la nuova password'), findsOneWidget);

    await tester.enterText(find.widgetWithText(TextFormField, 'Nuova password'), 'x');
    await tester.enterText(find.widgetWithText(TextFormField, 'Conferma nuova password'), 'x');

    // Schermo di default 800x600: il modulo è in uno scroll e il pulsante finisce sotto il bordo.
    // Come farebbe l'utente, si scorre fino al pulsante prima di toccarlo.
    final impostaPassword = find.text('Imposta nuova password');
    await tester.ensureVisible(impostaPassword);
    await tester.pump();
    await tester.tap(impostaPassword);
    await _assesta(tester);

    final reset = _richieste.lastWhere((r) => r.url.path == '/api/auth/reimposta-password');
    expect(jsonDecode(reset.body), {'token': 'tok-reset', 'nuova_password': 'x'}); // anche una password corta parte: decide il backend
    expect(tokenRecuperoDaLink.value, isNull);
    expect(find.text('Nuova spedizione'), findsOneWidget);
    await _chiudi(tester);
  });

  testWidgets('link di recupero senza sessione: si apre "nuova password" invece del login', (tester) async {
    tokenRecuperoDaLink.value = 'tok-reset';
    await _avvia(tester);
    expect(find.text('Scegli la nuova password'), findsOneWidget);
    expect(find.text('Accedi'), findsNothing);
  });
}
