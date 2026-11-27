// Motore della posizione nel browser: la simulazione e gli invii girano nel Web Worker
// web/posizione_worker.js, che il browser non rallenta quando la scheda è in
// background. La pagina gli passa backend, token e punto di partenza, e riceve l'esito
// di ogni invio.
import 'dart:async';
import 'dart:js_interop';

import 'package:web/web.dart' as web;

import 'api.dart';
import 'posizione.dart';

InvioPosizione creaInvioPosizione(ApiCorriere api) => InvioPosizioneWorker(api);

class InvioPosizioneWorker extends InvioPosizione {
  web.Worker? _worker;
  Timer? _riprova; // nuovo tentativo dopo un rinnovo non riuscito

  InvioPosizioneWorker(super.api) : super.base();

  // a ogni rinnovo il worker riceve il token nuovo: altrimenti dopo 15 minuti i suoi
  // invii verrebbero rifiutati
  void _nuovoToken(String token) => _worker?.postMessage({'tipo': 'token', 'token': token}.jsify());

  @override
  void avvia() {
    if (_worker != null) return;
    final w = web.Worker('posizione_worker.js'.toJS);
    w.onmessage = ((web.MessageEvent e) => _ricevi(e.data)).toJS;
    w.postMessage({
      'tipo': 'avvia',
      'base': api.base,
      'token': api.token,
      'lat': percorso.latitudine,
      'lon': percorso.longitudine,
      'direzione': percorso.direzione,
      'intervallo': intervalloSecondi,
    }.jsify());
    _worker = w;
    api.alRinnovoToken.add(_nuovoToken);
  }

  @override
  void ferma() {
    _riprova?.cancel();
    _riprova = null;
    final w = _worker;
    if (w == null) return;
    api.alRinnovoToken.remove(_nuovoToken);
    w.postMessage({'tipo': 'ferma'}.jsify());
    w.terminate();
    _worker = null;
  }

  void _ricevi(JSAny? dati) {
    if (_worker == null) return;
    final m = dati.dartify() as Map<Object?, Object?>;
    // la posizione la calcola il worker: qui si tiene allineata per mostrarla
    percorso
      ..latitudine = (m['lat'] as num).toDouble()
      ..longitudine = (m['lon'] as num).toDouble();
    final status = (m['status'] as num?)?.toInt();
    if (m['ok'] == true) {
      registraEsito(intervallo: (m['intervallo'] as num?)?.toInt());
      // il worker non passa da api.chiama: il rinnovo si fa qui, a ogni invio riuscito
      api.rinnovaSeServe();
      return;
    }
    registraEsito(
      intervallo: (m['intervallo'] as num?)?.toInt(),
      errore: ErroreServer(status, m['messaggio'] as String? ?? 'errore', const {}, m['tecnico'] as String? ?? ''),
    );
    if (status == 401 || status == 403) {
      // 401: token scaduto (scheda sospesa, rete assente); 403: forse account disattivato.
      // Il worker si è fermato. Si rinnova:
      //  - rinnovo rifiutato (401/403): rinnova() ha chiuso la sessione, si torna al login;
      //  - token nuovo dopo un 401: si riparte subito;
      //  - altrimenti (403 per un altro motivo, rete assente): si riprova dopo un
      //    intervallo, senza raffiche.
      ferma();
      final vecchio = api.token;
      api.rinnova().then((rinnovato) {
        if (api.token == null) return;
        if (status == 401 && rinnovato && api.token != vecchio) {
          avvia();
        } else {
          _riprova?.cancel();
          _riprova = Timer(Duration(seconds: intervalloSecondi), () {
            if (api.token != null) avvia();
          });
        }
      });
    }
  }
}
