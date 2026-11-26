// Elenco delle spedizioni con i filtri per stato, corriere, data e riferimento, comune a
// operatori e amministratori: cambia solo il dettaglio che si apre (con le azioni per
// l'operatore, in sola lettura per l'amministratore). L'amministratore sceglie sempre una
// sede (la prima attiva all'inizio) e vede le spedizioni di quella sola sede.
// Su schermo largo il dettaglio sta a destra dell'elenco, su schermo stretto si apre
// in una pagina a parte.
import 'package:flutter/material.dart';

import 'api.dart';
import 'comuni.dart';
import 'ricerca.dart';

class SpedizioniPage extends StatefulWidget {
  final ApiSessione api;

  /// Dettaglio della spedizione [id]; [alCambio] aggiorna l'elenco dopo una modifica.
  final Widget Function(int id, VoidCallback alCambio) dettaglio;

  /// Mostra la scelta della sede: solo per l'amministratore, che guarda sempre una sede
  /// alla volta (non c'è l'elenco di tutte le sedi insieme: ogni gruppo di sedi ha il suo
  /// database delle spedizioni). Operatori e corrieri vedono già solo la propria.
  final bool filtroSede;

  /// Cresce a ogni modifica delle sedi (solo con [filtroSede]): si rilegge l'elenco delle
  /// sedi, tenendo quella scelta se c'è ancora.
  final int versioneSedi;
  const SpedizioniPage(
      {super.key, required this.api, required this.dettaglio, this.filtroSede = false, this.versioneSedi = 0});
  @override
  State<SpedizioniPage> createState() => SpedizioniPageState();
}

class SpedizioniPageState extends State<SpedizioniPage> {
  final _codOrdine = TextEditingController();
  String? _stato;
  Persona? _corriere;
  DateTime? _dal, _al;
  int? _idSede;
  List<Sede>? _sedi; // con filtroSede: null finché non arrivano
  // sedi oltre la prima pagina (con molte città): si cercano con "Cerca un'altra sede…"
  bool _altreSedi = false;
  Object? _erroreSedi;
  // numero dell'ultima richiesta dell'elenco: la risposta di una richiesta superata (es.
  // l'amministratore ha già scelto un'altra sede) si scarta
  int _richiesta = 0;
  static const _cercaSede = -1; // voce della tendina che apre la ricerca

  int _pagina = 1;
  static const _perPagina = ApiSessione.perPaginaSpedizioni;
  Pagina<Spedizione>? _risultato;
  Object? _errore;
  bool _attesa = false;
  int? _selezionata;
  int _versioneDettaglio = 0;

  @override
  void initState() {
    super.initState();
    // l'amministratore vede le spedizioni solo dopo aver scelto la sede: prima le sedi
    if (widget.filtroSede) {
      _caricaSedi();
    } else {
      _carica();
    }
  }

  @override
  void didUpdateWidget(SpedizioniPage vecchio) {
    super.didUpdateWidget(vecchio);
    if (widget.filtroSede && vecchio.versioneSedi != widget.versioneSedi) _caricaSedi();
  }

  /// Sede proposta all'amministratore: la prima attiva, altrimenti la prima.
  static Sede? _sedePredefinita(List<Sede> sedi) => sedi.where((s) => s.attiva).firstOrNull ?? sedi.firstOrNull;

  /// Legge le sedi; tiene quella scelta se c'è ancora, altrimenti sceglie la predefinita,
  /// e poi legge le spedizioni di quella sede.
  Future<void> _caricaSedi() async {
    setState(() => _erroreSedi = null);
    try {
      final pagina = await widget.api.sedi();
      final sedi = pagina.dati;
      if (!mounted) return;
      final prima = _idSede;
      // con più sedi di una pagina, quella scelta con la ricerca può non essere nella prima:
      // resta in elenco. Con tutte le sedi in pagina, se manca non c'è più.
      final altre = pagina.totaleElementi > sedi.length;
      final scelta = altre ? _sedi?.where((s) => s.id == _idSede).firstOrNull : null;
      setState(() {
        _sedi = [...sedi, if (scelta != null && !sedi.any((s) => s.id == scelta.id)) scelta];
        _altreSedi = altre;
        if (!_sedi!.any((s) => s.id == _idSede)) _idSede = _sedePredefinita(sedi)?.id;
        if (_idSede == null) _risultato = null; // nessuna sede: nessun elenco
      });
      if (_idSede == null) return;
      // stessa sede di prima: si rilegge la pagina attuale (il nome della sede può essere cambiato)
      _idSede == prima ? await _carica(pagina: _pagina) : await _carica();
    } catch (e) {
      if (!mounted) return;
      if (_sedi == null) {
        setState(() => _erroreSedi = e); // senza sedi non si sa quali spedizioni chiedere
      } else {
        registraErrore(e); // restano le sedi già lette e l'elenco della sede scelta
      }
    }
  }

  FiltroSpedizioni get _filtro =>
      FiltroSpedizioni(stato: _stato, idCorriere: _corriere?.id, idSede: _idSede, dal: _dal, al: _al, codOrdine: _codOrdine.text);

  Future<void> _carica({int pagina = 1}) async {
    // l'amministratore chiede sempre le spedizioni di una sede: senza sede scelta nessuna richiesta
    if (widget.filtroSede && _idSede == null) return;
    final questa = ++_richiesta;
    setState(() {
      _attesa = true;
      _errore = null;
    });
    try {
      final r = await widget.api.spedizioni(_filtro, pagina: pagina, perPagina: _perPagina);
      // superata da una richiesta più recente (altra sede, altri filtri): non si mostra
      if (!mounted || questa != _richiesta) return;
      // Dopo una modifica la pagina attuale può non esistere più: si va all'ultima.
      if (r.oltreLaFine) return await _carica(pagina: r.totalePagine);
      setState(() {
        _risultato = r;
        _pagina = pagina;
      });
    } catch (e) {
      if (mounted && questa == _richiesta) setState(() => _errore = e);
    } finally {
      if (mounted && questa == _richiesta) setState(() => _attesa = false);
    }
  }

  /// Con più sedi di quante ne stanno nella tendina: l'amministratore cerca la sede per nome
  /// o città (GET /api/sedi?q=), e quella scelta entra nella tendina.
  Future<void> _scegliAltraSede() async {
    final sede = await showDialog<Sede>(
      context: context,
      builder: (_) => DialogoRicerca<Sede>(
        titolo: 'Scegli la sede',
        cerca: (q) => widget.api.sedi(q: q, perPagina: 20),
        titoloVoce: (s) => s.attiva ? s.nome : '${s.nome} (disattivata)',
        sottotitoloVoce: (s) => s.citta,
      ),
    );
    if (sede == null || !mounted || sede.id == _idSede) return;
    setState(() {
      if (!(_sedi ?? const <Sede>[]).any((s) => s.id == sede.id)) _sedi = [...?_sedi, sede];
      _idSede = sede.id;
    });
    _carica();
  }

  /// Azzera i filtri ma non la sede: l'amministratore ne ha sempre una scelta.
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
          body: widget.dettaglio(id, () => _carica(pagina: _pagina)),
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
              isExpanded: true, // il testo si adatta alla larghezza invece di sbordare
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
          if (widget.filtroSede && _idSede != null)
            SizedBox(
              width: 190,
              child: DropdownButtonFormField<int>(
                value: _idSede,
                isDense: true,
                isExpanded: true,
                decoration: const InputDecoration(labelText: 'Sede', isDense: true, border: OutlineInputBorder()),
                // solo sedi: non c'è la voce "tutte"
                items: [
                  for (final s in _sedi ?? const <Sede>[])
                    DropdownMenuItem(
                        value: s.id, child: Text(s.attiva ? s.nome : '${s.nome} (disattivata)', overflow: TextOverflow.ellipsis)),
                  if (_altreSedi)
                    const DropdownMenuItem(value: _cercaSede, child: Text('Cerca un\'altra sede…', overflow: TextOverflow.ellipsis)),
                ],
                onChanged: (id) {
                  if (id == _cercaSede) {
                    _scegliAltraSede();
                    return;
                  }
                  if (id == null || id == _idSede) return;
                  setState(() => _idSede = id);
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

  Widget _conRiprova(Object errore, VoidCallback riprova) => Center(
        child: Column(mainAxisSize: MainAxisSize.min, children: [
          Padding(padding: const EdgeInsets.all(16), child: Text(testoErrore(errore), textAlign: TextAlign.center)),
          OutlinedButton(onPressed: riprova, child: const Text('Riprova')),
        ]),
      );

  /// Con filtroSede, ciò che va al posto dell'elenco finché non c'è una sede scelta.
  Widget? _senzaSede() {
    if (!widget.filtroSede || _idSede != null) return null;
    if (_erroreSedi != null) return _conRiprova(_erroreSedi!, _caricaSedi);
    if (_sedi == null) return const Center(child: CircularProgressIndicator());
    return const Center(child: Text('Nessuna sede: creane una dalla sezione Sedi.'));
  }

  Widget _elenco() {
    final r = _risultato;
    final senzaSede = _senzaSede();
    return Column(children: [
      _filtri(),
      if (_attesa) const LinearProgressIndicator() else const SizedBox(height: 4),
      Expanded(
        child: senzaSede != null
            ? senzaSede
            : _errore != null
                ? _conRiprova(_errore!, () => _carica(pagina: _pagina))
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
      if (r != null && senzaSede == null) BarraPagine(pagina: r, attesa: _attesa, vaiA: (p) => _carica(pagina: p)),
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
            : KeyedSubtree(
                key: ValueKey('$_selezionata-$_versioneDettaglio'),
                child: widget.dettaglio(_selezionata!, () => _carica(pagina: _pagina)),
              ),
      ),
    ]);
  }
}
