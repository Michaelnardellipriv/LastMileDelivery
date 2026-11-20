// Dettaglio di una spedizione per l'amministratore: gli stessi dati, note e storico
// dell'operatore, in sola lettura. Nessun pulsante per cambiare stato, assegnare, segnare
// il reso o scrivere note: queste operazioni il backend le riserva a operatori e corrieri.
import 'package:flutter/material.dart';

import '../comune/comuni.dart';
import '../comune/scheda_spedizione.dart';
import 'api.dart';

class DettaglioSolaLettura extends StatefulWidget {
  final ApiAmministratore api;
  final int id;
  const DettaglioSolaLettura({super.key, required this.api, required this.id});
  @override
  State<DettaglioSolaLettura> createState() => _DettaglioSolaLetturaState();
}

class _DettaglioSolaLetturaState extends State<DettaglioSolaLettura> {
  Spedizione? _s;
  List<VoceStorico> _storico = [];
  Object? _errore;
  bool _attesa = true;

  @override
  void initState() {
    super.initState();
    _carica();
  }

  Future<void> _carica() async {
    setState(() {
      _attesa = true;
      _errore = null;
    });
    try {
      final (s, storico) = await (widget.api.dettaglio(widget.id), widget.api.storico(widget.id)).wait;
      if (!mounted) return;
      setState(() {
        _s = s;
        _storico = storico;
      });
    } catch (e) {
      if (mounted) setState(() => _errore = e);
    } finally {
      if (mounted) setState(() => _attesa = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_attesa && _s == null) return const Center(child: CircularProgressIndicator());
    if (_errore != null && _s == null) {
      return Center(
        child: Column(mainAxisSize: MainAxisSize.min, children: [
          Padding(padding: const EdgeInsets.all(16), child: Text(testoErrore(_errore!), textAlign: TextAlign.center)),
          OutlinedButton(onPressed: _carica, child: const Text('Riprova')),
        ]),
      );
    }
    final s = _s!;
    final colori = Theme.of(context).colorScheme;
    return Stack(children: [
      ListView(padding: const EdgeInsets.all(16), children: [
        Row(children: [
          Expanded(child: SelectableText(s.codOrdine, style: Theme.of(context).textTheme.headlineSmall)),
          ChipStato(s.stato),
          IconButton(tooltip: 'Aggiorna', onPressed: _attesa ? null : _carica, icon: const Icon(Icons.refresh)),
        ]),
        const SizedBox(height: 8),
        Card(
          color: colori.secondaryContainer,
          margin: const EdgeInsets.only(bottom: 12),
          child: ListTile(
            leading: Icon(Icons.visibility_outlined, color: colori.onSecondaryContainer),
            title: Text('Sola lettura', style: TextStyle(color: colori.onSecondaryContainer)),
            subtitle: Text('Come amministratore consulti spedizioni, note e storico ma non li modifichi.',
                style: TextStyle(color: colori.onSecondaryContainer)),
          ),
        ),
        ...sezioniSpedizione(s),
        Sezione('Note', vociNote(s.note)),
        SezioneStorico(_storico),
      ]),
      if (_attesa) const Positioned(top: 0, left: 0, right: 0, child: LinearProgressIndicator()),
    ]);
  }
}
