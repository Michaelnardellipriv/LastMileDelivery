// Area riservata collegata al backend: instradamento per ruolo DOPO il login.
//
// Il login è uno solo (auth/login_page.dart → POST /api/auth/login → `store.sessione`). Questo widget
// non ha un proprio login: riceve la sessione già autenticata e apre la pagina del ruolo che il
// backend ha restituito:
//   operatore → HomeOperatore · corriere → HomeCorriere · amministratore → HomeAmministratore ·
//   cliente (destinatario) → TrackingClientePage.
// Chi può entrare lo decide il backend (per esempio rifiuta i clienti che sono solo mittenti).
// Da ogni area si apre il profilo con il cambio password (auth/profile_page.dart) e si esce.
//
// Il token dura poco (15 minuti): qui si rinnova da solo (POST /api/auth/rinnova) finché la
// sessione è aperta, e il token nuovo si salva anche in `store` (così sopravvive a un ricaricamento
// della pagina). Il logout chiude la sessione anche sul server (POST /api/auth/esci): il token non
// vale più da subito.
import 'dart:async';

import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;

import 'amministratore/api.dart';
import 'amministratore/app.dart';
import 'api.dart';
import 'auth/profile_page.dart';
import 'corriere/api.dart';
import 'corriere/app.dart';
import 'operatore/api.dart';
import 'operatore/app.dart';
import 'store.dart';
import 'tracking_page.dart';

/// Indirizzo del backend. Con `--dart-define=API_BASE=...` vale quello. Senza, nel browser
/// l'API sta sullo stesso indirizzo della pagina: in Kubernetes l'Ingress manda `/api` al
/// backend e `/` al frontend, quindi la stessa immagine va bene in ogni ambiente. Fuori
/// dal browser (emulatore, desktop) resta il backend locale.
String baseApi(String definita, {bool web = kIsWeb, Uri? pagina}) {
  if (definita.isNotEmpty) return definita;
  if (web) return (pagina ?? Uri.base).origin;
  return 'http://localhost:8080';
}

class AreaRiservata extends StatefulWidget {
  final String apiBase;
  final Sessione sessione;

  /// Solo per i test: server finto al posto della rete.
  final http.Client? client;

  /// Predefiniti: `store.esci()` (cancella il JWT dal dispositivo e torna al login). Si sostituiscono nei test.
  final VoidCallback? alLogout;
  final VoidCallback? alSessioneScaduta;
  const AreaRiservata({
    super.key,
    required this.apiBase,
    required this.sessione,
    this.client,
    this.alLogout,
    this.alSessioneScaduta,
  });

  @override
  State<AreaRiservata> createState() => _AreaRiservataState();
}

class _AreaRiservataState extends State<AreaRiservata> {
  final _navigatoreCorriere = GlobalKey<NavigatorState>();
  final _navigatoreCliente = GlobalKey<NavigatorState>();
  bool _uscito = false;

  /// Stessa sessione del login, nel formato dei client per ruolo: il JWT parte come `Authorization: Bearer`.
  late final ApiSessione _accesso = ApiSessione(base: widget.apiBase, client: widget.client)
    ..usaToken(widget.sessione.token, scadeIl: widget.sessione.scadeIl)
    ..utente = UtenteSessione(
      widget.sessione.idUtente,
      widget.sessione.idRuolo,
      widget.sessione.ruolo.name,
      widget.sessione.nome,
      widget.sessione.cognome,
      widget.sessione.email,
      widget.sessione.idSede, // la sede dell'operatore: il nuovo indirizzo propone la sua città
    );

  /// La sessione del ruolo: è lei che rinnova il token, e tutti i client leggono il suo.
  late final ApiSessione _sessione = switch (widget.sessione.ruolo) {
    RuoloApi.operatore => ApiOperatore.da(_accesso),
    RuoloApi.corriere => ApiCorriere.da(_accesso),
    RuoloApi.amministratore => ApiAmministratore.da(_accesso),
    RuoloApi.cliente => ApiSessione.da(_accesso),
  };

  /// Client del profilo (cambio password) e del tracking cliente: stesso token, letto (e se serve
  /// rinnovato) a ogni richiesta; su un 401 prova prima a rinnovarlo.
  late final ApiClient _client = ApiClient.daSessione(
    widget.apiBase,
    widget.sessione,
    alScadereDelToken: _scaduto,
    client: widget.client,
    tokenAttuale: _sessione.tokenValido,
    rinnovaToken: () async {
      final vecchio = _sessione.token;
      return await _sessione.rinnova() && _sessione.token != null && _sessione.token != vecchio;
    },
  );

  @override
  void initState() {
    super.initState();
    _sessione
      ..alScadereDelToken = _scaduto
      // il token nuovo resta anche sul dispositivo: un ricaricamento della pagina non fa uscire
      ..alRinnovoToken.add((token) => store.aggiornaToken(token, _sessione.scadeIl))
      ..avviaRinnovoAutomatico();
  }

  @override
  void dispose() {
    _sessione.esci(); // ferma il rinnovo automatico
    super.dispose();
  }

  void _esci() {
    if (_uscito) return;
    _uscito = true;
    // la sessione si chiude anche sul server: il token non vale più da subito
    unawaited(_sessione.logout());
    // Le schermate aperte sopra la home (profilo, dettagli) si chiudono prima di cambiare home.
    Navigator.of(context).popUntil((r) => r.isFirst);
    final fuori = widget.alLogout;
    if (fuori != null) {
      fuori();
    } else {
      store.esci();
    }
  }

  /// Il backend ha rifiutato il token (401 scaduto o revocato, 403 utente disattivato):
  /// si torna al login con un avviso.
  void _scaduto() {
    if (!mounted || _uscito) return;
    _uscito = true;
    _sessione.esci();
    final scaduta = widget.alSessioneScaduta;
    if (scaduta != null) return scaduta();
    if (store.sessione == null) return; // già uscito (più richieste in volo possono fallire insieme)
    Navigator.of(context).popUntil((r) => r.isFirst);
    store.esci(avviso: 'Sessione scaduta: accedi di nuovo.');
  }

  void _apriProfilo() => Navigator.of(context).push(MaterialPageRoute<void>(
        builder: (_) => ProfilePage.backend(api: _client, sessione: widget.sessione, onLogout: _esci),
      ));

  @override
  Widget build(BuildContext context) => switch (_sessione) {
        final ApiOperatore operatore => HomeOperatore(api: operatore, alLogout: _esci, alProfilo: _apriProfilo),
        final ApiAmministratore amministratore =>
          HomeAmministratore(api: amministratore, alLogout: _esci, alProfilo: _apriProfilo),
        // Navigator proprio: il dettaglio si apre "dentro" l'area e sparisce con il logout.
        // NavigatorPopHandler fa funzionare il tasto indietro del telefono sul dettaglio.
        final ApiCorriere corriere => NavigatorPopHandler(
            onPopWithResult: (_) => _navigatoreCorriere.currentState?.maybePop(),
            child: Navigator(
              key: _navigatoreCorriere,
              onGenerateRoute: (_) => MaterialPageRoute(
                builder: (_) => HomeCorriere(api: corriere, alLogout: _esci, alProfilo: _apriProfilo),
              ),
            ),
          ),
        // il cliente destinatario ha il tracking dei suoi pacchi (tracking_page.dart)
        _ => NavigatorPopHandler(
            onPopWithResult: (_) => _navigatoreCliente.currentState?.maybePop(),
            child: Navigator(
              key: _navigatoreCliente,
              onGenerateRoute: (_) => MaterialPageRoute(
                builder: (_) => TrackingClientePage(api: _client, alLogout: _esci, alProfilo: _apriProfilo),
              ),
            ),
          ),
      };
}
