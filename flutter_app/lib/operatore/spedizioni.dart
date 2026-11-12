// Elenco di tutte le spedizioni con i filtri per stato, corriere, data e riferimento.
// Su schermo largo il dettaglio sta a destra dell'elenco, su schermo stretto si apre
// in una pagina a parte.
import 'package:flutter/material.dart';

import 'api.dart';
import 'comuni.dart';
import 'dettaglio.dart';
import 'selettori.dart';

class SpedizioniPage extends StatefulWidget {
  final ApiOperatore api;
  const SpedizioniPage({super.key, required this.api});
  @override
  State<SpedizioniPage> createState() => SpedizioniPageState();
}

class SpedizioniPageState extends State<SpedizioniPage> {
  final _codOrdine = TextEditingController();
  String? _stato;
  Persona? _corriere;
  DateTime? _dal, _al;

  int _pagina = 1;
  static const _perPagina = 25;
  Pagina<Spedizione>? _risultato;
  Object? _errore;
  bool _attesa = false;
  int? _selezionata;
  int _versioneDettaglio = 0;

  @override
  void initState() {
    super.initState();
    _carica();
  }

  FiltroSpedizioni get _filtro =>
      FiltroSpedizioni(stato: _stato, idCorriere: _corriere?.id, dal: _dal, al: _al, codOrdine: _codOrdine.text);

  Future<void> _carica({int pagina = 1}) async {
    setState(() {
      _attesa = true;
      _errore = null;
    });
    try {
      final r = await widget.api.spedizioni(_filtro, pagina: pagina, perPagina: _perPagina);
      if (mounted)
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

  void _azzera() {
    setState(() {
      _stato = null;
      _corriere = null;
      _dal = _al = null;
      _codOrdine.clear();
    });
    _carica();
  }

  /// Apre il dettaglio di una spedizione (dopo la creazione o dalla mappa).
  void apri(int id, {bool aggiornaElenco = false}) {
    if (aggiornaElenco) _carica(pagina: _pagina);
    _seleziona(id);
  }

  void _seleziona(int id) {
    if (MediaQuery.sizeOf(context).width >= 1100) {
      setState(() {
        _selezionata = id;
        _versioneDettaglio++;
      });
    } else {
      Navigator.of(context).push(MaterialPageRoute(
        builder: (_) => Scaffold(
          appBar: AppBar(title: const Text('Dettaglio spedizione')),
          body: DettaglioSpedizione(api: widget.api, id: id, alCambio: () => _carica(pagina: _pagina)),
        ),
      ));
    }
  }

  Future<void> _scegliData(bool inizio) async {
    final oggi = DateTime.now();
    final d = await showDatePicker(
      context: context,
      initialDate: (inizio ? _dal : _al) ?? oggi,
      firstDate: DateTime(2024),
      lastDate: DateTime(oggi.year + 2),
      helpText: inizio ? 'Creata dal' : 'Creata fino al',
    );
    if (d != null) setState(() => inizio ? _dal = d : _al = d);
  }

  Widget _filtri() => Padding(
        padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
        child: Wrap(spacing: 8, runSpacing: 8, crossAxisAlignment: WrapCrossAlignment.center, children: [
          SizedBox(
            width: 200,
            child: TextField(
              controller: _codOrdine,
              decoration: const InputDecoration(labelText: 'Riferimento ordine', isDense: true, border: OutlineInputBorder()),
              onSubmitted: (_) => _carica(),
            ),
          ),
          SizedBox(
            width: 170,
            child: DropdownButtonFormField<String?>(
              value: _stato,
              isDense: true,
              decoration: const InputDecoration(labelText: 'Stato', isDense: true, border: OutlineInputBorder()),
              items: [
                const DropdownMenuItem(value: null, child: Text('Tutti')),
                for (final s in statiSpedizione) DropdownMenuItem(value: s, child: Text(s)),
              ],
              onChanged: (s) {
                setState(() => _stato = s);
                _carica();
              },
            ),
          ),
          InputChip(
            avatar: const Icon(Icons.person_search, size: 18),
            label: Text(_corriere == null ? 'Corriere: tutti' : 'Corriere: ${_corriere!.nomeCompleto}'),
            onPressed: () async {
              final c = await scegliCorriere(context, widget.api);
              if (c != null) {
                setState(() => _corriere = c);
                _carica();
              }
            },
            onDeleted: _corriere == null
                ? null
                : () {
                    setState(() => _corriere = null);
                    _carica();
                  },
          ),
          InputChip(
            avatar: const Icon(Icons.event, size: 18),
            label: Text(_dal == null ? 'Dal: —' : 'Dal: ${fmtData(_dal!)}'),
            onPressed: () => _scegliData(true),
            onDeleted: _dal == null ? null : () => setState(() => _dal = null),
          ),
          InputChip(
            avatar: const Icon(Icons.event, size: 18),
            label: Text(_al == null ? 'Al: —' : 'Al: ${fmtData(_al!)}'),
            onPressed: () => _scegliData(false),
            onDeleted: _al == null ? null : () => setState(() => _al = null),
          ),
          FilledButton.icon(onPressed: _attesa ? null : () => _carica(), icon: const Icon(Icons.search), label: const Text('Cerca')),
          TextButton(onPressed: _attesa ? null : _azzera, child: const Text('Azzera filtri')),
        ]),
      );

  Widget _elenco() {
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
                    ? const Center(child: Text('Nessuna spedizione con questi filtri'))
                    : RefreshIndicator(
                        onRefresh: () => _carica(pagina: _pagina),
                        child: ListView.separated(
                          itemCount: r.dati.length,
                          separatorBuilder: (_, __) => const Divider(height: 1),
                          itemBuilder: (_, i) => _voce(r.dati[i]),
                        ),
                      ),
      ),
      if (r != null)
        Padding(
          padding: const EdgeInsets.all(8),
          child: Row(mainAxisAlignment: MainAxisAlignment.center, children: [
            IconButton(
              tooltip: 'Pagina precedente',
              onPressed: _attesa || _pagina <= 1 ? null : () => _carica(pagina: _pagina - 1),
              icon: const Icon(Icons.chevron_left),
            ),
            Text('Pagina $_pagina'),
            IconButton(
              tooltip: 'Pagina successiva',
              onPressed: _attesa || !r.forseAltre ? null : () => _carica(pagina: _pagina + 1),
              icon: const Icon(Icons.chevron_right),
            ),
          ]),
        ),
    ]);
  }

  Widget _voce(Spedizione s) => ListTile(
        selected: s.id == _selezionata,
        title: Text(s.codOrdine, style: const TextStyle(fontWeight: FontWeight.w600)),
        subtitle: Text(
          '${s.mittente?.nomeCompleto ?? '—'} → ${s.destinatario.nomeCompleto}\n'
          '${s.consegna.testo}\n'
          'Creata ${fmtDataOra(s.creataIl)}${s.corriere == null ? '' : ' · corriere ${s.corriere!.nomeCompleto}'}',
        ),
        isThreeLine: true,
        trailing: Column(mainAxisAlignment: MainAxisAlignment.center, crossAxisAlignment: CrossAxisAlignment.end, children: [
          ChipStato(s.stato),
          if (s.isReso) const Text('reso', style: TextStyle(fontSize: 11)),
        ]),
        onTap: () => _seleziona(s.id),
      );

  @override
  Widget build(BuildContext context) {
    final largo = MediaQuery.sizeOf(context).width >= 1100;
    if (!largo) return _elenco();
    return Row(children: [
      SizedBox(width: 520, child: _elenco()),
      const VerticalDivider(width: 1),
      Expanded(
        child: _selezionata == null
            ? const Center(child: Text('Seleziona una spedizione per vederne il dettaglio'))
            : DettaglioSpedizione(
                key: ValueKey('$_selezionata-$_versioneDettaglio'),
                api: widget.api,
                id: _selezionata!,
                alCambio: () => _carica(pagina: _pagina),
              ),
      ),
    ]);
  }
}
