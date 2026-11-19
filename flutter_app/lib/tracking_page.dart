// Tracking per il cliente destinatario: i pacchi diretti a lui, con filtri e dettaglio.
// Il backend restituisce al cliente solo le spedizioni di cui è destinatario, senza
// id_corriere e con le sole note pubbliche ridotte a motivo e data. La pagina comunque
// non mostra mai corriere, testo o autore delle note, anche se arrivassero.
import 'dart:async';
import 'package:flutter/material.dart';
import 'api.dart';
import 'backend_page.dart' show ChipStato, coloreStato, dataOra, statiSpedizione;

String _giorno(DateTime d) {
  String p(int n) => n.toString().padLeft(2, '0');
  return '${p(d.day)}/${p(d.month)}/${d.year}';
}

/// Elenco dei pacchi in arrivo, filtrabile per numero d'ordine, stato e data di creazione.
class TrackingClientePage extends StatefulWidget {
  final ApiClient api;
  final int perPagina;
  const TrackingClientePage({super.key, required this.api, this.perPagina = 20});
  @override
  State<TrackingClientePage> createState() => _TrackingClienteState();
}

class _TrackingClienteState extends State<TrackingClientePage> {
  final _codice = TextEditingController();
  Timer? _attesa;
  String? _stato;
  DateTimeRange? _periodo;
  int _azzeramenti = 0;

  final _voci = <SpedizioneApi>[];
  int _pagina = 0;
  bool _caricando = false, _altre = false;
  Object? _errore;
  int _richiesta = 0; // scarta le risposte arrivate dopo un cambio di filtro

  bool get _filtriAttivi => _codice.text.trim().isNotEmpty || _stato != null || _periodo != null;

  @override
  void initState() {
    super.initState();
    _carica(reset: true);
  }

  @override
  void dispose() {
    _attesa?.cancel();
    _codice.dispose();
    super.dispose();
  }

  Future<void> _carica({required bool reset}) async {
    final n = ++_richiesta;
    final pagina = reset ? 1 : _pagina + 1;
    setState(() {
      _caricando = true;
      _errore = null;
      if (reset) {
        _voci.clear();
        _altre = false;
      }
    });
    try {
      final p = await widget.api.elencoSpedizioni(
        stato: _stato,
        dal: _periodo?.start,
        al: _periodo?.end,
        codOrdine: _codice.text,
        pagina: pagina,
        perPagina: widget.perPagina,
      );
      if (!mounted || n != _richiesta) return;
      setState(() {
        _voci.addAll(p.dati);
        _pagina = pagina;
        _altre = p.dati.length == widget.perPagina;
      });
    } catch (e) {
      if (!mounted || n != _richiesta) return;
      setState(() => _errore = e);
    } finally {
      if (mounted && n == _richiesta) setState(() => _caricando = false);
    }
  }

  void _codiceCambiato(String _) {
    _attesa?.cancel();
    _attesa = Timer(const Duration(milliseconds: 400), () => _carica(reset: true));
    setState(() {}); // aggiorna la X per cancellare
  }

  Future<void> _scegliPeriodo() async {
    final oggi = DateTime.now();
    final p = await showDateRangePicker(
      context: context,
      firstDate: DateTime(2024),
      lastDate: DateTime(oggi.year, oggi.month, oggi.day),
      initialDateRange: _periodo,
      helpText: 'Data di creazione della spedizione',
    );
    if (p == null) return;
    _periodo = p;
    _carica(reset: true);
  }

  void _azzera() {
    _attesa?.cancel();
    _codice.clear();
    _stato = null;
    _periodo = null;
    _azzeramenti++;
    _carica(reset: true);
  }

  @override
  Widget build(BuildContext context) => Scaffold(
        appBar: AppBar(
          title: const Text('I miei pacchi'),
          actions: [
            IconButton(tooltip: 'Aggiorna', icon: const Icon(Icons.refresh), onPressed: () => _carica(reset: true)),
          ],
        ),
        body: Column(children: [
          _filtri(context),
          const Divider(height: 1),
          Expanded(child: _elenco(context)),
        ]),
      );

  Widget _filtri(BuildContext context) => Padding(
        padding: const EdgeInsets.fromLTRB(12, 12, 12, 8),
        child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          TextField(
            key: const Key('filtro-codice'),
            controller: _codice,
            onChanged: _codiceCambiato,
            onSubmitted: (_) {
              _attesa?.cancel();
              _carica(reset: true);
            },
            textInputAction: TextInputAction.search,
            decoration: InputDecoration(
              labelText: 'Numero ordine',
              prefixIcon: const Icon(Icons.search),
              border: const OutlineInputBorder(),
              isDense: true,
              suffixIcon: _codice.text.isEmpty
                  ? null
                  : IconButton(
                      tooltip: 'Cancella',
                      icon: const Icon(Icons.clear),
                      onPressed: () {
                        _codice.clear();
                        _codiceCambiato('');
                      },
                    ),
            ),
          ),
          const SizedBox(height: 8),
          Row(children: [
            Expanded(
              child: DropdownButtonFormField<String?>(
                key: ValueKey('filtro-stato-$_azzeramenti'), // nuova chiave = torna a "Tutti"
                initialValue: _stato,
                isExpanded: true,
                decoration: const InputDecoration(labelText: 'Stato', border: OutlineInputBorder(), isDense: true),
                items: [
                  const DropdownMenuItem(value: null, child: Text('Tutti')),
                  for (final s in statiSpedizione) DropdownMenuItem(value: s, child: Text(s)),
                ],
                onChanged: (v) {
                  _stato = v;
                  _carica(reset: true);
                },
              ),
            ),
            const SizedBox(width: 8),
            Expanded(
              child: InputDecorator(
                decoration: InputDecoration(
                  labelText: 'Data',
                  border: const OutlineInputBorder(),
                  isDense: true,
                  contentPadding: const EdgeInsets.fromLTRB(12, 4, 0, 4),
                  suffixIcon: _periodo == null
                      ? null
                      : IconButton(
                          tooltip: 'Togli il filtro data',
                          icon: const Icon(Icons.clear, size: 18),
                          onPressed: () {
                            _periodo = null;
                            _carica(reset: true);
                          },
                        ),
                ),
                child: InkWell(
                  key: const Key('filtro-data'),
                  onTap: _scegliPeriodo,
                  child: Padding(
                    padding: const EdgeInsets.symmetric(vertical: 8),
                    child: Text(
                      _periodo == null
                          ? 'Qualsiasi'
                          : _periodo!.start == _periodo!.end
                              ? _giorno(_periodo!.start)
                              : '${_giorno(_periodo!.start)} – ${_giorno(_periodo!.end)}',
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                ),
              ),
            ),
          ]),
          if (_filtriAttivi)
            Align(
              alignment: Alignment.centerRight,
              child: TextButton.icon(onPressed: _azzera, icon: const Icon(Icons.filter_alt_off, size: 18), label: const Text('Azzera filtri')),
            ),
        ]),
      );

  Widget _elenco(BuildContext context) {
    if (_voci.isEmpty && _caricando) return const Center(child: CircularProgressIndicator());
    if (_voci.isEmpty && _errore != null) return _messaggioErrore(_errore!);
    if (_voci.isEmpty) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Text(
            _filtriAttivi ? 'Nessun pacco corrisponde ai filtri.' : 'Non ci sono pacchi in arrivo per te.',
            textAlign: TextAlign.center,
          ),
        ),
      );
    }
    return RefreshIndicator(
      onRefresh: () => _carica(reset: true),
      child: ListView.separated(
        physics: const AlwaysScrollableScrollPhysics(),
        itemCount: _voci.length + 1,
        separatorBuilder: (_, __) => const Divider(height: 1),
        itemBuilder: (context, i) {
          if (i == _voci.length) return _fondo();
          final s = _voci[i];
          return ListTile(
            title: Text('${s.codOrdine}${s.isReso ? ' (reso)' : ''}'),
            subtitle: Text('Consegna: ${s.consegna.testo}\nCreata il ${dataOra(s.creataIl)}'),
            isThreeLine: true,
            trailing: ChipStato(s.stato),
            onTap: () => Navigator.of(context).push(
              MaterialPageRoute(builder: (_) => TrackingDettaglioPage(api: widget.api, id: s.id, codOrdine: s.codOrdine)),
            ),
          );
        },
      ),
    );
  }

  Widget _fondo() {
    if (_caricando) return const Padding(padding: EdgeInsets.all(16), child: Center(child: CircularProgressIndicator()));
    if (_errore != null) {
      final e = _errore;
      return ListTile(
        leading: const Icon(Icons.error_outline),
        title: Text(e is ErroreApi ? e.perUtente : 'Errore inatteso: $e'),
        trailing: TextButton(onPressed: () => _carica(reset: false), child: const Text('Riprova')),
      );
    }
    if (!_altre) return const SizedBox(height: 24);
    return Padding(
      padding: const EdgeInsets.all(12),
      child: Center(child: OutlinedButton(onPressed: () => _carica(reset: false), child: const Text('Carica altri'))),
    );
  }

  Widget _messaggioErrore(Object e) => _Errore(e, onRiprova: () => _carica(reset: true));
}

class _Errore extends StatelessWidget {
  final Object errore;
  final VoidCallback onRiprova;
  const _Errore(this.errore, {required this.onRiprova});
  @override
  Widget build(BuildContext context) {
    final e = errore;
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(mainAxisSize: MainAxisSize.min, children: [
          const Icon(Icons.cloud_off, size: 40),
          const SizedBox(height: 12),
          Text(e is ErroreApi ? e.perUtente : 'Errore inatteso: $e', textAlign: TextAlign.center),
          const SizedBox(height: 12),
          OutlinedButton(onPressed: onRiprova, child: const Text('Riprova')),
        ]),
      ),
    );
  }
}

const _percorso = ['creata', 'assegnata', 'ritirata', 'in consegna', 'consegnata'];

const _spiegazione = {
  'creata': 'Spedizione registrata, in attesa di essere affidata a un corriere.',
  'assegnata': 'Affidata a un corriere, in attesa del ritiro dal mittente.',
  'ritirata': 'Il pacco è stato ritirato dal mittente.',
  'in consegna': 'Il pacco è in viaggio verso di te.',
  'consegnata': 'Il pacco è stato consegnato.',
  'non consegnata': 'Non è stato possibile consegnare il pacco dopo i tentativi previsti.',
  'persa': 'Il pacco risulta smarrito.',
  'annullata': 'La spedizione è stata annullata.',
};

/// Dettaglio di un pacco: stato, indirizzi di mittente e destinatario, date e motivi
/// degli aggiornamenti pubblici. Niente corriere.
class TrackingDettaglioPage extends StatefulWidget {
  final ApiClient api;
  final int id;
  final String codOrdine;
  const TrackingDettaglioPage({super.key, required this.api, required this.id, required this.codOrdine});
  @override
  State<TrackingDettaglioPage> createState() => _TrackingDettaglioState();
}

class _TrackingDettaglioState extends State<TrackingDettaglioPage> {
  late Future<DettaglioSpedizioneApi> _caricamento = widget.api.dettaglioSpedizione(widget.id);

  void _ricarica() => setState(() => _caricamento = widget.api.dettaglioSpedizione(widget.id));

  @override
  Widget build(BuildContext context) => Scaffold(
        appBar: AppBar(
          title: Text(widget.codOrdine),
          actions: [IconButton(tooltip: 'Aggiorna', icon: const Icon(Icons.refresh), onPressed: _ricarica)],
        ),
        body: FutureBuilder<DettaglioSpedizioneApi>(
          future: _caricamento,
          builder: (context, snap) {
            if (snap.connectionState != ConnectionState.done) return const Center(child: CircularProgressIndicator());
            if (snap.hasError) return _Errore(snap.error!, onRiprova: _ricarica);
            return _contenuto(context, snap.data!);
          },
        ),
      );

  Widget _contenuto(BuildContext context, DettaglioSpedizioneApi d) {
    final s = d.spedizione;
    final t = Theme.of(context).textTheme;
    // Solo i motivi delle note pubbliche: il testo e l'autore non sono per il cliente.
    final motivi = s.note.where((n) => !n.interna && n.motivo != null).toList();
    return ListView(padding: const EdgeInsets.all(16), children: [
      Row(children: [
        Expanded(child: Text(s.codOrdine + (s.isReso ? ' (reso)' : ''), style: t.titleLarge)),
        ChipStato(s.stato),
      ]),
      const SizedBox(height: 4),
      Text(_spiegazione[s.stato] ?? s.stato),
      const SizedBox(height: 16),
      _Avanzamento(stato: s.stato),
      const Divider(height: 32),
      Text('Indirizzi', style: t.titleMedium),
      ListTile(
        contentPadding: EdgeInsets.zero,
        leading: const Icon(Icons.outbox_outlined),
        title: const Text('Mittente'),
        subtitle: Text(s.ritiro.testo.isEmpty ? '—' : s.ritiro.testo),
      ),
      ListTile(
        contentPadding: EdgeInsets.zero,
        leading: const Icon(Icons.home_outlined),
        title: const Text('Destinatario'),
        subtitle: Text(s.consegna.testo.isEmpty ? '—' : s.consegna.testo),
      ),
      const Divider(height: 32),
      Text('Date', style: t.titleMedium),
      const SizedBox(height: 8),
      _riga('Creata il', dataOra(s.creataIl)),
      _riga('Ritiro richiesto per il', dataOra(s.richiestaIl)),
      if (d.annullataIl != null)
        _riga('Annullata il', dataOra(d.annullataIl!))
      else if (s.conclusaIl != null)
        _riga('Chiusa il', dataOra(s.conclusaIl!)),
      if (d.tentativiRitiroFalliti > 0) _riga('Tentativi di ritiro non riusciti', '${d.tentativiRitiroFalliti}'),
      if (d.tentativiConsegnaFalliti > 0) _riga('Tentativi di consegna non riusciti', '${d.tentativiConsegnaFalliti}'),
      const Divider(height: 32),
      Text('Aggiornamenti', style: t.titleMedium),
      if (motivi.isEmpty)
        const Padding(padding: EdgeInsets.only(top: 8), child: Text('Nessun problema segnalato.'))
      else
        for (final n in motivi)
          ListTile(
            contentPadding: EdgeInsets.zero,
            leading: const Icon(Icons.info_outline),
            title: Text(n.motivo!),
            subtitle: Text(dataOra(n.creataIl)),
          ),
    ]);
  }

  Widget _riga(String etichetta, String valore) => Padding(
        padding: const EdgeInsets.only(bottom: 6),
        child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Expanded(child: Text(etichetta)),
          Text(valore, style: const TextStyle(fontWeight: FontWeight.w600)),
        ]),
      );
}

/// Passi del percorso normale; per non consegnata, persa e annullata basta il chip e la spiegazione.
class _Avanzamento extends StatelessWidget {
  final String stato;
  const _Avanzamento({required this.stato});
  @override
  Widget build(BuildContext context) {
    final i = _percorso.indexOf(stato);
    if (i < 0) {
      final c = coloreStato(stato);
      return Container(
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(color: c.withValues(alpha: .12), borderRadius: BorderRadius.circular(8)),
        child: Row(children: [
          Icon(Icons.report_outlined, color: c),
          const SizedBox(width: 8),
          Expanded(child: Text('Spedizione chiusa: $stato')),
        ]),
      );
    }
    final attivo = Theme.of(context).colorScheme.primary;
    final spento = Theme.of(context).colorScheme.outlineVariant;
    return Row(children: [
      for (var k = 0; k < _percorso.length; k++)
        Expanded(
          child: Column(children: [
            Icon(k <= i ? Icons.check_circle : Icons.radio_button_unchecked, color: k <= i ? attivo : spento, size: 20),
            const SizedBox(height: 4),
            Text(
              _percorso[k],
              textAlign: TextAlign.center,
              style: TextStyle(fontSize: 11, fontWeight: k == i ? FontWeight.w700 : FontWeight.normal),
            ),
          ]),
        ),
    ]);
  }
}
