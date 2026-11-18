// Recupero password in due passi, entrambi anonimi (chi ha dimenticato la password non ha un JWT):
//  1. richiesta: email → POST /api/auth/recupero-password → il backend manda una mail con il link
//     `LINK_RECUPERO_PASSWORD?token=<token>` (valido 30 minuti, monouso);
//  2. conferma: token + nuova password → POST /api/auth/reimposta-password → 204.
// Il token si prende dal link (main.dart lo legge da `?token=` e apre questa pagina già al passo 2)
// oppure si incolla a mano. Senza SMTP_HOST il backend non invia la mail ma scrive il testo nel log.
import 'package:material_ui/material_ui.dart';
import 'package:flutter/services.dart';
import 'package:http/http.dart' as http;
import '../api.dart';
import '../store.dart';
import 'ui_comune.dart';
import 'validatori.dart';

class ForgotPasswordPage extends StatefulWidget {
  /// `emailIniziale`: l'email già scritta nel login, per non farla riscrivere.
  /// `tokenIniziale`: token del link ricevuto per mail → la pagina si apre direttamente sul passo 2.
  /// `onConcluso`: chiamata dopo il reset riuscito; se nullo la pagina si chiude (pop).
  /// `client` permette di iniettare un server finto nei test.
  const ForgotPasswordPage({super.key, this.emailIniziale, this.tokenIniziale, this.onConcluso, this.client});
  final String? emailIniziale, tokenIniziale;
  final VoidCallback? onConcluso;
  final http.Client? client;

  @override
  State<ForgotPasswordPage> createState() => _ForgotPasswordPageState();
}

enum _Passo { richiesta, inviata, reimposta }

class _ForgotPasswordPageState extends State<ForgotPasswordPage> {
  final _formEmail = GlobalKey<FormState>();
  final _formReset = GlobalKey<FormState>();
  late final _email = TextEditingController(text: widget.emailIniziale);
  late final _token = TextEditingController(text: widget.tokenIniziale);
  final _nuova = TextEditingController();
  final _conferma = TextEditingController();
  final _confermaKey = GlobalKey<FormFieldState<String>>();
  final _emailFocus = FocusNode();
  final _tokenFocus = FocusNode();
  final _nuovaFocus = FocusNode();
  final _confermaFocus = FocusNode();

  late _Passo _passo = (widget.tokenIniziale ?? '').isNotEmpty ? _Passo.reimposta : _Passo.richiesta;
  bool _inCorso = false;

  /// Testo neutro mostrato dopo la richiesta (dal backend, con ripiego locale).
  String? _messaggioBackend;
  String? _emailInviata;

  /// Errori per campo restituiti dal backend nel 400 (`dettagli`); si azzerano quando si modifica il campo.
  String? _erroreToken, _erroreNuovaServer;

  bool get _demo => apiBase.isEmpty;

  @override
  void dispose() {
    for (final c in [_email, _token, _nuova, _conferma]) {
      c.dispose();
    }
    for (final f in [_emailFocus, _tokenFocus, _nuovaFocus, _confermaFocus]) {
      f.dispose();
    }
    super.dispose();
  }

  // ───────── Passo 1: richiesta del link ─────────

  Future<void> _richiedi() async {
    if (_inCorso) return; // niente doppio invio
    if (!_formEmail.currentState!.validate()) {
      _emailFocus.requestFocus();
      return;
    }
    FocusScope.of(context).unfocus();
    setState(() => _inCorso = true);
    try {
      final email = _email.text.trim();
      String? messaggio;
      if (_demo) {
        await Future<void>.delayed(const Duration(milliseconds: 700)); // demo: nessuna mail vera
      } else {
        // ── CHIAMATA API: POST /api/auth/recupero-password {"email": "..."} ──
        // Anonima. Risponde 202 {"messaggio": ...} anche se l'email NON esiste: il testo resta neutro.
        messaggio = await AuthApi.richiediRecupero(base: apiBase, email: email, client: widget.client);
      }
      if (!mounted) return;
      setState(() {
        _emailInviata = email;
        _messaggioBackend = messaggio;
        _passo = _Passo.inviata;
      });
    } on ErroreApi catch (e) {
      // 400 (email mancante), 415, 500, 503: testo del backend. Il recupero NON risponde mai 429.
      if (mounted) mostraEsito(context, e.perUtente, errore: true);
    } catch (_) {
      if (mounted) mostraEsito(context, 'Errore inatteso. Riprova.', errore: true);
    } finally {
      if (mounted) setState(() => _inCorso = false);
    }
  }

  // ───────── Passo 2: nuova password con il token ─────────

  Future<void> _reimposta() async {
    if (_inCorso) return;
    if (!_formReset.currentState!.validate()) {
      if (_validaToken(_token.text) != null) {
        _tokenFocus.requestFocus();
      } else if (validaNuovaPassword(_nuova.text) != null) {
        _nuovaFocus.requestFocus();
      } else {
        _confermaFocus.requestFocus();
      }
      return;
    }
    FocusScope.of(context).unfocus();
    setState(() => _inCorso = true);
    try {
      if (_demo) {
        await Future<void>.delayed(const Duration(milliseconds: 700));
      } else {
        // ── CHIAMATA API: POST /api/auth/reimposta-password {"token": "...", "nuova_password": "..."} ──
        // 204 ok · 400 con dettagli sul campo `token` (non valido, scaduto o già usato) o `nuova_password`.
        await AuthApi.reimpostaPassword(
          base: apiBase,
          token: _token.text.trim(),
          nuovaPassword: _nuova.text,
          client: widget.client,
        );
      }
      if (!mounted) return;
      mostraEsito(context, _demo ? 'Modalità demo: nessuna password è stata modificata.' : 'Password aggiornata: ora puoi accedere.');
      _chiudi();
    } on ErroreApi catch (e) {
      if (!mounted) return;
      if (e.status == 400) {
        setState(() {
          _erroreToken = e.messaggioCampo('token');
          _erroreNuovaServer = e.messaggioCampo('nuova_password');
        });
        _formReset.currentState!.validate();
      }
      mostraEsito(context, e.perUtente, errore: true);
    } catch (_) {
      if (mounted) mostraEsito(context, 'Errore inatteso. Riprova.', errore: true);
    } finally {
      if (mounted) setState(() => _inCorso = false);
    }
  }

  /// Esce dalla pagina: pagina aperta dal login → pop; aperta dal link della mail (home) → onConcluso.
  void _chiudi() {
    final fine = widget.onConcluso;
    fine != null ? fine() : Navigator.of(context).pop();
  }

  String? _validaToken(String? v) {
    if (_erroreToken != null) return _erroreToken;
    return (v ?? '').trim().isEmpty ? 'Inserisci il codice ricevuto per email' : null;
  }

  // ───────── UI ─────────

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final contenuto = switch (_passo) {
      _Passo.richiesta => KeyedSubtree(key: const ValueKey('richiesta'), child: _modulo(context)),
      _Passo.inviata => KeyedSubtree(
          key: const ValueKey('inviata'),
          child: _passoInviata(context),),
      _Passo.reimposta => KeyedSubtree(key: const ValueKey('reimposta'), child: _moduloReset(context)),
    };
    return Scaffold(
      backgroundColor: cs.surfaceContainerLow,
      appBar: AppBar(
        backgroundColor: Colors.transparent,
        scrolledUnderElevation: 0,
        title: const Text('Recupero password'),
      ),
      body: SafeArea(
        child: AuthCardLayout(
          child: AnimatedSwitcher(duration: const Duration(milliseconds: 250), child: contenuto),
        ),
      ),
    );
  }

  Widget _intestazione(BuildContext context, IconData icona, String titolo, Widget descrizione) {
    final tema = Theme.of(context);
    final cs = tema.colorScheme;
    return Column(mainAxisSize: MainAxisSize.min, children: [
      CircleAvatar(
        radius: 30,
        backgroundColor: cs.primaryContainer,
        child: Icon(icona, size: 30, color: cs.onPrimaryContainer),
      ),
      const SizedBox(height: 16),
      Text(titolo, textAlign: TextAlign.center, style: tema.textTheme.headlineSmall?.copyWith(fontWeight: FontWeight.w700)),
      const SizedBox(height: 8),
      descrizione,
    ]);
  }

  Widget _spinnerOTesto(ColorScheme cs, String testo) => _inCorso
      ? SizedBox(width: 22, height: 22, child: CircularProgressIndicator(strokeWidth: 2.5, color: cs.onPrimary))
      : Text(testo, style: const TextStyle(fontSize: 16));

  Widget _modulo(BuildContext context) {
    final tema = Theme.of(context);
    final cs = tema.colorScheme;
    final largo = MediaQuery.sizeOf(context).width >= larghezzaCard;
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _intestazione(
          context,
          Icons.lock_reset,
          'Password dimenticata?',
          Text(
            'Inserisci l\'email del tuo account: ti invieremo un link per scegliere una nuova password.',
            textAlign: TextAlign.center,
            style: tema.textTheme.bodyMedium?.copyWith(color: cs.onSurfaceVariant),
          ),
        ),
        const SizedBox(height: 28),
        Form(
          key: _formEmail,
          child: TextFormField(
            controller: _email,
            focusNode: _emailFocus,
            autofocus: largo,
            readOnly: _inCorso,
            keyboardType: TextInputType.emailAddress,
            textInputAction: TextInputAction.done,
            autofillHints: const [AutofillHints.email],
            autocorrect: false,
            enableSuggestions: false,
            inputFormatters: [FilteringTextInputFormatter.deny(RegExp(r'\s')), LengthLimitingTextInputFormatter(254)],
            autovalidateMode: AutovalidateMode.onUserInteraction,
            validator: validaEmail,
            onFieldSubmitted: (_) => _richiedi(),
            decoration: const InputDecoration(labelText: 'Email', prefixIcon: Icon(Icons.mail_outline), border: OutlineInputBorder()),
          ),
        ),
        const SizedBox(height: 24),
        SizedBox(height: 48, child: FilledButton(onPressed: _richiedi, child: _spinnerOTesto(cs, 'Invia link di recupero'))),
        const SizedBox(height: 8),
        TextButton(
          onPressed: _inCorso ? null : () => setState(() => _passo = _Passo.reimposta),
          child: const Text('Ho già il codice di recupero'),
        ),
        TextButton(onPressed: _inCorso ? null : _chiudi, child: const Text('Torna al login')),
      ],
    );
  }

  Widget _passoInviata(BuildContext context){
    final tema = Theme.of(context);
    final cs = tema.colorScheme;
    final stile = tema.textTheme.bodyMedium?.copyWith(color: cs.onSurfaceVariant);
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _intestazione(
          context,
          Icons.mark_email_read_outlined,
          'Controlla la tua email',
          // Testo neutro del backend: non conferma che l'indirizzo esista.
          Text(
            _messaggioBackend ??
                'Se $_emailInviata è registrata riceverai un link per scegliere una nuova password, valido 30 minuti.',
            textAlign: TextAlign.center,
            style: stile,
          ),
        ),
        const SizedBox(height: 8),
        Text('Controlla anche la cartella spam.', textAlign: TextAlign.center, style: stile),
        if (_demo) ...[
          const SizedBox(height: 12),
          Text('Modalità demo: nessuna email è stata realmente inviata.', textAlign: TextAlign.center, style: tema.textTheme.bodySmall),
        ],
        const SizedBox(height: 28),
        SizedBox(
          height: 48,
          child: FilledButton(
            onPressed: () => setState(() => _passo = _Passo.reimposta),
            child: const Text('Ho il codice: scegli la nuova password', style: TextStyle(fontSize: 16)),
          ),
        ),
        const SizedBox(height: 8),
        TextButton(onPressed: _chiudi, child: const Text('Torna al login')),
        TextButton(onPressed: () => setState(() => _passo = _Passo.richiesta), child: const Text('Usa un altro indirizzo')),
      ],
    );
  }

  Widget _moduloReset(BuildContext context) {
    final tema = Theme.of(context);
    final cs = tema.colorScheme;
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _intestazione(
          context,
          Icons.password,
          'Scegli la nuova password',
          Text(
            'Incolla il codice del link ricevuto per email (vale 30 minuti e una sola volta) e scegli la nuova password.',
            textAlign: TextAlign.center,
            style: tema.textTheme.bodyMedium?.copyWith(color: cs.onSurfaceVariant),
          ),
        ),
        const SizedBox(height: 28),
        AutofillGroup(
          child: Form(
            key: _formReset,
            child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
              TextFormField(
                controller: _token,
                focusNode: _tokenFocus,
                readOnly: _inCorso,
                textInputAction: TextInputAction.next,
                autocorrect: false,
                enableSuggestions: false,
                autovalidateMode: AutovalidateMode.onUserInteraction,
                validator: _validaToken,
                onChanged: (_) => _erroreToken = null,
                onFieldSubmitted: (_) => _nuovaFocus.requestFocus(),
                decoration: const InputDecoration(labelText: 'Codice di recupero', prefixIcon: Icon(Icons.key_outlined), border: OutlineInputBorder()),
              ),
              const SizedBox(height: 16),
              CampoPassword(
                controller: _nuova,
                focusNode: _nuovaFocus,
                etichetta: 'Nuova password',
                readOnly: _inCorso,
                textInputAction: TextInputAction.next,
                autofillHints: const [AutofillHints.newPassword],
                validator: (v) => _erroreNuovaServer ?? validaNuovaPassword(v),
                onChanged: (_) {
                  _erroreNuovaServer = null;
                  if (_conferma.text.isNotEmpty) _confermaKey.currentState?.validate();
                },
                onFieldSubmitted: (_) => _confermaFocus.requestFocus(),
              ),
              const SizedBox(height: 8),
              Text('Almeno $passwordMinima caratteri, al massimo $passwordMassimaByte byte.', style: tema.textTheme.bodySmall),
              const SizedBox(height: 16),
              CampoPassword(
                controller: _conferma,
                focusNode: _confermaFocus,
                fieldKey: _confermaKey,
                etichetta: 'Conferma nuova password',
                readOnly: _inCorso,
                textInputAction: TextInputAction.done,
                autofillHints: const [AutofillHints.newPassword],
                validator: (v) => validaConferma(v, _nuova.text),
                onFieldSubmitted: (_) => _reimposta(),
              ),
            ]),
          ),
        ),
        const SizedBox(height: 24),
        SizedBox(height: 48, child: FilledButton(onPressed: _reimposta, child: _spinnerOTesto(cs, 'Imposta nuova password'))),
        const SizedBox(height: 8),
        TextButton(
          onPressed: _inCorso ? null : () => setState(() => _passo = _Passo.richiesta),
          child: const Text('Richiedi un nuovo link'),
        ),
        TextButton(
          onPressed: _inCorso ? null : _chiudi,
          child: const Text('Torna al login'),
        ),
      ],
    );
  }
}
