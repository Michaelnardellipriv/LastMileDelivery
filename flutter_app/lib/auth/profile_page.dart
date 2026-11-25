// Area personale: dati dell'utente autenticato + cambio password.
//
// Due modalità, come il resto dell'app:
//  - ProfilePage.backend → utente entrato col login (JWT). Il backend NON ha un GET /api/auth/me:
//    i dati del profilo (id_utente, id del ruolo, ruolo, nome, cognome, email, scade_il) sono quelli
//    restituiti da POST /api/auth/login e salvati nella [Sessione]. Il cambio password usa
//    POST /api/auth/cambio-password con `Authorization: Bearer <JWT>` (lo aggiunge ApiClient).
//  - ProfilePage.demo → utente della demo locale (API_BASE vuoto): nessuna rete, nessun token.
//
// Layout responsive:
//  - ≥ 840 px (web/desktop/tablet orizzontale): due colonne, profilo a sinistra e password a destra;
//  - sotto: due schede (TabBar) "Profilo" e "Password", ciascuna centrata e larga al massimo 560 px.
import 'package:flutter/material.dart';
import '../api.dart';
import '../store.dart';
import 'ui_comune.dart';
import 'validatori.dart';

const _larghezzaDueColonne = 840.0;
const _larghezzaMassimaContenuto = 1000.0;
const _larghezzaMassimaScheda = 560.0;

/// Quello che la schermata mostra dell'utente, qualunque sia la fonte (sessione del backend o demo).
class _DatiProfilo {
  const _DatiProfilo({
    required this.idUtente,
    required this.nome,
    required this.ruolo,
    required this.email,
    this.idRuolo,
    this.telefono,
    this.scadenzaSessione,
  });
  final int idUtente;
  final int? idRuolo;
  final String nome, ruolo, email;
  final String? telefono; // solo demo: il backend non restituisce il telefono
  final DateTime? scadenzaSessione;

  String get iniziali {
    final parole = nome.split(RegExp(r'\s+')).where((p) => p.isNotEmpty).toList();
    if (parole.isEmpty) return '?';
    return (parole.first[0] + (parole.length > 1 ? parole.last[0] : '')).toUpperCase();
  }
}

class ProfilePage extends StatefulWidget {
  /// Utente col login: [api] è il client della sessione (già con il JWT), [sessione] i dati noti
  /// senza rete (nome, ruolo, scadenza del token).
  const ProfilePage.backend({super.key, required ApiClient api, required Sessione sessione, required this.onLogout})
      : _api = api,
        _sessione = sessione,
        _utenteDemo = null;

  const ProfilePage.demo({super.key, required Utente utente, required this.onLogout})
      : _api = null,
        _sessione = null,
        _utenteDemo = utente;

  final ApiClient? _api;
  final Sessione? _sessione;
  final Utente? _utenteDemo;

  /// Chi apre la pagina decide cosa significa uscire (in entrambi i casi: chiudere le schermate
  /// aperte e chiamare `store.esci()`, che cancella il JWT dal dispositivo).
  final VoidCallback onLogout;

  @override
  State<ProfilePage> createState() => _ProfilePageState();
}

class _ProfilePageState extends State<ProfilePage> {
  /// Dati del profilo: dalla sessione del login (backend) o dall'utente demo. Nessuna chiamata di rete.
  _DatiProfilo get _dati {
    final u = widget._utenteDemo;
    if (u != null) {
      return _DatiProfilo(idUtente: u.id, nome: u.nomeCompleto, ruolo: ruoloNome[u.ruolo]!, email: u.email, telefono: u.tel);
    }
    final s = widget._sessione!;
    return _DatiProfilo(
      idUtente: s.idUtente,
      idRuolo: s.idRuolo,
      nome: s.nomeCompleto,
      ruolo: s.ruolo.nome,
      email: s.email,
      scadenzaSessione: s.scadeIl,
    );
  }

  /// Chiamata dalla scheda "Password" quando il form è valido.
  Future<void> _cambiaPassword(String attuale, String nuova) async {
    final api = widget._api;
    if (api == null) {
      // Demo: nessun backend che verifichi o salvi la password, si simula solo l'attesa.
      await Future<void>.delayed(const Duration(milliseconds: 600));
      return;
    }
    // ── CHIAMATA API: POST /api/auth/cambio-password ──
    // Corpo {"password_attuale": "...", "nuova_password": "..."}; il JWT nell'header identifica
    // l'utente, quindi l'id NON viaggia nel corpo (non si può cambiare la password di un altro).
    // Esiti: 204 ok · 400 dati non validi (dettagli per campo) · 403 password attuale errata ·
    // 401 token scaduto → logout automatico (vedi ApiClient.cambiaPassword).
    await api.cambiaPassword(attuale: attuale, nuova: nuova);
  }

  @override
  Widget build(BuildContext context) {
    final profilo = _SezioneProfilo(dati: _dati, onLogout: widget.onLogout);
    final password = _SezioneCambioPassword(demo: widget._api == null, onInvia: _cambiaPassword);
    return Scaffold(
      appBar: AppBar(title: const Text('Il mio profilo')),
      body: SafeArea(
        child: LayoutBuilder(builder: (context, vincoli) {
          if (vincoli.maxWidth >= _larghezzaDueColonne) {
            // Web/desktop: tutto visibile insieme, niente schede.
            return SingleChildScrollView(
              padding: const EdgeInsets.all(24),
              child: Center(
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: _larghezzaMassimaContenuto),
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [Expanded(child: profilo), const SizedBox(width: 24), Expanded(child: password)],
                  ),
                ),
              ),
            );
          }
          // Mobile/tablet verticale: due schede.
          return DefaultTabController(
            length: 2,
            child: Column(children: [
              const TabBar(tabs: [
                Tab(icon: Icon(Icons.person_outline), text: 'Profilo'),
                Tab(icon: Icon(Icons.lock_outline), text: 'Password'),
              ]),
              Expanded(
                child: TabBarView(children: [_TabContenuto(child: profilo), _TabContenuto(child: password)]),
              ),
            ]),
          );
        }),
      ),
    );
  }
}

/// Contenuto di una scheda: scorrevole, centrato e con larghezza massima.
/// Resta vivo quando si cambia scheda, così ciò che si è scritto nel form non si perde.
class _TabContenuto extends StatefulWidget {
  const _TabContenuto({required this.child});
  final Widget child;

  @override
  State<_TabContenuto> createState() => _TabContenutoState();
}

class _TabContenutoState extends State<_TabContenuto> with AutomaticKeepAliveClientMixin {
  @override
  bool get wantKeepAlive => true;

  @override
  Widget build(BuildContext context) {
    super.build(context); // richiesto da AutomaticKeepAliveClientMixin
    return SingleChildScrollView(
      keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
      padding: const EdgeInsets.all(16),
      child: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: _larghezzaMassimaScheda),
          child: widget.child,
        ),
      ),
    );
  }
}

// ───────────────────────────── Scheda "Profilo" ─────────────────────────────

class _SezioneProfilo extends StatelessWidget {
  const _SezioneProfilo({required this.dati, required this.onLogout});
  final _DatiProfilo dati;
  final VoidCallback onLogout;

  @override
  Widget build(BuildContext context) {
    final tema = Theme.of(context);
    final cs = tema.colorScheme;
    final d = dati;
    return Card.outlined(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          Center(
            child: CircleAvatar(
              radius: 36,
              backgroundColor: cs.primaryContainer,
              child: Text(d.iniziali,
                  style: tema.textTheme.headlineSmall?.copyWith(color: cs.onPrimaryContainer, fontWeight: FontWeight.w600)),
            ),
          ),
          const SizedBox(height: 16),
          Text(d.nome, textAlign: TextAlign.center, style: tema.textTheme.headlineSmall?.copyWith(fontWeight: FontWeight.w700)),
          const SizedBox(height: 8),
          // Ruolo come lo ha emesso il backend (utente.ruolo del login).
          Center(child: Chip(avatar: const Icon(Icons.badge_outlined, size: 18), label: Text(d.ruolo))),
          const Divider(height: 32),
          _Riga(icona: Icons.mail_outline, etichetta: 'Email', valore: d.email),
          if (d.telefono != null) _Riga(icona: Icons.phone_outlined, etichetta: 'Telefono', valore: d.telefono!),
          _Riga(icona: Icons.tag, etichetta: 'ID utente', valore: '${d.idUtente}'),
          if (d.idRuolo != null) _Riga(icona: Icons.numbers, etichetta: 'ID nel ruolo', valore: '${d.idRuolo}'),
          if (d.scadenzaSessione != null)
            _Riga(icona: Icons.schedule, etichetta: 'Sessione valida fino al', valore: fd(d.scadenzaSessione!.toLocal())),
          const SizedBox(height: 16),
          OutlinedButton.icon(onPressed: onLogout, icon: const Icon(Icons.logout), label: const Text('Esci')),
        ]),
      ),
    );
  }
}

class _Riga extends StatelessWidget {
  const _Riga({required this.icona, required this.etichetta, required this.valore});
  final IconData icona;
  final String etichetta, valore;

  @override
  Widget build(BuildContext context) {
    final tema = Theme.of(context);
    return ListTile(
      contentPadding: EdgeInsets.zero,
      leading: Icon(icona),
      title: Text(etichetta, style: tema.textTheme.labelMedium?.copyWith(color: tema.colorScheme.onSurfaceVariant)),
      // SelectableText: su web si può copiare l'email.
      subtitle: SelectableText(valore, style: tema.textTheme.bodyLarge),
    );
  }
}

// ───────────────────────────── Scheda "Password" ─────────────────────────────

class _SezioneCambioPassword extends StatefulWidget {
  const _SezioneCambioPassword({required this.demo, required this.onInvia});
  final bool demo;

  /// Esegue il cambio (chiamata API o simulazione demo) e lancia ErroreApi se fallisce.
  final Future<void> Function(String attuale, String nuova) onInvia;

  @override
  State<_SezioneCambioPassword> createState() => _SezioneCambioPasswordState();
}

class _SezioneCambioPasswordState extends State<_SezioneCambioPassword> {
  final _form = GlobalKey<FormState>();
  final _confermaKey = GlobalKey<FormFieldState<String>>();
  final _attuale = TextEditingController();
  final _nuova = TextEditingController();
  final _conferma = TextEditingController();
  final _attualeFocus = FocusNode();
  final _nuovaFocus = FocusNode();
  final _confermaFocus = FocusNode();

  bool _inCorso = false;

  /// Il backend ha detto "password attuale errata": l'errore compare sotto il campo
  /// e sparisce appena l'utente la modifica.
  bool _attualeErrata = false;

  /// Messaggi del backend per campo (dettagli di un 400), mostrati sotto il campo giusto e
  /// azzerati appena l'utente modifica quel campo.
  String? _erroreNuovaServer, _erroreAttualeServer;

  @override
  void dispose() {
    _attuale.dispose();
    _nuova.dispose();
    _conferma.dispose();
    _attualeFocus.dispose();
    _nuovaFocus.dispose();
    _confermaFocus.dispose();
    super.dispose();
  }

  // ───────── Validazione ─────────

  String? _validaAttuale(String? v) {
    if (_attualeErrata) return 'Password attuale non corretta';
    return _erroreAttualeServer; // nessun controllo locale: password attuale e formato li verifica il backend
  }

  String? _validaNuova(String? v) => _erroreNuovaServer; // regole della password: le decide il backend

  String? _validaConferma(String? v) => validaConferma(v, _nuova.text);

  // ───────── Invio ─────────

  Future<void> _invia() async {
    if (_inCorso) return; // niente doppio invio
    // Errori dell'invio precedente: si azzerano, il backend rivaluta tutto da capo.
    _attualeErrata = false;
    _erroreAttualeServer = _erroreNuovaServer = null;
    if (validaConferma(_conferma.text, _nuova.text) != null) {
      _form.currentState!.validate(); // mostra "Le password non coincidono"
      _confermaFocus.requestFocus(); // l'unico controllo locale è la conferma
      return;
    }
    FocusScope.of(context).unfocus();
    setState(() => _inCorso = true); // → spinner nel pulsante, campi in sola lettura
    try {
      await widget.onInvia(_attuale.text, _nuova.text);
      if (!mounted) return;
      // Le password non vanno lasciate nei campi dopo l'uso.
      _form.currentState!.reset();
      _attuale.clear();
      _nuova.clear();
      _conferma.clear();
      mostraEsito(
        context,
        widget.demo ? 'Modalità demo: nessuna password è stata realmente modificata.' : 'Password aggiornata correttamente.',
      );
    } on ErroreApi catch (e) {
      if (mounted) _gestisciErrore(e);
    } catch (_) {
      if (mounted) mostraEsito(context, 'Errore inatteso durante il cambio password. Riprova.', errore: true);
    } finally {
      if (mounted) setState(() => _inCorso = false);
    }
  }

  /// Status che POST /api/auth/cambio-password può restituire: 400, 401, 403, 415, 500, 503.
  void _gestisciErrore(ErroreApi e) {
    switch (e.status) {
      case 401:
        // Token scaduto o non valido: ApiClient ha già avviato il logout (alScadereDelToken)
        // e l'utente sta tornando al login, che mostra "Sessione scaduta". Nessun messaggio qui.
        return;
      case 403:
        // Password attuale errata (messaggio del backend: "password attuale non corretta").
        // È 403 e non 401 apposta: un 401 farebbe uscire l'utente.
        _attualeErrata = true;
        _form.currentState!.validate(); // mostra l'errore sotto il campo
        _attualeFocus.requestFocus();
        mostraEsito(context, 'La password attuale non è corretta.', errore: true);
      case 400:
        // Validazione: `dettagli` dice quale campo e perché (es. nuova_password: "almeno 8 caratteri").
        _erroreNuovaServer = e.messaggioCampo('nuova_password');
        _erroreAttualeServer = e.messaggioCampo('password_attuale');
        _form.currentState!.validate();
        mostraEsito(context, e.perUtente, errore: true);
      default:
        mostraEsito(context, e.perUtente, errore: true); // null = backend non raggiungibile, 415, 429, 5xx
    }
  }

  // ───────── UI ─────────

  @override
  Widget build(BuildContext context) {
    final tema = Theme.of(context);
    final cs = tema.colorScheme;
    return Card.outlined(
      child: Padding(
        padding: const EdgeInsets.all(24),
        // AutofillGroup: i gestori di password riconoscono "password attuale" e "nuova password".
        child: AutofillGroup(
          child: Form(
            key: _form,
            child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
              Row(children: [
                Icon(Icons.lock_reset, color: cs.primary),
                const SizedBox(width: 12),
                Expanded(child: Text('Cambia password', style: tema.textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w700))),
              ]),
              const SizedBox(height: 8),
              Text(
                'Per sicurezza ti chiediamo la password attuale prima di impostarne una nuova.',
                style: tema.textTheme.bodyMedium?.copyWith(color: cs.onSurfaceVariant),
              ),
              if (widget.demo) ...[
                const SizedBox(height: 8),
                Text('Modalità demo: la password non viene modificata.', style: tema.textTheme.bodySmall),
              ],
              const SizedBox(height: 24),
              CampoPassword(
                controller: _attuale,
                focusNode: _attualeFocus,
                etichetta: 'Password attuale',
                readOnly: _inCorso,
                textInputAction: TextInputAction.next,
                autofillHints: const [AutofillHints.password],
                validator: _validaAttuale,
                onChanged: (_) {
                  if (_attualeErrata || _erroreAttualeServer != null) {
                    setState(() => _attualeErrata = false);
                    _erroreAttualeServer = null;
                  }
                },
                onFieldSubmitted: (_) => _nuovaFocus.requestFocus(),
              ),
              const SizedBox(height: 16),
              CampoPassword(
                controller: _nuova,
                focusNode: _nuovaFocus,
                etichetta: 'Nuova password',
                readOnly: _inCorso,
                textInputAction: TextInputAction.next,
                autofillHints: const [AutofillHints.newPassword],
                validator: _validaNuova,
                // Se la conferma è già scritta, va riconfrontata con la nuova password.
                onChanged: (_) {
                  _erroreNuovaServer = null;
                  if (_conferma.text.isNotEmpty) _confermaKey.currentState?.validate();
                },
                onFieldSubmitted: (_) => _confermaFocus.requestFocus(),
              ),
              const SizedBox(height: 16),
              CampoPassword(
                controller: _conferma,
                focusNode: _confermaFocus,
                fieldKey: _confermaKey,
                etichetta: 'Conferma nuova password',
                readOnly: _inCorso,
                textInputAction: TextInputAction.done, // "Fine"/Invio invia il form
                autofillHints: const [AutofillHints.newPassword],
                validator: _validaConferma,
                onFieldSubmitted: (_) => _invia(),
              ),
              const SizedBox(height: 24),
              SizedBox(
                height: 48,
                child: FilledButton(
                  onPressed: _invia, // se _inCorso, _invia ritorna subito: il pulsante resta ben visibile
                  child: _inCorso
                      ? SizedBox(
                          width: 22,
                          height: 22,
                          child: CircularProgressIndicator(strokeWidth: 2.5, color: cs.onPrimary),
                        )
                      : const Text('Aggiorna password', style: TextStyle(fontSize: 16)),
                ),
              ),
            ]),
          ),
        ),
      ),
    );
  }
}
