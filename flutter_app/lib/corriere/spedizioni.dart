// Elenco delle spedizioni del corriere, con la ricerca per numero d'ordine, stato e
// data. Quali spedizioni vede (oggi quelle assegnate a lui in assegnata, ritirata e in
// consegna) lo decide il backend: la pagina mostra ciò che riceve.
// Toccando una spedizione si apre il dettaglio.
import 'package:flutter/material.dart';

import '../comune/comuni.dart';
import 'api.dart';
import 'dettaglio.dart';

class SpedizioniCorrierePage extends StatefulWidget {
  final ApiCorriere api;
  const SpedizioniCorrierePage({super.key, required this.api});
  @override
  State<SpedizioniCorrierePage> createState() => _SpedizioniCorrierePageState();
}

class _SpedizioniCorrierePageState extends State<SpedizioniCorrierePage> {
  final _codOrdine = TextEditingController();
  String? _stato;
  DateTime? _dal, _al;

  int _pagina = 1;
  static const _perPagina = ApiSessione.perPaginaSpedizioni;
  Pagina<Spedizione>? _risultato;
  Object? _errore;
  bool _attesa = false;

  @override
  void initState() {
    super.initState();
    _carica();
  }

  @override
  void dispose() {
    _codOrdine.dispose();
    super.dispose();
  }

  FiltroSpedizioni get _filtro => FiltroSpedizioni(stato: _stato, dal: _dal, al: _al, codOrdine: _codOrdine.text);

  bool get _conFiltri => _stato != null || _dal != null || _al != null || _codOrdine.text.isNotEmpty;

  Future<void> _carica({int pagina = 1}) async {
    setState(() {
      _attesa = true;
      _errore = null;
    });
    try {
      final r = await widget.api.spedizioni(_filtro, pagina: pagina, perPagina: _perPagina);
      if (!mounted) return;
      // Dopo una consegna la pagina attuale può non esistere più: si va all'ultima.
      if (r.oltreLaFine) return _carica(pagina: r.totalePagine);
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
      _dal = _al = null;
      _codOrdine.clear();
    });
    _carica();
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
    if (d == null) return;
    setState(() => inizio ? _dal = d : _al = d);
    _carica();
  }

  Future<void> _apri(Spedizione s) async {
    final messaggio = await Navigator.of(context).push<String>(
      MaterialPageRoute(builder: (_) => DettaglioCorrierePage(api: widget.api, id: s.id, codOrdine: s.codOrdine)),
    );
    if (!mounted) return;
    if (messaggio != null) avviso(context, messaggio);
    // Al ritorno stato e contatori possono essere cambiati: l'elenco si rilegge sempre.
    _carica(pagina: _pagina);
  }

  Widget _filtri() => Padding(
        padding: const EdgeInsets.fromLTRB(12, 12, 12, 4),
        child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          TextField(
            controller: _codOrdine,
            textInputAction: TextInputAction.search,
            decoration: InputDecoration(
              labelText: 'Numero ordine',
              isDense: true,
              border: const OutlineInputBorder(),
              errorText: erroreCampo(_errore, 'cod_ordine'),
              suffixIcon: IconButton(tooltip: 'Cerca', icon: const Icon(Icons.search), onPressed: _attesa ? null : () => _carica()),
            ),
            onSubmitted: (_) => _carica(),
          ),
          const SizedBox(height: 8),
          Wrap(spacing: 8, runSpacing: 4, crossAxisAlignment: WrapCrossAlignment.center, children: [
            SizedBox(
              width: 160,
              child: DropdownButtonFormField<String?>(
                value: _stato,
                isDense: true,
                isExpanded: true, // il testo si adatta alla larghezza invece di sbordare
                decoration: InputDecoration(
                  labelText: 'Stato',
                  isDense: true,
                  border: const OutlineInputBorder(),
                  errorText: erroreCampo(_errore, 'stato'),
                ),
                items: [
                  const DropdownMenuItem(value: null, child: Text('Tutti')),
                  // tutti gli stati, senza filtri: cosa restituire lo decide il backend
                  for (final s in statiSpedizione) DropdownMenuItem(value: s, child: Text(s)),
                ],
                onChanged: (s) {
                  setState(() => _stato = s);
                  _carica();
                },
              ),
            ),
            InputChip(
              avatar: const Icon(Icons.event, size: 18),
              label: Text(_dal == null ? 'Dal: —' : 'Dal: ${fmtData(_dal!)}'),
              onPressed: () => _scegliData(true),
              onDeleted: _dal == null
                  ? null
                  : () {
                      setState(() => _dal = null);
                      _carica();
                    },
            ),
            InputChip(
              avatar: const Icon(Icons.event, size: 18),
              label: Text(_al == null ? 'Al: —' : 'Al: ${fmtData(_al!)}'),
              onPressed: () => _scegliData(false),
              onDeleted: _al == null
                  ? null
                  : () {
                      setState(() => _al = null);
                      _carica();
                    },
            ),
            if (_conFiltri) TextButton(onPressed: _attesa ? null : _azzera, child: const Text('Azzera filtri')),
          ]),
          if (erroreCampo(_errore, 'al') != null || erroreCampo(_errore, 'dal') != null)
            Padding(
              padding: const EdgeInsets.only(top: 4),
              child: Text('Date: ${erroreCampo(_errore, 'al') ?? erroreCampo(_errore, 'dal')}',
                  style: TextStyle(color: Theme.of(context).colorScheme.error, fontSize: 12)),
            ),
        ]),
      );

  Widget _voce(Spedizione s) => Card(
        margin: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
        clipBehavior: Clip.antiAlias,
        child: InkWell(
          onTap: () => _apri(s),
          child: Padding(
            padding: const EdgeInsets.all(12),
            child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              Row(children: [
                Expanded(child: Text(s.codOrdine, style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 16))),
                ChipStato(s.stato),
              ]),
              const SizedBox(height: 4),
              _riga(Icons.outbox_outlined, 'Ritiro', '${s.mittente?.nomeCompleto ?? '—'} · ${s.ritiro.testo}'),
              _riga(Icons.move_to_inbox_outlined, 'Consegna', '${s.destinatario.nomeCompleto} · ${s.consegna.testo}'),
              _riga(Icons.schedule, 'Richiesta il', fmtDataOra(s.richiestaIl)),
            ]),
          ),
        ),
      );

  Widget _riga(IconData icona, String etichetta, String testo) => Padding(
        padding: const EdgeInsets.only(top: 4),
        child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Icon(icona, size: 16, color: Theme.of(context).colorScheme.outline),
          const SizedBox(width: 6),
          Expanded(child: Text('$etichetta: $testo', style: Theme.of(context).textTheme.bodyMedium)),
        ]),
      );

  @override
  Widget build(BuildContext context) {
    final r = _risultato;
    final err = _errore;
    // Gli errori sui filtri compaiono accanto ai campi; qui solo quelli generali.
    final erroreGenerale = err != null && (err is! ErroreServer || err.campi.isEmpty);
    return Column(children: [
      _filtri(),
      if (_attesa) const LinearProgressIndicator() else const SizedBox(height: 4),
      Expanded(
        child: erroreGenerale
            ? Center(
                child: Column(mainAxisSize: MainAxisSize.min, children: [
                  Padding(padding: const EdgeInsets.all(16), child: Text(testoErrore(err), textAlign: TextAlign.center)),
                  OutlinedButton(onPressed: () => _carica(pagina: _pagina), child: const Text('Riprova')),
                ]),
              )
            : r == null
                ? const SizedBox()
                : RefreshIndicator(
                    onRefresh: () => _carica(pagina: _pagina),
                    child: r.dati.isEmpty
                        // ListView anche da vuota: così si può trascinare per aggiornare.
                        ? ListView(children: [
                            Padding(
                              padding: const EdgeInsets.all(32),
                              child: Text(
                                _conFiltri ? 'Nessuna spedizione con questi filtri' : 'Nessuna spedizione',
                                textAlign: TextAlign.center,
                              ),
                            ),
                          ])
                        : ListView.builder(
                            padding: const EdgeInsets.symmetric(vertical: 4),
                            itemCount: r.dati.length,
                            itemBuilder: (_, i) => _voce(r.dati[i]),
                          ),
                  ),
      ),
      if (r != null) BarraPagine(pagina: r, attesa: _attesa, vaiA: (p) => _carica(pagina: p)),
    ]);
  }
}
