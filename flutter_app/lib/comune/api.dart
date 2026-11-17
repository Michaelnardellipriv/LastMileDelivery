// Parte comune ai client HTTP delle aree operatori e corrieri, secondo
// backend/api/openapi.yaml: modelli, login con token (POST /api/auth/login,
// Authorization: Bearer) e chiamate condivise. Le POST/PATCH mandano una
// Idempotency-Key nuova per ogni azione dell'utente, così un nuovo tentativo dopo
// un errore di rete non crea doppioni.
import 'dart:async';
import 'dart:convert';
import 'dart:math';

import 'package:flutter/foundation.dart' show protected;
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
  /// Operatore che ha fatto l'assegnazione attuale (assente per il cliente).
  final Persona? assegnataDa;
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
    this.assegnataDa,
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
        assegnataDa: j['assegnata_da'] == null ? null : Persona.daJson(j['assegnata_da'] as Map<String, dynamic>),
        idCorriere: j['id_corriere'] as int?,
        ritiro: Indirizzo.daJson(j['indirizzo_ritiro'] as Map<String, dynamic>),
        consegna: Indirizzo.daJson(j['indirizzo_spedizione'] as Map<String, dynamic>),
        note: [for (final n in (j['note'] as List? ?? const [])) Nota.daJson(n as Map<String, dynamic>)],
        tentativiRitiro: j['tentativi_ritiro_falliti'] as int?,
        tentativiConsegna: j['tentativi_consegna_falliti'] as int?,
      );
}

/// Una pagina di un elenco, con i totali calcolati dal backend
/// (totale_elementi, totale_pagine).
class Pagina<T> {
  final List<T> dati;
  final int pagina, perPagina, totaleElementi, totalePagine;
  Pagina(this.dati, this.pagina, this.perPagina, this.totaleElementi, this.totalePagine);

  factory Pagina.daJson(Map<String, dynamic> j, T Function(Map<String, dynamic>) daJson, {int pagina = 1, int perPagina = 20}) {
    final dati = [for (final v in j['dati'] as List) daJson(v as Map<String, dynamic>)];
    return Pagina(dati, j['pagina'] as int? ?? pagina, j['per_pagina'] as int? ?? perPagina,
        j['totale_elementi'] as int? ?? dati.length, j['totale_pagine'] as int? ?? 1);
  }

  bool get haPrecedente => pagina > 1;
  bool get haSuccessiva => pagina < totalePagine;

  /// Posizione degli elementi mostrati: "da–a di totaleElementi" (0–0 se la pagina è vuota).
  int get da => dati.isEmpty ? 0 : (pagina - 1) * perPagina + 1;
  int get a => dati.isEmpty ? 0 : da + dati.length - 1;

  /// La pagina richiesta è oltre l'ultima (per esempio dopo che gli elementi sono diminuiti).
  bool get oltreLaFine => dati.isEmpty && totalePagine > 0 && pagina > totalePagine;
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

/// Sessione di un utente verso il backend. Il login vale per tutti i ruoli: chi può
/// entrare e cosa può fare lo decide il backend. Ogni area (operatori, corrieri) la
/// estende con le chiamate del proprio ruolo.
class ApiSessione {
  final String base; // es. http://localhost:8080, senza barra finale
  final http.Client _http;
  final Random _caso = Random.secure();
  String? token;
  UtenteSessione? utente;
  void Function()? alScadereDelToken;

  ApiSessione({required String base, http.Client? client})
      : base = base.endsWith('/') ? base.substring(0, base.length - 1) : base,
        _http = client ?? http.Client();

  /// Stesso backend e stessa sessione di [altra]: dopo il login unico, l'area del ruolo
  /// continua con il token già ottenuto.
  ApiSessione.da(ApiSessione altra)
      : base = altra.base,
        _http = altra._http,
        token = altra.token,
        utente = altra.utente;

  String nuovaChiave() => List.generate(16, (_) => _caso.nextInt(256).toRadixString(16).padLeft(2, '0')).join();

  // ───── autenticazione ─────

  Future<UtenteSessione> login(String email, String password) async {
    final j = await chiama('POST', '/api/auth/login', corpo: {'email': email, 'password': password}) as Map<String, dynamic>;
    final u = UtenteSessione.daJson(j['utente'] as Map<String, dynamic>);
    token = j['token'] as String;
    utente = u;
    return u;
  }

  void esci() {
    token = null;
    utente = null;
  }

  // ───── spedizioni: chiamate comuni a più ruoli ─────

  /// Spedizioni per pagina, uguale al predefinito del backend.
  static const perPaginaSpedizioni = 10;

  Future<Pagina<Spedizione>> spedizioni(FiltroSpedizioni f, {int pagina = 1, int perPagina = perPaginaSpedizioni}) async {
    final j = await chiama('GET', '/api/spedizioni', query: {
      if (f.stato != null) 'stato': f.stato!,
      if (f.idCorriere != null) 'id_corriere': '${f.idCorriere}',
      if (f.dal != null) 'dal': dataIso(f.dal!),
      if (f.al != null) 'al': dataIso(f.al!),
      if (f.codOrdine != null && f.codOrdine!.isNotEmpty) 'cod_ordine': f.codOrdine!,
      'pagina': '$pagina',
      'per_pagina': '$perPagina',
    }) as Map<String, dynamic>;
    return Pagina.daJson(j, Spedizione.daJson, pagina: pagina, perPagina: perPagina);
  }

  Future<Spedizione> dettaglio(int id) async => Spedizione.daJson(await chiama('GET', '/api/spedizioni/$id') as Map<String, dynamic>);

  /// Cambia lo stato e restituisce quello registrato dal backend.
  Future<String> cambiaStato(int id, String stato, {String? motivo, String? nota}) async {
    final j = await chiama('PATCH', '/api/spedizioni/$id/stato', chiave: nuovaChiave(), corpo: {
      'stato': stato,
      if (motivo != null) 'motivo': motivo,
      if (nota != null) 'nota': nota,
    }) as Map<String, dynamic>;
    return j['stato'] as String;
  }

  /// Nota interna libera (POST /api/spedizioni/{id}/note).
  Future<void> aggiungiNota(int id, String testo) =>
      chiama('POST', '/api/spedizioni/$id/note', chiave: nuovaChiave(), corpo: {'nota': testo});

  /// Elenco unico dei motivi pubblici (GET /api/motivi): non è filtrato per stato.
  Future<List<String>>? _motivi;

  Future<List<String>> motivi() => _motivi ??= _caricaMotivi();

  Future<List<String>> _caricaMotivi() async {
    try {
      final j = await chiama('GET', '/api/motivi') as Map<String, dynamic>;
      return [for (final m in j['dati'] as List) m as String];
    } catch (_) {
      _motivi = null; // in caso di errore si riprova alla prossima richiesta
      rethrow;
    }
  }

  // ───── HTTP ─────

  /// Chiamata HTTP autenticata: restituisce il JSON della risposta o lancia [ErroreServer].
  @protected
  Future<Object?> chiama(String metodo, String percorso, {Map<String, String>? query, Object? corpo, String? chiave}) async {
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
