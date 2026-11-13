// Area operatori (pagina web): login, spedizioni con filtri e dettaglio, nuova
// spedizione, mappa dei corrieri in consegna.
import 'package:flutter/material.dart';

import 'api.dart';
import 'comuni.dart';
import 'mappa.dart';
import 'nuova_spedizione.dart';
import 'spedizioni.dart';

const _seme = Color(0xFF0B5FFF);

class OperatoreApp extends StatelessWidget {
  final String apiBase;
  const OperatoreApp({super.key, required this.apiBase});

  @override
  Widget build(BuildContext context) => MaterialApp(
        title: 'Veloce24 · Operatori',
        debugShowCheckedModeBanner: false,
        theme: ThemeData(colorSchemeSeed: _seme, useMaterial3: true),
        darkTheme: ThemeData(colorSchemeSeed: _seme, brightness: Brightness.dark, useMaterial3: true),
        home: AreaOperatore(apiBase: apiBase),
      );
}

/// Punto di ingresso riutilizzabile anche dentro l'app esistente (Navigator.push).
class AreaOperatore extends StatefulWidget {
  final String apiBase;
  const AreaOperatore({super.key, required this.apiBase});
  @override
  State<AreaOperatore> createState() => _AreaOperatoreState();
}

class _AreaOperatoreState extends State<AreaOperatore> {
  late final ApiOperatore api = ApiOperatore(base: widget.apiBase)..alScadereDelToken = _scaduto;
  bool _scadutoMostrato = false;

  void _scaduto() {
    if (!mounted) return;
    setState(() {});
    if (!_scadutoMostrato) {
      _scadutoMostrato = true;
      avviso(context, 'Sessione scaduta: rifai il login.');
    }
  }

  @override
  Widget build(BuildContext context) => api.utente == null
      ? LoginOperatore(api: api, alLogin: () => setState(() => _scadutoMostrato = false))
      : HomeOperatore(api: api, alLogout: () => setState(api.esci));
}

class LoginOperatore extends StatefulWidget {
  final ApiOperatore api;
  final VoidCallback alLogin;
  const LoginOperatore({super.key, required this.api, required this.alLogin});
  @override
  State<LoginOperatore> createState() => _LoginOperatoreState();
}

class _LoginOperatoreState extends State<LoginOperatore> {
  final _email = TextEditingController();
  final _password = TextEditingController();
  bool _attesa = false;
  bool _nascosta = true;
  String? _errore;

  Future<void> _entra() async {
    setState(() {
      _attesa = true;
      _errore = null;
    });
    try {
      await widget.api.login(_email.text, _password.text);
      widget.alLogin();
    } catch (e) {
      setState(() => _errore = testoErrore(e));
    } finally {
      if (mounted) setState(() => _attesa = false);
    }
  }

  @override
  Widget build(BuildContext context) => Scaffold(
        body: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 420),
            child: Card(
              margin: const EdgeInsets.all(16),
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: AutofillGroup(
                  child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.stretch, children: [
                    Text('Veloce24 · Operatori', style: Theme.of(context).textTheme.headlineSmall),
                    const SizedBox(height: 4),
                    Text('Accedi con email e password', style: Theme.of(context).textTheme.bodyMedium),
                    const SizedBox(height: 24),
                    TextField(
                      controller: _email,
                      decoration: const InputDecoration(labelText: 'Email', border: OutlineInputBorder()),
                      keyboardType: TextInputType.emailAddress,
                      autofillHints: const [AutofillHints.email],
                      textInputAction: TextInputAction.next,
                    ),
                    const SizedBox(height: 12),
                    TextField(
                      controller: _password,
                      obscureText: _nascosta,
                      autofillHints: const [AutofillHints.password],
                      decoration: InputDecoration(
                        labelText: 'Password',
                        border: const OutlineInputBorder(),
                        suffixIcon: IconButton(
                          tooltip: _nascosta ? 'Mostra' : 'Nascondi',
                          icon: Icon(_nascosta ? Icons.visibility : Icons.visibility_off),
                          onPressed: () => setState(() => _nascosta = !_nascosta),
                        ),
                      ),
                      onSubmitted: (_) => _attesa ? null : _entra(),
                    ),
                    if (_errore != null) ...[
                      const SizedBox(height: 12),
                      Text(_errore!, style: TextStyle(color: Theme.of(context).colorScheme.error)),
                    ],
                    const SizedBox(height: 20),
                    FilledButton(
                      onPressed: _attesa ? null : _entra,
                      child: _attesa
                          ? const SizedBox(height: 18, width: 18, child: CircularProgressIndicator(strokeWidth: 2))
                          : const Text('Entra'),
                    ),
                    const SizedBox(height: 12),
                    Text('Backend: ${widget.api.base}', style: Theme.of(context).textTheme.bodySmall, textAlign: TextAlign.center),
                  ]),
                ),
              ),
            ),
          ),
        ),
      );
}

class HomeOperatore extends StatefulWidget {
  final ApiOperatore api;
  final VoidCallback alLogout;
  const HomeOperatore({super.key, required this.api, required this.alLogout});
  @override
  State<HomeOperatore> createState() => _HomeOperatoreState();
}

class _HomeOperatoreState extends State<HomeOperatore> {
  int _sezione = 0;
  final _spedizioni = GlobalKey<SpedizioniPageState>();

  void _apriSpedizione(int id) {
    setState(() => _sezione = 0);
    WidgetsBinding.instance.addPostFrameCallback((_) => _spedizioni.currentState?.apri(id, aggiornaElenco: true));
  }

  @override
  Widget build(BuildContext context) {
    final api = widget.api;
    final pagine = [
      SpedizioniPage(key: _spedizioni, api: api),
      NuovaSpedizionePage(api: api, alCreata: _apriSpedizione),
      MappaCorrieriPage(api: api, alApriSpedizione: _apriSpedizione),
    ];
    const voci = [
      (Icons.local_shipping_outlined, 'Spedizioni'),
      (Icons.add_box_outlined, 'Nuova spedizione'),
      (Icons.map_outlined, 'Corrieri'),
    ];
    final largo = MediaQuery.sizeOf(context).width >= 900;
    final corpo = IndexedStack(index: _sezione, children: pagine);

    return Scaffold(
      appBar: AppBar(
        title: Text('Veloce24 · ${api.utente!.nomeCompleto}', style: const TextStyle(fontSize: 16)),
        actions: [
          IconButton(tooltip: 'Esci', icon: const Icon(Icons.logout), onPressed: widget.alLogout),
        ],
      ),
      body: largo
          ? Row(children: [
              NavigationRail(
                selectedIndex: _sezione,
                labelType: NavigationRailLabelType.all,
                onDestinationSelected: (i) => setState(() => _sezione = i),
                destinations: [for (final v in voci) NavigationRailDestination(icon: Icon(v.$1), label: Text(v.$2))],
              ),
              const VerticalDivider(width: 1),
              Expanded(child: corpo),
            ])
          : corpo,
      bottomNavigationBar: largo
          ? null
          : NavigationBar(
              selectedIndex: _sezione,
              onDestinationSelected: (i) => setState(() => _sezione = i),
              destinations: [for (final v in voci) NavigationDestination(icon: Icon(v.$1), label: v.$2)],
            ),
    );
  }
}
