// Area riservata collegata al backend: instradamento per ruolo DOPO il login.
//
// Il login è uno solo (auth/login_page.dart → POST /api/auth/login → `store.sessione`). Questo widget
// non ha un proprio login: riceve la sessione già autenticata e apre la pagina del ruolo che il
// backend ha restituito:
//   operatore → HomeOperatore · corriere → HomeCorriere · amministratore → HomeAmministratore ·
//   cliente (destinatario) → TrackingClientePage.
// Chi può entrare lo decide il backend (per esempio rifiuta i clienti che sono solo mittenti).
// Da ogni area si apre il profilo con il cambio password (auth/profile_page.dart) e si esce.
import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;

import 'amministratore/api.dart';
import 'amministratore/app.dart';
import 'api.dart';
import 'auth/profile_page.dart';
import 'comune/api.dart' show ApiSessione, UtenteSessione;
import 'corriere/api.dart';
import 'corriere/app.dart';
import 'operatore/api.dart';
import 'operatore/app.dart';
import 'store.dart';
import 'tracking_page.dart';

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

  /// Stessa sessione del login, nel formato dei client per ruolo: il JWT parte come `Authorization: Bearer`.
  late final ApiSessione _accesso = ApiSessione(base: widget.apiBase, client: widget.client)
    ..token = widget.sessione.token
    ..utente = UtenteSessione(
      widget.sessione.idUtente,
      widget.sessione.idRuolo,
      widget.sessione.ruolo.name,
      widget.sessione.nome,
      widget.sessione.cognome,
      widget.sessione.email,
    );

  /// Client del profilo (cambio password) e del tracking cliente: stesso JWT.
  late final ApiClient _client =
  ApiClient.daSessione(widget.apiBase, widget.sessione, alScadereDelToken: _scaduto, client: widget.client);

  late final ApiOperatore _operatore = ApiOperatore.da(_accesso)..alScadereDelToken = _scaduto;
  late final ApiCorriere _corriere = ApiCorriere.da(_accesso)..alScadereDelToken = _scaduto;
  late final ApiAmministratore _amministratore = ApiAmministratore.da(_accesso)..alScadereDelToken = _scaduto;

  void _esci() {
    // Le schermate aperte sopra la home (profilo, dettagli) si chiudono prima di cambiare home.
    Navigator.of(context).popUntil((r) => r.isFirst);
    final fuori = widget.alLogout;
    if (fuori != null) {
      fuori();
    } else {
      store.esci();
    }
  }

  /// Il backend ha risposto 401 al token (scaduto o revocato): si torna al login con un avviso.
  void _scaduto() {
    if (!mounted) return;
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
  Widget build(BuildContext context) => switch (widget.sessione.ruolo) {
    RuoloApi.operatore => HomeOperatore(api: _operatore, alLogout: _esci, alProfilo: _apriProfilo),
    RuoloApi.amministratore => HomeAmministratore(api: _amministratore, alLogout: _esci, alProfilo: _apriProfilo),
  // Navigator proprio: il dettaglio si apre "dentro" l'area e sparisce con il logout.
  // NavigatorPopHandler fa funzionare il tasto indietro del telefono sul dettaglio.
    RuoloApi.corriere => NavigatorPopHandler(
      onPopWithResult: (_) => _navigatoreCorriere.currentState?.maybePop(),
      child: Navigator(
        key: _navigatoreCorriere,
        onGenerateRoute: (_) => MaterialPageRoute(
          builder: (_) => HomeCorriere(api: _corriere, alLogout: _esci, alProfilo: _apriProfilo),
        ),
      ),
    ),
  // il cliente destinatario ha il tracking dei suoi pacchi (tracking_page.dart)
    RuoloApi.cliente => NavigatorPopHandler(
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
