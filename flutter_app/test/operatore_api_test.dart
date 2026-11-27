// flutter test — client dell'area operatori con un server finto (MockClient).
import 'dart:async';
import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:veloce24_app/operatore/api.dart';

const _sessione = {
  'token': 'tok-123',
  'tipo': 'Bearer',
  'scade_il': '2026-11-12T18:00:00Z',
  'utente': {'id_utente': 1, 'ruolo': 'operatore', 'id': 3, 'nome': 'Oscar', 'cognome': 'Operatore', 'email': 'op@test.it'},
};

const _spedizione = {
  'id': 42,
  'cod_ordine': 'ORD-42',
  'stato': 'in consegna',
  'is_reso': false,
  'creata_il': '2026-11-10T09:00:00Z',
  'dataora_richiesta_spedizione': '2026-11-11T09:00:00Z',
  'id_cliente_mittente': 3,
  'id_cliente_destinatario': 7,
  'indirizzo_ritiro': {
    'via': 'Via Roma',
    'civico': '1',
    'citta': 'Trento',
    'cap': '38121',
    'regione': 'Trentino-Alto Adige',
    'nazione': 'Italia'
  },
  'indirizzo_spedizione': {
    'via': 'Via Verdi',
    'civico': '10',
    'citta': 'Trento',
    'cap': '38121',
    'regione': 'Trentino-Alto Adige',
    'nazione': 'Italia'
  },
  'mittente': {'id': 3, 'nome': 'Marco', 'cognome': 'Mittente', 'email': 'm@test.it', 'telefono': '+39 3330000005'},
  'destinatario': {'id': 7, 'nome': 'Dora', 'cognome': 'Destinataria', 'email': 'd@test.it', 'telefono': '+39 3330000006'},
  'id_corriere': 1,
  'corriere': {'id': 1, 'nome': 'Carlo', 'cognome': 'Corriere', 'telefono': '+39 3330000003'},
  'conclusa_il': null,
  'annullata_il': null,
  'tentativi_ritiro_falliti': 0,
  'tentativi_consegna_falliti': 1,
  'note': [
    {
      'id': 5,
      'nota': 'citofono rotto',
      'is_interna': false,
      'motivo': 'destinatario assente',
      'creata_il': '2026-11-11T10:00:00Z',
      'autore': {'ruolo': 'corriere', 'id': 1}
    },
  ],
};

http.Response _json(Object corpo, int status) =>
    http.Response.bytes(utf8.encode(jsonEncode(corpo)), status, headers: {'content-type': 'application/json; charset=utf-8'});

ApiOperatore _api(Future<http.Response> Function(http.Request) server) =>
    ApiOperatore(base: 'http://backend:8080/', client: MockClient(server));

void main() {
  _testRinnovo();

  test('login (i valori partono come inseriti): salva il token e lo manda come Bearer nelle chiamate successive', () async {
    final richieste = <http.Request>[];
    final api = _api((req) async {
      richieste.add(req);
      if (req.url.path == '/api/auth/login') return http.Response(jsonEncode(_sessione), 200);
      return http.Response(
          jsonEncode({
            'dati': [_spedizione],
            'pagina': 1,
            'per_pagina': 25
          }),
          200);
    });
    final u = await api.login(' op@test.it ', 'Password1');
    expect(u.nomeCompleto, 'Oscar Operatore');
    expect(jsonDecode(richieste.first.body), {'email': ' op@test.it ', 'password': 'Password1'});

    final p = await api.spedizioni(
      FiltroSpedizioni(stato: 'in consegna', idCorriere: 1, dal: DateTime(2026, 11, 1), al: DateTime(2026, 11, 30), codOrdine: ' ORD '),
      perPagina: 25,
    );
    final elenco = richieste.last;
    expect(elenco.headers['Authorization'], 'Bearer tok-123');
    expect(elenco.url.queryParameters, {
      'stato': 'in consegna',
      'id_corriere': '1',
      'dal': '2026-11-01',
      'al': '2026-11-30',
      'cod_ordine': ' ORD ',
      'pagina': '1',
      'per_pagina': '25',
    });
    final s = p.dati.single;
    expect(s.mittente!.nomeCompleto, 'Marco Mittente');
    expect(s.consegna.testo, 'Via Verdi 10, 38121 Trento (Trentino-Alto Adige) Italia');
    expect(s.note.single.interna, isFalse);
  });

  test('il login vale per ogni ruolo: chi può entrare lo decide il backend', () async {
    for (final ruolo in ['operatore', 'corriere', 'amministratore', 'cliente']) {
      final api = _api((_) async => http.Response(
          jsonEncode({
            ..._sessione,
            'utente': {...(_sessione['utente'] as Map), 'ruolo': ruolo}
          }),
          200));
      expect((await api.login('u@test.it', 'x')).ruolo, ruolo);
      expect(api.token, 'tok-123');
    }
    // il rifiuto del backend (es. cliente solo mittente) arriva all'utente con il suo messaggio
    final api = _api((_) async => _json({'errore': 'accesso riservato ai clienti destinatari: non risulta nessuna spedizione destinata a te'}, 403));
    await expectLater(
      api.login('mittente@test.it', 'x'),
      throwsA(isA<ErroreServer>().having((e) => e.perUtente, 'messaggio', contains('clienti destinatari'))),
    );
    expect(api.token, isNull);
  });

  test('cambio di stato: PATCH con motivo, nota e Idempotency-Key', () async {
    late http.Request inviata;
    final api = _api((req) async {
      inviata = req;
      return http.Response(jsonEncode(_spedizione), 200);
    });
    await api.cambiaStato(42, 'persa', motivo: 'furto', nota: ' furgone forzato ');
    expect(inviata.method, 'PATCH');
    expect(inviata.url.path, '/api/spedizioni/42/stato');
    expect(jsonDecode(inviata.body), {'stato': 'persa', 'motivo': 'furto', 'nota': ' furgone forzato '});
    expect(inviata.headers['Idempotency-Key'], hasLength(32));
  });

  test('gli errori per campo arrivano nell\'eccezione', () async {
    final api = _api((_) async => _json({
          'errore': 'dati non validi',
          'dettagli': [
            {'campo': 'email', 'messaggio': 'già usata'}
          ]
        }, 409));
    await expectLater(
      api.creaCliente(nome: 'A', cognome: 'B', prefisso: '+39', telefono: '333', email: 'x@test.it', password: 'Password1', chiave: 'k'),
      throwsA(isA<ErroreServer>().having((e) => e.campi['email'], 'campo email', 'già usata')),
    );
  });

  test('401 con token: esce e avvisa che la sessione è scaduta', () async {
    var risposte = 0;
    final api = _api((req) async =>
        risposte++ == 0 ? http.Response(jsonEncode(_sessione), 200) : http.Response('{"errore":"token non valido o scaduto"}', 401));
    var scaduto = false;
    api.alScadereDelToken = () => scaduto = true;
    await api.login('op@test.it', 'Password1');
    await expectLater(api.dettaglio(42), throwsA(isA<ErroreServer>()));
    expect(scaduto, isTrue);
    expect(api.utente, isNull);
  });

  test('creazione: legge la risposta piatta del backend, manda la data richiesta e non quella del ritiro', () async {
    late http.Request inviata;
    final api = _api((req) async {
      inviata = req;
      // POST /api/spedizioni risponde con la spedizione "piatta": niente persone né indirizzi
      return _json({
        'id': 77,
        'cod_ordine': 'ORD-77',
        'stato': 'assegnata',
        'is_reso': false,
        'creata_il': '2026-11-13T09:00:00Z',
        'dataora_richiesta_spedizione': '2026-11-13T09:00:00Z',
        'ritirata_il': null,
        'id_cliente_mittente': 3,
        'id_cliente_destinatario': 7,
        'id_indirizzo_ritiro': 1,
        'id_indirizzo_spedizione': 2,
        'id_corriere': 1,
        'annullata_il': null,
        'tentativi_ritiro_falliti': 0,
        'tentativi_consegna_falliti': 0,
      }, 201);
    });
    final s = await api.creaSpedizione(
        codOrdine: 'ORD-77',
        idMittente: 3,
        idDestinatario: 7,
        idRitiro: 1,
        idConsegna: 2,
        richiestaIl: DateTime.utc(2026, 11, 20, 9),
        idCorriere: 1,
        chiave: 'k');
    expect((s.id, s.codOrdine, s.stato), (77, 'ORD-77', 'assegnata'));
    final corpo = jsonDecode(inviata.body) as Map<String, dynamic>;
    expect(corpo['dataora_richiesta_spedizione'], '2026-11-20T09:00:00.000Z');
    expect(corpo.containsKey('ritirata_il'), isFalse);
    expect(corpo['id_corriere'], 1);
  });

  test('messaggi per l\'utente brevi, dettagli tecnici a parte', () async {
    final api = _api((_) async => _json({
          'errore': 'dati non validi',
          'dettagli': [
            {'campo': 'cod_ordine', 'messaggio': 'obbligatorio'},
            {'campo': 'id_cliente_mittente', 'messaggio': 'obbligatorio'},
          ]
        }, 400));
    try {
      await api.creaSpedizione(codOrdine: '', chiave: 'k');
      fail('atteso un errore');
    } on ErroreServer catch (e) {
      expect(e.perUtente, 'Alcuni dati non sono validi: controlla i campi evidenziati.');
      expect(e.campi['cod_ordine'], 'obbligatorio');
      expect(e.tecnico, contains('POST http://backend:8080/api/spedizioni → 400'));
    }
    expect(ErroreServer(409, 'cod_ordine già usato').perUtente, 'cod_ordine già usato');
    expect(ErroreServer(500, 'panic: nil pointer').perUtente, isNot(contains('panic')));
    expect(ErroreServer(null, 'XMLHttpRequest error').perUtente, isNot(contains('XMLHttpRequest')));
  });

  test('motivi: elenco unico da GET /api/motivi, senza filtri', () async {
    final api = _api((req) async {
      expect(req.url.path, '/api/motivi');
      return _json({
        'dati': ['mittente assente', 'richiesta del mittente', 'furto', 'altro'],
      }, 200);
    });
    expect(await api.motivi(), ['mittente assente', 'richiesta del mittente', 'furto', 'altro']);
  });

  test('autore: nome, cognome, ruolo e id', () {
    expect(Autore.daJson({'ruolo': 'corriere', 'id': 4, 'nome': 'Carlo', 'cognome': 'Corriere'}).testo,
        'Carlo Corriere (corriere · id 4)');
    expect(Autore.daJson({'ruolo': 'operatore', 'id': 1}).testo, 'operatore · id 1');
  });

  test('posizioni: trova il corriere di una spedizione in consegna', () async {
    final api = _api((_) async => http.Response(
        jsonEncode({
          'intervallo_secondi': 30,
          'corrieri': [
            {
              'id_corriere': 1,
              'nome': 'Carlo',
              'cognome': 'Corriere',
              'latitudine': 46,
              'longitudine': 11.12,
              'rilevata_il': '2026-11-12T10:00:00Z',
              'spedizioni_attive': [42, 43]
            },
          ],
        }),
        200));
    final p = await api.posizioni();
    expect(p.perSpedizione(43)!.latitudine, 46.0);
    expect(p.perSpedizione(99), isNull);
  });

  test("paginazione: totali dal backend, posizione da–a di N e niente pagina successiva dopo l'ultima", () async {
    final api = _api((req) async => _json({
          'dati': [_spedizione, _spedizione],
          'pagina': 3,
          'per_pagina': 25,
          'totale_elementi': 52,
          'totale_pagine': 3,
        }, 200));
    final p = await api.spedizioni(const FiltroSpedizioni(), pagina: 3, perPagina: 25);
    expect((p.totaleElementi, p.totalePagine, p.da, p.a), (52, 3, 51, 52));
    expect(p.haPrecedente, isTrue);
    expect(p.haSuccessiva, isFalse); // ultima pagina: "successiva" spenta

    // pagina oltre la fine (es. dopo una modifica): va riconosciuta per tornare all'ultima
    final oltre = Pagina<int>([], 4, 25, 52, 3);
    expect(oltre.oltreLaFine, isTrue);
    expect(Pagina<int>([], 1, 25, 0, 0).oltreLaFine, isFalse);
  });

  test("assegnata da: nome, cognome, email e telefono dell'operatore", () {
    final s = Spedizione.daJson({
      ..._spedizione,
      'assegnata_da': {'id': 2, 'nome': 'Olga', 'cognome': 'Operatrice', 'email': 'olga@test.it', 'telefono': '+39 3330000012'},
    });
    expect((s.assegnataDa!.id, s.assegnataDa!.nomeCompleto, s.assegnataDa!.email, s.assegnataDa!.telefono),
        (2, 'Olga Operatrice', 'olga@test.it', '+39 3330000012'));
    expect(Spedizione.daJson(_spedizione).assegnataDa, isNull);
  });
}

// Il token dura poco (15 minuti nel backend): oltre metà della durata l'app lo rinnova da
// sola prima della richiesta; se il backend rifiuta il rinnovo (utente disattivato) la
// sessione si chiude.
Map<String, Object?> _sessioneCheScadeTra(Duration d, String token) => {
      ..._sessione,
      'token': token,
      'scade_il': DateTime.now().add(d).toUtc().toIso8601String(),
    };

void _testRinnovo() {
  test('a ridosso della scadenza il token si rinnova prima della richiesta', () async {
    final viste = <String>[];
    final api = _api((req) async {
      viste.add('${req.method} ${req.url.path} ${req.headers['Authorization']}');
      return switch (req.url.path) {
        '/api/auth/login' => http.Response(jsonEncode(_sessioneCheScadeTra(const Duration(seconds: 2), 'tok-vecchio')), 200),
        '/api/auth/rinnova' => http.Response(jsonEncode(_sessioneCheScadeTra(const Duration(minutes: 15), 'tok-nuovo')), 200),
        _ => http.Response(jsonEncode(_spedizione), 200),
      };
    });
    final ricevuti = <String>[];
    api.alRinnovoToken.add(ricevuti.add);
    await api.login('op@test.it', 'x');
    // token di 2 s: dopo 1,6 s manca meno di un quarto della durata, quindi si aspetta il rinnovo
    await Future<void>.delayed(const Duration(milliseconds: 1600));
    await api.dettaglio(42);
    expect(viste, [
      'POST /api/auth/login null',
      'POST /api/auth/rinnova Bearer tok-vecchio',
      'GET /api/spedizioni/42 Bearer tok-nuovo',
    ]);
    expect(ricevuti, ['tok-nuovo']); // es. il worker del GPS
    await api.dettaglio(42); // token appena rinnovato: niente altro rinnovo
    expect(viste.where((v) => v.contains('rinnova')), hasLength(1));
  });

  test('a metà della durata il rinnovo parte in sottofondo: la richiesta non lo aspetta', () async {
    final viste = <String>[];
    final rinnovo = Completer<http.Response>();
    final api = _api((req) {
      viste.add('${req.method} ${req.url.path} ${req.headers['Authorization']}');
      if (req.url.path == '/api/auth/rinnova') return rinnovo.future; // servizio utenti lento
      if (req.url.path == '/api/auth/login') {
        return Future.value(http.Response(jsonEncode({..._sessioneCheScadeTra(const Duration(seconds: 8), 'tok-vecchio'), 'durata_secondi': 8}), 200));
      }
      return Future.value(http.Response(jsonEncode(_spedizione), 200));
    });
    await api.login('op@test.it', 'x');
    await Future<void>.delayed(const Duration(milliseconds: 4500)); // oltre metà di 8 s, lontano dalla scadenza
    await api.dettaglio(42); // non aspetta il rinnovo ancora in corso
    expect(viste, ['POST /api/auth/login null', 'POST /api/auth/rinnova Bearer tok-vecchio', 'GET /api/spedizioni/42 Bearer tok-vecchio']);
    rinnovo.complete(http.Response(jsonEncode({..._sessioneCheScadeTra(const Duration(minutes: 15), 'tok-nuovo'), 'durata_secondi': 900}), 200));
    await Future<void>.delayed(Duration.zero);
    await api.dettaglio(42);
    expect(viste.last, 'GET /api/spedizioni/42 Bearer tok-nuovo');
  });

  test('rinnovo non riuscito per la rete: un 401 non fa uscire, si riprova più tardi', () async {
    var scaduto = false;
    final api = _api((req) async {
      switch (req.url.path) {
        case '/api/auth/login':
          return http.Response(jsonEncode({..._sessioneCheScadeTra(const Duration(minutes: 15), 'tok'), 'durata_secondi': 900}), 200);
        case '/api/auth/rinnova':
          throw http.ClientException('rete assente');
      }
      return http.Response('{"errore":"token non valido o scaduto"}', 401);
    });
    api.alScadereDelToken = () => scaduto = true;
    await api.login('op@test.it', 'x');
    await expectLater(api.dettaglio(42), throwsA(isA<ErroreServer>()));
    expect(scaduto, isFalse);
    expect(api.token, 'tok');
  });

  test('rinnovo rifiutato (utente disattivato): la sessione si chiude', () async {
    final api = _api((req) async => switch (req.url.path) {
          '/api/auth/login' => http.Response(jsonEncode(_sessioneCheScadeTra(const Duration(seconds: 2), 'tok')), 200),
          '/api/auth/rinnova' => http.Response('{"errore":"account disattivato"}', 403),
          _ => http.Response(jsonEncode(_spedizione), 200),
        });
    var scaduto = false;
    api.alScadereDelToken = () => scaduto = true;
    await api.login('op@test.it', 'x');
    await Future<void>.delayed(const Duration(milliseconds: 1200));
    await api.rinnovaSeServe();
    expect(scaduto, isTrue);
    expect(api.token, isNull);
  });

  test("403 da un servizio: il rinnovo verifica l'account e, se disattivato, chiude la sessione", () async {
    final api = _api((req) async => switch (req.url.path) {
          '/api/auth/login' => http.Response(jsonEncode({..._sessioneCheScadeTra(const Duration(minutes: 15), 'tok'), 'durata_secondi': 900}), 200),
          '/api/auth/rinnova' => http.Response('{"errore":"account disattivato"}', 403),
          _ => http.Response('{"errore":"account disattivato"}', 403),
        });
    var scaduto = false;
    api.alScadereDelToken = () => scaduto = true;
    await api.login('op@test.it', 'x');
    await expectLater(api.dettaglio(42), throwsA(isA<ErroreServer>().having((e) => e.status, 'status', 403)));
    expect(scaduto, isTrue);
    expect(api.token, isNull);
  });

  test('403 per un permesso mancante: la sessione resta', () async {
    final api = _api((req) async => switch (req.url.path) {
          '/api/auth/login' => http.Response(jsonEncode({..._sessioneCheScadeTra(const Duration(minutes: 15), 'tok'), 'durata_secondi': 900}), 200),
          '/api/auth/rinnova' => http.Response(jsonEncode({..._sessioneCheScadeTra(const Duration(minutes: 15), 'tok-nuovo'), 'durata_secondi': 900}), 200),
          _ => http.Response('{"errore":"solo un operatore"}', 403),
        });
    var scaduto = false;
    api.alScadereDelToken = () => scaduto = true;
    await api.login('op@test.it', 'x');
    await expectLater(api.dettaglio(42), throwsA(isA<ErroreServer>().having((e) => e.status, 'status', 403)));
    expect(scaduto, isFalse);
    expect(api.token, 'tok-nuovo');
  });

  test('token scaduto (app sospesa): il 401 rinnova e ripete la richiesta una volta', () async {
    final viste = <String>[];
    var primaVolta = true;
    final api = _api((req) async {
      viste.add('${req.method} ${req.url.path} ${req.headers['Authorization']}');
      switch (req.url.path) {
        case '/api/auth/login':
          return http.Response(jsonEncode({..._sessioneCheScadeTra(const Duration(minutes: 15), 'tok-vecchio'), 'durata_secondi': 900}), 200);
        case '/api/auth/rinnova':
          return http.Response(jsonEncode({..._sessioneCheScadeTra(const Duration(minutes: 15), 'tok-nuovo'), 'durata_secondi': 900}), 200);
      }
      if (primaVolta) {
        primaVolta = false;
        return http.Response('{"errore":"token non valido o scaduto"}', 401);
      }
      return http.Response(jsonEncode(_spedizione), 200);
    });
    await api.login('op@test.it', 'x');
    await api.dettaglio(42);
    expect(viste, [
      'POST /api/auth/login null',
      'GET /api/spedizioni/42 Bearer tok-vecchio',
      'POST /api/auth/rinnova Bearer tok-vecchio',
      'GET /api/spedizioni/42 Bearer tok-nuovo',
    ]);
    expect(api.token, 'tok-nuovo');
  });

  test('logout: chiude la sessione sul server, poi esce anche se il server non risponde', () async {
    final viste = <String>[];
    var rete = true;
    final api = _api((req) async {
      viste.add('${req.method} ${req.url.path} ${req.headers['Authorization']}');
      if (!rete) throw http.ClientException('rete assente');
      return req.url.path == '/api/auth/login'
          ? http.Response(jsonEncode({..._sessioneCheScadeTra(const Duration(minutes: 15), 'tok'), 'durata_secondi': 900}), 200)
          : http.Response('', 204);
    });
    await api.login('op@test.it', 'x');
    await api.logout();
    expect(viste.last, 'POST /api/auth/esci Bearer tok');
    expect(api.token, isNull);

    await api.login('op@test.it', 'x');
    rete = false;
    await api.logout();
    expect(api.token, isNull);
  });

  test('logout durante un rinnovo: il token che arriva dopo non riapre la sessione', () async {
    final risposta = Completer<http.Response>();
    final api = _api((req) {
      if (req.url.path == '/api/auth/rinnova') return risposta.future; // risponde dopo il logout
      return Future.value(http.Response(jsonEncode(_sessioneCheScadeTra(const Duration(minutes: 15), 'tok')), 200));
    });
    await api.login('op@test.it', 'x');
    final rinnovo = api.rinnova();
    api.esci();
    risposta.complete(http.Response(jsonEncode(_sessioneCheScadeTra(const Duration(minutes: 15), 'tok-nuovo')), 200));
    expect(await rinnovo, isFalse);
    expect(api.token, isNull);
    expect(api.utente, isNull);
  });
}
