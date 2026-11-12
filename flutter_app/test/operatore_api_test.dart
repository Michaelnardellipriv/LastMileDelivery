// flutter test — client dell'area operatori con un server finto (MockClient).
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

  test('il login di un non operatore viene rifiutato', () async {
    final api = _api((_) async => http.Response(
        jsonEncode({
          ..._sessione,
          'utente': {...(_sessione['utente'] as Map), 'ruolo': 'corriere'}
        }),
        200));
    await expectLater(api.login('c@test.it', 'x'), throwsA(isA<ErroreServer>().having((e) => e.status, 'status', 403)));
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
}
