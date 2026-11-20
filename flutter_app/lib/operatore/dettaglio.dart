// Dettaglio di una spedizione per l'operatore: dati, cambio di stato, assegnazione
// del corriere, note, posizione del corriere (aggiornata da sola) se la spedizione è
// in consegna, storico. La pagina non valida nulla: manda quello che l'operatore
// inserisce e mostra gli errori restituiti dal backend.
import 'dart:async';

import 'package:flutter/material.dart';

import 'api.dart';
import '../comune/comuni.dart';
import '../comune/scheda_spedizione.dart';
import 'mappa.dart';
import 'selettori.dart';

class DettaglioSpedizione extends StatefulWidget {
  final ApiOperatore api;
  final int id;
  final VoidCallback? alCambio; // per aggiornare l'elenco dopo una modifica
  const DettaglioSpedizione({super.key, required this.api, required this.id, this.alCambio});
  @override
  State<DettaglioSpedizione> createState() => _DettaglioSpedizioneState();
}

class _DettaglioSpedizioneState extends State<DettaglioSpedizione> {
  Spedizione? _s;
  List<VoceStorico> _storico = [];
  PosizioneCorriere? _posizione;
  DateTime? _posizioneLetta;
  Timer? _timerPosizione;
  Object? _errore;
  Object? _erroreNota;
  bool _attesa = true;
  bool _invio = false;
  final _nota = TextEditingController();

  ApiOperatore get api => widget.api;

  @override
  void initState() {
    super.initState();
    _carica();
  }

  @override
  void dispose() {
    _timerPosizione?.cancel();
    super.dispose();
  }

  Future<void> _carica() async {
    setState(() {
      _attesa = true;
      _errore = null;
    });
    try {
      // dettaglio e storico insieme: una sola attesa invece di due
      final (s, storico) = await (api.dettaglio(widget.id), api.storico(widget.id)).wait;
      if (!mounted) return;
      setState(() {
        _s = s;
        _storico = storico;
      });
      _aggiornaPosizione();
    } catch (e) {
      if (mounted) setState(() => _errore = e);
    } finally {
      if (mounted) setState(() => _attesa = false);
    }
  }

  /// Rilegge la posizione del corriere all'intervallo impostato dall'amministratore,
  /// finché la spedizione è in consegna e il dettaglio è aperto.
  Future<void> _aggiornaPosizione() async {
    _timerPosizione?.cancel();
    if (_s?.stato != 'in consegna') {
      setState(() => _posizione = null);
      return;
    }
    var intervallo = 30;
    try {
      final p = await api.posizioni();
      intervallo = p.intervalloSecondi;
      if (mounted) {
        setState(() {
          _posizione = p.perSpedizione(widget.id);
          _posizioneLetta = DateTime.now();
        });
      }
    } catch (_) {
      // la posizione è un'informazione in più: se non arriva si riprova al giro dopo
    }
    if (mounted) _timerPosizione = Timer(Duration(seconds: intervallo.clamp(5, 3600)), _aggiornaPosizione);
  }

  Future<void> _esegui(Future<void> Function() azione, String fatto) async {
    setState(() => _invio = true);
    try {
      await azione();
      if (!mounted) return;
      avviso(context, fatto);
      widget.alCambio?.call();
      await _carica();
    } catch (e) {
      if (mounted) mostraErrore(context, e);
    } finally {
      if (mounted) setState(() => _invio = false);
    }
  }

  Future<void> _cambiaStato() async {
    final fatto = await showDialog<String>(
      context: context,
      builder: (_) => _DialogoStato(api: api, idSpedizione: widget.id, statoAttuale: _s!.stato),
    );
    if (fatto == null || !mounted) return;
    avviso(context, 'Stato aggiornato: $fatto');
    widget.alCambio?.call();
    await _carica();
  }

  Future<void> _reso(bool reso) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(reso ? 'Segnare come reso?' : 'Togliere il reso?'),
        content: Text(reso
            ? 'La spedizione ${_s!.codOrdine} torna al mittente. La modifica resta nello storico.'
            : 'La spedizione ${_s!.codOrdine} non sarà più segnata come reso. La modifica resta nello storico.'),
        actions: [
          TextButton(onPressed: () => Navigator.of(ctx).pop(false), child: const Text('Annulla')),
          FilledButton(onPressed: () => Navigator.of(ctx).pop(true), child: const Text('Conferma')),
        ],
      ),
    );
    if (ok != true || !mounted) return;
    await _esegui(() => api.impostaReso(widget.id, reso), reso ? 'Spedizione segnata come reso' : 'Reso tolto');
  }

  Future<void> _assegna() async {
    final c = await scegliCorriere(context, api);
    final idCorriere = c?.id;
    if (c == null || idCorriere == null) return;
    await _esegui(() => api.assegna(widget.id, idCorriere), 'Spedizione assegnata a ${c.nomeCompleto}');
  }

  Future<void> _aggiungiNota() async {
    setState(() {
      _invio = true;
      _erroreNota = null;
    });
    try {
      await api.aggiungiNota(widget.id, _nota.text);
      if (!mounted) return;
      _nota.clear();
      avviso(context, 'Nota interna aggiunta');
      await _carica();
    } catch (e) {
      if (!mounted) return;
      if (erroreCampo(e, 'nota') != null) {
        setState(() => _erroreNota = e);
      } else {
        mostraErrore(context, e);
      }
    } finally {
      if (mounted) setState(() => _invio = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_attesa && _s == null) return const Center(child: CircularProgressIndicator());
    if (_errore != null && _s == null) {
      return Center(
        child: Column(mainAxisSize: MainAxisSize.min, children: [
          Text(testoErrore(_errore!)),
          const SizedBox(height: 8),
          OutlinedButton(onPressed: _carica, child: const Text('Riprova')),
        ]),
      );
    }
    final s = _s!;
    return Stack(children: [
      ListView(padding: const EdgeInsets.all(16), children: [
        Row(children: [
          Expanded(child: SelectableText(s.codOrdine, style: Theme.of(context).textTheme.headlineSmall)),
          ChipStato(s.stato),
          IconButton(tooltip: 'Aggiorna', onPressed: _carica, icon: const Icon(Icons.refresh)),
        ]),
        const SizedBox(height: 8),
        Padding(
          padding: const EdgeInsets.only(bottom: 12),
          child: Wrap(spacing: 8, runSpacing: 8, children: [
            FilledButton.icon(
              onPressed: _invio ? null : _cambiaStato,
              icon: const Icon(Icons.swap_horiz),
              label: const Text('Cambia stato'),
            ),
            FilledButton.tonalIcon(
              onPressed: _invio ? null : _assegna,
              icon: const Icon(Icons.person_add_alt),
              label: Text(s.corriere == null ? 'Assegna corriere' : 'Riassegna corriere'),
            ),
            // sempre disponibile: se lo stato non ammette il reso risponde il backend
            OutlinedButton.icon(
              onPressed: _invio ? null : () => _reso(!s.isReso),
              icon: Icon(s.isReso ? Icons.undo : Icons.assignment_return_outlined),
              label: Text(s.isReso ? 'Togli reso' : 'Segna come reso'),
            ),
          ]),
        ),
        ...sezioniSpedizione(s),
        if (s.stato == 'in consegna') _sezionePosizione(),
        _sezioneNote(s),
        SezioneStorico(_storico),
      ]),
      if (_invio) const Positioned(top: 0, left: 0, right: 0, child: LinearProgressIndicator()),
    ]);
  }

  Widget _sezionePosizione() {
    final p = _posizione;
    return Sezione('Posizione del corriere', [
      if (p == null)
        const Text('Il corriere non ha ancora inviato una posizione (avvia il simulatore per vederla).')
      else ...[
        Riga('Corriere', p.nomeCompleto),
        Riga('Coordinate', '${p.latitudine.toStringAsFixed(5)}, ${p.longitudine.toStringAsFixed(5)}'),
        Riga('Rilevata il', '${fmtData(p.rilevataIl)} ${fmtOra(p.rilevataIl)}'),
        if (_posizioneLetta != null) Riga('Controllata alle', fmtOra(_posizioneLetta!)),
        const SizedBox(height: 8),
        // la stessa mappa per tutto il tempo: si sposta solo il furgone (niente tessere ricaricate)
        SizedBox(height: 260, child: MappaPosizioni(posizioni: [p], segui: true)),
      ],
    ]);
  }

  Widget _sezioneNote(Spedizione s) => Sezione('Note', [
        ...vociNote(s.note),
        const Divider(),
        Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Expanded(
            child: TextField(
              controller: _nota,
              minLines: 1,
              maxLines: 4,
              decoration: InputDecoration(
                labelText: 'Nuova nota interna',
                helperText: 'Le note pubbliche (motivo per il cliente) si scrivono cambiando lo stato in annullata o persa',
                errorText: erroreCampo(_erroreNota, 'nota'),
                border: const OutlineInputBorder(),
              ),
            ),
          ),
          const SizedBox(width: 8),
          Padding(
            padding: const EdgeInsets.only(top: 4),
            child: FilledButton(onPressed: _invio ? null : _aggiungiNota, child: const Text('Aggiungi')),
          ),
        ]),
      ]);
}

/// Cambio di stato: l'operatore sceglie lo stato e, se vuole aggiungere la nota pubblica,
/// motivo (pubblico) e nota (interna). La richiesta parte così com'è, con i campi anche
/// vuoti: se lo stato non è raggiungibile o motivo e nota mancano, sono vuoti o non sono
/// ammessi, il backend risponde con l'errore e la finestra lo mostra.
class _DialogoStato extends StatefulWidget {
  final ApiOperatore api;
  final int idSpedizione;
  final String statoAttuale;
  const _DialogoStato({required this.api, required this.idSpedizione, required this.statoAttuale});
  @override
  State<_DialogoStato> createState() => _DialogoStatoState();
}

class _DialogoStatoState extends State<_DialogoStato> {
  String? _stato;
  String? _motivo;
  bool _conNota = false; // se mandare anche motivo e nota
  final _nota = TextEditingController();
  List<String>? _motivi; // null finché non arrivano
  Object? _erroreMotivi;
  Object? _errore;
  bool _invio = false;

  @override
  void initState() {
    super.initState();
    _caricaMotivi();
  }

  void _caricaMotivi() {
    setState(() => _erroreMotivi = null);
    widget.api.motivi().then((m) {
      if (mounted) setState(() => _motivi = m);
    }, onError: (Object e) {
      if (mounted) setState(() => _erroreMotivi = e);
    });
  }

  Future<void> _invia() async {
    final stato = _stato;
    setState(() {
      _invio = true;
      _errore = null;
    });
    try {
      // con la nota pubblica, motivo e nota partono come sono a schermo, anche vuoti
      await widget.api.cambiaStato(widget.idSpedizione, stato ?? '',
          motivo: _conNota ? _motivo ?? '' : null, nota: _conNota ? _nota.text : null);
      if (mounted) Navigator.of(context).pop(stato);
    } catch (e) {
      if (mounted) setState(() => _errore = e);
    } finally {
      if (mounted) setState(() => _invio = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final err = _errore;
    final senzaCampi = err != null && (err is! ErroreServer || err.campi.isEmpty);
    return AlertDialog(
      title: Text('Cambia stato (ora: ${widget.statoAttuale})'),
      content: SizedBox(
        width: 480,
        child: SingleChildScrollView(
          child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.stretch, children: [
            DropdownButtonFormField<String>(
              value: _stato,
              decoration:
                  InputDecoration(labelText: 'Nuovo stato', errorText: erroreCampo(err, 'stato'), border: const OutlineInputBorder()),
              items: [for (final s in statiSpedizione) DropdownMenuItem(value: s, child: Text(s))],
              onChanged: (s) => setState(() => _stato = s),
            ),
            SwitchListTile(
              contentPadding: EdgeInsets.zero,
              value: _conNota,
              title: const Text('Aggiungi motivo e nota'),
              subtitle: const Text('Nota pubblica: il backend la richiede per annullata e persa'),
              onChanged: (v) => setState(() => _conNota = v),
            ),
            if (!_conNota) ...[
              // errori del backend su campi che non sono a schermo
              for (final campo in const ['motivo', 'nota'])
                if (erroreCampo(err, campo) != null)
                  Text('$campo: ${erroreCampo(err, campo)}', style: TextStyle(color: Theme.of(context).colorScheme.error)),
            ] else if (_erroreMotivi != null)
              Row(children: [
                Expanded(
                  child: Text('Elenco dei motivi non disponibile. ${testoErrore(_erroreMotivi!)}',
                      style: TextStyle(color: Theme.of(context).colorScheme.error)),
                ),
                TextButton(onPressed: _caricaMotivi, child: const Text('Riprova')),
              ])
            else if (_motivi == null)
              const LinearProgressIndicator()
            else
              DropdownButtonFormField<String?>(
                value: _motivo,
                isExpanded: true,
                decoration: InputDecoration(
                  labelText: 'Motivo (pubblico, lo vede il cliente)',
                  errorText: erroreCampo(err, 'motivo'),
                  errorMaxLines: 3,
                  border: const OutlineInputBorder(),
                ),
                items: [
                  const DropdownMenuItem(value: null, child: Text('Nessun motivo')),
                  // tutti i motivi, senza filtri per stato: quale è ammesso lo decide il backend
                  for (final m in _motivi!) DropdownMenuItem(value: m, child: Text(m)),
                ],
                onChanged: (m) => setState(() => _motivo = m),
              ),
            if (_conNota) ...[
              const SizedBox(height: 12),
              TextField(
                controller: _nota,
                minLines: 2,
                maxLines: 5,
                decoration: InputDecoration(
                  labelText: 'Nota interna',
                  errorText: erroreCampo(err, 'nota'),
                  errorMaxLines: 3,
                  border: const OutlineInputBorder(),
                ),
              ),
            ],
            if (senzaCampi)
              Padding(
                padding: const EdgeInsets.only(top: 12),
                child: Text(testoErrore(err), style: TextStyle(color: Theme.of(context).colorScheme.error)),
              ),
          ]),
        ),
      ),
      actions: [
        TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Annulla')),
        FilledButton(
          onPressed: _invio ? null : _invia,
          child: _invio ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2)) : const Text('Conferma'),
        ),
      ],
    );
  }
}
