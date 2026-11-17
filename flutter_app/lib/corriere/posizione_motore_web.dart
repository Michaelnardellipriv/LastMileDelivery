// Motore della posizione nel browser: la simulazione e gli invii girano nel Web Worker
// web/posizione_worker.js, che il browser non rallenta quando la scheda è in
// background. La pagina gli passa backend, token e punto di partenza, e riceve l'esito
// di ogni invio.
import 'dart:js_interop';

import 'package:web/web.dart' as web;

import 'api.dart';
import 'posizione.dart';

InvioPosizione creaInvioPosizione(ApiCorriere api) => InvioPosizioneWorker(api);

class InvioPosizioneWorker extends InvioPosizione {
  web.Worker? _worker;

  InvioPosizioneWorker(super.api) : super.base();

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
  }

  @override
  void ferma() {
    final w = _worker;
    if (w == null) return;
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
      return;
    }
    registraEsito(
      intervallo: (m['intervallo'] as num?)?.toInt(),
      errore: ErroreServer(status, m['messaggio'] as String? ?? 'errore', const {}, m['tecnico'] as String? ?? ''),
    );
    if (status == 401) {
      // sessione scaduta: il worker si è già fermato, la pagina torna al login
      ferma();
      api.esci();
      api.alScadereDelToken?.call();
    }
  }
}
