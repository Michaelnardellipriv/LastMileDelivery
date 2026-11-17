// Client HTTP dell'area corrieri: alla sessione comune aggiunge la chiamata che il
// backend riserva al corriere, cioè la registrazione di un tentativo non riuscito.
import '../comune/api.dart';

export '../comune/api.dart';

const tipiTentativo = ['ritiro', 'consegna'];

/// Esito di un tentativo: lo stato e i contatori registrati dal backend.
class EsitoSpedizione {
  final String stato;
  final int tentativiRitiro, tentativiConsegna;
  EsitoSpedizione(this.stato, this.tentativiRitiro, this.tentativiConsegna);
  factory EsitoSpedizione.daJson(Map<String, dynamic> j) => EsitoSpedizione(
        j['stato'] as String,
        j['tentativi_ritiro_falliti'] as int? ?? 0,
        j['tentativi_consegna_falliti'] as int? ?? 0,
      );
}

class ApiCorriere extends ApiSessione {
  ApiCorriere({required super.base, super.client});

  /// Continua la sessione aperta dal login unico.
  ApiCorriere.da(super.sessione) : super.da();

  /// Manda la posizione (simulata) del corriere: POST /api/corrieri/posizione.
  /// Restituisce ogni quanti secondi il backend vuole il prossimo invio.
  Future<int> inviaPosizione(double latitudine, double longitudine) async {
    final j = await chiama('POST', '/api/corrieri/posizione', corpo: {'latitudine': latitudine, 'longitudine': longitudine})
        as Map<String, dynamic>;
    return j['intervallo_secondi'] as int? ?? 0;
  }

  /// Registra un tentativo di ritiro o di consegna non riuscito
  /// (POST /api/spedizioni/{id}/tentativi). Motivo e nota formano una nota pubblica:
  /// il cliente vede solo il motivo. Il backend aumenta il contatore dei tentativi e,
  /// esauriti quelli di consegna, porta la spedizione in "non consegnata".
  /// I valori partono così come sono, anche vuoti: li controlla il backend.
  Future<EsitoSpedizione> registraTentativo(int id,
      {required String tipo, required String motivo, required String nota, required String chiave}) async {
    final j = await chiama('POST', '/api/spedizioni/$id/tentativi',
        chiave: chiave, corpo: {'tipo': tipo, 'motivo': motivo, 'nota': nota}) as Map<String, dynamic>;
    return EsitoSpedizione.daJson(j);
  }
}
