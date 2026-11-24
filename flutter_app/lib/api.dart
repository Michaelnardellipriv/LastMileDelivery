// Client HTTP verso il backend Go LastMileDelivery.
// Contratto di riferimento: backend/api/openapi.yaml, docs/api.md e gli handler in
// backend/internal/autenticazione/handler.go (se contratto e codice divergono, vince il contratto).
//
// ┌─ Endpoint di autenticazione REALI del backend (nessun altro esiste) ───────────────────────┐
// │ POST /api/auth/login              anonimo  → AuthApi.login                                 │
// │ POST /api/auth/cambio-password    Bearer   → ApiClient.cambiaPassword                      │
// │ POST /api/auth/recupero-password  anonimo  → AuthApi.richiediRecupero                      │
// │ POST /api/auth/reimposta-password anonimo  → AuthApi.reimpostaPassword                     │
// └────────────────────────────────────────────────────────────────────────────────────────────┘
// NON ESISTONO: GET /api/auth/me (il "profilo" è la risposta del login: vedi [Sessione]),
// refresh token (il JWT dura DURATA_TOKEN, predefinito 8 h, poi si rifà il login), codice 422
// (la validazione risponde 400) e Idempotency-Key per le rotte /api/auth/* (serve solo alle
// POST/PATCH di /api/spedizioni).
//
// Identificazione sulle rotte protette (backend/internal/identita/middleware.go):
//  - `Authorization: Bearer <token>` (formato esatto, con UN spazio);
//  - solo sviluppo, se IDENTITA_HEADER_SVILUPPO=attiva e NON c'è Authorization: UN header tra
//    X-Id-Operatore, X-Id-Corriere, X-Id-Cliente, X-Id-Amministratore, con l'id *della tabella
//    del ruolo* (non l'id_utente dell'account).
import 'dart:async';
import 'dart:convert';
import 'package:http/http.dart' as http;

/// Ruoli esattamente come li scrive il backend (identita.Ruolo in Go): il `name` dell'enum è il
/// valore JSON (`"operatore"`, `"corriere"`, `"cliente"`, `"amministratore"`).
enum RuoloApi {
  operatore('X-Id-Operatore', 'Operatore logistico'),
  corriere('X-Id-Corriere', 'Corriere'),
  cliente('X-Id-Cliente', 'Cliente'),
  amministratore('X-Id-Amministratore', 'Amministratore');

  /// Header di sviluppo che identifica il ruolo (usato solo senza JWT).
  final String header;

  /// Nome per l'interfaccia (identita.Ruolo.Nome() lato Go).
  final String nome;
  const RuoloApi(this.header, this.nome);
}

/// Converte il ruolo del backend nel nostro enum. Corrispondenza ESATTA: il backend emette solo
/// i quattro valori sopra, quindi qualsiasi altro testo (anche un alias) è una risposta non valida.
RuoloApi? ruoloDaTesto(String? t) {
  for (final r in RuoloApi.values) {
    if (r.name == t) return r;
  }
  return null;
}

/// Una voce di `dettagli` negli errori 400/409: `{"campo": "...", "messaggio": "..."}`
/// (errori.ErroreCampo in Go).
class ErroreCampo {
  final String campo, messaggio;
  const ErroreCampo(this.campo, this.messaggio);
}

/// Errore con lo status HTTP. Rispecchia httpx.RispondiErrore (backend/internal/httpx/errori.go):
/// corpo `{"errore": "<messaggio>", "dettagli": [{"campo", "messaggio"}]}` (`dettagli` solo se non vuoto).
///
/// Status che il backend può restituire e significato:
///  400 dati non validi (campi in `dettagli`) o JSON non valido · 401 non autenticato (+ header
///  WWW-Authenticate: Bearer) · 403 ruolo non ammesso / password attuale errata · 404 non trovato ·
///  405 metodo non ammesso · 409 conflitto · 415 Content-Type non JSON · 429 troppo presto (+ Retry-After) ·
///  500 errore interno · 503 database non disponibile (+ Retry-After: 5).
class ErroreApi implements Exception {
  final int? status; // null = backend non raggiungibile (rete, CORS, timeout)
  final String messaggio; // campo `errore` del corpo
  final List<ErroreCampo> dettagli;
  final int? riprovaTra; // secondi, dall'header Retry-After (429 e 503)
  ErroreApi(this.status, this.messaggio, {this.dettagli = const [], this.riprovaTra});

  /// Costruisce l'errore da una risposta non riuscita del backend.
  factory ErroreApi.daRisposta(http.Response r) {
    var messaggio = '';
    var dettagli = const <ErroreCampo>[];
    try {
      final j = jsonDecode(utf8.decode(r.bodyBytes));
      if (j is Map<String, dynamic>) {
        final e = j['errore'];
        if (e is String) messaggio = e;
        final d = j['dettagli'];
        if (d is List) {
          dettagli = [
            for (final v in d)
              if (v is Map<String, dynamic> && v['campo'] is String && v['messaggio'] is String)
                ErroreCampo(v['campo'] as String, v['messaggio'] as String),
          ];
        }
      }
    } catch (_) {
      // corpo non JSON (es. pagina di errore di un proxy): si usa il testo grezzo
    }
    if (messaggio.isEmpty) messaggio = r.body.trim();
    return ErroreApi(r.statusCode, messaggio,
        dettagli: dettagli, riprovaTra: int.tryParse(r.headers['retry-after'] ?? ''));
  }

  /// Messaggio del backend per [campo] (es. `nuova_password`, `password_attuale`, `token`, `ruolo`).
  String? messaggioCampo(String campo) {
    for (final d in dettagli) {
      if (d.campo == campo) return d.messaggio;
    }
    return null;
  }

  /// 400 con dettagli → "campo: messaggio"; senza dettagli → il solo `errore`.
  String get _conDettagli => dettagli.isEmpty
      ? messaggio
      : dettagli.map((d) => d.messaggio).toSet().join(' · ');

  String get _attesa => riprovaTra == null ? 'tra poco' : 'tra $riprovaTra secondi';

  /// Testo per l'utente, uno per ogni status che il backend può restituire.
  String get perUtente => switch (status) {
    null => 'Backend non raggiungibile. Controlla che sia avviato e, '
        'se usi il browser, che CORS_ORIGIN coincida con la porta del frontend.',
    400 => _conDettagli.isEmpty ? 'Dati non validi.' : _conDettagli,
    401 => messaggio.isEmpty ? 'Autenticazione richiesta.' : messaggio,
    403 => messaggio.isEmpty ? 'Non hai i permessi per questa operazione.' : messaggio,
    404 => 'Spedizione non trovata.',
    405 => 'Operazione non supportata dal backend ($messaggio).',
    409 => messaggio.isEmpty ? 'Conflitto con lo stato attuale dei dati.' : messaggio,
    415 => 'Il backend richiede JSON (Content-Type: application/json).',
    429 => 'Troppe richieste: riprova $_attesa.',
    503 => 'Servizio non disponibile, riprova $_attesa.',
    _ when status! >= 500 => 'Errore del server (HTTP $status).',
    _ => '$messaggio (HTTP $status)',
  };

  @override
  String toString() => 'ErroreApi($status, $messaggio)';
}

/// Legge il payload di un JWT SENZA verificarne la firma (serve solo come ripiego per la scadenza;
/// l'autorizzazione vera la fa sempre il backend). Claim emessi dal backend (token.go):
/// `sub` = id_utente (stringa), `ruolo`, `id` (id nella tabella del ruolo), `iat`, `exp`.
Map<String, dynamic>? decodificaJwt(String token) {
  final parti = token.split('.');
  if (parti.length != 3) return null;
  try {
    final j = jsonDecode(utf8.decode(base64Url.decode(base64Url.normalize(parti[1]))));
    return j is Map<String, dynamic> ? j : null;
  } catch (_) {
    return null;
  }
}

/// Utente autenticato: tutto ciò che restituisce POST /api/auth/login.
///
/// Risposta 200 (autenticazione/handler.go, `sessioneDTO` + `utenteDTO`):
/// ```json
/// {"token": "<jwt HS256>", "tipo": "Bearer", "scade_il": "2026-11-09T18:00:00Z",
///  "utente": {"id_utente": 4, "ruolo": "cliente", "id": 2,
///             "nome": "Dora", "cognome": "Destinataria", "email": "destinatario@veloce24.test"}}
/// ```
/// `id_utente` = account (tabella utenti); `id` = riga nella tabella del ruolo (è quello degli header X-Id-*).
/// Non c'è refresh token: a `scade_il` si rifà il login.
class Sessione {
  final String token;
  final DateTime? scadeIl; // `scade_il` (UTC); null solo se manca anche il claim exp
  final int idUtente; // utente.id_utente
  final int idRuolo; // utente.id
  final RuoloApi ruolo; // utente.ruolo
  final String nome, cognome, email;
  const Sessione({
    required this.token,
    required this.scadeIl,
    required this.idUtente,
    required this.idRuolo,
    required this.ruolo,
    required this.nome,
    required this.cognome,
    required this.email,
  });

  String get nomeCompleto {
    final n = '$nome $cognome'.trim();
    return n.isEmpty ? ruolo.nome : n;
  }

  /// Scaduta secondo `scade_il`: serve a non riusare un token già morto all'avvio.
  /// Se manca la data, decide il backend (401).
  bool get scaduta {
    final s = scadeIl;
    return s != null && !s.isAfter(DateTime.now().toUtc());
  }

  /// Costruisce la sessione dal JSON di login; 502 se la risposta non rispetta il contratto.
  factory Sessione.daRispostaLogin(Map<String, dynamic> j) {
    final token = j['token'];
    if (token is! String || token.isEmpty) throw ErroreApi(502, 'risposta di login senza token');
    final u = j['utente'];
    if (u is! Map<String, dynamic>) throw ErroreApi(502, 'risposta di login senza utente');
    final ruolo = ruoloDaTesto(u['ruolo'] is String ? u['ruolo'] as String : null);
    final idUtente = u['id_utente'], idRuolo = u['id'], email = u['email'];
    if (ruolo == null || idUtente is! int || idUtente <= 0 || idRuolo is! int || idRuolo <= 0 || email is! String) {
      throw ErroreApi(502, 'risposta di login con utente non valido');
    }
    var scadeIl = j['scade_il'] is String ? DateTime.tryParse(j['scade_il'] as String)?.toUtc() : null;
    if (scadeIl == null) {
      final exp = decodificaJwt(token)?['exp']; // ripiego: claim exp del token
      if (exp is num) scadeIl = DateTime.fromMillisecondsSinceEpoch(exp.toInt() * 1000, isUtc: true);
    }
    return Sessione(
      token: token,
      scadeIl: scadeIl,
      idUtente: idUtente,
      idRuolo: idRuolo,
      ruolo: ruolo,
      nome: u['nome'] is String ? u['nome'] as String : '',
      cognome: u['cognome'] is String ? u['cognome'] as String : '',
      email: email,
    );
  }

  Map<String, dynamic> toJson() => {
    'token': token,
    'scade_il': scadeIl?.toIso8601String(),
    'id_utente': idUtente,
    'id': idRuolo,
    'ruolo': ruolo.name,
    'nome': nome,
    'cognome': cognome,
    'email': email,
  };

  /// Ricostruisce la sessione salvata su disco; null se manca un campo (anche un salvataggio
  /// fatto da una versione precedente dell'app: basta rifare il login).
  static Sessione? daJsonSalvato(Map<String, dynamic> j) {
    final token = j['token'], idUtente = j['id_utente'], idRuolo = j['id'], email = j['email'];
    final ruolo = ruoloDaTesto(j['ruolo'] is String ? j['ruolo'] as String : null);
    if (token is! String || token.isEmpty || idUtente is! int || idRuolo is! int || email is! String || ruolo == null) {
      return null;
    }
    return Sessione(
      token: token,
      scadeIl: j['scade_il'] is String ? DateTime.tryParse(j['scade_il'] as String)?.toUtc() : null,
      idUtente: idUtente,
      idRuolo: idRuolo,
      ruolo: ruolo,
      nome: j['nome'] is String ? j['nome'] as String : '',
      cognome: j['cognome'] is String ? j['cognome'] as String : '',
      email: email,
    );
  }
}

/// Chiamate di autenticazione che NON richiedono di essere già loggati (niente `Authorization`).
/// Quella che richiede il JWT (cambio password) sta in [ApiClient.cambiaPassword].
abstract final class AuthApi {
  /// `POST /api/auth/login` — `{"email", "password", "ruolo"?}`.
  ///  200 → [Sessione] · 400 `email`/`password` mancanti, oppure utente con più ruoli senza `ruolo`
  ///  (in `dettagli`, campo `ruolo`, l'elenco dei ruoli) · 401 "email o password non corrette"
  ///  (stesso messaggio per email sconosciuta e password errata) · 403 `ruolo` che l'utente non ha ·
  ///  415 · 500 · 503. L'email non distingue maiuscole/minuscole; il backend ne toglie gli spazi.
  static Future<Sessione> login({
    required String base,
    required String email,
    required String password,
    RuoloApi? ruolo, // solo se l'utente ha più ruoli (altrimenti il campo NON va inviato)
    http.Client? client,
  }) async {
    // ── CHIAMATA API: invio delle credenziali al backend ──
    final r = await _postJson(base, '/api/auth/login', {
      'email': email,
      'password': password,
      if (ruolo != null) 'ruolo': ruolo.name,
    }, client);
    if (r.statusCode != 200) throw ErroreApi.daRisposta(r);
    try {
      final j = jsonDecode(utf8.decode(r.bodyBytes));
      if (j is! Map<String, dynamic>) throw const FormatException();
      return Sessione.daRispostaLogin(j);
    } on FormatException {
      throw ErroreApi(502, 'risposta di login non valida');
    }
  }

  /// `POST /api/auth/recupero-password` — `{"email"}`.
  ///  202 `{"messaggio": "..."}` SEMPRE, anche per email sconosciute (e se l'utente ha già fatto 3
  ///  richieste nell'ultima ora la mail non parte, ma la risposta resta 202) → il testo mostrato
  ///  all'utente deve restare neutro · 400 `email` mancante · 415 · 500 · 503.
  /// Il link nella mail è `LINK_RECUPERO_PASSWORD?token=<token>`, valido 30 minuti.
  /// Restituisce il `messaggio` del backend (null se assente).
  static Future<String?> richiediRecupero({
    required String base,
    required String email,
    http.Client? client,
  }) async {
    final r = await _postJson(base, '/api/auth/recupero-password', {'email': email}, client);
    if (r.statusCode != 202) throw ErroreApi.daRisposta(r);
    try {
      final j = jsonDecode(utf8.decode(r.bodyBytes));
      return j is Map<String, dynamic> && j['messaggio'] is String ? j['messaggio'] as String : null;
    } on FormatException {
      return null; // l'esito è comunque 202
    }
  }

  /// `POST /api/auth/reimposta-password` — `{"token", "nuova_password"}`.
  ///  204 password impostata · 400 `token` mancante/non valido/scaduto/già usato (dettaglio sul campo
  ///  `token`) o `nuova_password` non valida (campo `nuova_password`) · 415 · 500 · 503.
  /// Il token vale una sola volta e 30 minuti.
  static Future<void> reimpostaPassword({
    required String base,
    required String token,
    required String nuovaPassword,
    http.Client? client,
  }) async {
    final r = await _postJson(base, '/api/auth/reimposta-password', {'token': token, 'nuova_password': nuovaPassword}, client);
    if (r.statusCode != 204) throw ErroreApi.daRisposta(r);
  }

  /// POST JSON senza autenticazione, con timeout e gli errori di rete già tradotti in [ErroreApi].
  /// `Content-Type: application/json` è obbligatorio (altrimenti il backend risponde 415) e il
  /// backend rifiuta i campi sconosciuti (400): si mandano solo quelli del contratto.
  static Future<http.Response> _postJson(
      String base,
      String percorso,
      Map<String, dynamic> corpo,
      http.Client? client,
      ) async {
    final b = base.endsWith('/') ? base.substring(0, base.length - 1) : base;
    final c = client ?? http.Client();
    try {
      return await c
          .post(
        Uri.parse('$b$percorso'),
        headers: {'Accept': 'application/json', 'Content-Type': 'application/json'},
        body: jsonEncode(corpo),
      )
          .timeout(const Duration(seconds: 8));
    } on TimeoutException {
      throw ErroreApi(null, 'timeout');
    } on http.ClientException catch (e) {
      throw ErroreApi(null, e.message); // rete, backend spento o CORS (nel browser non distinguibili)
    } finally {
      if (client == null) c.close();
    }
  }
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
  /// null per il cliente: il backend non gli manda l'id del mittente.
  final int? idMittente;
  final int idDestinatario;
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
    idMittente: j['id_cliente_mittente'] as int?,
    idDestinatario: j['id_cliente_destinatario'] as int,
    ritiro: IndirizzoApi.daJson(j['indirizzo_ritiro'] as Map<String, dynamic>),
    consegna: IndirizzoApi.daJson(j['indirizzo_spedizione'] as Map<String, dynamic>),
    idCorriere: j['id_corriere'] as int?,
    conclusaIl: j['conclusa_il'] == null ? null : DateTime.parse(j['conclusa_il'] as String).toLocal(),
    note: [for (final n in (j['note'] as List? ?? const [])) NotaApi.daJson(n as Map<String, dynamic>)],
  );
}

/// GET /api/spedizioni/{id}: la voce dell'elenco più date e tentativi falliti.
class DettaglioSpedizioneApi {
  final SpedizioneApi spedizione;
  final DateTime? annullataIl;
  final int tentativiRitiroFalliti, tentativiConsegnaFalliti;
  DettaglioSpedizioneApi(this.spedizione, this.annullataIl, this.tentativiRitiroFalliti, this.tentativiConsegnaFalliti);

  factory DettaglioSpedizioneApi.daJson(Map<String, dynamic> j) => DettaglioSpedizioneApi(
    SpedizioneApi.daJson(j),
    j['annullata_il'] == null ? null : DateTime.parse(j['annullata_il'] as String).toLocal(),
    j['tentativi_ritiro_falliti'] as int? ?? 0,
    j['tentativi_consegna_falliti'] as int? ?? 0,
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

  /// Id nella tabella del ruolo (`utente.id` del login), che è il valore degli header di sviluppo
  /// X-Id-*: NON è l'`id_utente` dell'account (quello sta in [Sessione.idUtente]).
  final int idUtente;
  final http.Client _http;

  /// Token del login unico (area riservata): se c'è, l'identità viaggia come
  /// Authorization: Bearer invece che con l'header X-Id-* (solo sviluppo).
  final String? token;

  /// Chiamata quando il backend rifiuta il token (401): sessione scaduta.
  final void Function()? alScadereDelToken;

  ApiClient({
    required String base,
    required this.ruolo,
    required this.idUtente,
    http.Client? client,
    this.token,
    this.alScadereDelToken,
  })  : base = base.endsWith('/') ? base.substring(0, base.length - 1) : base,
        _http = client ?? http.Client();

  /// Client autenticato col JWT di una sessione di login (`id` nella tabella del ruolo).
  factory ApiClient.daSessione(String base, Sessione s, {void Function()? alScadereDelToken, http.Client? client}) =>
      ApiClient(
        base: base,
        ruolo: s.ruolo,
        idUtente: s.idRuolo,
        token: s.token,
        alScadereDelToken: alScadereDelToken,
        client: client,
      );

  Map<String, String> get _intestazioni => {
    'Accept': 'application/json',
    if (token != null) 'Authorization': 'Bearer $token' else ruolo.header: '$idUtente',
  };

  /// GET /api/spedizioni — il backend filtra già per ruolo (il corriere vede solo le sue,
  /// il cliente solo quelle di cui è destinatario). [dal] e [al] sono giorni di creazione,
  /// inclusi; [codOrdine] cerca una parte del codice, senza distinguere maiuscole.
  Future<PaginaSpedizioni> elencoSpedizioni({
    String? stato,
    DateTime? dal,
    DateTime? al,
    String? codOrdine,
    int pagina = 1,
    int perPagina = 20,
  }) async {
    final cod = codOrdine?.trim();
    final uri = Uri.parse('$base/api/spedizioni').replace(queryParameters: {
      if (stato != null) 'stato': stato,
      if (dal != null) 'dal': _giorno(dal),
      if (al != null) 'al': _giorno(al),
      if (cod != null && cod.isNotEmpty) 'cod_ordine': cod,
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

  /// GET /api/spedizioni/{id} — 404 anche se la spedizione esiste ma chi chiama non può vederla.
  Future<DettaglioSpedizioneApi> dettaglioSpedizione(int id) async =>
      DettaglioSpedizioneApi.daJson(await _getJson(Uri.parse('$base/api/spedizioni/$id')));

  /// AAAA-MM-GG, il formato che il backend accetta per dal/al.
  static String _giorno(DateTime d) =>
      '${d.year.toString().padLeft(4, '0')}-${d.month.toString().padLeft(2, '0')}-${d.day.toString().padLeft(2, '0')}';

  /// `POST /api/auth/cambio-password` — `{"password_attuale", "nuova_password"}` (l'utente lo dà il JWT).
  ///  204 password cambiata · 400 dati non validi (dettagli sul campo `nuova_password` o
  ///  `password_attuale`, validati dal backend) · 401 token mancante, scaduto o non valido → logout
  ///  ([alScadereDelToken]) · 403 password attuale NON corretta (non 401: non deve far uscire l'utente) ·
  ///  415 · 500 · 503. Le password non vanno mai scritte nei log.
  Future<void> cambiaPassword({required String attuale, required String nuova}) async {
    // ── CHIAMATA API: il JWT parte in `Authorization: Bearer <token>` (vedi _intestazioni) ──
    final r = await _esegui(() => _http.post(
      Uri.parse('$base/api/auth/cambio-password'),
      headers: {..._intestazioni, 'Content-Type': 'application/json'},
      body: jsonEncode({'password_attuale': attuale, 'nuova_password': nuova}),
    ));
    if (r.statusCode != 204) throw _errore(r);
  }

  /// Costruisce l'errore e, se il backend ha risposto 401 a un token, avvisa la UI di tornare al login.
  ErroreApi _errore(http.Response r) {
    if (r.statusCode == 401 && token != null) alScadereDelToken?.call();
    return ErroreApi.daRisposta(r);
  }

  Future<Map<String, dynamic>> _getJson(Uri uri) async {
    final r = await _get(uri);
    if (r.statusCode != 200) throw _errore(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  Future<http.Response> _get(Uri uri) => _esegui(() => _http.get(uri, headers: _intestazioni));

  /// Esegue una richiesta con timeout e traduce gli errori di rete in [ErroreApi] senza status.
  Future<http.Response> _esegui(Future<http.Response> Function() invio) async {
    try {
      return await invio().timeout(const Duration(seconds: 8));
    } on TimeoutException {
      throw ErroreApi(null, 'timeout');
    } on http.ClientException catch (e) {
      // Nel browser un blocco CORS arriva qui, senza status: non è distinguibile da "backend spento".
      throw ErroreApi(null, e.message);
    }
  }
}
