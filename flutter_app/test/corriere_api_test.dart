// flutter test — client dell'area corrieri con un server finto (MockClient).
import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:veloce24_app/corriere/api.dart';

const _sessione = {
  'token': 'tok-corriere',
  'tipo': 'Bearer',
  'scade_il': '2026-11-16T18:00:00Z',
  'utente': {'id_utente': 2, 'ruolo': 'corriere', 'id': 1, 'nome': 'Carlo', 'cognome': 'Corriere', 'email': 'c@test.it'},
};

/// Risposta "piatta" di PATCH /stato e POST /tentativi.
Map<String, Object?> _piatta(String stato, {int ritiri = 0, int consegne = 0}) => {
      'id': 42,
      'cod_ordine': 'ORD-42',
      'stato': stato,
      'is_reso': false,
      'creata_il': '2026-11-10T09:00:00Z',
      'dataora_richiesta_spedizione': '2026-11-11T09:00:00Z',
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

http.Response _json(Object corpo, int status) =>
    http.Response.bytes(utf8.encode(jsonEncode(corpo)), status, headers: {'content-type': 'application/json; charset=utf-8'});

ApiCorriere _api(Future<http.Response> Function(http.Request) server) =>
    ApiCorriere(base: 'http://backend:8080', client: MockClient(server));

void main() {
  test('login del corriere: salva il token e lo manda come Bearer', () async {
    final richieste = <http.Request>[];
    final api = _api((req) async {
      richieste.add(req);
      if (req.url.path == '/api/auth/login') return _json(_sessione, 200);
      return _json({'dati': [], 'pagina': 1, 'per_pagina': 20}, 200);
    });
    final u = await api.login('c@test.it', 'Password1');
    expect(u.nomeCompleto, 'Carlo Corriere');

    await api.spedizioni(FiltroSpedizioni(stato: 'in consegna', dal: DateTime(2026, 11, 1), al: DateTime(2026, 11, 30), codOrdine: 'ORD'));
    expect(richieste.last.headers['Authorization'], 'Bearer tok-corriere');
    // ricerca per numero ordine, stato e data; nessun filtro per corriere: le sue spedizioni le sceglie il backend
    expect(richieste.last.url.queryParameters, {
      'stato': 'in consegna',
      'dal': '2026-11-01',
      'al': '2026-11-30',
      'cod_ordine': 'ORD',
      'pagina': '1',
      'per_pagina': '10', // 10 spedizioni per pagina, come il backend
    });
  });

  test('tentativo non riuscito: POST con tipo, motivo, nota e Idempotency-Key', () async {
    late http.Request inviata;
    final api = _api((req) async {
      inviata = req;
      return _json(_piatta('assegnata', ritiri: 2), 201);
    });
    final esito = await api.registraTentativo(42, tipo: 'ritiro', motivo: 'mittente assente', nota: ' negozio chiuso ', chiave: 'chiave-1');
    expect(inviata.method, 'POST');
    expect(inviata.url.path, '/api/spedizioni/42/tentativi');
    // i valori partono come inseriti: li controlla il backend
    expect(jsonDecode(inviata.body), {'tipo': 'ritiro', 'motivo': 'mittente assente', 'nota': ' negozio chiuso '});
    expect(inviata.headers['Idempotency-Key'], 'chiave-1');
    expect((esito.stato, esito.tentativiRitiro), ('assegnata', 2));
  });

  test('tentativo senza dati: i campi partono vuoti e gli errori del backend arrivano per campo', () async {
    late http.Request inviata;
    final api = _api((req) async {
      inviata = req;
      return _json({
        'errore': 'dati non validi',
        'dettagli': [
          {'campo': 'tipo', 'messaggio': 'valore non previsto: ammessi ritiro, consegna'},
          {'campo': 'nota', 'messaggio': 'non può essere vuota'},
          {'campo': 'motivo', 'messaggio': 'valore non previsto: usare uno dei motivi di GET /api/motivi'},
        ]
      }, 400);
    });
    try {
      await api.registraTentativo(42, tipo: '', motivo: '', nota: '', chiave: 'k');
      fail('atteso un errore');
    } on ErroreServer catch (e) {
      // nota vuota e motivo vuoto arrivano davvero al backend
      expect(jsonDecode(inviata.body), {'tipo': '', 'motivo': '', 'nota': ''});
      expect(e.campi.keys, containsAll(['tipo', 'nota', 'motivo']));
      expect(e.perUtente, 'Alcuni dati non sono validi: controlla i campi evidenziati.');
    }
  });

  test('ultimo tentativo di consegna: lo stato registrato dal backend è non consegnata', () async {
    final api = _api((_) async => _json(_piatta('non consegnata', consegne: 3), 201));
    final esito = await api.registraTentativo(42, tipo: 'consegna', motivo: 'destinatario assente', nota: 'x', chiave: 'k');
    expect((esito.stato, esito.tentativiConsegna), ('non consegnata', 3));
  });

  test('cambio di stato: PATCH e stato registrato dal backend', () async {
    late http.Request inviata;
    final api = _api((req) async {
      inviata = req;
      return _json(_piatta('ritirata'), 200);
    });
    expect(await api.cambiaStato(42, 'ritirata'), 'ritirata');
    expect(inviata.method, 'PATCH');
    expect(jsonDecode(inviata.body), {'stato': 'ritirata'});
    expect(inviata.headers['Idempotency-Key'], hasLength(32));
  });

  test('passaggio non ammesso: il messaggio del backend arriva all\'utente', () async {
    final api = _api((_) async => _json({'errore': 'da "assegnata" non si può passare a "consegnata"'}, 409));
    await expectLater(
      api.cambiaStato(42, 'consegnata'),
      throwsA(isA<ErroreServer>().having((e) => e.perUtente, 'messaggio', 'da "assegnata" non si può passare a "consegnata"')),
    );
  });

  test('posizione simulata: POST con le coordinate, il backend risponde con il suo intervallo', () async {
    late http.Request inviata;
    final api = _api((req) async {
      inviata = req;
      return _json({'intervallo_secondi': 15}, 201);
    });
    expect(await api.inviaPosizione(46.0679, 11.1211), 15);
    expect(inviata.method, 'POST');
    expect(inviata.url.path, '/api/corrieri/posizione');
    expect(jsonDecode(inviata.body), {'latitudine': 46.0679, 'longitudine': 11.1211});
  });

  test('nota interna: POST /note con il testo inserito', () async {
    late http.Request inviata;
    final api = _api((req) async {
      inviata = req;
      return _json({'id': 9, 'nota': 'pacco caricato', 'is_interna': true, 'motivo': null, 'creata_il': '2026-11-16T10:00:00Z'}, 201);
    });
    await api.aggiungiNota(42, 'pacco caricato');
    expect(inviata.url.path, '/api/spedizioni/42/note');
    expect(jsonDecode(inviata.body), {'nota': 'pacco caricato'});
  });
}
