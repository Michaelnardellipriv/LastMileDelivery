// Viste collegate al backend Go vero (non allo store demo in memoria).
// Avvio: flutter run -d chrome --web-port 5173 --dart-define=API_BASE=http://localhost:8080
import 'package:flutter/material.dart';
import 'api.dart';

String _data(DateTime d) {
  String p(int n) => n.toString().padLeft(2, '0');
  return '${p(d.day)}/${p(d.month)}/${d.year} ${p(d.hour)}:${p(d.minute)}';
}

Color _colore(String s) => switch (s) {
      'consegnata' => Colors.green,
      'non consegnata' || 'persa' => Colors.red,
      'ritirata' || 'in consegna' => Colors.orange,
      'assegnata' => Colors.blue,
      _ => Colors.grey,
    };

class _ChipStato extends StatelessWidget {
  final String stato;
  const _ChipStato(this.stato);
  @override
  Widget build(BuildContext context) {
    final c = _colore(stato);
    return Chip(
      label: Text(stato, style: TextStyle(color: c, fontWeight: FontWeight.w600, fontSize: 12)),
      backgroundColor: c.withValues(alpha: .15),
      side: BorderSide.none,
      visualDensity: VisualDensity.compact,
    );
  }
}

/// Scelta dell'identità da mandare negli header X-Id-* (il backend non ha ancora il login).
class BackendAccessoPage extends StatefulWidget {
  final String apiBase;
  const BackendAccessoPage({super.key, required this.apiBase});
  @override
  State<BackendAccessoPage> createState() => _BackendAccessoState();
}

class _BackendAccessoState extends State<BackendAccessoPage> {
  RuoloApi ruolo = RuoloApi.operatore;
  final id = TextEditingController(text: '1');

  void _entra() {
    final n = int.tryParse(id.text.trim());
    if (n == null || n <= 0) {
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text("L'ID deve essere un numero intero positivo")));
      return;
    }
    final api = ApiClient(base: widget.apiBase, ruolo: ruolo, idUtente: n);
    Navigator.of(context).push(MaterialPageRoute(builder: (_) => BackendElencoPage(api: api)));
  }

  @override
  Widget build(BuildContext context) => Scaffold(
        appBar: AppBar(title: const Text('Accesso al backend')),
        body: ListView(padding: const EdgeInsets.all(16), children: [
          Text('Backend: ${widget.apiBase}', style: Theme.of(context).textTheme.bodySmall),
          const SizedBox(height: 8),
          const Text('Scegli ruolo e ID di un utente che esiste nel database '
              '(con db/seed.sql: operatore 1, corriere 1, clienti 1 e 2).'),
          const SizedBox(height: 16),
          DropdownButtonFormField<RuoloApi>(
            initialValue: ruolo,
            decoration: const InputDecoration(labelText: 'Ruolo', border: OutlineInputBorder()),
            items: [for (final r in RuoloApi.values) DropdownMenuItem(value: r, child: Text(r.nome))],
            onChanged: (v) => setState(() => ruolo = v ?? ruolo),
          ),
          const SizedBox(height: 12),
          TextField(
            controller: id,
            keyboardType: TextInputType.number,
            decoration: const InputDecoration(labelText: 'ID utente', border: OutlineInputBorder()),
            onSubmitted: (_) => _entra(),
          ),
          const SizedBox(height: 16),
          FilledButton.icon(onPressed: _entra, icon: const Icon(Icons.login), label: const Text('Entra')),
        ]),
      );
}

/// Elenco spedizioni da GET /api/spedizioni, con filtro per stato.
class BackendElencoPage extends StatefulWidget {
  final ApiClient api;
  const BackendElencoPage({super.key, required this.api});
  @override
  State<BackendElencoPage> createState() => _BackendElencoState();
}

const _stati = ['creata', 'assegnata', 'ritirata', 'in consegna', 'consegnata', 'non consegnata', 'persa', 'annullata'];

class _BackendElencoState extends State<BackendElencoPage> {
  String? stato;
  late Future<PaginaSpedizioni> _caricamento = _carica();

  Future<PaginaSpedizioni> _carica() => widget.api.elencoSpedizioni(stato: stato);

  Future<void> _ricarica() async {
    final f = _carica();
    setState(() => _caricamento = f);
    try {
      await f;
    } catch (_) {
      // l'errore lo mostra il FutureBuilder
    }
  }

  @override
  Widget build(BuildContext context) => Scaffold(
        appBar: AppBar(
          title: Text('${widget.api.ruolo.nome} #${widget.api.idUtente}', style: const TextStyle(fontSize: 15)),
          actions: [IconButton(tooltip: 'Aggiorna', icon: const Icon(Icons.refresh), onPressed: _ricarica)],
        ),
        body: Column(children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(12, 8, 12, 0),
            child: DropdownButtonFormField<String?>(
              initialValue: stato,
              decoration: const InputDecoration(labelText: 'Stato', border: OutlineInputBorder(), isDense: true),
              items: [
                const DropdownMenuItem(value: null, child: Text('Tutti')),
                for (final s in _stati) DropdownMenuItem(value: s, child: Text(s)),
              ],
              onChanged: (v) {
                stato = v;
                _ricarica();
              },
            ),
          ),
          Expanded(
            child: FutureBuilder<PaginaSpedizioni>(
              future: _caricamento,
              builder: (context, snap) {
                if (snap.connectionState != ConnectionState.done) {
                  return const Center(child: CircularProgressIndicator());
                }
                if (snap.hasError) {
                  final e = snap.error;
                  final testo = e is ErroreApi ? e.perUtente : 'Errore inatteso: $e';
                  return Center(
                    child: Padding(
                      padding: const EdgeInsets.all(24),
                      child: Column(mainAxisSize: MainAxisSize.min, children: [
                        const Icon(Icons.cloud_off, size: 40),
                        const SizedBox(height: 12),
                        Text(testo, textAlign: TextAlign.center),
                        const SizedBox(height: 12),
                        OutlinedButton(onPressed: _ricarica, child: const Text('Riprova')),
                      ]),
                    ),
                  );
                }
                final l = snap.data!.dati;
                if (l.isEmpty) return const Center(child: Text('Nessuna spedizione.'));
                return RefreshIndicator(
                  onRefresh: _ricarica,
                  child: ListView.separated(
                    itemCount: l.length,
                    separatorBuilder: (_, __) => const Divider(height: 1),
                    itemBuilder: (_, i) {
                      final s = l[i];
                      return ListTile(
                        title: Text('${s.codOrdine}${s.isReso ? ' (reso)' : ''}'),
                        subtitle: Text('Consegna: ${s.consegna.testo}\nRichiesta: ${_data(s.richiestaIl)}'),
                        isThreeLine: true,
                        trailing: _ChipStato(s.stato),
                        onTap: () => _dettaglio(context, s),
                      );
                    },
                  ),
                );
              },
            ),
          ),
        ]),
      );

  void _dettaglio(BuildContext context, SpedizioneApi s) => showModalBottomSheet<void>(
        context: context,
        showDragHandle: true,
        builder: (_) => ListView(padding: const EdgeInsets.fromLTRB(16, 0, 16, 24), shrinkWrap: true, children: [
          Row(children: [
            Expanded(child: Text(s.codOrdine, style: Theme.of(context).textTheme.titleLarge)),
            _ChipStato(s.stato),
          ]),
          const SizedBox(height: 8),
          Text('Ritiro: ${s.ritiro.testo}'),
          Text('Consegna: ${s.consegna.testo}'),
          Text('Creata il ${_data(s.creataIl)}'),
          if (s.idCorriere != null) Text('Corriere #${s.idCorriere}'),
          if (s.conclusaIl != null) Text('Conclusa il ${_data(s.conclusaIl!)}'),
          const SizedBox(height: 12),
          Text('Note', style: Theme.of(context).textTheme.titleSmall),
          if (s.note.isEmpty) const Text('Nessuna nota.'),
          for (final n in s.note)
            ListTile(
              contentPadding: EdgeInsets.zero,
              leading: Icon(n.interna ? Icons.lock_outline : Icons.chat_bubble_outline),
              title: Text(n.testo),
              subtitle: Text([_data(n.creataIl), if (n.motivo != null) n.motivo!, if (n.interna) 'interna'].join(' · ')),
            ),
        ]),
      );
}
