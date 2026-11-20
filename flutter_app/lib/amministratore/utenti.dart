// Gestione degli utenti: corrieri, operatori e clienti, con ricerca e filtri per ruolo e
// stato. Corrieri e operatori si creano qui (con il ruolo, che poi non si cambia), i
// clienti li crea l'operatore. Per tutti: modifica dei dati, nuova password,
// disattivazione e riattivazione; di un cliente solo mittente (non accede all'app) il
// backend accetta solo i dati. La pagina non valida nulla: manda ciò che è scritto e
// mostra gli errori del backend.
import 'package:flutter/material.dart';

import '../comune/comuni.dart';
import '../comune/ricerca.dart';
import 'api.dart';

class UtentiPage extends StatefulWidget {
  final ApiAmministratore api;
  const UtentiPage({super.key, required this.api});
  @override
  State<UtentiPage> createState() => _UtentiPageState();
}

class _UtentiPageState extends State<UtentiPage> {
  final _q = TextEditingController();
  String? _ruolo;
  bool? _attivo;
  int _pagina = 1;
  Pagina<UtenteGestito>? _risultato;
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
      final r = await api.utenti(FiltroUtenti(q: _q.text, ruolo: _ruolo, attivo: _attivo), pagina: pagina);
      if (!mounted) return;
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

  Future<void> _nuovo() async {
    final creato = await showDialog<UtenteGestito>(context: context, builder: (_) => DialogoUtente(api: api));
    if (creato != null) _carica(pagina: _pagina);
  }

  Future<void> _modifica(UtenteGestito u) async {
    final fatto = await showDialog<UtenteGestito>(context: context, builder: (_) => DialogoUtente(api: api, utente: u));
    if (fatto == null || !mounted) return;
    avviso(context, 'Dati di ${fatto.nomeCompleto} aggiornati');
    _carica(pagina: _pagina);
  }

  Future<void> _cambiaAttivo(UtenteGestito u) async {
    final attiva = !u.attivo;
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(attiva ? 'Riattivare ${u.nomeCompleto}?' : 'Disattivare ${u.nomeCompleto}?'),
        content: Text(attiva
            ? 'Potrà di nuovo accedere${u.ruolo == 'operatore' ? '' : ' e ricevere spedizioni'}.'
            : 'Non potrà più accedere e le sessioni aperte smettono di funzionare${switch (u.ruolo) {
                'corriere' => '; non gli si potranno assegnare spedizioni (quelle che ha restano sue finché un operatore non le riassegna)',
                'cliente' => '; non potrà essere destinatario di nuove spedizioni (quelle che ha restano sue)',
                _ => '',
              }}.'),
        actions: [
          TextButton(onPressed: () => Navigator.of(ctx).pop(false), child: const Text('Annulla')),
          FilledButton(onPressed: () => Navigator.of(ctx).pop(true), child: Text(attiva ? 'Riattiva' : 'Disattiva')),
        ],
      ),
    );
    if (ok != true || !mounted) return;
    setState(() => _attesa = true);
    try {
      await api.modificaUtenteGestito(u.id, {'attivo': attiva});
      if (!mounted) return;
      avviso(context, attiva ? '${u.nomeCompleto} riattivato' : '${u.nomeCompleto} disattivato');
      await _carica(pagina: _pagina);
    } catch (e) {
      if (mounted) mostraErrore(context, e);
    } finally {
      if (mounted) setState(() => _attesa = false);
    }
  }

  Widget _filtri() => Padding(
        padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
        child: Wrap(spacing: 8, runSpacing: 8, crossAxisAlignment: WrapCrossAlignment.center, children: [
          SizedBox(
            width: 240,
            child: TextField(
              controller: _q,
              decoration: const InputDecoration(
                  labelText: 'Cerca nome, email o telefono', isDense: true, border: OutlineInputBorder(), prefixIcon: Icon(Icons.search)),
              onSubmitted: (_) => _carica(),
            ),
          ),
          SizedBox(
            width: 190,
            child: DropdownButtonFormField<String?>(
              value: _ruolo,
              isDense: true,
              isExpanded: true,
              decoration: const InputDecoration(labelText: 'Ruolo', isDense: true, border: OutlineInputBorder()),
              items: [
                const DropdownMenuItem(value: null, child: Text('Tutti')),
                for (final r in etichetteRuoli.entries) DropdownMenuItem(value: r.key, child: Text(r.value)),
              ],
              onChanged: (r) {
                setState(() => _ruolo = r);
                _carica();
              },
            ),
          ),
          SizedBox(
            width: 150,
            child: DropdownButtonFormField<bool?>(
              value: _attivo,
              isDense: true,
              isExpanded: true,
              decoration: const InputDecoration(labelText: 'Stato', isDense: true, border: OutlineInputBorder()),
              items: const [
                DropdownMenuItem(value: null, child: Text('Tutti')),
                DropdownMenuItem(value: true, child: Text('Attivi')),
                DropdownMenuItem(value: false, child: Text('Disattivati')),
              ],
              onChanged: (a) {
                setState(() => _attivo = a);
                _carica();
              },
            ),
          ),
          FilledButton.tonalIcon(onPressed: _attesa ? null : () => _carica(), icon: const Icon(Icons.search), label: const Text('Cerca')),
          FilledButton.icon(onPressed: _nuovo, icon: const Icon(Icons.person_add_alt_1), label: const Text('Nuovo utente')),
        ]),
      );

  Widget _voce(UtenteGestito u) {
    final colori = Theme.of(context).colorScheme;
    return ListTile(
      leading: CircleAvatar(
        backgroundColor: u.attivo ? colori.primaryContainer : colori.surfaceContainerHighest,
        child: Icon(switch (u.ruolo) {
          'corriere' => Icons.local_shipping_outlined,
          'operatore' => Icons.support_agent,
          _ => Icons.person_outline,
        }, color: u.attivo ? colori.onPrimaryContainer : colori.outline),
      ),
      title: Wrap(spacing: 8, crossAxisAlignment: WrapCrossAlignment.center, children: [
        Text(u.nomeCompleto, style: TextStyle(fontWeight: FontWeight.w600, color: u.attivo ? null : colori.outline)),
        Text(u.descrizioneRuolo, style: Theme.of(context).textTheme.bodySmall),
        if (!u.attivo)
          Chip(
            label: const Text('disattivato', style: TextStyle(fontSize: 11)),
            visualDensity: VisualDensity.compact,
            side: BorderSide.none,
            backgroundColor: colori.errorContainer,
          ),
      ]),
      subtitle: Text('${u.email} · ${u.telefono}\nid utente ${u.id} · id ${u.ruolo} ${u.idRuolo}'),
      isThreeLine: true,
      trailing: Row(mainAxisSize: MainAxisSize.min, children: [
        IconButton(tooltip: 'Modifica dati', onPressed: _attesa ? null : () => _modifica(u), icon: const Icon(Icons.edit_outlined)),
        TextButton(
          onPressed: _attesa ? null : () => _cambiaAttivo(u),
          child: Text(u.attivo ? 'Disattiva' : 'Riattiva'),
        ),
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
                    ? const Center(child: Text('Nessun utente con questi filtri'))
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

/// Creazione (senza [utente]) o modifica di un corriere o di un operatore.
/// In modifica partono solo i campi cambiati; la password solo se si sceglie di
/// reimpostarla. Il ruolo si sceglie solo alla creazione.
class DialogoUtente extends StatefulWidget {
  final ApiAmministratore api;
  final UtenteGestito? utente;
  const DialogoUtente({super.key, required this.api, this.utente});
  @override
  State<DialogoUtente> createState() => _DialogoUtenteState();
}

class _DialogoUtenteState extends State<DialogoUtente> {
  static const _nomiCampi = ['nome', 'cognome', 'prefisso', 'nr_telefono', 'email', 'password'];
  final _campi = Campi(_nomiCampi);
  String? _ruolo;
  bool _nuovaPassword = false;
  late String _chiave = widget.api.nuovaChiave();
  Object? _errore;
  bool _attesa = false;

  bool get _creazione => widget.utente == null;

  @override
  void initState() {
    super.initState();
    final u = widget.utente;
    if (u == null) {
      _campi.c['prefisso']!.text = '+39';
    } else {
      for (final e in u.dati.entries) {
        _campi.c[e.key]!.text = e.value;
      }
    }
  }

  /// Corpo della richiesta: tutto in creazione, solo i campi cambiati in modifica.
  Map<String, Object?> get _corpo {
    final u = widget.utente;
    if (u == null) {
      return {
        if (_ruolo != null) 'ruolo': _ruolo,
        for (final n in _nomiCampi) n: _campi[n],
      };
    }
    return {
      for (final e in u.dati.entries)
        if (_campi[e.key] != e.value) e.key: _campi[e.key],
      if (_nuovaPassword) 'password': _campi['password'],
    };
  }

  Future<void> _salva() async {
    setState(() {
      _attesa = true;
      _errore = null;
    });
    try {
      final UtenteGestito risultato;
      if (_creazione) {
        final (creato, mailInviata) = await widget.api.creaUtente(_corpo, chiave: _chiave);
        if (!mounted) return;
        avviso(
            context,
            mailInviata
                ? '${ruoliGestiti[creato.ruolo]} creato: le credenziali sono state inviate a ${creato.email}.'
                : '${ruoliGestiti[creato.ruolo]} creato, ma la mail con le credenziali NON è partita: comunicagliele tu.');
        risultato = creato;
      } else {
        risultato = await widget.api.modificaUtenteGestito(widget.utente!.id, _corpo, chiave: _chiave);
      }
      if (mounted) Navigator.of(context).pop(risultato);
    } catch (e) {
      // Il backend ha rifiutato la richiesta: non è cambiato nulla, il prossimo invio
      // (con i dati corretti) è una richiesta nuova e usa una chiave nuova.
      if (e is ErroreServer && e.status != null) _chiave = widget.api.nuovaChiave();
      if (mounted) setState(() => _errore = e);
    } finally {
      if (mounted) setState(() => _attesa = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final u = widget.utente;
    final mostrati = {..._nomiCampi.where((n) => n != 'password' || _creazione || _nuovaPassword), if (_creazione) 'ruolo'};
    final errore = erroreGenerale(context, _errore, campiMostrati: mostrati);
    return AlertDialog(
      title: Text(_creazione ? 'Nuovo corriere o operatore' : 'Modifica ${u!.nomeCompleto}'),
      content: SizedBox(
        width: 520,
        child: SingleChildScrollView(
          child: Wrap(spacing: 12, runSpacing: 12, children: [
            if (_creazione)
              SizedBox(
                width: 492,
                child: DropdownButtonFormField<String>(
                  value: _ruolo,
                  decoration: InputDecoration(
                    labelText: 'Ruolo',
                    helperText: 'Il ruolo non si potrà cambiare dopo la creazione',
                    errorText: erroreCampo(_errore, 'ruolo'),
                    border: const OutlineInputBorder(),
                  ),
                  items: [for (final r in ruoliGestiti.entries) DropdownMenuItem(value: r.key, child: Text(r.value))],
                  onChanged: (r) => setState(() => _ruolo = r),
                ),
              )
            else
              SizedBox(
                width: 492,
                child: InputDecorator(
                  decoration: const InputDecoration(labelText: 'Ruolo', helperText: 'Il ruolo non si cambia', border: OutlineInputBorder()),
                  child: Text('${u!.descrizioneRuolo} · id utente ${u.id} · id ${u.ruolo} ${u.idRuolo}'),
                ),
              ),
            _campi.campo('nome', 'Nome', _errore),
            _campi.campo('cognome', 'Cognome', _errore),
            // nessun filtro su ciò che si scrive: prefisso e telefono li controlla il backend
            _campi.campo('prefisso', 'Prefisso', _errore, larghezza: 90, tastiera: TextInputType.phone),
            _campi.campo('nr_telefono', 'Telefono', _errore, larghezza: 390, tastiera: TextInputType.phone),
            _campi.campo('email', 'Email', _errore, larghezza: 492),
            if (!_creazione)
              SizedBox(
                width: 492,
                child: SwitchListTile(
                  contentPadding: EdgeInsets.zero,
                  value: _nuovaPassword,
                  title: const Text('Reimposta la password'),
                  subtitle: const Text('La nuova password viene inviata all\'utente per mail'),
                  onChanged: (v) => setState(() => _nuovaPassword = v),
                ),
              ),
            if (_creazione || _nuovaPassword)
              _campi.campo('password', _creazione ? 'Password iniziale' : 'Nuova password', _errore,
                  password: true, larghezza: 492, suggerimento: 'Verrà inviata all\'utente per mail'),
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
              : Text(_creazione ? 'Crea utente' : 'Salva modifiche'),
        ),
      ],
    );
  }
}
