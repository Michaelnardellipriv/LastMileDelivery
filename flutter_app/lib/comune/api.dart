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

/// Sede operativa di una città (GET /api/sedi): le sue spedizioni hanno ritiro e
/// consegna in quella città. Una sede disattivata non accetta nuove spedizioni né nuovi
/// corrieri o operatori.
/// [gruppoSpedizioni] è il database delle spedizioni che la sede usa (0 = principale):
/// si sceglie alla creazione e poi non cambia.
class Sede {
  final int id;
  final String nome;
  final Indirizzo indirizzo; // dà anche città, regione e nazione della sede
  final bool attiva;
  final int gruppoSpedizioni;
  Sede(this.id, this.nome, this.indirizzo, [this.attiva = true, this.gruppoSpedizioni = 0]);
  factory Sede.daJson(Map<String, dynamic> j) => Sede(j['id'] as int, j['nome'] as String? ?? '',
      Indirizzo.daJson(j['indirizzo'] as Map<String, dynamic>? ?? const {}), j['attiva'] as bool? ?? true,
      j['gruppo_spedizioni'] as int? ?? 0);
  String get citta => indirizzo.citta ?? '';
}

/// Voce di GET /api/spedizioni o dettaglio di GET /api/spedizioni/{id}.
class Spedizione {
  final int id;

  /// Sede che gestisce la spedizione e il suo nome.
  final int? idSede;
  final String? sede;
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
    this.idSede,
    this.sede,
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
        idSede: j['id_sede'] as int?,
        sede: j['sede'] as String?,
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
    return Pagina(dati, j['pagina'] as int? ?? pagina, j['per_pagina'] as int? ?? perPagina, j['totale_elementi'] as int? ?? dati.length,
        j['totale_pagine'] as int? ?? 1);
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

  /// Sede di operatori e corrieri (null per clienti e amministratori).
  final int? idSede;
  UtenteSessione(this.idUtente, this.id, this.ruolo, this.nome, this.cognome, this.email, [this.idSede]);
  factory UtenteSessione.daJson(Map<String, dynamic> j) => UtenteSessione(j['id_utente'] as int, j['id'] as int, j['ruolo'] as String,
      j['nome'] as String? ?? '', j['cognome'] as String? ?? '', j['email'] as String? ?? '', j['id_sede'] as int?);
  String get nomeCompleto => '$nome $cognome'.trim();
}

class FiltroSpedizioni {
  final String? stato, codOrdine;
  final int? idCorriere;

  /// Solo per l'amministratore: operatori e corrieri vedono già solo la propria sede.
  final int? idSede;
  final DateTime? dal, al;
  const FiltroSpedizioni({this.stato, this.codOrdine, this.idCorriere, this.idSede, this.dal, this.al});
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

  /// Scadenza del token e sua durata. Il token è breve (15 minuti): il servizio spedizioni
  /// se ne fida fino alla scadenza, quindi è il tempo massimo in cui un utente disattivato
  /// può ancora lavorare. Si rinnova da solo (POST /api/auth/rinnova) a metà della durata,
  /// finché il backend dice che l'utente è attivo. La scadenza è calcolata con l'orologio
  /// di questo dispositivo (adesso + durata_secondi), non confrontando la scade_il del
  /// server con un orologio che può essere sbagliato.
  DateTime? scadeIl;
  Duration? _durata;
  Timer? _timerRinnovo;
  Future<bool>? _rinnovoInCorso;

  /// Cambia a ogni [esci]: un rinnovo partito prima del logout non riapre la sessione.
  int _generazione = 0;

  /// Chi ha una copia del token e deve ricevere quello nuovo (es. il worker del GPS).
  final List<void Function(String token)> alRinnovoToken = [];

  ApiSessione({required String base, http.Client? client})
      : base = base.endsWith('/') ? base.substring(0, base.length - 1) : base,
        _http = client ?? http.Client();

  /// Stesso backend e stessa sessione di [altra]: dopo il login unico, l'area del ruolo
  /// continua con il token già ottenuto.
  ApiSessione.da(ApiSessione altra)
      : base = altra.base,
        _http = altra._http,
        token = altra.token,
        utente = altra.utente,
        scadeIl = altra.scadeIl,
        _durata = altra._durata {
    // da qui in poi rinnova questa sessione, non quella del login
    altra._fermaRinnovo();
  }

  String nuovaChiave() => List.generate(16, (_) => _caso.nextInt(256).toRadixString(16).padLeft(2, '0')).join();

  // ───── autenticazione ─────

  /// Login a livello di API. L'app NON lo usa: l'unico login dell'interfaccia è auth/login_page.dart
  /// (AuthApi.login), che alimenta `store.sessione`; `AreaRiservata` ne ricava questa sessione con il
  /// suo token. Resta qui per i test dei client per ruolo e per gli script.
  Future<UtenteSessione> login(String email, String password) async {
    final j = await chiama('POST', '/api/auth/login', corpo: {'email': email, 'password': password}) as Map<String, dynamic>;
    final u = UtenteSessione.daJson(j['utente'] as Map<String, dynamic>);
    _nuovaSessione(j);
    utente = u;
    return u;
  }

  /// Parte da un token già ottenuto (login unico, auth/login_page.dart): la durata che resta si
  /// ricava da [scadeIl], così il rinnovo automatico sa quando partire. Dal primo rinnovo in poi
  /// vale la durata che dice il backend (durata_secondi).
  void usaToken(String token, {DateTime? scadeIl}) {
    this.token = token;
    final resta = scadeIl?.difference(DateTime.now().toUtc());
    if (resta != null && resta > Duration.zero) {
      _durata = resta;
      this.scadeIl = DateTime.now().add(resta);
    } else {
      // già scaduto (o senza scadenza): si rinnova subito, il backend accetta un token appena scaduto
      _durata = const Duration(seconds: 2);
      this.scadeIl = DateTime.now();
    }
  }

  void _nuovaSessione(Map<String, dynamic> j) {
    token = j['token'] as String;
    final adesso = DateTime.now();
    final secondi = j['durata_secondi'];
    Duration? durata = secondi is int && secondi > 0 ? Duration(seconds: secondi) : null;
    // backend senza durata_secondi: si ricava da scade_il (vale se l'orologio è giusto)
    durata ??= DateTime.tryParse(j['scade_il'] as String? ?? '')?.difference(adesso);
    if (durata != null && durata > Duration.zero) {
      _durata = durata;
      scadeIl = adesso.add(durata);
    } else {
      _durata = null;
      scadeIl = null;
    }
  }

  /// Rinnovo automatico a metà della durata del token, finché la sessione è aperta. Lo
  /// avvia l'area riservata dopo il login; [esci] lo ferma. Se un rinnovo non riesce
  /// (rete assente) si riprova ogni 30 secondi: il backend rinnova anche un token appena
  /// scaduto, finché la sessione non supera la durata massima (DURATA_SESSIONE).
  void avviaRinnovoAutomatico({Duration minimo = const Duration(seconds: 5)}) {
    _fermaRinnovo();
    final durata = _durata, scade = scadeIl;
    if (durata == null || scade == null || token == null) return;
    var attesa = scade.subtract(durata ~/ 2).difference(DateTime.now());
    if (attesa < minimo) attesa = minimo;
    final generazione = _generazione;
    _timerRinnovo = Timer(attesa, () async {
      final riuscito = await rinnova();
      if (generazione == _generazione && token != null) {
        avviaRinnovoAutomatico(minimo: riuscito ? const Duration(seconds: 5) : const Duration(seconds: 30));
      }
    });
  }

  void _fermaRinnovo() {
    _timerRinnovo?.cancel();
    _timerRinnovo = null;
  }

  /// Vero se il token ha superato metà della sua durata (o è già scaduto).
  bool get tokenDaRinnovare {
    final durata = _durata, scade = scadeIl;
    if (token == null || durata == null || scade == null) return false;
    return DateTime.now().isAfter(scade.subtract(durata ~/ 2));
  }

  /// Vero se al token manca poco (un minuto, o un quarto della durata se il token è più
  /// corto di 4 minuti) o è già scaduto.
  bool get _tokenInScadenza {
    final scade = scadeIl, durata = _durata;
    if (token == null || scade == null || durata == null) return false;
    var margine = const Duration(minutes: 1);
    if (durata ~/ 4 < margine) margine = durata ~/ 4;
    return DateTime.now().isAfter(scade.subtract(margine));
  }

  /// Il token da usare adesso. Se sta per scadere lo si rinnova prima; se ha solo
  /// superato metà della durata il rinnovo parte in sottofondo e la richiesta non aspetta
  /// (un servizio utenti lento non rallenta le spedizioni).
  Future<String?> tokenValido() async {
    if (_tokenInScadenza) {
      await rinnova();
    } else if (tokenDaRinnovare) {
      unawaited(rinnova());
    }
    return token;
  }

  /// Rinnova il token se ha superato metà della durata ([forza]: comunque).
  Future<void> rinnovaSeServe({bool forza = false}) async {
    if (forza || tokenDaRinnovare) await rinnova();
  }

  /// Chiede un token nuovo (POST /api/auth/rinnova); più chiamate insieme ne fanno una
  /// sola. Vero se è arrivato. Se il backend lo rifiuta (401: sessione oltre la durata
  /// massima o password cambiata; 403: utente disattivato) la sessione si chiude; se non
  /// risponde si riprova più tardi.
  Future<bool> rinnova() {
    if (token == null) return Future.value(false);
    return _rinnovoInCorso ??= _rinnova().whenComplete(() => _rinnovoInCorso = null);
  }

  Future<bool> _rinnova() async {
    final generazione = _generazione;
    try {
      final j = await chiama('POST', '/api/auth/rinnova', rinnovo: true) as Map<String, dynamic>;
      // nel frattempo l'utente è uscito: il token nuovo non serve più
      if (generazione != _generazione || token == null) return false;
      _nuovaSessione(j);
      utente = UtenteSessione.daJson(j['utente'] as Map<String, dynamic>);
      for (final f in List.of(alRinnovoToken)) {
        f(token!);
      }
      return true;
    } on ErroreServer catch (e) {
      if ((e.status == 401 || e.status == 403) && generazione == _generazione && token != null) _fineSessione();
      return false;
    } catch (_) {
      return false; // risposta inattesa: si riprova più tardi
    }
  }

  /// Logout: chiude la sessione sul server (POST /api/auth/esci), così il token smette
  /// subito di valere anche se qualcuno l'avesse copiato, poi esce. Se il server non
  /// risponde si esce lo stesso: la sessione scadrà da sola.
  Future<void> logout() async {
    if (token != null) {
      try {
        await chiama('POST', '/api/auth/esci', rinnovo: true);
      } catch (_) {
        // rete assente o sessione già chiusa: non cambia niente per l'utente
      }
    }
    esci();
  }

  void _fineSessione() {
    esci();
    alScadereDelToken?.call();
  }

  void esci() {
    _generazione++;
    _fermaRinnovo();
    token = null;
    utente = null;
    scadeIl = null;
    _durata = null;
  }

  // ───── spedizioni: chiamate comuni a più ruoli ─────

  /// Spedizioni per pagina, uguale al predefinito del backend.
  static const perPaginaSpedizioni = 10;

  Future<Pagina<Spedizione>> spedizioni(FiltroSpedizioni f, {int pagina = 1, int perPagina = perPaginaSpedizioni}) async {
    final j = await chiama('GET', '/api/spedizioni', query: {
      if (f.stato != null) 'stato': f.stato!,
      if (f.idCorriere != null) 'id_corriere': '${f.idCorriere}',
      if (f.idSede != null) 'id_sede': '${f.idSede}',
      if (f.dal != null) 'dal': dataIso(f.dal!),
      if (f.al != null) 'al': dataIso(f.al!),
      if (f.codOrdine != null && f.codOrdine!.isNotEmpty) 'cod_ordine': f.codOrdine!,
      'pagina': '$pagina',
      'per_pagina': '$perPagina',
    }) as Map<String, dynamic>;
    return Pagina.daJson(j, Spedizione.daJson, pagina: pagina, perPagina: perPagina);
  }

  Future<Spedizione> dettaglio(int id) async => Spedizione.daJson(await chiama('GET', '/api/spedizioni/$id') as Map<String, dynamic>);

  /// Una sede: l'amministratore le vede tutte, operatori e corrieri solo la propria.
  Future<Sede> sede(int id) async => Sede.daJson(await chiama('GET', '/api/sedi/$id') as Map<String, dynamic>);

  /// Sedi (GET /api/sedi): l'amministratore le vede tutte, operatori e corrieri solo la
  /// propria. [attiva] filtra per stato (null = tutte).
  Future<Pagina<Sede>> sedi({String q = '', bool? attiva, int pagina = 1, int perPagina = 100}) async {
    final j = await chiama('GET', '/api/sedi', query: {
      if (q.trim().isNotEmpty) 'q': q.trim(),
      if (attiva != null) 'attiva': '$attiva',
      'pagina': '$pagina',
      'per_pagina': '$perPagina',
    }) as Map<String, dynamic>;
    return Pagina.daJson(j, Sede.daJson, pagina: pagina, perPagina: perPagina);
  }

  /// Storico delle modifiche (operatori e amministratori).
  Future<List<VoceStorico>> storico(int id) async {
    final j = await chiama('GET', '/api/spedizioni/$id/storico') as Map<String, dynamic>;
    return [for (final v in j['dati'] as List) VoceStorico.daJson(v as Map<String, dynamic>)];
  }

  /// Corrieri attivi (operatori e amministratori), per il filtro delle spedizioni.
  Future<Pagina<Persona>> corrieri(String q, {int perPagina = 20}) => elenco('/api/corrieri', q, Persona.daJson, perPagina: perPagina);

  /// PATCH /api/utenti/{id} con i soli campi in [modifiche]. L'amministratore cambia
  /// dati, password e stato; l'operatore solo i dati dei clienti: lo decide il backend.
  Future<Map<String, dynamic>> modificaUtente(int idUtente, Map<String, Object?> modifiche, {String? chiave}) async =>
      await chiama('PATCH', '/api/utenti/$idUtente', chiave: chiave ?? nuovaChiave(), corpo: modifiche) as Map<String, dynamic>;

  /// Ricerca in un'anagrafica (GET con q e per_pagina).
  @protected
  Future<Pagina<T>> elenco<T>(String percorso, String q, T Function(Map<String, dynamic>) daJson, {int perPagina = 20}) async {
    final j =
    await chiama('GET', percorso, query: {if (q.trim().isNotEmpty) 'q': q.trim(), 'per_pagina': '$perPagina'}) as Map<String, dynamic>;
    return Pagina.daJson(j, daJson, perPagina: perPagina);
  }

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
  Future<Object?> chiama(String metodo, String percorso,
      {Map<String, String>? query, Object? corpo, String? chiave, bool rinnovo = false, bool riprova = true}) async {
    // un utente attivo non vede mai scadere il token (vedi tokenValido)
    if (!rinnovo && token != null) await tokenValido();
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
    if (r.statusCode == 401 && token != null && !rinnovo) {
      // token scaduto (app sospesa o senza rete): si rinnova e si ripete una volta la
      // richiesta, con la stessa Idempotency-Key.
      //  - rinnovo rifiutato (401/403): rinnova() ha già chiuso la sessione;
      //  - rinnovo non riuscito per la rete: la sessione resta, la richiesta fallisce e si
      //    riprova più tardi;
      //  - anche il token appena rinnovato è rifiutato: la sessione non vale, si esce.
      if (!riprova) {
        _fineSessione();
      } else {
        final vecchio = token;
        if (await rinnova() && token != null && token != vecchio) {
          return chiama(metodo, percorso, query: query, corpo: corpo, chiave: chiave, riprova: false);
        }
      }
    }
    if (r.statusCode == 403 && token != null && !rinnovo) {
      // 403 può voler dire "account disattivato" (il servizio spedizioni non lo distingue
      // da un permesso mancante): il rinnovo lo verifica. Se l'utente è disattivato
      // risponde 403 e rinnova() chiude la sessione; altrimenti dà solo un token nuovo.
      await rinnova();
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
