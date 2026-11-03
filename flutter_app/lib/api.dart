// Client HTTP verso il backend Go, secondo il contratto di backend/api/openapi.yaml.
// Il backend non ha ancora il login: l'utente si identifica con UN header X-Id-*
// (X-Id-Operatore, X-Id-Corriere, X-Id-Cliente, X-Id-Amministratore).
import 'dart:async';
import 'dart:convert';
import 'package:http/http.dart' as http;

/// Ruoli come li intende il backend, con l'header che li identifica.
enum RuoloApi {
  operatore('X-Id-Operatore', 'Operatore logistico'),
  corriere('X-Id-Corriere', 'Corriere'),
  cliente('X-Id-Cliente', 'Cliente'),
  amministratore('X-Id-Amministratore', 'Amministratore');

  final String header, nome;
  const RuoloApi(this.header, this.nome);
}

/// Errore con lo status HTTP: il frontend reagisce in modo diverso a 401, 403, 404, 5xx.
class ErroreApi implements Exception {
  final int? status; // null = backend non raggiungibile (rete, CORS, timeout)
  final String messaggio;
  ErroreApi(this.status, this.messaggio);

  String get perUtente => switch (status) {
        null => 'Backend non raggiungibile. Controlla che sia avviato e, '
            'se usi il browser, che CORS_ORIGIN coincida con la porta del frontend.',
        401 => 'Utente non riconosciuto dal backend: controlla ruolo e ID.',
        403 => 'Non hai i permessi per questa operazione.',
        404 => 'Spedizione non trovata.',
        503 => 'Database non disponibile, riprova tra poco.',
        _ when status! >= 500 => 'Errore del server (HTTP $status).',
        _ => '$messaggio (HTTP $status)',
      };

  @override
  String toString() => 'ErroreApi($status, $messaggio)';
}

class IndirizzoApi {
  final String via, civico;
  IndirizzoApi(this.via, this.civico);
  factory IndirizzoApi.daJson(Map<String, dynamic> j) => IndirizzoApi(j['via'] as String? ?? '', j['civico'] as String? ?? '');
  String get testo => '$via $civico'.trim();
}

class NotaApi {
  final int id;
  final String testo;
  final bool interna;
  final String? motivo;
  final DateTime creataIl;
  NotaApi(this.id, this.testo, this.interna, this.motivo, this.creataIl);
  factory NotaApi.daJson(Map<String, dynamic> j) => NotaApi(
        j['id'] as int,
        j['nota'] as String? ?? '',
        j['is_interna'] as bool? ?? false,
        j['motivo'] as String?,
        DateTime.parse(j['creata_il'] as String).toLocal(),
      );
}

/// Una voce di GET /api/spedizioni (stessi campi del JSON del backend).
class SpedizioneApi {
  final int id;
  final String codOrdine, stato;
  final bool isReso;
  final DateTime creataIl, richiestaIl;
  final int idMittente, idDestinatario;
  final IndirizzoApi ritiro, consegna;
  final int? idCorriere;
  final DateTime? conclusaIl;
  final List<NotaApi> note;

  SpedizioneApi({
    required this.id, required this.codOrdine, required this.stato, required this.isReso,
    required this.creataIl, required this.richiestaIl, required this.idMittente,
    required this.idDestinatario, required this.ritiro, required this.consegna,
    required this.idCorriere, required this.conclusaIl, required this.note,
  });

  factory SpedizioneApi.daJson(Map<String, dynamic> j) => SpedizioneApi(
        id: j['id'] as int,
        codOrdine: j['cod_ordine'] as String,
        stato: j['stato'] as String,
        isReso: j['is_reso'] as bool? ?? false,
        creataIl: DateTime.parse(j['creata_il'] as String).toLocal(),
        richiestaIl: DateTime.parse(j['dataora_richiesta_spedizione'] as String).toLocal(),
        idMittente: j['id_cliente_mittente'] as int,
        idDestinatario: j['id_cliente_destinatario'] as int,
        ritiro: IndirizzoApi.daJson(j['indirizzo_ritiro'] as Map<String, dynamic>),
        consegna: IndirizzoApi.daJson(j['indirizzo_spedizione'] as Map<String, dynamic>),
        idCorriere: j['id_corriere'] as int?,
        conclusaIl: j['conclusa_il'] == null ? null : DateTime.parse(j['conclusa_il'] as String).toLocal(),
        note: [for (final n in (j['note'] as List? ?? const [])) NotaApi.daJson(n as Map<String, dynamic>)],
      );
}

class PaginaSpedizioni {
  final List<SpedizioneApi> dati;
  final int pagina, perPagina;
  PaginaSpedizioni(this.dati, this.pagina, this.perPagina);
}

class ApiClient {
  final String base; // es. http://localhost:8080, senza barra finale
  final RuoloApi ruolo;
  final int idUtente;
  final http.Client _http;

  ApiClient({required String base, required this.ruolo, required this.idUtente, http.Client? client})
      : base = base.endsWith('/') ? base.substring(0, base.length - 1) : base,
        _http = client ?? http.Client();

  Map<String, String> get _intestazioni => {
        'Accept': 'application/json',
        ruolo.header: '$idUtente',
      };

  /// GET /api/spedizioni — il backend filtra già per ruolo (il corriere vede solo le sue).
  Future<PaginaSpedizioni> elencoSpedizioni({String? stato, int pagina = 1, int perPagina = 20}) async {
    final uri = Uri.parse('$base/api/spedizioni').replace(queryParameters: {
      if (stato != null) 'stato': stato,
      'pagina': '$pagina',
      'per_pagina': '$perPagina',
    });
    final j = await _getJson(uri);
    return PaginaSpedizioni(
      [for (final v in (j['dati'] as List? ?? const [])) SpedizioneApi.daJson(v as Map<String, dynamic>)],
      j['pagina'] as int? ?? pagina,
      j['per_pagina'] as int? ?? perPagina,
    );
  }

  Future<Map<String, dynamic>> _getJson(Uri uri) async {
    final r = await _get(uri);
    if (r.statusCode != 200) {
      throw ErroreApi(r.statusCode, _messaggio(r.body));
    }
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  Future<http.Response> _get(Uri uri) async {
    try {
      return await _http.get(uri, headers: _intestazioni).timeout(const Duration(seconds: 8));
    } on TimeoutException {
      throw ErroreApi(null, 'timeout');
    } on http.ClientException catch (e) {
      // Nel browser un blocco CORS arriva qui, senza status: non è distinguibile da "backend spento".
      throw ErroreApi(null, e.message);
    }
  }

  static String _messaggio(String corpo) {
    try {
      return (jsonDecode(corpo) as Map<String, dynamic>)['errore'] as String? ?? corpo;
    } catch (_) {
      return corpo.trim();
    }
  }
}
