// Creazione di una spedizione: mittente e destinatario, indirizzi di ritiro e
// consegna (cercati o creati al momento), data del ritiro e, se si vuole, il
// corriere a cui assegnarla subito.
import 'package:flutter/material.dart';

import 'api.dart';
import 'comuni.dart';
import 'selettori.dart';

class NuovaSpedizionePage extends StatefulWidget {
  final ApiOperatore api;
  final void Function(int id) alCreata;
  const NuovaSpedizionePage({super.key, required this.api, required this.alCreata});
  @override
  State<NuovaSpedizionePage> createState() => _NuovaSpedizionePageState();
}

class _NuovaSpedizionePageState extends State<NuovaSpedizionePage> {
  final _codOrdine = TextEditingController();
  Persona? _mittente, _destinatario, _corriere;
  Indirizzo? _ritiro, _consegna;
  DateTime _richiesta = DateTime.now().add(const Duration(days: 1)).copyWith(hour: 9, minute: 0, second: 0, millisecond: 0, microsecond: 0);
  late String _chiave = widget.api.nuovaChiave();
  Object? _errore;
  bool _attesa = false;

  void _cambiato() => _chiave = widget.api.nuovaChiave();

  Future<void> _scegliData() async {
    final d = await showDatePicker(
      context: context,
      initialDate: _richiesta,
      firstDate: DateTime(2000),
      lastDate: DateTime(2100),
      helpText: 'Giorno del ritiro',
    );
    if (d == null || !mounted) return;
    final o = await showTimePicker(context: context, initialTime: TimeOfDay.fromDateTime(_richiesta), helpText: 'Ora del ritiro');
    if (o == null) return;
    setState(() {
      _richiesta = DateTime(d.year, d.month, d.day, o.hour, o.minute);
      _cambiato();
    });
  }

  /// Manda quello che è stato inserito: i campi mancanti o non validi li segnala il backend.
  Future<void> _crea() async {
    setState(() {
      _attesa = true;
      _errore = null;
    });
    try {
      final s = await widget.api.creaSpedizione(
        codOrdine: _codOrdine.text,
        idMittente: _mittente?.id,
        idDestinatario: _destinatario?.id,
        idRitiro: _ritiro?.id,
        idConsegna: _consegna?.id,
        richiestaIl: _richiesta,
        idCorriere: _corriere?.id,
        chiave: _chiave,
      );
      if (!mounted) return;
      avviso(context, 'Spedizione ${s.codOrdine} creata (${s.stato})');
      setState(() {
        _codOrdine.clear();
        _mittente = _destinatario = _corriere = null;
        _ritiro = _consegna = null;
        _cambiato();
      });
      widget.alCreata(s.id);
    } catch (e) {
      if (mounted) setState(() => _errore = e);
    } finally {
      if (mounted) setState(() => _attesa = false);
    }
  }

  Widget _scelta({
    required String etichetta,
    required IconData icona,
    required String? valore,
    String? dettaglio,
    required Future<void> Function() scegli,
    VoidCallback? togli,
    String? errore,
  }) =>
      Card(
        margin: const EdgeInsets.only(bottom: 12),
        child: ListTile(
          leading: Icon(icona),
          title: Text(valore ?? etichetta, style: valore == null ? TextStyle(color: Theme.of(context).colorScheme.outline) : null),
          subtitle: Text([
            if (valore != null) etichetta,
            if (dettaglio != null && dettaglio.isNotEmpty) dettaglio,
            if (errore != null) errore,
          ].join('\n')),
          isThreeLine: dettaglio != null && dettaglio.isNotEmpty,
          trailing: Wrap(spacing: 4, children: [
            if (togli != null && valore != null) IconButton(tooltip: 'Togli', onPressed: togli, icon: const Icon(Icons.close)),
            OutlinedButton(onPressed: _attesa ? null : scegli, child: Text(valore == null ? 'Scegli' : 'Cambia')),
          ]),
        ),
      );

  @override
  Widget build(BuildContext context) {
    final api = widget.api;
    final err = _errore;
    return Align(
      alignment: Alignment.topCenter,
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 820),
        child: ListView(padding: const EdgeInsets.all(16), children: [
          Text('Nuova spedizione', style: Theme.of(context).textTheme.headlineSmall),
          const SizedBox(height: 16),
          TextField(
            controller: _codOrdine,
            onChanged: (_) => _cambiato(),
            decoration: InputDecoration(
              labelText: 'Riferimento ordine *',
              helperText: 'Codice dell\'ordine del negozio: lettere, cifre e . _ / - senza spazi',
              errorText: erroreCampo(err, 'cod_ordine'),
              border: const OutlineInputBorder(),
            ),
          ),
          const SizedBox(height: 16),
          _scelta(
            etichetta: 'Mittente *',
            icona: Icons.storefront_outlined,
            valore: _mittente?.nomeCompleto,
            dettaglio: _mittente == null ? null : descriviPersona(_mittente),
            errore: erroreCampo(err, 'id_cliente_mittente'),
            scegli: () async {
              final c = await scegliCliente(context, api, 'Mittente');
              if (c != null)
                setState(() {
                  _mittente = c;
                  _chiave = api.nuovaChiave();
                });
            },
          ),
          _scelta(
            etichetta: 'Indirizzo di ritiro *',
            icona: Icons.outbox_outlined,
            valore: _ritiro?.riga1,
            dettaglio: _ritiro?.riga2,
            errore: erroreCampo(err, 'id_indirizzo_ritiro'),
            scegli: () async {
              final i = await scegliIndirizzo(context, api, 'Indirizzo di ritiro');
              if (i != null)
                setState(() {
                  _ritiro = i;
                  _chiave = api.nuovaChiave();
                });
            },
          ),
          _scelta(
            etichetta: 'Destinatario *',
            icona: Icons.person_outline,
            valore: _destinatario?.nomeCompleto,
            dettaglio: _destinatario == null ? null : descriviPersona(_destinatario),
            errore: erroreCampo(err, 'id_cliente_destinatario'),
            scegli: () async {
              final c = await scegliCliente(context, api, 'Destinatario');
              if (c != null)
                setState(() {
                  _destinatario = c;
                  _chiave = api.nuovaChiave();
                });
            },
          ),
          _scelta(
            etichetta: 'Indirizzo di consegna *',
            icona: Icons.move_to_inbox_outlined,
            valore: _consegna?.riga1,
            dettaglio: _consegna?.riga2,
            errore: erroreCampo(err, 'id_indirizzo_spedizione'),
            scegli: () async {
              final i = await scegliIndirizzo(context, api, 'Indirizzo di consegna');
              if (i != null)
                setState(() {
                  _consegna = i;
                  _chiave = api.nuovaChiave();
                });
            },
          ),
          _scelta(
            etichetta: 'Data e ora del ritiro *',
            icona: Icons.schedule,
            valore: fmtDataOra(_richiesta),
            errore: erroreCampo(err, 'dataora_richiesta_spedizione'),
            scegli: _scegliData,
          ),
          _scelta(
            etichetta: 'Corriere (facoltativo: se lo scegli la spedizione nasce già assegnata)',
            icona: Icons.local_shipping_outlined,
            valore: _corriere?.nomeCompleto,
            dettaglio: _corriere == null ? null : descriviPersona(_corriere),
            errore: erroreCampo(err, 'id_corriere'),
            togli: () => setState(() {
              _corriere = null;
              _chiave = api.nuovaChiave();
            }),
            scegli: () async {
              final c = await scegliCorriere(context, api);
              if (c != null)
                setState(() {
                  _corriere = c;
                  _chiave = api.nuovaChiave();
                });
            },
          ),
          if (err != null)
            Padding(
              padding: const EdgeInsets.only(bottom: 12),
              child: Text(testoErrore(err), style: TextStyle(color: Theme.of(context).colorScheme.error)),
            ),
          Align(
            alignment: Alignment.centerRight,
            child: FilledButton.icon(
              onPressed: _attesa ? null : _crea,
              icon: _attesa
                  ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
                  : const Icon(Icons.check),
              label: const Text('Crea spedizione'),
            ),
          ),
        ]),
      ),
    );
  }
}
