// Client HTTP dell'area operatori, secondo backend/api/openapi.yaml.
// L'operatore fa il login (POST /api/auth/login) e ogni chiamata porta il token
// in Authorization: Bearer. Le POST/PATCH mandano una Idempotency-Key nuova per
// ogni azione dell'utente, così un nuovo tentativo dopo un errore di rete non
// crea doppioni.
import 'dart:async';
import 'dart:convert';
import 'dart:math';

import 'package:http/http.dart' as http;

class ErroreServer implements Exception {
  final int? status; // null = backend non raggiungibile (rete, CORS, timeout)
  final String messaggio;
  final Map<String, String> campi; // errori per campo, dal campo "dettagli"
  /// Dettagli tecnici (metodo, percorso, risposta grezza): vanno in console, non a schermo.
  final String tecnico;
  ErroreServer(this.status, this.messaggio, [this.campi = const {}, this.tecnico = '']);

  /// Messaggio breve per l'utente. I dettagli dei campi si mostrano accanto ai campi.
  String get perUtente => switch (status) {
        null => 'Impossibile contattare il server. Controlla la connessione e riprova.',
        401 => 'Sessione scaduta o credenziali non valide: accedi di nuovo.',
        404 => 'Elemento non trovato: potrebbe essere stato modificato. Aggiorna la pagina.',
        >= 500 => 'Il server ha avuto un problema. Riprova tra poco.',
        _ when campi.isNotEmpty => 'Alcuni dati non sono validi: controlla i campi evidenziati.',
        _ => messaggio,
      };

  @override
  String toString() => 'ErroreServer($status, $messaggio, campi: $campi${tecnico.isEmpty ? '' : ', $tecnico'})';
}

DateTime? _data(Object? v) => v == null ? null : DateTime.parse(v as String).toLocal();

String? _telefono(Map<String, dynamic> j) =>
    j['telefono'] as String? ?? (j['prefisso'] == null ? null : '${j['prefisso']} ${j['nr_telefono']}');

class Persona {
  final int? id;
  final String nome, cognome;
  final String? email, telefono;
  Persona(this.id, this.nome, this.cognome, this.email, this.telefono);
  factory Persona.daJson(Map<String, dynamic> j) =>
      Persona(j['id'] as int?, j['nome'] as String? ?? '', j['cognome'] as String? ?? '', j['email'] as String?, _telefono(j));
  String get nomeCompleto => '$nome $cognome'.trim();
}

class Indirizzo {
  final int? id;
  final String via, civico;
  final String? citta, cap, regione, nazione;
  Indirizzo(this.id, this.via, this.civico, this.citta, this.cap, this.regione, this.nazione);
  factory Indirizzo.daJson(Map<String, dynamic> j) => Indirizzo(j['id'] as int?, j['via'] as String? ?? '', j['civico'] as String? ?? '',
      j['citta'] as String?, j['cap'] as String?, j['regione'] as String?, j['nazione'] as String?);
  String get riga1 => '$via $civico'.trim();
  String get riga2 =>
      [if (cap != null) cap, if (citta != null) citta, if (regione != null) '($regione)', if (nazione != null) nazione].join(' ');
  String get testo => [riga1, riga2].where((s) => s.isNotEmpty).join(', ');
}

class Autore {
  final String ruolo;
  final int id;
  final String? nome, cognome;
  Autore(this.ruolo, this.id, this.nome, this.cognome);
  factory Autore.daJson(Map<String, dynamic> j) =>
      Autore(j['ruolo'] as String? ?? '', j['id'] as int? ?? 0, j['nome'] as String?, j['cognome'] as String?);
  /// Chi ha fatto l'azione: nome, cognome, ruolo e id.
  String get testo => nome == null ? '$ruolo · id $id' : '$nome $cognome ($ruolo · id $id)';
}

class Nota {
  final int id;
  final String testo;
  final bool interna;
  final String? motivo;
  final DateTime creataIl;
  final Autore? autore;
  Nota(this.id, this.testo, this.interna, this.motivo, this.creataIl, this.autore);
  factory Nota.daJson(Map<String, dynamic> j) => Nota(j['id'] as int, j['nota'] as String? ?? '', j['is_interna'] as bool? ?? true,
      j['motivo'] as String?, _data(j['creata_il'])!, j['autore'] == null ? null : Autore.daJson(j['autore'] as Map<String, dynamic>));
}

/// Voce di GET /api/spedizioni o dettaglio di GET /api/spedizioni/{id}.
class Spedizione {
  final int id;
  final String codOrdine, stato;
  final bool isReso;
  final DateTime creataIl, richiestaIl;
  final DateTime? ritirataIl, conclusaIl, annullataIl;
  final Persona? mittente, corriere;
  final Persona destinatario;
  final int? idCorriere;
  final Indirizzo ritiro, consegna;
  final List<Nota> note;
  final int? tentativiRitiro, tentativiConsegna;

  Spedizione({
    required this.id,
    required this.codOrdine,
    required this.stato,
    required this.isReso,
    required this.creataIl,
    required this.richiestaIl,
    this.ritirataIl,
    this.conclusaIl,
    this.annullataIl,
    this.mittente,
    required this.destinatario,
    this.corriere,
    this.idCorriere,
    required this.ritiro,
    required this.consegna,
    required this.note,
    this.tentativiRitiro,
    this.tentativiConsegna,
  });

  factory Spedizione.daJson(Map<String, dynamic> j) => Spedizione(
        id: j['id'] as int,
        codOrdine: j['cod_ordine'] as String,
        stato: j['stato'] as String,
        isReso: j['is_reso'] as bool? ?? false,
        creataIl: _data(j['creata_il'])!,
        richiestaIl: _data(j['dataora_richiesta_spedizione'])!,
        ritirataIl: _data(j['ritirata_il']),
        conclusaIl: _data(j['conclusa_il']),
        annullataIl: _data(j['annullata_il']),
        mittente: j['mittente'] == null ? null : Persona.daJson(j['mittente'] as Map<String, dynamic>),
        destinatario: Persona.daJson(j['destinatario'] as Map<String, dynamic>? ?? {'id': j['id_cliente_destinatario']}),
        corriere: j['corriere'] == null ? null : Persona.daJson(j['corriere'] as Map<String, dynamic>),
        idCorriere: j['id_corriere'] as int?,
        ritiro: Indirizzo.daJson(j['indirizzo_ritiro'] as Map<String, dynamic>),
        consegna: Indirizzo.daJson(j['indirizzo_spedizione'] as Map<String, dynamic>),
        note: [for (final n in (j['note'] as List? ?? const [])) Nota.daJson(n as Map<String, dynamic>)],
        tentativiRitiro: j['tentativi_ritiro_falliti'] as int?,
        tentativiConsegna: j['tentativi_consegna_falliti'] as int?,
      );
}

class Pagina<T> {
  final List<T> dati;
  final int pagina, perPagina;
  Pagina(this.dati, this.pagina, this.perPagina);
  bool get forseAltre => dati.length == perPagina;
}

class VoceStorico {
  final int id;
  final String operazione;
  final Map<String, dynamic>? prima;
  final Map<String, dynamic> dopo;
  final DateTime quando;
  final Autore autore;
  VoceStorico(this.id, this.operazione, this.prima, this.dopo, this.quando, this.autore);
  factory VoceStorico.daJson(Map<String, dynamic> j) => VoceStorico(
        j['id'] as int,
        j['operazione'] as String,
        j['valore_vecchio'] as Map<String, dynamic>?,
        j['valore_nuovo'] as Map<String, dynamic>? ?? const {},
        _data(j['modificato_il'])!,
        Autore.daJson(j['autore'] as Map<String, dynamic>),
      );
}

class PosizioneCorriere {
  final int idCorriere;
  final String nome, cognome;
  final double latitudine, longitudine;
  final DateTime rilevataIl;
  final List<int> spedizioni;
  PosizioneCorriere(this.idCorriere, this.nome, this.cognome, this.latitudine, this.longitudine, this.rilevataIl, this.spedizioni);
  factory PosizioneCorriere.daJson(Map<String, dynamic> j) => PosizioneCorriere(
        j['id_corriere'] as int,
        j['nome'] as String? ?? '',
        j['cognome'] as String? ?? '',
        (j['latitudine'] as num).toDouble(),
        (j['longitudine'] as num).toDouble(),
        _data(j['rilevata_il'])!,
        [for (final s in (j['spedizioni_attive'] as List? ?? const [])) s as int],
      );
  String get nomeCompleto => '$nome $cognome'.trim();
}

class Posizioni {
  final int intervalloSecondi;
  final List<PosizioneCorriere> corrieri;
  Posizioni(this.intervalloSecondi, this.corrieri);
  PosizioneCorriere? perSpedizione(int id) => corrieri.where((c) => c.spedizioni.contains(id)).firstOrNull;
}

class UtenteSessione {
  final int idUtente, id;
  final String ruolo, nome, cognome, email;
  UtenteSessione(this.idUtente, this.id, this.ruolo, this.nome, this.cognome, this.email);
  factory UtenteSessione.daJson(Map<String, dynamic> j) => UtenteSessione(j['id_utente'] as int, j['id'] as int, j['ruolo'] as String,
      j['nome'] as String? ?? '', j['cognome'] as String? ?? '', j['email'] as String? ?? '');
  String get nomeCompleto => '$nome $cognome'.trim();
}

class FiltroSpedizioni {
  final String? stato, codOrdine;
  final int? idCorriere;
  final DateTime? dal, al;
  const FiltroSpedizioni({this.stato, this.codOrdine, this.idCorriere, this.dal, this.al});
}

String dataIso(DateTime d) =>
    '${d.year.toString().padLeft(4, '0')}-${d.month.toString().padLeft(2, '0')}-${d.day.toString().padLeft(2, '0')}';

const statiSpedizione = ['creata', 'assegnata', 'ritirata', 'in consegna', 'consegnata', 'non consegnata', 'persa', 'annullata'];

class ApiOperatore {
  final String base; // es. http://localhost:8080, senza barra finale
  final http.Client _http;
  final Random _caso = Random.secure();
  String? token;
  UtenteSessione? utente;
  void Function()? alScadereDelToken;

  ApiOperatore({required String base, http.Client? client})
      : base = base.endsWith('/') ? base.substring(0, base.length - 1) : base,
        _http = client ?? http.Client();

  String nuovaChiave() => List.generate(16, (_) => _caso.nextInt(256).toRadixString(16).padLeft(2, '0')).join();

  // ───── autenticazione ─────

  Future<UtenteSessione> login(String email, String password) async {
    final j = await _chiama('POST', '/api/auth/login', corpo: {'email': email, 'password': password}) as Map<String, dynamic>;
    final u = UtenteSessione.daJson(j['utente'] as Map<String, dynamic>);
    if (u.ruolo != 'operatore') {
      throw ErroreServer(403, 'Questa pagina è riservata agli operatori logistici (il tuo ruolo è ${u.ruolo}).');
    }
    token = j['token'] as String;
    utente = u;
    return u;
  }

  void esci() {
    token = null;
    utente = null;
  }

  // ───── spedizioni ─────

  Future<Pagina<Spedizione>> spedizioni(FiltroSpedizioni f, {int pagina = 1, int perPagina = 20}) async {
    final j = await _chiama('GET', '/api/spedizioni', query: {
      if (f.stato != null) 'stato': f.stato!,
      if (f.idCorriere != null) 'id_corriere': '${f.idCorriere}',
      if (f.dal != null) 'dal': dataIso(f.dal!),
      if (f.al != null) 'al': dataIso(f.al!),
      if (f.codOrdine != null && f.codOrdine!.isNotEmpty) 'cod_ordine': f.codOrdine!,
      'pagina': '$pagina',
      'per_pagina': '$perPagina',
    }) as Map<String, dynamic>;
    return Pagina([for (final v in j['dati'] as List) Spedizione.daJson(v as Map<String, dynamic>)], j['pagina'] as int? ?? pagina,
        j['per_pagina'] as int? ?? perPagina);
  }

  Future<Spedizione> dettaglio(int id) async => Spedizione.daJson(await _chiama('GET', '/api/spedizioni/$id') as Map<String, dynamic>);

  Future<List<VoceStorico>> storico(int id) async {
    final j = await _chiama('GET', '/api/spedizioni/$id/storico') as Map<String, dynamic>;
    return [for (final v in j['dati'] as List) VoceStorico.daJson(v as Map<String, dynamic>)];
  }

  /// Crea la spedizione. La risposta di POST /api/spedizioni è la spedizione "piatta"
  /// (solo id, senza persone e indirizzi): da qui servono id, codice e stato, il
  /// dettaglio completo si legge poi con [dettaglio]. La data richiesta la sceglie
  /// l'operatore; la data del ritiro la registra il sistema, quindi non si manda.
  Future<({int id, String codOrdine, String stato})> creaSpedizione({
    required String codOrdine,
    int? idMittente,
    int? idDestinatario,
    int? idRitiro,
    int? idConsegna,
    DateTime? richiestaIl,
    int? idCorriere,
    required String chiave,
  }) async {
    // I campi non scelti non vengono mandati: è il backend a dire quali mancano.
    final j = await _chiama('POST', '/api/spedizioni', chiave: chiave, corpo: {
      'cod_ordine': codOrdine,
      if (idMittente != null) 'id_cliente_mittente': idMittente,
      if (idDestinatario != null) 'id_cliente_destinatario': idDestinatario,
      if (idRitiro != null) 'id_indirizzo_ritiro': idRitiro,
      if (idConsegna != null) 'id_indirizzo_spedizione': idConsegna,
      if (richiestaIl != null) 'dataora_richiesta_spedizione': richiestaIl.toUtc().toIso8601String(),
      if (idCorriere != null) 'id_corriere': idCorriere,
    }) as Map<String, dynamic>;
    return (id: j['id'] as int, codOrdine: j['cod_ordine'] as String, stato: j['stato'] as String);
  }

  Future<void> assegna(int id, int idCorriere) =>
      _chiama('POST', '/api/spedizioni/$id/assegnazioni', chiave: nuovaChiave(), corpo: {'id_corriere': idCorriere});

  Future<void> cambiaStato(int id, String stato, {String? motivo, String? nota}) =>
      _chiama('PATCH', '/api/spedizioni/$id/stato', chiave: nuovaChiave(), corpo: {
        'stato': stato,
        if (motivo != null) 'motivo': motivo,
        if (nota != null) 'nota': nota,
      });

  Future<void> aggiungiNota(int id, String testo) =>
      _chiama('POST', '/api/spedizioni/$id/note', chiave: nuovaChiave(), corpo: {'nota': testo});

  /// Elenco unico dei motivi pubblici (GET /api/motivi): non è filtrato per stato.
  Future<List<String>> motivi() async {
    final j = await _chiama('GET', '/api/motivi') as Map<String, dynamic>;
    return [for (final m in j['dati'] as List) m as String];
  }

  Future<Posizioni> posizioni() async {
    final j = await _chiama('GET', '/api/corrieri/posizioni') as Map<String, dynamic>;
    return Posizioni(j['intervallo_secondi'] as int? ?? 30,
        [for (final c in j['corrieri'] as List) PosizioneCorriere.daJson(c as Map<String, dynamic>)]);
  }

  // ───── anagrafiche ─────

  Future<List<Persona>> clienti(String q) => _elenco('/api/clienti', q, Persona.daJson);
  Future<List<Persona>> corrieri(String q, {int perPagina = 20}) => _elenco('/api/corrieri', q, Persona.daJson, perPagina: perPagina);
  Future<List<Indirizzo>> indirizzi(String q) => _elenco('/api/indirizzi', q, Indirizzo.daJson);

  /// Restituisce il cliente creato e se la mail con le credenziali è partita.
  Future<(Persona, bool)> creaCliente({
    required String nome,
    required String cognome,
    required String prefisso,
    required String telefono,
    required String email,
    required String password,
    required String chiave,
  }) async {
    final j = await _chiama('POST', '/api/clienti', chiave: chiave, corpo: {
      'nome': nome,
      'cognome': cognome,
      'prefisso': prefisso,
      'nr_telefono': telefono,
      'email': email,
      'password': password,
    }) as Map<String, dynamic>;
    return (Persona.daJson(j['cliente'] as Map<String, dynamic>), j['mail_inviata'] as bool? ?? false);
  }

  Future<Indirizzo> creaIndirizzo({
    required String via,
    required String civico,
    required String citta,
    required String cap,
    required String regione,
    required String nazione,
    required String chiave,
  }) async =>
      Indirizzo.daJson(await _chiama('POST', '/api/indirizzi', chiave: chiave, corpo: {
        'via': via,
        'civico': civico,
        'citta': citta,
        'cap': cap,
        'regione': regione,
        'nazione': nazione,
      }) as Map<String, dynamic>);

  Future<List<T>> _elenco<T>(String percorso, String q, T Function(Map<String, dynamic>) daJson, {int perPagina = 20}) async {
    final j =
        await _chiama('GET', percorso, query: {if (q.trim().isNotEmpty) 'q': q.trim(), 'per_pagina': '$perPagina'}) as Map<String, dynamic>;
    return [for (final v in j['dati'] as List) daJson(v as Map<String, dynamic>)];
  }

  // ───── HTTP ─────

  Future<Object?> _chiama(String metodo, String percorso, {Map<String, String>? query, Object? corpo, String? chiave}) async {
    final uri = Uri.parse('$base$percorso').replace(queryParameters: query == null || query.isEmpty ? null : query);
    final intestazioni = {
      'Accept': 'application/json',
      if (corpo != null) 'Content-Type': 'application/json',
      if (token != null) 'Authorization': 'Bearer $token',
      if (chiave != null) 'Idempotency-Key': chiave,
    };
    final richiesta = http.Request(metodo, uri)..headers.addAll(intestazioni);
    if (corpo != null) richiesta.body = jsonEncode(corpo);

    late http.Response r;
    try {
      r = await http.Response.fromStream(await _http.send(richiesta).timeout(const Duration(seconds: 10)));
    } on TimeoutException {
      throw ErroreServer(null, 'timeout', const {}, '$metodo $uri: nessuna risposta entro 10 s');
    } on http.ClientException catch (e) {
      // Nel browser un blocco CORS arriva qui, senza status: non è distinguibile da "backend spento".
      throw ErroreServer(null, e.message, const {}, '$metodo $uri: ${e.message} (backend spento o CORS_ORIGIN diverso da questa pagina?)');
    }

    if (r.statusCode == 204) return null;
    final testo = utf8.decode(r.bodyBytes, allowMalformed: true);
    if (r.statusCode >= 200 && r.statusCode < 300) {
      return testo.isEmpty ? null : jsonDecode(testo);
    }
    if (r.statusCode == 401 && token != null) {
      esci();
      alScadereDelToken?.call();
    }
    throw _errore(r.statusCode, testo, '$metodo $uri → ${r.statusCode}: $testo');
  }

  static ErroreServer _errore(int status, String testo, String tecnico) {
    try {
      final j = jsonDecode(testo) as Map<String, dynamic>;
      final campi = <String, String>{
        for (final d in (j['dettagli'] as List? ?? const [])) (d as Map<String, dynamic>)['campo'] as String: d['messaggio'] as String,
      };
      return ErroreServer(status, j['errore'] as String? ?? 'errore $status', campi, tecnico);
    } catch (_) {
      return ErroreServer(status, testo.trim().isEmpty ? 'errore HTTP $status' : testo.trim(), const {}, tecnico);
    }
  }
}
