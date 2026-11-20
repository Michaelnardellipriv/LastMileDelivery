// Area riservata collegata al backend: un solo login per tutti gli utenti. Chi può
// entrare lo decide il backend (per esempio rifiuta i clienti che sono solo mittenti);
// qui si sceglie soltanto quale pagina aprire per il ruolo che il backend restituisce.
import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;

import 'amministratore/api.dart';
import 'api.dart' show ApiClient, RuoloApi;
import 'amministratore/app.dart';
import 'comune/comuni.dart';
import 'comune/login.dart';
import 'corriere/api.dart';
import 'corriere/app.dart';
import 'operatore/api.dart';
import 'operatore/app.dart';
import 'tracking_page.dart';

const _seme = Color(0xFF0B5FFF);

class AreaRiservataApp extends StatelessWidget {
  final String apiBase;
  const AreaRiservataApp({super.key, required this.apiBase});

  @override
  Widget build(BuildContext context) => MaterialApp(
        title: 'Veloce24',
        debugShowCheckedModeBanner: false,
        theme: ThemeData(colorSchemeSeed: _seme, useMaterial3: true),
        darkTheme: ThemeData(colorSchemeSeed: _seme, brightness: Brightness.dark, useMaterial3: true),
        home: AreaRiservata(apiBase: apiBase),
      );
}

/// Login e, dopo il login, la pagina del ruolo. Riutilizzabile anche dentro l'app
/// esistente (Navigator.push).
class AreaRiservata extends StatefulWidget {
  final String apiBase;
  final http.Client? client; // solo per i test
  const AreaRiservata({super.key, required this.apiBase, this.client});
  @override
  State<AreaRiservata> createState() => _AreaRiservataState();
}

class _AreaRiservataState extends State<AreaRiservata> {
  late final ApiSessione _accesso = ApiSessione(base: widget.apiBase, client: widget.client);
  ApiSessione? _sessione; // la sessione del ruolo, dopo il login
  final _navigatoreCorriere = GlobalKey<NavigatorState>();
  final _navigatoreCliente = GlobalKey<NavigatorState>();

  void _entrato() {
    final sessione = switch (_accesso.utente!.ruolo) {
      'operatore' => ApiOperatore.da(_accesso),
      'corriere' => ApiCorriere.da(_accesso),
      'amministratore' => ApiAmministratore.da(_accesso),
      _ => ApiSessione.da(_accesso),
    };
    sessione.alScadereDelToken = _scaduto;
    setState(() => _sessione = sessione);
  }

  void _esci() {
    _accesso.esci();
    setState(() => _sessione = null);
  }

  void _scaduto() {
    if (!mounted || _sessione == null) return;
    _esci();
    avviso(context, 'Sessione scaduta: rifai il login.');
  }

  @override
  Widget build(BuildContext context) {
    final sessione = _sessione;
    return switch (sessione) {
      null => PaginaLogin(api: _accesso, titolo: 'Veloce24', alLogin: _entrato),
      final ApiOperatore operatore => HomeOperatore(api: operatore, alLogout: _esci),
      final ApiAmministratore amministratore => HomeAmministratore(api: amministratore, alLogout: _esci),
      // Navigator proprio: il dettaglio si apre "dentro" l'area e sparisce con il logout.
      // NavigatorPopHandler fa funzionare il tasto indietro del telefono sul dettaglio.
      final ApiCorriere corriere => NavigatorPopHandler(
          onPopWithResult: (_) => _navigatoreCorriere.currentState?.maybePop(),
          child: Navigator(
            key: _navigatoreCorriere,
            onGenerateRoute: (_) => MaterialPageRoute(builder: (_) => HomeCorriere(api: corriere, alLogout: _esci)),
          ),
        ),
      // il cliente destinatario ha il tracking dei suoi pacchi (tracking_page.dart)
      _ when sessione.utente!.ruolo == 'cliente' => NavigatorPopHandler(
          onPopWithResult: (_) => _navigatoreCliente.currentState?.maybePop(),
          child: Navigator(
            key: _navigatoreCliente,
            onGenerateRoute: (_) => MaterialPageRoute(
              builder: (_) => TrackingClientePage(
                api: ApiClient(
                  base: sessione.base,
                  ruolo: RuoloApi.cliente,
                  idUtente: sessione.utente!.id,
                  token: sessione.token,
                  client: widget.client,
                  alScadereDelToken: _scaduto,
                ),
                alLogout: _esci,
              ),
            ),
          ),
        ),
      _ => PaginaSenzaArea(utente: sessione.utente!, alLogout: _esci),
    };
  }
}

/// Per i ruoli che non hanno una pagina (oggi nessuno: resta per sicurezza).
class PaginaSenzaArea extends StatelessWidget {
  final UtenteSessione utente;
  final VoidCallback alLogout;
  const PaginaSenzaArea({super.key, required this.utente, required this.alLogout});

  @override
  Widget build(BuildContext context) => Scaffold(
        appBar: AppBar(
          title: Text('Veloce24 · ${utente.nomeCompleto}', style: const TextStyle(fontSize: 16)),
          actions: [IconButton(tooltip: 'Esci', icon: const Icon(Icons.logout), onPressed: alLogout)],
        ),
        body: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 420),
            child: Padding(
              padding: const EdgeInsets.all(24),
              child: Column(mainAxisSize: MainAxisSize.min, children: [
                Icon(Icons.construction_outlined, size: 48, color: Theme.of(context).colorScheme.outline),
                const SizedBox(height: 16),
                Text('Accesso riuscito come ${utente.ruolo}', style: Theme.of(context).textTheme.titleLarge, textAlign: TextAlign.center),
                const SizedBox(height: 8),
                const Text('Area non ancora disponibile per il tuo ruolo.', textAlign: TextAlign.center),
                const SizedBox(height: 24),
                OutlinedButton.icon(onPressed: alLogout, icon: const Icon(Icons.logout), label: const Text('Esci')),
              ]),
            ),
          ),
        ),
      );
}
