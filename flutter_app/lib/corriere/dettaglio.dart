// Dettaglio di una spedizione per il corriere: dati, mittente e destinatario, tentativi
// falliti, note. Da qui il corriere cambia lo stato, registra un tentativo non riuscito
// (nota pubblica: motivo per il cliente + nota interna) e aggiunge note interne.
// La pagina non valida nulla: manda ciò che il corriere inserisce e mostra gli errori
// restituiti dal backend.
import 'package:flutter/material.dart';

import '../comune/comuni.dart';
import 'api.dart';

class DettaglioCorrierePage extends StatefulWidget {
  final ApiCorriere api;
  final int id;
  final String codOrdine;
  const DettaglioCorrierePage({super.key, required this.api, required this.id, required this.codOrdine});
  @override
  State<DettaglioCorrierePage> createState() => _DettaglioCorrierePageState();
}

class _DettaglioCorrierePageState extends State<DettaglioCorrierePage> {
  Spedizione? _s;
  Object? _errore;
  Object? _erroreNota;
  bool _attesa = true;
  bool _invio = false;
  final _nota = TextEditingController();

  ApiCorriere get api => widget.api;

  @override
  void initState() {
    super.initState();
    _carica();
  }

  @override
  void dispose() {
    _nota.dispose();
    super.dispose();
  }

  Future<void> _carica() async {
    setState(() {
      _attesa = true;
      _errore = null;
    });
    try {
      final s = await api.dettaglio(widget.id);
      if (mounted) setState(() => _s = s);
    } catch (e) {
      if (!mounted) return;
      setState(() => _errore = e);
      // Se i dati di prima restano a schermo, l'errore dell'aggiornamento va comunque detto.
      if (_s != null) mostraErrore(context, e);
    } finally {
      if (mounted) setState(() => _attesa = false);
    }
  }

  /// Dopo un'azione si rilegge il dettaglio dal backend. Se il backend risponde che la
  /// spedizione non c'è più per il corriere (404: per esempio dopo la consegna) si torna
  /// all'elenco con l'esito; la pagina non decide da sola quali stati sono visibili.
  Future<void> _dopoAzione(String fatto) async {
    if (!mounted) return;
    setState(() => _attesa = true);
    try {
      final s = await api.dettaglio(widget.id);
      if (!mounted) return;
      setState(() => _s = s);
      avviso(context, fatto);
    } catch (e) {
      if (!mounted) return;
      if (e is ErroreServer && e.status == 404) {
        Navigator.of(context).pop('${widget.codOrdine}: $fatto');
      } else {
        avviso(context, fatto);
        mostraErrore(context, e);
      }
    } finally {
      if (mounted) setState(() => _attesa = false);
    }
  }

  Future<void> _cambiaStato() async {
    final stato = await showDialog<String>(
      context: context,
      builder: (_) => _DialogoStato(api: api, idSpedizione: widget.id, statoAttuale: _s!.stato),
    );
    if (stato != null) await _dopoAzione('Stato aggiornato: $stato');
  }

  Future<void> _tentativo() async {
    final esito = await showDialog<EsitoSpedizione>(
      context: context,
      builder: (_) => _DialogoTentativo(api: api, idSpedizione: widget.id),
    );
    if (esito == null) return;
    await _dopoAzione('Tentativo registrato: stato "${esito.stato}", '
        'ritiri falliti ${esito.tentativiRitiro}, consegne fallite ${esito.tentativiConsegna}');
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
  Widget build(BuildContext context) => Scaffold(
        appBar: AppBar(
          title: const Text('Spedizione'),
          actions: [IconButton(tooltip: 'Aggiorna', onPressed: _attesa ? null : _carica, icon: const Icon(Icons.refresh))],
        ),
        body: SafeArea(
          child: Align(
            alignment: Alignment.topCenter,
            child: ConstrainedBox(constraints: const BoxConstraints(maxWidth: 640), child: _corpo()),
          ),
        ),
      );

  Widget _corpo() {
    final s = _s;
    if (s == null) {
      if (_attesa) return const Center(child: CircularProgressIndicator());
      return Center(
        child: Column(mainAxisSize: MainAxisSize.min, children: [
          Padding(padding: const EdgeInsets.all(16), child: Text(testoErrore(_errore!), textAlign: TextAlign.center)),
          OutlinedButton(onPressed: _carica, child: const Text('Riprova')),
        ]),
      );
    }
    return Stack(children: [
      RefreshIndicator(
        onRefresh: _carica,
        child: ListView(padding: const EdgeInsets.all(12), children: [
          Row(children: [
            Expanded(child: SelectableText(s.codOrdine, style: Theme.of(context).textTheme.headlineSmall)),
            ChipStato(s.stato),
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
                onPressed: _invio ? null : _tentativo,
                icon: const Icon(Icons.report_problem_outlined),
                label: const Text('Tentativo non riuscito'),
              ),
            ]),
          ),
          Sezione('Spedizione', [
            Riga('Richiesta il', fmtDataOra(s.richiestaIl)),
            Riga('Creata il', fmtDataOra(s.creataIl)),
            Riga('Ritirata il', s.ritirataIl == null ? 'non ancora ritirata' : fmtDataOra(s.ritirataIl!)),
            Riga('Ritiri falliti', '${s.tentativiRitiro ?? 0}'),
            Riga('Consegne fallite', '${s.tentativiConsegna ?? 0}'),
          ]),
          Sezione('Ritiro', [
            Riga('Mittente', s.mittente?.nomeCompleto),
            Riga('Telefono', s.mittente?.telefono),
            Riga('Email', s.mittente?.email),
            Riga('Indirizzo', s.ritiro.testo),
          ]),
          Sezione('Consegna', [
            Riga('Destinatario', s.destinatario.nomeCompleto),
            Riga('Telefono', s.destinatario.telefono),
            Riga('Email', s.destinatario.email),
            Riga('Indirizzo', s.consegna.testo),
          ]),
          Sezione('Assegnata da', [
            if (s.assegnataDa == null) const Text('—'),
            if (s.assegnataDa != null) ...[
              Riga('Operatore', nomeConId(s.assegnataDa)),
              Riga('Email', s.assegnataDa!.email),
              Riga('Telefono', s.assegnataDa!.telefono),
            ],
          ]),
          _sezioneNote(s),
        ]),
      ),
      if (_attesa || _invio) const Positioned(top: 0, left: 0, right: 0, child: LinearProgressIndicator()),
    ]);
  }

  Widget _sezioneNote(Spedizione s) => Sezione('Note', [
        if (s.note.isEmpty) const Text('Nessuna nota'),
        for (final n in s.note)
          ListTile(
            contentPadding: EdgeInsets.zero,
            leading: Icon(n.interna ? Icons.lock_outline : Icons.public, color: n.interna ? null : Colors.teal),
            title: Text(n.testo),
            subtitle: Text([
              n.interna ? 'interna' : 'pubblica · il cliente vede il motivo "${n.motivo ?? ''}"',
              fmtDataOra(n.creataIl),
              if (n.autore != null) n.autore!.testo,
            ].join(' · ')),
          ),
        const Divider(),
        TextField(
          controller: _nota,
          minLines: 1,
          maxLines: 4,
          decoration: InputDecoration(
            labelText: 'Nuova nota interna',
            helperText: 'La nota pubblica (motivo per il cliente) si scrive registrando un tentativo non riuscito',
            helperMaxLines: 2,
            errorText: erroreCampo(_erroreNota, 'nota'),
            errorMaxLines: 3,
            border: const OutlineInputBorder(),
          ),
        ),
        const SizedBox(height: 8),
        Align(
          alignment: Alignment.centerRight,
          child: FilledButton.tonal(onPressed: _invio ? null : _aggiungiNota, child: const Text('Aggiungi nota interna')),
        ),
      ]);
}

/// Cambio di stato: l'elenco mostra tutti gli stati, senza filtri. Se il passaggio non è
/// ammesso (per il ruolo o per questa spedizione) il backend risponde con l'errore.
class _DialogoStato extends StatefulWidget {
  final ApiCorriere api;
  final int idSpedizione;
  final String statoAttuale;
  const _DialogoStato({required this.api, required this.idSpedizione, required this.statoAttuale});
  @override
  State<_DialogoStato> createState() => _DialogoStatoState();
}

class _DialogoStatoState extends State<_DialogoStato> {
  String? _stato;
  Object? _errore;
  bool _invio = false;

  Future<void> _invia() async {
    setState(() {
      _invio = true;
      _errore = null;
    });
    try {
      final nuovo = await widget.api.cambiaStato(widget.idSpedizione, _stato ?? '');
      if (mounted) Navigator.of(context).pop(nuovo);
    } catch (e) {
      if (mounted) setState(() => _errore = e);
    } finally {
      if (mounted) setState(() => _invio = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final err = _errore;
    final colori = Theme.of(context).colorScheme;
    return AlertDialog(
      title: Text('Cambia stato (ora: ${widget.statoAttuale})'),
      content: SingleChildScrollView(
        child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          for (final s in statiSpedizione)
            RadioListTile<String>(
              contentPadding: EdgeInsets.zero,
              dense: true,
              value: s,
              groupValue: _stato,
              title: Text(s),
              onChanged: _invio ? null : (v) => setState(() => _stato = v),
            ),
          if (err != null)
            Padding(
              padding: const EdgeInsets.only(top: 8),
              child: Text(erroreCampo(err, 'stato') ?? testoErrore(err), style: TextStyle(color: colori.error)),
            ),
        ]),
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

/// Tentativo di ritiro o di consegna non riuscito. Motivo (pubblico, lo vede il cliente)
/// e nota (interna) formano la nota pubblica della spedizione; il backend aumenta il
/// numero dei tentativi falliti.
class _DialogoTentativo extends StatefulWidget {
  final ApiCorriere api;
  final int idSpedizione;
  const _DialogoTentativo({required this.api, required this.idSpedizione});
  @override
  State<_DialogoTentativo> createState() => _DialogoTentativoState();
}

class _DialogoTentativoState extends State<_DialogoTentativo> {
  String? _tipo;
  String? _motivo;
  final _nota = TextEditingController();
  List<String>? _motivi; // null finché non arrivano
  Object? _erroreMotivi;
  Object? _errore;
  bool _invio = false;
  // Una chiave per ogni tentativo che il corriere vuole registrare: un doppio tocco o
  // un nuovo invio dopo un errore di rete non lo registrano due volte.
  late String _chiave = widget.api.nuovaChiave();

  @override
  void initState() {
    super.initState();
    _caricaMotivi();
  }

  @override
  void dispose() {
    _nota.dispose();
    super.dispose();
  }

  void _caricaMotivi() {
    setState(() => _erroreMotivi = null);
    widget.api.motivi().then((m) {
      if (mounted) setState(() => _motivi = m);
    }, onError: (Object e) {
      if (mounted) setState(() => _erroreMotivi = e);
    });
  }

  /// Ogni modifica dei dati è una richiesta nuova, quindi una chiave nuova.
  void _cambiato() => _chiave = widget.api.nuovaChiave();

  Future<void> _invia() async {
    setState(() {
      _invio = true;
      _errore = null;
    });
    try {
      final esito = await widget.api.registraTentativo(
        widget.idSpedizione,
        // i campi partono come sono a schermo, anche vuoti: li controlla il backend
        tipo: _tipo ?? '',
        motivo: _motivo ?? '',
        nota: _nota.text,
        chiave: _chiave,
      );
      if (mounted) Navigator.of(context).pop(esito);
    } catch (e) {
      // Il backend ha risposto con un errore: non ha registrato nulla, quindi il prossimo
      // invio (con i dati corretti) è una richiesta nuova.
      if (e is ErroreServer && e.status != null) _cambiato();
      if (mounted) setState(() => _errore = e);
    } finally {
      if (mounted) setState(() => _invio = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final err = _errore;
    final colori = Theme.of(context).colorScheme;
    final senzaCampi = err != null && (err is! ErroreServer || err.campi.isEmpty);
    return AlertDialog(
      title: const Text('Tentativo non riuscito'),
      content: SizedBox(
        width: 480,
        child: SingleChildScrollView(
          child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.stretch, children: [
            DropdownButtonFormField<String>(
              value: _tipo,
              isExpanded: true,
              decoration: InputDecoration(
                labelText: 'Tentativo di',
                errorText: erroreCampo(err, 'tipo'),
                errorMaxLines: 3,
                border: const OutlineInputBorder(),
              ),
              items: [for (final t in tipiTentativo) DropdownMenuItem(value: t, child: Text(t))],
              onChanged: (t) => setState(() {
                _tipo = t;
                _cambiato();
              }),
            ),
            const SizedBox(height: 12),
            if (_erroreMotivi != null)
              Row(children: [
                Expanded(
                  child: Text('Elenco dei motivi non disponibile. ${testoErrore(_erroreMotivi!)}', style: TextStyle(color: colori.error)),
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
                  labelText: 'Motivo pubblico',
                  helperText: 'Lo vede il cliente',
                  errorText: erroreCampo(err, 'motivo'),
                  errorMaxLines: 3,
                  border: const OutlineInputBorder(),
                ),
                items: [
                  const DropdownMenuItem(value: null, child: Text('Nessun motivo')),
                  // tutti i motivi, senza filtri: quale è ammesso lo decide il backend
                  for (final m in _motivi!) DropdownMenuItem(value: m, child: Text(m, overflow: TextOverflow.ellipsis)),
                ],
                onChanged: (m) => setState(() {
                  _motivo = m;
                  _cambiato();
                }),
              ),
            const SizedBox(height: 12),
            TextField(
              controller: _nota,
              minLines: 2,
              maxLines: 5,
              onChanged: (_) => _cambiato(),
              decoration: InputDecoration(
                labelText: 'Nota interna',
                helperText: 'Il cliente non la vede',
                errorText: erroreCampo(err, 'nota'),
                errorMaxLines: 3,
                border: const OutlineInputBorder(),
              ),
            ),
            if (senzaCampi)
              Padding(
                padding: const EdgeInsets.only(top: 12),
                child: Text(testoErrore(err), style: TextStyle(color: colori.error)),
              ),
          ]),
        ),
      ),
      actions: [
        TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Annulla')),
        FilledButton(
          onPressed: _invio ? null : _invia,
          child: _invio ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2)) : const Text('Registra'),
        ),
      ],
    );
  }
}
