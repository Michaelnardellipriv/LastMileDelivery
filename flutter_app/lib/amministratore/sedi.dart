// Gestione delle sedi: una per città. L'amministratore le crea, ne cambia nome e
// indirizzo (nella stessa città: le spedizioni della sede sono legate alla sua città), le
// disattiva e riattiva, e ne apre i parametri. Alla creazione si sceglie anche il gruppo
// del database delle spedizioni (0 = principale), che poi non cambia. Una sede
// disattivata non accetta nuove spedizioni né nuovi corrieri o operatori; chi ci lavora
// chiude quelle già aperte.
// Come le altre pagine non valida nulla: manda ciò che è scritto e mostra gli errori del
// backend.
import 'package:flutter/material.dart';

import '../comune/comuni.dart';
import '../comune/ricerca.dart';
import 'api.dart';
import 'configurazione.dart';

class SediPage extends StatefulWidget {
  final ApiAmministratore api;

  /// Avvisa le altre pagine (utenti, configurazione) che l'elenco delle sedi è cambiato.
  final VoidCallback? alCambio;
  const SediPage({super.key, required this.api, this.alCambio});
  @override
  State<SediPage> createState() => _SediPageState();
}

class _SediPageState extends State<SediPage> {
  final _q = TextEditingController();
  bool? _attiva;
  int _pagina = 1;
  Pagina<Sede>? _risultato;
  Object? _errore;
  bool _attesa = false;

  ApiAmministratore get api => widget.api;

  @override
  void initState() {
    super.initState();
    _carica();
  }

  Future<void> _carica({int pagina = 1}) async {
    setState(() {
      _attesa = true;
      _errore = null;
    });
    try {
      final r = await api.sedi(q: _q.text, attiva: _attiva, pagina: pagina, perPagina: 20);
      if (!mounted) return;
      if (r.oltreLaFine) return await _carica(pagina: r.totalePagine);
      setState(() {
        _risultato = r;
        _pagina = pagina;
      });
    } catch (e) {
      if (mounted) setState(() => _errore = e);
    } finally {
      if (mounted) setState(() => _attesa = false);
    }
  }

  void _cambiata() {
    widget.alCambio?.call();
    _carica(pagina: _pagina);
  }

  Future<void> _apri([Sede? sede]) async {
    final fatta = await showDialog<Sede>(context: context, builder: (_) => DialogoSede(api: api, sede: sede));
    if (fatta == null || !mounted) return;
    avviso(context, sede == null ? 'Sede ${fatta.nome} creata con i parametri predefiniti' : 'Sede ${fatta.nome} aggiornata');
    _cambiata();
  }

  Future<void> _cambiaAttiva(Sede s) async {
    final attiva = !s.attiva;
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(attiva ? 'Riattivare ${s.nome}?' : 'Disattivare ${s.nome}?'),
        content: Text(attiva
            ? 'La sede tornerà ad accettare nuove spedizioni, corrieri e operatori.'
            : 'La sede non accetterà più nuove spedizioni né nuovi corrieri o operatori. '
                'Chi ci lavora continua ad accedere e porta a termine le spedizioni già aperte.'),
        actions: [
          TextButton(onPressed: () => Navigator.of(ctx).pop(false), child: const Text('Annulla')),
          FilledButton(onPressed: () => Navigator.of(ctx).pop(true), child: Text(attiva ? 'Riattiva' : 'Disattiva')),
        ],
      ),
    );
    if (ok != true || !mounted) return;
    setState(() => _attesa = true);
    try {
      await api.modificaSede(s.id, {'attiva': attiva});
      if (!mounted) return;
      avviso(context, attiva ? '${s.nome} riattivata' : '${s.nome} disattivata');
      _cambiata();
    } catch (e) {
      if (mounted) mostraErrore(context, e);
    } finally {
      if (mounted) setState(() => _attesa = false);
    }
  }

  void _parametri(Sede s) => showDialog<void>(
        context: context,
        builder: (ctx) => Dialog(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 860),
            child: SingleChildScrollView(
              padding: const EdgeInsets.all(8),
              child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.end, children: [
                ParametriCard(api: api, sede: s),
                TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Chiudi')),
              ]),
            ),
          ),
        ),
      );

  Widget _filtri() => Padding(
        padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
        child: Wrap(spacing: 8, runSpacing: 8, crossAxisAlignment: WrapCrossAlignment.center, children: [
          SizedBox(
            width: 240,
            child: TextField(
              controller: _q,
              decoration: const InputDecoration(
                  labelText: 'Cerca nome, città, regione', isDense: true, border: OutlineInputBorder(), prefixIcon: Icon(Icons.search)),
              onSubmitted: (_) => _carica(),
            ),
          ),
          SizedBox(
            width: 150,
            child: DropdownButtonFormField<bool?>(
              value: _attiva,
              isDense: true,
              isExpanded: true,
              decoration: const InputDecoration(labelText: 'Stato', isDense: true, border: OutlineInputBorder()),
              items: const [
                DropdownMenuItem(value: null, child: Text('Tutte')),
                DropdownMenuItem(value: true, child: Text('Attive')),
                DropdownMenuItem(value: false, child: Text('Disattivate')),
              ],
              onChanged: (a) {
                setState(() => _attiva = a);
                _carica();
              },
            ),
          ),
          FilledButton.tonalIcon(onPressed: _attesa ? null : () => _carica(), icon: const Icon(Icons.search), label: const Text('Cerca')),
          FilledButton.icon(onPressed: () => _apri(), icon: const Icon(Icons.add_business_outlined), label: const Text('Nuova sede')),
        ]),
      );

  Widget _voce(Sede s) {
    final colori = Theme.of(context).colorScheme;
    return ListTile(
      leading: CircleAvatar(
        backgroundColor: s.attiva ? colori.primaryContainer : colori.surfaceContainerHighest,
        child: Icon(Icons.store_mall_directory_outlined, color: s.attiva ? colori.onPrimaryContainer : colori.outline),
      ),
      title: Wrap(spacing: 8, crossAxisAlignment: WrapCrossAlignment.center, children: [
        Text(s.nome, style: TextStyle(fontWeight: FontWeight.w600, color: s.attiva ? null : colori.outline)),
        Text(s.citta, style: Theme.of(context).textTheme.bodySmall),
        if (!s.attiva)
          Chip(
            label: const Text('disattivata', style: TextStyle(fontSize: 11)),
            visualDensity: VisualDensity.compact,
            side: BorderSide.none,
            backgroundColor: colori.errorContainer,
          ),
      ]),
      subtitle: Text('${s.indirizzo.testo} · id sede ${s.id} · Database spedizioni: gruppo ${s.gruppoSpedizioni}'),
      trailing: Row(mainAxisSize: MainAxisSize.min, children: [
        IconButton(tooltip: 'Parametri della sede', onPressed: _attesa ? null : () => _parametri(s), icon: const Icon(Icons.tune)),
        IconButton(tooltip: 'Modifica sede', onPressed: _attesa ? null : () => _apri(s), icon: const Icon(Icons.edit_outlined)),
        TextButton(onPressed: _attesa ? null : () => _cambiaAttiva(s), child: Text(s.attiva ? 'Disattiva' : 'Riattiva')),
      ]),
    );
  }

  @override
  Widget build(BuildContext context) {
    final r = _risultato;
    return Column(children: [
      _filtri(),
      if (_attesa) const LinearProgressIndicator() else const SizedBox(height: 4),
      Expanded(
        child: _errore != null
            ? Center(
                child: Column(mainAxisSize: MainAxisSize.min, children: [
                  Padding(padding: const EdgeInsets.all(16), child: Text(testoErrore(_errore!), textAlign: TextAlign.center)),
                  OutlinedButton(onPressed: () => _carica(pagina: _pagina), child: const Text('Riprova')),
                ]),
              )
            : r == null
                ? const SizedBox()
                : r.dati.isEmpty
                    ? const Center(child: Text('Nessuna sede con questi filtri'))
                    : ListView.separated(
                        itemCount: r.dati.length,
                        separatorBuilder: (_, __) => const Divider(height: 1),
                        itemBuilder: (_, i) => _voce(r.dati[i]),
                      ),
      ),
      if (r != null) BarraPagine(pagina: r, attesa: _attesa, vaiA: (p) => _carica(pagina: p)),
    ]);
  }
}

/// Creazione (senza [sede]) o modifica di una sede. In modifica partono solo i campi
/// cambiati; l'indirizzo, se cambia, parte intero, e città, regione e nazione restano
/// quelle della sede. Il gruppo del database delle spedizioni si sceglie solo in
/// creazione (vuoto = 0, il principale) e in modifica non parte mai.
class DialogoSede extends StatefulWidget {
  final ApiAmministratore api;
  final Sede? sede;
  const DialogoSede({super.key, required this.api, this.sede});
  @override
  State<DialogoSede> createState() => _DialogoSedeState();
}

class _DialogoSedeState extends State<DialogoSede> {
  static const _indirizzo = ['via', 'civico', 'citta', 'cap', 'regione', 'nazione'];
  final _campi = Campi(['nome', ..._indirizzo, 'gruppo_spedizioni']);
  late String _chiave = widget.api.nuovaChiave();
  Object? _errore;
  bool _attesa = false;

  bool get _creazione => widget.sede == null;

  Map<String, String> _valori(Sede s) => {
        'nome': s.nome,
        'via': s.indirizzo.via,
        'civico': s.indirizzo.civico,
        'citta': s.indirizzo.citta ?? '',
        'cap': s.indirizzo.cap ?? '',
        'regione': s.indirizzo.regione ?? '',
        'nazione': s.indirizzo.nazione ?? '',
      };

  @override
  void initState() {
    super.initState();
    final s = widget.sede;
    if (s == null) {
      _campi.c['nazione']!.text = 'Italia';
    } else {
      for (final e in _valori(s).entries) {
        _campi.c[e.key]!.text = e.value;
      }
    }
  }

  Map<String, Object?> get _corpo {
    final indirizzo = {for (final n in _indirizzo) n: _campi[n]};
    final s = widget.sede;
    if (s == null) {
      // il gruppo parte solo se scritto; un numero parte come numero, il resto così com'è
      final gruppo = _campi['gruppo_spedizioni'].trim();
      return {
        'nome': _campi['nome'],
        'indirizzo': indirizzo,
        if (gruppo.isNotEmpty) 'gruppo_spedizioni': int.tryParse(gruppo) ?? gruppo,
      };
    }
    final prima = _valori(s);
    return {
      if (_campi['nome'] != prima['nome']) 'nome': _campi['nome'],
      if (_indirizzo.any((n) => _campi[n] != prima[n])) 'indirizzo': indirizzo,
    };
  }

  Future<void> _salva() async {
    setState(() {
      _attesa = true;
      _errore = null;
    });
    try {
      final s = widget.sede;
      final fatta = s == null
          ? await widget.api.creaSede(_corpo, chiave: _chiave)
          : await widget.api.modificaSede(s.id, _corpo, chiave: _chiave);
      if (mounted) Navigator.of(context).pop(fatta);
    } catch (e) {
      // rifiutata dal backend: nulla è cambiato, il prossimo invio è una richiesta nuova
      if (e is ErroreServer && e.status != null) _chiave = widget.api.nuovaChiave();
      if (mounted) setState(() => _errore = e);
    } finally {
      if (mounted) setState(() => _attesa = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    // in modifica la città non cambia: le spedizioni della sede sono legate a quella città
    final fissa = !_creazione;
    final errore = erroreGenerale(context, _errore,
        campiMostrati: {'nome', for (final n in _indirizzo) 'indirizzo.$n', if (_creazione) 'gruppo_spedizioni'});
    // gli errori sull'indirizzo arrivano come "indirizzo.via": li si mostra sotto il campo "via"
    final e = _errore;
    final perCampo = e is ErroreServer
        ? ErroreServer(e.status, e.messaggio, {for (final x in e.campi.entries) x.key.replaceFirst('indirizzo.', ''): x.value})
        : null;
    Widget c(String n, String etichetta,
            {double larghezza = 240, String? suggerimento, bool soloLettura = false, TextInputType? tastiera}) =>
        _campi.campo(n, etichetta, perCampo,
            larghezza: larghezza, suggerimento: suggerimento, soloLettura: soloLettura, tastiera: tastiera);

    return AlertDialog(
      title: Text(_creazione ? 'Nuova sede' : 'Modifica ${widget.sede!.nome}'),
      content: SizedBox(
        width: 520,
        child: SingleChildScrollView(
          child: Wrap(spacing: 12, runSpacing: 12, children: [
            c('nome', 'Nome della sede', larghezza: 492),
            c('via', 'Via', larghezza: 390),
            c('civico', 'Civico', larghezza: 90),
            c('citta', 'Città',
                larghezza: 390,
                soloLettura: fissa,
                suggerimento: fissa ? 'La città della sede non cambia' : 'Una sola sede per città'),
            c('cap', 'CAP', larghezza: 90),
            c('regione', 'Regione', soloLettura: fissa),
            c('nazione', 'Nazione', soloLettura: fissa),
            if (_creazione)
              c('gruppo_spedizioni', 'Gruppo database spedizioni',
                  larghezza: 492,
                  tastiera: TextInputType.number,
                  suggerimento: 'Quale database delle spedizioni usa la sede: 0 = principale. Non si cambia dopo la creazione.')
            else
              SizedBox(width: 492, child: Text('Database spedizioni: gruppo ${widget.sede!.gruppoSpedizioni} (non si cambia)')),
            if (_creazione)
              const SizedBox(
                width: 492,
                child: Text('La sede nasce con i parametri predefiniti, che poi si cambiano da "Parametri della sede".'),
              ),
            if (errore != null) SizedBox(width: 492, child: errore),
          ]),
        ),
      ),
      actions: [
        TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Annulla')),
        FilledButton(
          onPressed: _attesa ? null : _salva,
          child: _attesa
              ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
              : Text(_creazione ? 'Crea sede' : 'Salva modifiche'),
        ),
      ],
    );
  }
}
