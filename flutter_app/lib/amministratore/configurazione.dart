// Configurazione della gestione delle spedizioni: i parametri di ogni sede (giorni di
// visibilità per il cliente destinatario, tentativi massimi, intervallo della posizione
// del corriere) e l'elenco dei motivi pubblici, uguale per tutte le sedi. I valori partono come sono scritti: i limiti li
// controlla il backend, che risponde con l'errore sotto il campo.
import 'package:flutter/material.dart';

import '../comune/comuni.dart';
import '../comune/ricerca.dart';
import 'api.dart';

class ConfigurazionePage extends StatefulWidget {
  final ApiAmministratore api;

  /// Cambia quando l'elenco delle sedi cambia (creata, rinominata...): si ricarica.
  final int versioneSedi;
  const ConfigurazionePage({super.key, required this.api, this.versioneSedi = 0});
  @override
  State<ConfigurazionePage> createState() => _ConfigurazionePageState();
}

class _ConfigurazionePageState extends State<ConfigurazionePage> {
  List<Sede>? _sedi;
  Sede? _sede;
  Object? _errore;

  @override
  void initState() {
    super.initState();
    _caricaSedi();
  }

  @override
  void didUpdateWidget(ConfigurazionePage vecchio) {
    super.didUpdateWidget(vecchio);
    if (vecchio.versioneSedi != widget.versioneSedi) _caricaSedi();
  }

  Future<void> _caricaSedi() async {
    try {
      final r = await widget.api.sedi();
      if (!mounted) return;
      setState(() {
        _sedi = r.dati;
        _errore = null;
        // resta sulla sede scelta, se c'è ancora; altrimenti la prima
        _sede = r.dati.where((s) => s.id == _sede?.id).firstOrNull ?? r.dati.firstOrNull;
      });
    } catch (e) {
      if (mounted) setState(() => _errore = e);
    }
  }

  @override
  Widget build(BuildContext context) {
    final sedi = _sedi;
    final sede = _sede;
    return ListView(padding: const EdgeInsets.all(16), children: [
      Align(
        alignment: Alignment.topCenter,
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 860),
          child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
            if (_errore != null && sedi == null)
              Row(children: [
                Expanded(child: Text(testoErrore(_errore!), style: TextStyle(color: Theme.of(context).colorScheme.error))),
                TextButton(onPressed: _caricaSedi, child: const Text('Riprova')),
              ])
            else if (sedi != null && sedi.isEmpty)
              const Text('Nessuna sede: creane una dalla sezione Sedi.')
            else if (sedi != null)
              Padding(
                padding: const EdgeInsets.only(bottom: 8),
                child: Align(
                  alignment: Alignment.centerLeft,
                  child: SizedBox(
                    width: 320,
                    child: DropdownButtonFormField<int>(
                      value: sede?.id,
                      isExpanded: true,
                      decoration: const InputDecoration(
                          labelText: 'Sede', helperText: 'Ogni sede ha i suoi parametri', border: OutlineInputBorder()),
                      items: [
                        for (final s in sedi)
                          DropdownMenuItem(value: s.id, child: Text('${s.nome} · ${s.citta}${s.attiva ? '' : ' (disattivata)'}')),
                      ],
                      onChanged: (id) => setState(() => _sede = sedi.firstWhere((s) => s.id == id)),
                    ),
                  ),
                ),
              ),
            // la chiave cambia con la sede: il riquadro rilegge i parametri di quella sede
            if (sede != null) ParametriCard(key: ValueKey(sede.id), api: widget.api, sede: sede),
            MotiviCard(api: widget.api),
          ]),
        ),
      ),
    ]);
  }
}

/// Parametri di una sede (GET/PATCH /api/sedi/{id}/impostazioni). Al salvataggio
/// partono solo i valori cambiati.
class ParametriCard extends StatefulWidget {
  final ApiAmministratore api;
  final Sede sede;
  const ParametriCard({super.key, required this.api, required this.sede});
  @override
  State<ParametriCard> createState() => _ParametriCardState();
}

class _ParametriCardState extends State<ParametriCard> {
  static const _voci = [
    ('giorni_visibilita_cliente', 'Giorni di visibilità per il cliente',
        'Per quanti giorni dalla chiusura il cliente destinatario vede una spedizione (da 1 a 365)'),
    ('intervallo_posizione_sec', 'Intervallo della posizione (secondi)',
        'Ogni quanti secondi il corriere invia la posizione e la mappa degli operatori si aggiorna (da 5 a 3600)'),
    ('max_tentativi_ritiro', 'Tentativi massimi di ritiro', 'Ritiri falliti dopo i quali la spedizione va annullata (da 1 a 10)'),
    ('max_tentativi_consegna', 'Tentativi massimi di consegna',
        'Consegne fallite dopo le quali la spedizione diventa non consegnata (da 1 a 10)'),
  ];
  final _campi = Campi([for (final v in _voci) v.$1]);
  Impostazioni? _attuali;
  Object? _erroreLettura, _errore;
  bool _attesa = true;
  late String _chiave = widget.api.nuovaChiave();

  @override
  void initState() {
    super.initState();
    _carica();
  }

  Future<void> _carica() async {
    setState(() {
      _attesa = true;
      _erroreLettura = null;
    });
    try {
      _mostra(await widget.api.impostazioni(widget.sede.id));
    } catch (e) {
      if (mounted) setState(() => _erroreLettura = e);
    } finally {
      if (mounted) setState(() => _attesa = false);
    }
  }

  void _mostra(Impostazioni imp) {
    if (!mounted) return;
    setState(() {
      _attuali = imp;
      _errore = null;
      for (final e in imp.valori.entries) {
        _campi.c[e.key]!.text = '${e.value}';
      }
    });
  }

  /// Solo i valori cambiati. Un numero parte come numero, qualsiasi altra cosa come
  /// testo: lo rifiuta il backend.
  Map<String, Object?> get _modifiche {
    final attuali = _attuali!.valori;
    return {
      for (final v in _voci)
        if (_campi[v.$1] != '${attuali[v.$1]}') v.$1: int.tryParse(_campi[v.$1]) ?? _campi[v.$1],
    };
  }

  Future<void> _salva() async {
    setState(() {
      _attesa = true;
      _errore = null;
    });
    try {
      final imp = await widget.api.modificaImpostazioni(widget.sede.id, _modifiche, chiave: _chiave);
      _chiave = widget.api.nuovaChiave(); // la prossima modifica è un'operazione nuova
      _mostra(imp);
      if (mounted) avviso(context, 'Impostazioni salvate');
    } catch (e) {
      if (e is ErroreServer && e.status != null) _chiave = widget.api.nuovaChiave();
      if (mounted) setState(() => _errore = e);
    } finally {
      if (mounted) setState(() => _attesa = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final imp = _attuali;
    final errore = erroreGenerale(context, _errore, campiMostrati: {for (final v in _voci) v.$1});
    return Sezione(
      'Parametri delle spedizioni · ${widget.sede.nome}',
      [
        if (_attesa) const LinearProgressIndicator(),
        if (_erroreLettura != null && imp == null)
          Row(children: [
            Expanded(child: Text(testoErrore(_erroreLettura!), style: TextStyle(color: Theme.of(context).colorScheme.error))),
            TextButton(onPressed: _carica, child: const Text('Riprova')),
          ])
        else if (imp != null) ...[
          if (imp.modificateIl != null)
            Padding(
              padding: const EdgeInsets.only(bottom: 12),
              child: Text(
                'Ultima modifica: ${fmtDataOra(imp.modificateIl!)}'
                '${imp.idAmministratoreModifica == null ? '' : ' · amministratore id ${imp.idAmministratoreModifica}'}',
                style: Theme.of(context).textTheme.bodySmall,
              ),
            ),
          Wrap(spacing: 16, runSpacing: 16, children: [
            for (final v in _voci) _campi.campo(v.$1, v.$2, _errore, suggerimento: v.$3, larghezza: 400, tastiera: TextInputType.number),
          ]),
          if (errore != null) Padding(padding: const EdgeInsets.only(top: 12), child: errore),
          const SizedBox(height: 16),
          FilledButton.icon(onPressed: _attesa ? null : _salva, icon: const Icon(Icons.save_outlined), label: const Text('Salva modifiche')),
        ],
      ],
      azione: IconButton(tooltip: 'Ricarica', onPressed: _attesa ? null : _carica, icon: const Icon(Icons.refresh)),
    );
  }
}

/// Motivi pubblici: si aggiungono e si attivano o disattivano. Non si cancellano né si
/// rinominano (le note già scritte li usano).
class MotiviCard extends StatefulWidget {
  final ApiAmministratore api;
  const MotiviCard({super.key, required this.api});
  @override
  State<MotiviCard> createState() => _MotiviCardState();
}

class _MotiviCardState extends State<MotiviCard> {
  final _campi = Campi(['descrizione']);
  List<MotivoGestito>? _motivi;
  Object? _erroreLettura, _errore;
  bool _attesa = true;
  late String _chiave = widget.api.nuovaChiave();

  @override
  void initState() {
    super.initState();
    _carica();
  }

  Future<void> _carica() async {
    setState(() {
      _attesa = true;
      _erroreLettura = null;
    });
    try {
      final m = await widget.api.motiviGestiti();
      if (mounted) setState(() => _motivi = m);
    } catch (e) {
      if (mounted) setState(() => _erroreLettura = e);
    } finally {
      if (mounted) setState(() => _attesa = false);
    }
  }

  Future<void> _aggiungi() async {
    setState(() {
      _attesa = true;
      _errore = null;
    });
    try {
      final m = await widget.api.creaMotivo(_campi['descrizione'], chiave: _chiave);
      _chiave = widget.api.nuovaChiave();
      _campi.c['descrizione']!.clear();
      if (mounted) avviso(context, 'Motivo "${m.descrizione}" aggiunto');
      await _carica();
    } catch (e) {
      if (e is ErroreServer && e.status != null) _chiave = widget.api.nuovaChiave();
      if (mounted) setState(() => _errore = e);
    } finally {
      if (mounted) setState(() => _attesa = false);
    }
  }

  Future<void> _attiva(MotivoGestito m, bool attivo) async {
    setState(() => _attesa = true);
    try {
      await widget.api.attivaMotivo(m.id, attivo);
      if (mounted) avviso(context, attivo ? 'Motivo "${m.descrizione}" riattivato' : 'Motivo "${m.descrizione}" disattivato');
      await _carica();
    } catch (e) {
      if (mounted) mostraErrore(context, e);
    } finally {
      if (mounted) setState(() => _attesa = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final motivi = _motivi;
    final errore = erroreGenerale(context, _errore, campiMostrati: const {'descrizione'});
    final attivi = motivi?.where((m) => m.attivo).length ?? 0;
    return Sezione(
      motivi == null ? 'Motivi' : 'Motivi ($attivi attivi su ${motivi.length})',
      [
        const Text('Il motivo pubblico è quello che il cliente vede quando una spedizione non va a buon fine. '
            'Un motivo disattivato non compare più nei menu e non è più accettato; le note che lo usano restano come sono.'),
        const SizedBox(height: 8),
        if (_attesa) const LinearProgressIndicator(),
        if (_erroreLettura != null && motivi == null)
          Row(children: [
            Expanded(child: Text(testoErrore(_erroreLettura!), style: TextStyle(color: Theme.of(context).colorScheme.error))),
            TextButton(onPressed: _carica, child: const Text('Riprova')),
          ])
        else if (motivi != null) ...[
          for (final m in motivi)
            SwitchListTile(
              contentPadding: EdgeInsets.zero,
              value: m.attivo,
              title: Text(m.descrizione,
                  style: TextStyle(
                      color: m.attivo ? null : Theme.of(context).colorScheme.outline,
                      decoration: m.attivo ? null : TextDecoration.lineThrough)),
              subtitle: Text([m.attivo ? 'attivo' : 'disattivato', if (m.creatoIl != null) 'creato il ${fmtDataOra(m.creatoIl!)}'].join(' · ')),
              onChanged: _attesa ? null : (v) => _attiva(m, v),
            ),
          const Divider(),
          Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Expanded(child: _campi.campo('descrizione', 'Nuovo motivo', _errore, larghezza: double.infinity)),
            const SizedBox(width: 8),
            Padding(
              padding: const EdgeInsets.only(top: 4),
              child: FilledButton.icon(onPressed: _attesa ? null : _aggiungi, icon: const Icon(Icons.add), label: const Text('Aggiungi')),
            ),
          ]),
          if (errore != null) Padding(padding: const EdgeInsets.only(top: 8), child: errore),
        ],
      ],
      azione: IconButton(tooltip: 'Ricarica', onPressed: _attesa ? null : _carica, icon: const Icon(Icons.refresh)),
    );
  }
}
