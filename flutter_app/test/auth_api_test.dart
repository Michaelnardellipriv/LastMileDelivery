// flutter test — login, cambio password, recupero e reset password, validatori (non bloccanti), link di recupero.
// Senza backend (MockClient): le risposte riproducono quelle di backend/internal/autenticazione/handler.go.
import 'dart:convert';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:veloce24_app/api.dart';
import 'package:veloce24_app/auth/recupero_link.dart';
import 'package:veloce24_app/auth/validatori.dart';

const _base = 'http://backend:8080';

String _b64(Map<String, dynamic> m) => base64Url.encode(utf8.encode(jsonEncode(m))).replaceAll('=', '');
String _jwt(Map<String, dynamic> c) => '${_b64({'alg': 'HS256', 'typ': 'JWT'})}.${_b64(c)}.firma';

http.Response _json(int status, Object corpo, {Map<String, String> headers = const {}}) =>
    http.Response(jsonEncode(corpo), status, headers: {'content-type': 'application/json', ...headers});

Map<String, dynamic> _rispostaLogin(String token) => {
      'token': token,
      'tipo': 'Bearer',
      'scade_il': '2099-11-09T18:00:00Z',
      'utente': {'id_utente': 4, 'ruolo': 'cliente', 'id': 2, 'nome': 'Dora', 'cognome': 'Destinataria', 'email': 'destinatario@veloce24.test'},
    };

Sessione _sessione({String token = 'abc.def.ghi'}) => Sessione(
      token: token,
      scadeIl: DateTime.utc(2099),
      idUtente: 4,
      idRuolo: 2,
      ruolo: RuoloApi.cliente,
      nome: 'Dora',
      cognome: 'Destinataria',
      email: 'destinatario@veloce24.test',
    );

ApiClient _client(http.Client c, {void Function()? scaduta}) =>
    ApiClient.daSessione(_base, _sessione(), alScadereDelToken: scaduta, client: c);

void main() {
  group('POST /api/auth/login', () {
    test('invia email e password; legge tutti i campi della risposta', () async {
      late http.Request inviata;
      final token = _jwt({'sub': '4', 'ruolo': 'cliente', 'id': 2});
      final s = await AuthApi.login(
        base: '$_base/',
        email: 'destinatario@veloce24.test',
        password: 'segreta',
        client: MockClient((req) async {
          inviata = req;
          return _json(200, _rispostaLogin(token));
        }),
      );
      expect(inviata.method, 'POST');
      expect(inviata.url.toString(), '$_base/api/auth/login');
      expect(inviata.headers['Content-Type'], startsWith('application/json'));
      expect(jsonDecode(inviata.body), {'email': 'destinatario@veloce24.test', 'password': 'segreta'}); // niente `ruolo`
      expect(s.token, token);
      expect(s.idUtente, 4);
      expect(s.idRuolo, 2);
      expect(s.ruolo, RuoloApi.cliente);
      expect(s.nomeCompleto, 'Dora Destinataria');
      expect(s.email, 'destinatario@veloce24.test');
      expect(s.scadeIl, DateTime.utc(2099, 11, 9, 18));
      expect(s.scaduta, isFalse);
    });

    test('con più ruoli invia `ruolo`', () async {
      late http.Request inviata;
      await AuthApi.login(
        base: _base,
        email: 'a@b.it',
        password: 'x',
        ruolo: RuoloApi.corriere,
        client: MockClient((req) async {
          inviata = req;
          return _json(200, _rispostaLogin('a.b.c'));
        }),
      );
      expect(jsonDecode(inviata.body)['ruolo'], 'corriere');
    });

    test('400 con dettagli sul campo ruolo', () async {
      await expectLater(
        AuthApi.login(
          base: _base,
          email: 'a@b.it',
          password: 'x',
          client: MockClient((_) async => _json(400, {
                'errore': 'dati non validi',
                'dettagli': [{'campo': 'ruolo', 'messaggio': "l'utente ha più ruoli: indicarne uno tra corriere, cliente"}],
              })),
        ),
        throwsA(isA<ErroreApi>()
            .having((e) => e.status, 'status', 400)
            .having((e) => e.messaggioCampo('ruolo'), 'ruolo', contains('corriere, cliente'))),
      );
    });

    test('401: messaggio del backend', () async {
      await expectLater(
        AuthApi.login(base: _base, email: 'a@b.it', password: 'x', client: MockClient((_) async => _json(401, {'errore': 'email o password non corrette'}))),
        throwsA(isA<ErroreApi>().having((e) => e.status, 'status', 401).having((e) => e.messaggio, 'messaggio', 'email o password non corrette')),
      );
    });

    test('backend irraggiungibile: ErroreApi senza status', () async {
      await expectLater(
        AuthApi.login(base: _base, email: 'a@b.it', password: 'x', client: MockClient((_) async => throw http.ClientException('boom'))),
        throwsA(isA<ErroreApi>().having((e) => e.status, 'status', isNull)),
      );
    });

    test('risposta fuori contratto (ruolo sconosciuto, alias, id mancante): 502', () async {
      for (final utente in [
        {'id_utente': 1, 'ruolo': 'pirata', 'id': 1, 'email': 'a@b.it'},
        {'id_utente': 1, 'ruolo': 'admin', 'id': 1, 'email': 'a@b.it'}, // il backend emette "amministratore"
        {'id_utente': 1, 'ruolo': 'cliente', 'email': 'a@b.it'},
      ]) {
        await expectLater(
          AuthApi.login(base: _base, email: 'a@b.it', password: 'x', client: MockClient((_) async => _json(200, {'token': 'a.b.c', 'utente': utente}))),
          throwsA(isA<ErroreApi>().having((e) => e.status, 'status', 502)),
        );
      }
    });

    test('sessione: salvataggio e ripristino conservano tutti i campi', () {
      final r = Sessione.daJsonSalvato(_sessione().toJson())!;
      expect([r.idUtente, r.idRuolo, r.ruolo, r.nome, r.cognome, r.email], [4, 2, RuoloApi.cliente, 'Dora', 'Destinataria', 'destinatario@veloce24.test']);
      expect(Sessione.daJsonSalvato({'token': 'x', 'id': 1, 'ruolo': 'cliente', 'nome': 'vecchio formato'}), isNull);
    });

    test("operatore: la sede del login (utente.id_sede) resta nella sessione, anche salvata e riletta", () {
      final risposta = _rispostaLogin('a.b.c');
      final utente = {...risposta['utente'] as Map<String, dynamic>, 'ruolo': 'operatore', 'id_sede': 3};
      final s = Sessione.daRispostaLogin({...risposta, 'utente': utente});
      expect(s.idSede, 3);
      expect(Sessione.daJsonSalvato(s.toJson())!.idSede, 3);
      expect(s.conToken('x.y.z', DateTime.utc(2099)).idSede, 3); // anche dopo un rinnovo del token
      // cliente: nessuna sede
      expect(Sessione.daRispostaLogin(_rispostaLogin('a.b.c')).idSede, isNull);
    });

    test('scade_il nel passato: sessione scaduta', () {
      final s = Sessione.daRispostaLogin({..._rispostaLogin('a.b.c'), 'scade_il': '2020-01-01T00:00:00Z'});
      expect(s.scaduta, isTrue);
    });
  });

  group('ErroreApi: struttura di httpx/errori.go', () {
    test('errore + dettagli + Retry-After', () {
      final e = ErroreApi.daRisposta(_json(
        429,
        {'errore': 'troppe richieste', 'dettagli': [{'campo': 'x', 'messaggio': 'y'}]},
        headers: {'retry-after': '30'},
      ));
      expect(e.status, 429);
      expect(e.riprovaTra, 30);
      expect(e.messaggioCampo('x'), 'y');
      expect(e.perUtente, contains('30'));
    });

    test('corpo non JSON: usa il testo grezzo', () {
      expect(ErroreApi.daRisposta(http.Response('Bad Gateway', 502)).messaggio, 'Bad Gateway');
    });
  });

  group('POST /api/auth/cambio-password', () {
    Future<void> cambia(ApiClient a) => a.cambiaPassword(attuale: 'VecchiaPwd1', nuova: 'NuovaPwd22');

    test('JWT in Authorization e campi password_attuale / nuova_password', () async {
      late http.Request inviata;
      await cambia(_client(MockClient((req) async {
        inviata = req;
        return http.Response('', 204);
      })));
      expect(inviata.method, 'POST');
      expect(inviata.url.toString(), '$_base/api/auth/cambio-password');
      expect(inviata.headers['Authorization'], 'Bearer abc.def.ghi');
      expect(inviata.headers['Content-Type'], startsWith('application/json'));
      expect(jsonDecode(inviata.body), {'password_attuale': 'VecchiaPwd1', 'nuova_password': 'NuovaPwd22'});
    });

    test('403 (password attuale errata) NON fa uscire l\'utente', () async {
      var scaduta = false;
      await expectLater(
        cambia(_client(MockClient((_) async => _json(403, {'errore': 'password attuale non corretta'})), scaduta: () => scaduta = true)),
        throwsA(isA<ErroreApi>().having((e) => e.status, 'status', 403)),
      );
      expect(scaduta, isFalse);
    });

    test('400: dettagli sul campo nuova_password', () async {
      await expectLater(
        cambia(_client(MockClient((_) async => _json(400, {
              'errore': 'dati non validi',
              'dettagli': [{'campo': 'nuova_password', 'messaggio': 'almeno 8 caratteri'}],
            })))),
        throwsA(isA<ErroreApi>().having((e) => e.messaggioCampo('nuova_password'), 'campo', 'almeno 8 caratteri')),
      );
    });

    test('401 (token scaduto) avvia il logout', () async {
      var scaduta = false;
      await expectLater(
        cambia(_client(MockClient((_) async => _json(401, {'errore': 'token non valido o scaduto: rifare il login'})), scaduta: () => scaduta = true)),
        throwsA(isA<ErroreApi>().having((e) => e.status, 'status', 401)),
      );
      expect(scaduta, isTrue);
    });
  });

  group('recupero e reset password (anonimi)', () {
    test('recupero: POST {email}, senza Authorization, 202 con messaggio', () async {
      late http.Request inviata;
      final m = await AuthApi.richiediRecupero(
        base: _base,
        email: 'olga@veloce24.test',
        client: MockClient((req) async {
          inviata = req;
          return _json(202, {'messaggio': 'se l\'email è registrata riceverai un link'});
        }),
      );
      expect(inviata.url.toString(), '$_base/api/auth/recupero-password');
      expect(inviata.headers.containsKey('Authorization'), isFalse);
      expect(jsonDecode(inviata.body), {'email': 'olga@veloce24.test'});
      expect(m, contains('riceverai un link'));
    });

    test('recupero: 400 email mancante', () async {
      await expectLater(
        AuthApi.richiediRecupero(base: _base, email: '', client: MockClient((_) async => _json(400, {'errore': 'dati non validi', 'dettagli': [{'campo': 'email', 'messaggio': 'obbligatoria'}]}))),
        throwsA(isA<ErroreApi>().having((e) => e.status, 'status', 400)),
      );
    });

    test('reimposta: POST {token, nuova_password}, 204', () async {
      late http.Request inviata;
      await AuthApi.reimpostaPassword(
        base: _base,
        token: 'tok123',
        nuovaPassword: 'NuovaPwd22',
        client: MockClient((req) async {
          inviata = req;
          return http.Response('', 204);
        }),
      );
      expect(inviata.url.toString(), '$_base/api/auth/reimposta-password');
      expect(inviata.headers.containsKey('Authorization'), isFalse);
      expect(jsonDecode(inviata.body), {'token': 'tok123', 'nuova_password': 'NuovaPwd22'});
    });

    test('reimposta: 400 token non valido / scaduto / già usato', () async {
      await expectLater(
        AuthApi.reimpostaPassword(
          base: _base,
          token: 'x',
          nuovaPassword: 'NuovaPwd22',
          client: MockClient((_) async => _json(400, {
                'errore': 'dati non validi',
                'dettagli': [{'campo': 'token', 'messaggio': 'non valido, scaduto o già usato: richiedere un nuovo recupero'}],
              })),
        ),
        throwsA(isA<ErroreApi>().having((e) => e.messaggioCampo('token'), 'token', contains('scaduto'))),
      );
    });
  });

  group('validatori: la validazione dei dati è del backend, il client non blocca', () {
    test('email e nuova password non vengono mai rifiutate dal client', () {
      for (final v in ['', 'senza-chiocciola', ' olga@veloce24.test ', 'a' * 300]) {
        expect(validaEmail(v), isNull, reason: 'email "$v"');
      }
      for (final v in ['', '        ', 'abc', '12345678', 'a' * 73]) {
        expect(validaNuovaPassword(v), isNull, reason: 'password "$v"');
      }
      expect(validaNuovaPassword('Password123', attuale: 'Password123'), isNull); // 400 dal backend, non dal client
    });

    test('conferma: unico controllo locale, perché il backend non riceve questo campo', () {
      expect(validaConferma('Password124', 'Password123'), 'Le password non coincidono');
      expect(validaConferma('Password123', 'Password123'), isNull);
    });
  });

  group('link di recupero: lettura del token', () {
    test('nella query', () {
      expect(tokenRecuperoDaUri(Uri.parse('http://localhost:5173/?token=abc123')), 'abc123');
    });

    test('nel frammento (strategia di URL a hash di Flutter web)', () {
      expect(tokenRecuperoDaUri(Uri.parse('http://localhost:5173/#/?token=xyz')), 'xyz');
    });

    test('token percent-encoded: viene decodificato', () {
      expect(tokenRecuperoDaUri(Uri.parse('http://localhost:5173/?token=a%2Bb%3D')), 'a+b=');
    });

    test('assente, vuoto o di soli spazi: null', () {
      expect(tokenRecuperoDaUri(Uri.parse('http://localhost:5173/')), isNull);
      expect(tokenRecuperoDaUri(Uri.parse('http://localhost:5173/?token=')), isNull);
      expect(tokenRecuperoDaUri(Uri.parse('http://localhost:5173/?token=%20%20')), isNull);
    });
  });
}
