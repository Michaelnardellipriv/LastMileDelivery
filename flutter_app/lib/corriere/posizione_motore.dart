// Motore della posizione fuori dal browser (test, futura app Android): timer Dart.
import 'api.dart';
import 'posizione.dart';

InvioPosizione creaInvioPosizione(ApiCorriere api) => InvioPosizioneTimer(api);
