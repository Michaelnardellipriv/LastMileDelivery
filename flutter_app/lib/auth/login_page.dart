// Pagina di login: email + password (+ ruolo, solo se l'utente ne ha più d'uno) →
// POST /api/auth/login → sessione (JWT + utente) salvata in modo sicuro.
// Dopo l'accesso non si naviga da qui: `store` notifica e `App` (main.dart) mostra la home del ruolo.
import 'package:flutter/foundation.dart';
import 'package:material_ui/material_ui.dart';
import 'package:flutter/services.dart';
import 'package:http/http.dart' as http;
import '../api.dart';
import '../backend_page.dart';
import 'forgot_password_page.dart';
import '../store.dart';
import 'validatori.dart';

/// Sotto questa larghezza il layout è "mobile" (a tutto schermo); sopra, card centrata.
const _larghezzaCard = 600.0;
const _larghezzaMassimaCard = 450.0;

class LoginPage extends StatefulWidget {
  /// `client` permette di iniettare un server finto nei test.
  const LoginPage({super.key, this.client});
  final http.Client? client;

  @override
  State<LoginPage> createState() => _LoginPageState();
}

class _LoginPageState extends State<LoginPage> {
  final _form = GlobalKey<FormState>();
  final _email = TextEditingController();
  final _password = TextEditingController();
  final _emailFocus = FocusNode();
  final _passwordFocus = FocusNode();
  // L'icona occhio non deve rubare il focus al campo password (il cursore resta dov'è).
  final _occhioFocus = FocusNode(canRequestFocus: false);

  bool _nascosta = true;
  bool _inCorso = false;

  /// Ruoli tra cui scegliere. Non null solo dopo che il backend ha risposto 400 con un dettaglio sul
  /// campo `ruolo` ("l'utente ha più ruoli: indicarne uno tra ..."): allora si mostra la scelta.
  List<RuoloApi>? _ruoliAmmessi;
  RuoloApi? _ruoloScelto;

  bool get _demo => apiBase.isEmpty;

  @override
  void initState() {
    super.initState();
    // Messaggio lasciato da un logout forzato (es. token scaduto), mostrato una sola volta.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      final a = store.avviso;
      if (a != null && mounted) {
        store.avviso = null;
        _mostraMessaggio(a);
      }
    });
  }

  @override
  void dispose() {
    _email.dispose();
    _password.dispose();
    _emailFocus.dispose();
    _passwordFocus.dispose();
    _occhioFocus.dispose();
    super.dispose();
  }

  // ───────── Validazione ─────────

  // L'email si valida con `validaEmail` (validatori.dart), condiviso con il recupero password.

  // Al login non si impongono regole di complessità: quelle valgono alla creazione/cambio password.
  String? _validaPassword(String? v) => (v == null || v.isEmpty) ? 'Inserisci la password' : null;

  // ───────── Invio ─────────

  Future<void> _invia() async {
    if (_inCorso) return; // niente doppio invio (tap ripetuto o Invio dalla tastiera)
    if (!_form.currentState!.validate()) {
      // Il cursore va sul primo campo da correggere.
      (validaEmail(_email.text) != null ? _emailFocus : _passwordFocus).requestFocus();
      return;
    }
    FocusScope.of(context).unfocus();
    setState(() => _inCorso = true);
    try {
      final email = _email.text.trim();
      if (_ruoliAmmessi != null && _ruoloScelto == null) {
        _mostraMessaggio('Scegli con quale ruolo accedere.');
        return;
      }
      if (_demo) {
        _accediDemo(email);
      } else {
        // ── CHIAMATA API: POST /api/auth/login con le credenziali ──
        final sessione = await AuthApi.login(
          base: apiBase,
          email: email,
          password: _password.text,
          ruolo: _ruoloScelto, // null se l'utente ha un solo ruolo: il campo non viene inviato
          client: widget.client,
        );
        // ── JWT: salvato in modo sicuro (flutter_secure_storage) e tenuto in memoria nello store.
        // `store` notifica e main.dart instrada l'utente alla schermata del suo ruolo
        // (HomePerRuolo), quindi questa pagina viene sostituita: nessun Navigator qui.
        await store.avviaSessione(sessione);
      }
    } on ErroreApi catch (e) {
      if (!mounted) return;
      final dettaglioRuolo = e.status == 400 ? e.messaggioCampo('ruolo') : null;
      if (dettaglioRuolo != null) {
        // Utente con più ruoli: il backend elenca i nomi nel messaggio; si estraggono i valori validi.
        final trovati = [for (final r in RuoloApi.values) if (RegExp(r'\b' + r.name + r'\b').hasMatch(dettaglioRuolo)) r];
        setState(() {
          _ruoliAmmessi = trovati.isEmpty ? RuoloApi.values.toList() : trovati;
          _ruoloScelto = null;
        });
        _mostraMessaggio('Questo account ha più ruoli: scegli quello con cui accedere.');
      } else {
        _mostraMessaggio(_testoErrore(e));
      }
    } catch (_) {
      if (mounted) _mostraMessaggio('Errore inatteso durante l\'accesso. Riprova.');
    } finally {
      if (mounted) setState(() => _inCorso = false);
    }
  }

  /// Modalità demo (API_BASE vuoto): non c'è un backend che verifichi la password,
  /// si controlla solo che l'email sia di un utente di prova.
  void _accediDemo(String email) {
    final u = store.utenti.where((u) => u.email == email.toLowerCase()).firstOrNull;
    if (u == null) throw ErroreApi(401, 'utente demo sconosciuto');
    store.login(u.id);
  }

  /// Testi per gli status che POST /api/auth/login può restituire (vedi ErroreApi.perUtente):
  /// 400 campi mancanti · 401 "email o password non corrette" · 403 ruolo non posseduto ·
  /// 415 · 500 · 503. Si mostra il messaggio del backend, che è già in italiano.
  String _testoErrore(ErroreApi e) => switch (e.status) {
        401 => e.messaggio.isEmpty ? 'Email o password non corrette.' : e.messaggio,
        _ => e.perUtente,
      };

  void _mostraMessaggio(String m) {
    final cs = Theme.of(context).colorScheme;
    final largo = MediaQuery.sizeOf(context).width >= _larghezzaCard;
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(
        content: Text(m, style: TextStyle(color: cs.onErrorContainer)),
        backgroundColor: cs.errorContainer,
        behavior: SnackBarBehavior.floating,
        width: largo ? _larghezzaMassimaCard : null,
        showCloseIcon: true,
        closeIconColor: cs.onErrorContainer,
        duration: const Duration(seconds: 6),
      ));
  }

  /// Apre il recupero password, riportando l'email già scritta nel campo.
  void _apriRecupero() => Navigator.of(context).push(MaterialPageRoute<void>(
        builder: (_) => ForgotPasswordPage(emailIniziale: _email.text.trim(), client: widget.client),
      ));

  // ───────── UI ─────────

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return Scaffold(
      backgroundColor: cs.surfaceContainerLow,
      body: SafeArea(
        child: LayoutBuilder(builder: (context, vincoli) {
          final largo = vincoli.maxWidth >= _larghezzaCard;
          // Margini fluidi su mobile: 6% della larghezza, tra 16 e 32 px.
          final margine = largo ? 24.0 : (vincoli.maxWidth * 0.06).clamp(16.0, 32.0);
          final contenuto = _contenuto(context, largo);
          return SingleChildScrollView(
            keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
            child: ConstrainedBox(
              // Il contenuto è centrato in verticale ma scorre se la tastiera lo schiaccia.
              constraints: BoxConstraints(minHeight: vincoli.maxHeight),
              child: Center(
                child: Padding(
                  padding: EdgeInsets.symmetric(horizontal: margine, vertical: 24),
                  child: ConstrainedBox(
                    constraints: BoxConstraints(maxWidth: largo ? _larghezzaMassimaCard : double.infinity),
                    child: largo ? _card(cs, contenuto) : contenuto,
                  ),
                ),
              ),
            ),
          );
        }),
      ),
    );
  }

  /// Web/desktop: il form dentro una card con ombra morbida a due livelli.
  Widget _card(ColorScheme cs, Widget figlio) => Container(
        padding: const EdgeInsets.fromLTRB(32, 36, 32, 28),
        decoration: BoxDecoration(
          color: cs.surface,
          borderRadius: BorderRadius.circular(20),
          border: Border.all(color: cs.outlineVariant.withValues(alpha: .5)),
          boxShadow: [
            BoxShadow(color: Colors.black.withValues(alpha: .10), blurRadius: 32, offset: const Offset(0, 12)),
            BoxShadow(color: Colors.black.withValues(alpha: .06), blurRadius: 6, offset: const Offset(0, 2)),
          ],
        ),
        child: figlio,
      );

  Widget _contenuto(BuildContext context, bool largo) {
    final tema = Theme.of(context);
    final cs = tema.colorScheme;
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Center(
          child: CircleAvatar(
            radius: 30,
            backgroundColor: cs.primaryContainer,
            child: Icon(Icons.local_shipping_outlined, size: 30, color: cs.onPrimaryContainer),
          ),
        ),
        const SizedBox(height: 16),
        Text('Veloce24', textAlign: TextAlign.center, style: tema.textTheme.headlineMedium?.copyWith(fontWeight: FontWeight.w700)),
        const SizedBox(height: 4),
        Text('Accedi per gestire le spedizioni',
            textAlign: TextAlign.center, style: tema.textTheme.bodyMedium?.copyWith(color: cs.onSurfaceVariant)),
        const SizedBox(height: 28),
        // AutofillGroup: gestori di password e browser propongono e salvano le credenziali.
        AutofillGroup(
          child: Form(
            key: _form,
            child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
              _campoEmail(largo),
              const SizedBox(height: 16),
              _campoPassword(),
              if (_ruoliAmmessi != null) ...[const SizedBox(height: 16), _sceltaRuolo()],
            ]),
          ),
        ),
        // Recupero password: flusso "email → link di reset" (ForgotPasswordPage).
        Align(
          alignment: Alignment.centerRight,
          child: TextButton(
            onPressed: _inCorso ? null : _apriRecupero,
            child: const Text('Password dimenticata?'),
          ),
        ),
        const SizedBox(height: 8),
        _pulsante(cs),
        if (_demo) _sezioneDemo(tema) else if (kDebugMode) _accessoSviluppo(),
      ],
    );
  }

  Widget _campoEmail(bool largo) => TextFormField(
        controller: _email,
        focusNode: _emailFocus,
        autofocus: largo, // su mobile non apre subito la tastiera davanti al form
        readOnly: _inCorso,
        keyboardType: TextInputType.emailAddress,
        textInputAction: TextInputAction.next, // "Avanti" porta il cursore sulla password
        autofillHints: const [AutofillHints.username, AutofillHints.email],
        autocorrect: false,
        enableSuggestions: false,
        // Gli spazi non servono in un'email: il formatter li toglie (anche dal testo incollato)
        // e Flutter mantiene il cursore nella posizione giusta.
        inputFormatters: [FilteringTextInputFormatter.deny(RegExp(r'\s')), LengthLimitingTextInputFormatter(254)],
        autovalidateMode: AutovalidateMode.onUserInteraction,
        validator: validaEmail,
        onChanged: (_) {
          if (_ruoliAmmessi != null) setState(() => _ruoliAmmessi = _ruoloScelto = null); // altro account: ruoli diversi
        },
        onFieldSubmitted: (_) => _passwordFocus.requestFocus(),
        decoration: const InputDecoration(
          labelText: 'Email',
          prefixIcon: Icon(Icons.mail_outline),
          border: OutlineInputBorder(),
        ),
      );

  /// Campo `ruolo` di POST /api/auth/login: compare solo per gli utenti con più ruoli.
  Widget _sceltaRuolo() => DropdownButtonFormField<RuoloApi>(
        initialValue: _ruoloScelto,
        decoration: const InputDecoration(labelText: 'Accedi come', prefixIcon: Icon(Icons.badge_outlined), border: OutlineInputBorder()),
        items: [for (final r in _ruoliAmmessi!) DropdownMenuItem(value: r, child: Text(r.nome))],
        onChanged: _inCorso ? null : (v) => setState(() => _ruoloScelto = v),
      );

  Widget _campoPassword() => TextFormField(
        controller: _password,
        focusNode: _passwordFocus,
        readOnly: _inCorso,
        obscureText: _nascosta,
        keyboardType: TextInputType.visiblePassword,
        textInputAction: TextInputAction.done, // "Fine"/Invio invia il form
        autofillHints: const [AutofillHints.password],
        autocorrect: false,
        enableSuggestions: false,
        validator: _validaPassword,
        onFieldSubmitted: (_) => _invia(),
        decoration: InputDecoration(
          labelText: 'Password',
          prefixIcon: const Icon(Icons.lock_outline),
          border: const OutlineInputBorder(),
          suffixIcon: IconButton(
            focusNode: _occhioFocus,
            tooltip: _nascosta ? 'Mostra password' : 'Nascondi password',
            icon: Icon(_nascosta ? Icons.visibility_outlined : Icons.visibility_off_outlined),
            onPressed: () => setState(() => _nascosta = !_nascosta),
          ),
        ),
      );

  Widget _pulsante(ColorScheme cs) => SizedBox(
        height: 48,
        child: FilledButton(
          onPressed: _invia, // se _inCorso, _invia ritorna subito: il pulsante resta ben visibile
          child: _inCorso
              ? SizedBox(
                  width: 22,
                  height: 22,
                  child: CircularProgressIndicator(strokeWidth: 2.5, color: cs.onPrimary),
                )
              : const Text('Accedi', style: TextStyle(fontSize: 16)),
        ),
      );

  /// Modalità demo: spiega cosa succede e permette di compilare il form con un utente di prova.
  Widget _sezioneDemo(ThemeData tema) => Padding(
        padding: const EdgeInsets.only(top: 24),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Text('Modalità demo: dati locali, la password non viene verificata.', style: tema.textTheme.bodySmall),
          const SizedBox(height: 8),
          Wrap(spacing: 8, runSpacing: 4, children: [
            for (final u in store.utenti)
              ActionChip(
                avatar: const Icon(Icons.person_outline, size: 16),
                label: Text('${u.nomeCompleto} · ${ruoloNome[u.ruolo]}', style: const TextStyle(fontSize: 12)),
                onPressed: _inCorso
                    ? null
                    : () {
                        _email.text = u.email;
                        _password.text = 'demo';
                        _form.currentState?.validate();
                      },
              ),
          ]),
        ]),
      );

  /// Solo in debug: il backend accetta ancora gli header X-Id-* se IDENTITA_HEADER_SVILUPPO=attiva.
  Widget _accessoSviluppo() => Padding(
        padding: const EdgeInsets.only(top: 12),
        child: TextButton(
          onPressed: _inCorso
              ? null
              : () => Navigator.of(context).push(
                    MaterialPageRoute(builder: (_) => const BackendAccessoPage(apiBase: apiBase)),
                  ),
          child: const Text('Accesso di sviluppo (header X-Id)'),
        ),
      );
}
