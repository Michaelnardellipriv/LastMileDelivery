import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;

import 'area_riservata.dart';
import 'auth/forgot_password_page.dart';
import 'auth/login_page.dart';
import 'auth/profile_page.dart';
import 'auth/recupero_link.dart';
import 'store.dart';

/// Avvio comune a tutti gli entry point. [apiBaseDiDefault]: backend da usare se API_BASE non è stato
/// passato con --dart-define (lo usano main_operatore.dart e main_corriere.dart; main.dart no, e senza
/// API_BASE parte la demo locale).
Future<void> avvia({String? apiBaseDiDefault}) async {
  WidgetsFlutterBinding.ensureInitialized();
  if (apiBase.isEmpty && apiBaseDiDefault != null) apiBase = apiBaseDiDefault;
  await store.ripristina(); // se c'è un JWT valido salvato, si entra senza rifare il login
  runApp(const App());
}

Future<void> main() => avvia();

const _seed = Color(0xFF0B5FFF);

class App extends StatefulWidget {
  /// Solo per i test: server finto al posto della rete, passato a login, recupero password e area riservata.
  final http.Client? client;
  const App({super.key, this.client});
  @override
  State<App> createState() => _AppState();
}

class _AppState extends State<App> with WidgetsBindingObserver {
  final _navigatore = GlobalKey<NavigatorState>();

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    tokenRecuperoDaLink.addListener(_tokenArrivato);
  }

  @override
  void dispose() {
    tokenRecuperoDaLink.removeListener(_tokenArrivato);
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  /// Un link di recupero arrivato mentre l'app gira (anche con l'utente già dentro): le schermate
  /// impilate sopra la home (dettagli, profilo...) coprirebbero la pagina "nuova password", quindi si chiudono.
  void _tokenArrivato() {
    if (tokenRecuperoDaLink.value != null) _navigatore.currentState?.popUntil((r) => r.isFirst);
  }

  /// L'app è già aperta e il sistema le passa un nuovo indirizzo (link profondo, URL del browser).
  @override
  Future<bool> didPushRouteInformation(RouteInformation routeInformation) async {
    final token = tokenRecuperoDaUri(routeInformation.uri);
    if (token == null) return false;
    tokenRecuperoDaLink.value = token;
    return true;
  }

  @override
  Widget build(BuildContext context) => ListenableBuilder(
    listenable: Listenable.merge([store, tokenRecuperoDaLink]),
    builder: (_, __) => MaterialApp(
      navigatorKey: _navigatore,
      title: 'Veloce24',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(colorSchemeSeed: _seed, useMaterial3: true),
      darkTheme: ThemeData(colorSchemeSeed: _seed, brightness: Brightness.dark, useMaterial3: true),
      home: _home(),
    ),
  );

  /// Schermata iniziale. Un SOLO login (LoginPage); dopo il login `AreaRiservata` sceglie la pagina del ruolo.
  ///  1. link di recupero password → "nuova password" (ha la precedenza anche su una sessione aperta);
  ///  2. demo locale (API_BASE vuoto) → Shell;
  ///  3. sessione col backend (JWT) → area del suo ruolo;
  ///  4. altrimenti → login.
  Widget _home() {
    final token = tokenRecuperoDaLink.value;
    if (token != null) {
      return ForgotPasswordPage(
        key: ValueKey('reset-da-link-$token'),
        tokenIniziale: token,
        client: widget.client,
        onConcluso: () => tokenRecuperoDaLink.value = null, // finito: si torna a login o area del ruolo
      );
    }
    if (store.me != null) return const Shell();
    final s = store.sessione;
    // chiave: la sessione del login (sid), che non cambia quando il token si rinnova
    if (s != null) return AreaRiservata(key: ValueKey(s.idSessione), apiBase: apiBase, sessione: s, client: widget.client);
    return LoginPage(client: widget.client);
  }
}

void toast(BuildContext c, String m) => ScaffoldMessenger.of(c).showSnackBar(SnackBar(content: Text(m)));

Color _statoColor(String s) => switch (s) {
  'consegnata' => Colors.green,
  'non consegnata' || 'persa' => Colors.red,
  'ritirata' || 'in consegna' => Colors.orange,
  'assegnata' => Colors.blue,
  _ => Colors.grey,
};

class StatoChip extends StatelessWidget {
  final String stato;
  const StatoChip(this.stato, {super.key});
  @override
  Widget build(BuildContext context) {
    final c = _statoColor(stato);
    return Chip(
      label: Text(stato, style: TextStyle(color: c, fontWeight: FontWeight.w600, fontSize: 12)),
      backgroundColor: c.withValues(alpha: .15),
      side: BorderSide.none,
      visualDensity: VisualDensity.compact,
    );
  }
}

// ───────── Shell con navigazione per ruolo ─────────
class Shell extends StatefulWidget {
  const Shell({super.key});
  @override
  State<Shell> createState() => _ShellState();
}

class _ShellState extends State<Shell> {
  int i = 0;

  void _esci() {
    Navigator.of(context).popUntil((r) => r.isFirst);
    store.esci();
  }

  @override
  Widget build(BuildContext context) {
    final u = store.me!;
    final voci = <(String, IconData, Widget)>[
      ('Spedizioni', Icons.local_shipping_outlined, const ElencoPage()),
      if (u.ruolo == Ruolo.operatore) ('Nuova', Icons.add_box_outlined, const NuovaPage()),
      if (u.ruolo == Ruolo.admin) ('Utenti', Icons.people_outline, const UtentiPage()),
    ];
    if (i >= voci.length) i = 0;
    return Scaffold(
      appBar: AppBar(
        title: Text('${u.nomeCompleto} · ${ruoloNome[u.ruolo]}', style: const TextStyle(fontSize: 15)),
        actions: [
          IconButton(
            tooltip: 'Il mio profilo',
            icon: const Icon(Icons.account_circle_outlined),
            onPressed: () => Navigator.of(context).push(
              MaterialPageRoute<void>(builder: (_) => ProfilePage.demo(utente: u, onLogout: _esci)),
            ),
          ),
          IconButton(tooltip: 'Esci', icon: const Icon(Icons.logout), onPressed: _esci),
        ],
      ),
      body: voci[i].$3,
      bottomNavigationBar: voci.length < 2
          ? null
          : NavigationBar(
        selectedIndex: i,
        onDestinationSelected: (v) => setState(() => i = v),
        destinations: [for (final v in voci) NavigationDestination(icon: Icon(v.$2), label: v.$1)],
      ),
    );
  }
}

// ───────── Elenco con filtri ─────────
class ElencoPage extends StatefulWidget {
  const ElencoPage({super.key});
  @override
  State<ElencoPage> createState() => _ElencoState();
}

class _ElencoState extends State<ElencoPage> {
  String? stato;
  String q = '';
  @override
  Widget build(BuildContext context) => ListenableBuilder(
    listenable: store,
    builder: (_, __) {
      final u = store.me!;
      final l = store.visibili(u).where((s) =>
      (stato == null || s.stato == stato) &&
          '${s.codOrdine} ${store.indirizzo(s.idConsegna)}'.toLowerCase().contains(q.toLowerCase())).toList();
      return Column(children: [
        Padding(
          padding: const EdgeInsets.all(12),
          child: Row(children: [
            Expanded(
              child: TextField(
                decoration: const InputDecoration(labelText: 'Codice ordine / città', border: OutlineInputBorder(), isDense: true),
                onChanged: (v) => setState(() => q = v),
              ),
            ),
            const SizedBox(width: 8),
            DropdownButton<String?>(
              value: stato,
              hint: const Text('Stato'),
              items: [
                const DropdownMenuItem(value: null, child: Text('Tutti')),
                for (final s in tuttiStati) DropdownMenuItem(value: s, child: Text(s)),
              ],
              onChanged: (v) => setState(() => stato = v),
            ),
          ]),
        ),
        Expanded(
          child: l.isEmpty
              ? const Center(child: Text('Nessuna spedizione.'))
              : ListView(children: [
            for (final s in l)
              ListTile(
                title: Text('${s.codOrdine}${s.isReso ? ' (reso)' : ''}'),
                subtitle: Text(u.ruolo == Ruolo.cliente
                    ? '${store.indirizzo(s.idConsegna)}\nDa ${store.nome(s.idMittente)}'
                    : '${store.indirizzo(s.idConsegna)}\nCorriere: ${store.nome(store.corriereDi(s))}'),
                isThreeLine: true,
                trailing: StatoChip(s.stato),
                onTap: () => Navigator.of(context).push(MaterialPageRoute(builder: (_) => DettaglioPage(s.id))),
              ),
          ]),
        ),
      ]);
    },
  );
}

// ───────── Dettaglio ─────────
class DettaglioPage extends StatefulWidget {
  final int id;
  const DettaglioPage(this.id, {super.key});
  @override
  State<DettaglioPage> createState() => _DettaglioState();
}

class _DettaglioState extends State<DettaglioPage> {
  String? nuovo;
  int? corriere;
  bool interna = true;
  final nota = TextEditingController();
  final nota2 = TextEditingController();

  @override
  Widget build(BuildContext context) => ListenableBuilder(
    listenable: store,
    builder: (context, _) {
      final u = store.me;
      final s = store.byId(widget.id);
      if (u == null || s == null || !store.visibili(u).contains(s)) {
        return Scaffold(appBar: AppBar(), body: const Center(child: Text('Spedizione non trovata o non accessibile.')));
      }
      final staff = u.ruolo != Ruolo.cliente;
      final next = store.prossimiStati(s, u);
      if (nuovo == null || !next.contains(nuovo)) nuovo = next.isEmpty ? null : next.first;
      final cour = store.utenti.where((x) => x.ruolo == Ruolo.corriere).toList();
      final notes = store.note.where((n) => n.idSpedizione == s.id && (staff || !n.interna)).toList();
      final st = Theme.of(context).textTheme;
      return Scaffold(
        appBar: AppBar(title: Text(s.codOrdine)),
        body: ListView(padding: const EdgeInsets.all(16), children: [
          Row(mainAxisAlignment: MainAxisAlignment.spaceBetween, children: [
            Flexible(child: Text('Da ${store.nome(s.idMittente)} a ${store.nome(s.idDestinatario)}', style: st.titleMedium)),
            StatoChip(s.stato),
          ]),
          const SizedBox(height: 8),
          Text('Ritiro: ${store.indirizzo(s.idRitiro)}\nConsegna: ${store.indirizzo(s.idConsegna)}\nRichiesta: ${fd(s.richiestaIl)}'
              '${staff ? '\nCorriere: ${store.nome(store.corriereDi(s))}' : ''}'),
          const Divider(height: 32),
          Text('Storico', style: st.titleMedium),
          for (final c in store.camb.where((c) => c.idSpedizione == s.id))
            ListTile(
              dense: true,
              leading: const Icon(Icons.circle, size: 12),
              title: Text(c.operazione == 'creazione' ? 'Spedizione creata' : 'Stato: ${c.vecchio?['stato']} → ${c.nuovo['stato']}'),
              subtitle: Text('${fd(c.il)}${staff ? ' · ${store.nome(c.idAutore)}' : ''}'),
            ),
          if (notes.isNotEmpty) ...[
            const Divider(height: 32),
            Text('Note', style: st.titleMedium),
            for (final n in notes)
              Card(
                color: n.interna ? Colors.amber.withValues(alpha: .2) : null,
                child: ListTile(
                  title: Text(n.testo),
                  subtitle: Text('${store.nome(n.idAutore)} · ${fd(n.il)}${n.motivo != null ? ' · ${n.motivo}' : ''}${n.interna ? ' · interna' : ''}'),
                ),
              ),
          ],
          if (staff && next.isNotEmpty) ...[
            const Divider(height: 32),
            Text('Cambia stato', style: st.titleMedium),
            const SizedBox(height: 8),
            DropdownButtonFormField<String>(
              value: nuovo,
              decoration: const InputDecoration(labelText: 'Nuovo stato', border: OutlineInputBorder()),
              items: [for (final a in next) DropdownMenuItem(value: a, child: Text(a))],
              onChanged: (v) => setState(() => nuovo = v),
            ),
            if (nuovo == 'assegnata') ...[
              const SizedBox(height: 12),
              DropdownButtonFormField<int>(
                value: corriere,
                decoration: const InputDecoration(labelText: 'Corriere', border: OutlineInputBorder()),
                items: [for (final x in cour) DropdownMenuItem(value: x.id, child: Text(x.nomeCompleto))],
                onChanged: (v) => setState(() => corriere = v),
              ),
            ],
            const SizedBox(height: 12),
            TextField(
              controller: nota,
              maxLines: 3,
              decoration: const InputDecoration(labelText: 'Nota (obbligatoria se la consegna non riesce)', border: OutlineInputBorder()),
            ),
            SwitchListTile(title: const Text('Nota interna'), value: interna, onChanged: (v) => setState(() => interna = v)),
            FilledButton(
              onPressed: () {
                final err = store.cambiaStato(s.id, nuovo!, corriere, nota.text, interna);
                if (err != null) return toast(context, err);
                nota.clear();
                toast(context, 'Stato aggiornato');
              },
              child: const Text('Aggiorna'),
            ),
          ],
          if (staff) ...[
            const Divider(height: 32),
            Text('Aggiungi nota', style: st.titleMedium),
            const SizedBox(height: 8),
            TextField(controller: nota2, maxLines: 2, decoration: const InputDecoration(border: OutlineInputBorder())),
            const SizedBox(height: 8),
            OutlinedButton(
              onPressed: () {
                if (nota2.text.trim().isEmpty) return toast(context, 'Scrivi una nota');
                store.aggiungiNota(s.id, nota2.text, interna);
                nota2.clear();
              },
              child: const Text('Salva nota'),
            ),
          ],
        ]),
      );
    },
  );
}

// ───────── Nuova spedizione (POST /api/spedizioni) ─────────
class NuovaPage extends StatefulWidget {
  const NuovaPage({super.key});
  @override
  State<NuovaPage> createState() => _NuovaState();
}

class _NuovaState extends State<NuovaPage> {
  final key = GlobalKey<FormState>();
  final cod = TextEditingController();
  bool reso = false, invio = false;
  int? mitt, dest, rit, con;
  DateTime quando = DateTime.now();

  Future<void> scegliData() async {
    final d = await showDatePicker(context: context, initialDate: quando, firstDate: DateTime(2024), lastDate: DateTime(2100));
    if (d == null || !mounted) return;
    final t = await showTimePicker(context: context, initialTime: TimeOfDay.fromDateTime(quando));
    if (t != null) setState(() => quando = DateTime(d.year, d.month, d.day, t.hour, t.minute));
  }

  Future<void> invia() async {
    if (!key.currentState!.validate()) return;
    setState(() => invio = true);
    final r = await store.creaSpedizione(
        cod: cod.text, reso: reso, mittente: mitt!, destinatario: dest!, richiesta: quando, ritiro: rit!, consegna: con!);
    if (!mounted) return;
    setState(() => invio = false);
    if (r.errore != null) return toast(context, r.errore!);
    toast(context, 'Spedizione creata');
    cod.clear();
    Navigator.of(context).push(MaterialPageRoute(builder: (_) => DettaglioPage(r.id!)));
  }

  Widget drop<T>(String label, T? v, List<DropdownMenuItem<T>> items, void Function(T?) f) => Padding(
    padding: const EdgeInsets.only(top: 12),
    child: DropdownButtonFormField<T>(
      value: v,
      items: items,
      onChanged: f,
      validator: (x) => x == null ? 'Obbligatorio' : null,
      decoration: InputDecoration(labelText: label, border: const OutlineInputBorder()),
      isExpanded: true,
    ),
  );

  @override
  Widget build(BuildContext context) {
    final cli = [for (final u in store.utenti.where((u) => u.ruolo == Ruolo.cliente)) DropdownMenuItem(value: u.id, child: Text(u.nomeCompleto))];
    final ind = [for (final a in store.indirizzi) DropdownMenuItem(value: a.id, child: Text(a.testo, overflow: TextOverflow.ellipsis))];
    return Form(
      key: key,
      child: ListView(padding: const EdgeInsets.all(16), children: [
        Text('Nuova spedizione', style: Theme.of(context).textTheme.headlineSmall),
        const SizedBox(height: 12),
        TextFormField(
          controller: cod,
          maxLength: 50,
          decoration: const InputDecoration(labelText: 'Codice ordine', border: OutlineInputBorder()),
          validator: (v) => (v == null || v.trim().isEmpty) ? 'cod_ordine è obbligatorio' : null,
        ),
        SwitchListTile(title: const Text('È un reso'), value: reso, onChanged: (v) => setState(() => reso = v)),
        ListTile(
          contentPadding: EdgeInsets.zero,
          title: Text('Data e ora richiesta: ${fd(quando)}'),
          trailing: const Icon(Icons.event),
          onTap: scegliData,
        ),
        drop<int>('Mittente', mitt, cli, (v) => setState(() => mitt = v)),
        drop<int>('Destinatario', dest, cli, (v) => setState(() => dest = v)),
        drop<int>('Indirizzo di ritiro', rit, ind, (v) => setState(() => rit = v)),
        drop<int>('Indirizzo di consegna', con, ind, (v) => setState(() => con = v)),
        const SizedBox(height: 16),
        FilledButton(onPressed: invio ? null : invia, child: Text(invio ? 'Invio…' : 'Crea spedizione')),
      ]),
    );
  }
}

// ───────── Utenti (admin) ─────────
class UtentiPage extends StatelessWidget {
  const UtentiPage({super.key});
  @override
  Widget build(BuildContext context) => ListView(children: [
    for (final u in store.utenti)
      ListTile(
        leading: const Icon(Icons.person_outline),
        title: Text(u.nomeCompleto),
        subtitle: Text('${u.email}\n${u.tel}'),
        isThreeLine: true,
        trailing: Text(ruoloNome[u.ruolo]!),
      ),
  ]);
}
