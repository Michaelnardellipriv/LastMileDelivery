// Pagina di login con email e password, comune alle aree operatori e corrieri.
// Non controlla nulla: manda le credenziali al backend e mostra il suo esito.
import 'package:flutter/material.dart';

import 'api.dart';
import 'comuni.dart';

class PaginaLogin extends StatefulWidget {
  final ApiSessione api;
  final String titolo; // es. "Veloce24 · Corrieri"
  final VoidCallback alLogin;
  const PaginaLogin({super.key, required this.api, required this.titolo, required this.alLogin});
  @override
  State<PaginaLogin> createState() => _PaginaLoginState();
}

class _PaginaLoginState extends State<PaginaLogin> {
  final _email = TextEditingController();
  final _password = TextEditingController();
  bool _attesa = false;
  bool _nascosta = true;
  Object? _errore; // esito del backend: i messaggi per campo vanno sotto i campi

  @override
  void dispose() {
    _email.dispose();
    _password.dispose();
    super.dispose();
  }

  Future<void> _entra() async {
    setState(() {
      _attesa = true;
      _errore = null;
    });
    try {
      await widget.api.login(_email.text, _password.text);
      widget.alLogin();
    } catch (e) {
      if (mounted) setState(() => _errore = e);
    } finally {
      if (mounted) setState(() => _attesa = false);
    }
  }

  @override
  Widget build(BuildContext context) => Scaffold(
        body: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 420),
            child: SingleChildScrollView(
              child: Card(
                margin: const EdgeInsets.all(16),
                child: Padding(
                  padding: const EdgeInsets.all(24),
                  child: AutofillGroup(
                    child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.stretch, children: [
                      Text(widget.titolo, style: Theme.of(context).textTheme.headlineSmall),
                      const SizedBox(height: 4),
                      Text('Accedi con email e password', style: Theme.of(context).textTheme.bodyMedium),
                      const SizedBox(height: 24),
                      TextField(
                        controller: _email,
                        decoration: InputDecoration(
                          labelText: 'Email',
                          errorText: erroreCampo(_errore, 'email'),
                          border: const OutlineInputBorder(),
                        ),
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
                          errorText: erroreCampo(_errore, 'password'),
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
                        Text(testoErrore(_errore!), style: TextStyle(color: Theme.of(context).colorScheme.error)),
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
        ),
      );
}
