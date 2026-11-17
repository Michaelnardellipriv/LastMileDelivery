// Posizione simulata del corriere: finché il corriere è nell'app, al backend arriva una
// posizione finta che si sposta un po' a ogni invio (POST /api/corrieri/posizione),
// con l'intervallo deciso dall'amministratore. Se registrarla o no lo decide il backend
// (oggi solo con una spedizione in consegna): qui si mostra il suo esito.
//
// L'invio è separato dalla pagina e ha due motori:
//  - web (posizione_motore_web.dart): un Web Worker (web/posizione_worker.js), che il
//    browser non rallenta quando la scheda è in background; a scheda chiusa si ferma.
//  - altrove (InvioPosizioneTimer): un timer Dart. È la base per l'app Android: per
//    continuare anche ad app chiusa basterà eseguirlo dentro un servizio in background
//    (foreground service), senza cambiare né la simulazione né la pagina.
import 'dart:async';
import 'dart:math';

import 'package:flutter/material.dart';

import '../comune/comuni.dart';
import 'api.dart';
import 'posizione_motore.dart' if (dart.library.js_interop) 'posizione_motore_web.dart';

/// Percorso simulato: si parte dal centro di Trento (ogni corriere un po' spostato) e a
/// ogni passo si avanza di circa 150 metri con una piccola curva.
/// La stessa formula è in web/posizione_worker.js: se si cambia qui, va cambiata anche là.
class PercorsoSimulato {
  double latitudine, longitudine, direzione;
  final Random _caso;

  PercorsoSimulato.per(int idCorriere, {Random? caso})
      : latitudine = 46.0679 + (idCorriere % 5) * 0.004,
        longitudine = 11.1211 + (idCorriere % 3) * 0.005,
        direzione = (idCorriere % 8) * pi / 4,
        _caso = caso ?? Random();

  void muovi() {
    direzione += (_caso.nextDouble() - 0.5) * 0.8;
    latitudine += cos(direzione) * 0.0014;
    longitudine += sin(direzione) * 0.0020;
  }
}

/// Stato dell'invio della posizione, osservato dalla barra in fondo all'area corrieri.
abstract class InvioPosizione extends ChangeNotifier {
  final ApiCorriere api;
  final PercorsoSimulato percorso;

  /// Secondi tra un invio e il successivo: li comunica il backend a ogni risposta.
  int intervalloSecondi = 30;
  DateTime? ultimoInvio; // ultimo invio registrato dal backend
  Object? errore; // esito dell'ultimo invio, se rifiutato o non riuscito

  InvioPosizione.base(this.api, {Random? caso}) : percorso = PercorsoSimulato.per(api.utente?.id ?? 0, caso: caso);

  /// Sceglie il motore adatto alla piattaforma: Web Worker sul web, timer altrove.
  factory InvioPosizione(ApiCorriere api) => creaInvioPosizione(api);

  double get latitudine => percorso.latitudine;
  double get longitudine => percorso.longitudine;

  void avvia();
  void ferma();

  @override
  void dispose() {
    ferma();
    super.dispose();
  }

  /// Registra l'esito di un invio (lo chiamano i motori).
  @protected
  void registraEsito({int? intervallo, Object? errore}) {
    if (intervallo != null && intervallo > 0) intervalloSecondi = intervallo;
    this.errore = errore;
    if (errore == null) ultimoInvio = DateTime.now();
    notifyListeners();
  }
}

/// Motore con un timer Dart: lo usano i test e, in futuro, l'app Android.
class InvioPosizioneTimer extends InvioPosizione {
  Timer? _timer;
  bool _attivo = false;

  InvioPosizioneTimer(super.api, {super.caso}) : super.base();

  @override
  void avvia() {
    if (_attivo) return;
    _attivo = true;
    _invia();
  }

  @override
  void ferma() {
    _attivo = false;
    _timer?.cancel();
  }

  Future<void> _invia() async {
    percorso.muovi();
    int? intervallo;
    Object? errore;
    try {
      intervallo = await api.inviaPosizione(latitudine, longitudine);
    } catch (e) {
      errore = e;
    }
    if (!_attivo) return;
    registraEsito(intervallo: intervallo, errore: errore);
    _timer = Timer(Duration(seconds: intervalloSecondi), _invia);
  }
}

/// Riga in fondo all'area corrieri con l'esito dell'ultimo invio della posizione.
class BarraPosizione extends StatelessWidget {
  final InvioPosizione invio;
  const BarraPosizione({super.key, required this.invio});

  @override
  Widget build(BuildContext context) => ListenableBuilder(
        listenable: invio,
        builder: (context, _) {
          final colori = Theme.of(context).colorScheme;
          final err = invio.errore;
          final String testo;
          if (err != null) {
            // il rifiuto del backend (es. nessuna spedizione in consegna) è un esito normale: niente log
            testo = 'Posizione non registrata: ${err is ErroreServer ? err.perUtente : testoErrore(err)}';
          } else if (invio.ultimoInvio == null) {
            testo = 'Invio della posizione simulata…';
          } else {
            testo = 'Posizione inviata alle ${fmtOra(invio.ultimoInvio!)} · '
                '${invio.latitudine.toStringAsFixed(5)}, ${invio.longitudine.toStringAsFixed(5)} · ogni ${invio.intervalloSecondi} s';
          }
          return Material(
            color: colori.surfaceContainerHighest,
            child: SafeArea(
              top: false,
              child: Padding(
                padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                child: Row(children: [
                  Icon(err == null ? Icons.my_location : Icons.location_disabled, size: 18, color: err == null ? colori.primary : colori.outline),
                  const SizedBox(width: 8),
                  Expanded(child: Text(testo, style: Theme.of(context).textTheme.bodySmall)),
                ]),
              ),
            ),
          );
        },
      );
}
