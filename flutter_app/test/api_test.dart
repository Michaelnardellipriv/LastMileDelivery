// flutter test — prova il client senza backend, con un server finto (MockClient).
import 'dart:convert';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:veloce24_app/api.dart';

const _voce = {
  'id': 4, 'cod_ordine': 'ORD-4', 'stato': 'consegnata', 'is_reso': false,
  'creata_il': '2026-11-03T21:46:04Z', 'dataora_richiesta_spedizione': '2026-11-03T21:46:04Z',
  'id_cliente_mittente': 1, 'id_cliente_destinatario': 2,
  'indirizzo_ritiro': {'via': 'Via Roma', 'civico': '1'},
  'indirizzo_spedizione': {'via': 'Via Verdi', 'civico': '10'},
  'id_corriere': 1, 'conclusa_il': '2026-11-03T21:46:04Z', 'note': [],
};

void main() {
  test('manda l\'header di identità e legge l\'elenco', () async {
    late http.Request inviata;
    final api = ApiClient(
      base: 'http://backend:8080/',
      ruolo: RuoloApi.corriere,
      idUtente: 7,
      client: MockClient((req) async {
        inviata = req;
        return http.Response(jsonEncode({'dati': [_voce], 'pagina': 1, 'per_pagina': 20}), 200);
      }),
    );
    final p = await api.elencoSpedizioni(stato: 'in consegna');
    expect(inviata.headers['X-Id-Corriere'], '7');
    expect(inviata.url.toString(), startsWith('http://backend:8080/api/spedizioni?'));
    expect(inviata.url.queryParameters['stato'], 'in consegna');
    expect(p.dati.single.codOrdine, 'ORD-4');
    expect(p.dati.single.consegna.testo, 'Via Verdi 10');
  });

  test('filtri per data e numero ordine nel formato del backend', () async {
    late Uri url;
    final api = ApiClient(
      base: 'http://backend:8080',
      ruolo: RuoloApi.cliente,
      idUtente: 2,
      client: MockClient((req) async {
        url = req.url;
        return http.Response(jsonEncode({'dati': [], 'pagina': 1, 'per_pagina': 20}), 200);
      }),
    );
    await api.elencoSpedizioni(dal: DateTime(2026, 11, 1), al: DateTime(2026, 11, 9), codOrdine: '  ord-4 ');
    expect(url.queryParameters['dal'], '2026-11-01');
    expect(url.queryParameters['al'], '2026-11-09');
    expect(url.queryParameters['cod_ordine'], 'ord-4');

    await api.elencoSpedizioni(codOrdine: '   ');
    expect(url.queryParameters.containsKey('cod_ordine'), isFalse, reason: 'il backend rifiuta cod_ordine vuoto');
  });

  test('dettaglio: GET /api/spedizioni/{id} con tentativi e annullamento', () async {
    late http.Request inviata;
    final api = ApiClient(
      base: 'http://backend:8080',
      ruolo: RuoloApi.cliente,
      idUtente: 2,
      client: MockClient((req) async {
        inviata = req;
        return http.Response(
            jsonEncode({..._voce, 'annullata_il': null, 'tentativi_ritiro_falliti': 0, 'tentativi_consegna_falliti': 2}), 200);
      }),
    );
    final d = await api.dettaglioSpedizione(4);
    expect(inviata.url.path, '/api/spedizioni/4');
    expect(inviata.headers['X-Id-Cliente'], '2');
    expect(d.spedizione.ritiro.testo, 'Via Roma 1');
    expect(d.tentativiConsegnaFalliti, 2);
    expect(d.annullataIl, isNull);
  });

  test('401 diventa ErroreApi con lo status', () async {
    final api = ApiClient(
      base: 'http://backend:8080',
      ruolo: RuoloApi.operatore,
      idUtente: 99,
      client: MockClient((_) async => http.Response('{"errore":"utente non riconosciuto"}', 401)),
    );
    await expectLater(
      api.elencoSpedizioni(),
      throwsA(isA<ErroreApi>().having((e) => e.status, 'status', 401)),
    );
  });

  test('rete o CORS: ErroreApi senza status', () async {
    final api = ApiClient(
      base: 'http://backend:8080',
      ruolo: RuoloApi.operatore,
      idUtente: 1,
      client: MockClient((_) async => throw http.ClientException('Failed to fetch')),
    );
    await expectLater(api.elencoSpedizioni(), throwsA(isA<ErroreApi>().having((e) => e.status, 'status', null)));
  });
}
