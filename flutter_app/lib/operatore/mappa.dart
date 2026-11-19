// Posizione dei corrieri con spedizioni in consegna, su mappa OpenStreetMap.
// Si aggiorna da sola all'intervallo impostato dall'amministratore.
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_map/flutter_map.dart';
import 'package:latlong2/latlong.dart';

import 'api.dart';
import '../comune/comuni.dart';

/// Mappa OpenStreetMap con i corrieri. Resta la stessa per tutta la vita della pagina:
/// quando arrivano posizioni nuove si spostano solo i marcatori, senza ricreare la
/// mappa né riscaricare le tessere.
class MappaPosizioni extends StatefulWidget {
  final List<PosizioneCorriere> posizioni;
  final void Function(PosizioneCorriere)? alTocco;

  /// Con un solo corriere (dettaglio della spedizione) la mappa lo segue quando si sposta.
  final bool segui;
  const MappaPosizioni({super.key, required this.posizioni, this.alTocco, this.segui = false});

  @override
  State<MappaPosizioni> createState() => _MappaPosizioniState();
}

class _MappaPosizioniState extends State<MappaPosizioni> {
  final _controllo = MapController();
  bool _pronta = false;

  // inquadratura iniziale: calcolata una volta sola, poi la mappa la gestisce l'utente
  late final MapOptions _opzioni = () {
    final punti = _punti(widget.posizioni);
    return punti.length > 1
        ? MapOptions(
            initialCameraFit: CameraFit.coordinates(coordinates: punti, padding: const EdgeInsets.all(48), maxZoom: 15),
            onMapReady: () => _pronta = true,
          )
        : MapOptions(
            initialCenter: punti.isEmpty ? const LatLng(46.0679, 11.1211) : punti.first,
            initialZoom: 14,
            onMapReady: () => _pronta = true,
          );
  }();

  static List<LatLng> _punti(List<PosizioneCorriere> p) => [for (final c in p) LatLng(c.latitudine, c.longitudine)];

  @override
  void didUpdateWidget(MappaPosizioni vecchia) {
    super.didUpdateWidget(vecchia);
    if (!widget.segui || !_pronta || widget.posizioni.isEmpty) return;
    final punto = _punti(widget.posizioni).first;
    if (vecchia.posizioni.isEmpty || _punti(vecchia.posizioni).first != punto) {
      _controllo.move(punto, _controllo.camera.zoom);
    }
  }

  @override
  void dispose() {
    _controllo.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => ClipRRect(
        borderRadius: BorderRadius.circular(12),
        child: FlutterMap(
          mapController: _controllo,
          options: _opzioni,
          children: [
            TileLayer(
              urlTemplate: 'https://tile.openstreetmap.org/{z}/{x}/{y}.png',
              userAgentPackageName: 'it.veloce24.app',
            ),
            MarkerLayer(markers: [
              for (final p in widget.posizioni)
                Marker(
                  key: ValueKey(p.idCorriere),
                  point: LatLng(p.latitudine, p.longitudine),
                  width: 180,
                  height: 64,
                  child: GestureDetector(
                    onTap: widget.alTocco == null ? null : () => widget.alTocco!(p),
                    child: _Furgone(nome: p.nomeCompleto),
                  ),
                ),
            ]),
            const RichAttributionWidget(attributions: [TextSourceAttribution('© OpenStreetMap contributors')]),
          ],
        ),
      );
}

class _Furgone extends StatelessWidget {
  final String nome;
  const _Furgone({required this.nome});

  @override
  Widget build(BuildContext context) => Column(mainAxisSize: MainAxisSize.min, children: [
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
          decoration: BoxDecoration(
            color: Theme.of(context).colorScheme.surface,
            borderRadius: BorderRadius.circular(6),
            boxShadow: const [BoxShadow(blurRadius: 3, color: Colors.black26)],
          ),
          child: Text(nome, style: const TextStyle(fontSize: 12), overflow: TextOverflow.ellipsis),
        ),
        const Icon(Icons.local_shipping, color: Colors.deepOrange, size: 30),
      ]);
}

class MappaCorrieriPage extends StatefulWidget {
  final ApiOperatore api;
  final void Function(int idSpedizione) alApriSpedizione;

  /// Se la sezione è a schermo: solo allora la pagina interroga il backend.
  final bool attiva;
  const MappaCorrieriPage({super.key, required this.api, required this.alApriSpedizione, this.attiva = true});
  @override
  State<MappaCorrieriPage> createState() => _MappaCorrieriPageState();
}

class _MappaCorrieriPageState extends State<MappaCorrieriPage> {
  Posizioni? _posizioni;
  Object? _errore;
  DateTime? _aggiornata;
  Timer? _timer;

  @override
  void initState() {
    super.initState();
    if (widget.attiva) _carica();
  }

  @override
  void didUpdateWidget(MappaCorrieriPage vecchia) {
    super.didUpdateWidget(vecchia);
    if (widget.attiva && !vecchia.attiva) _carica(); // tornati sulla sezione: dati freschi subito
    if (!widget.attiva) _timer?.cancel(); // sezione nascosta: niente richieste né ridisegni
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  Future<void> _carica() async {
    _timer?.cancel();
    try {
      final p = await widget.api.posizioni();
      if (!mounted) return;
      setState(() {
        _posizioni = p;
        _errore = null;
        _aggiornata = DateTime.now();
      });
      if (widget.attiva) _timer = Timer(Duration(seconds: p.intervalloSecondi.clamp(5, 3600)), _carica);
    } catch (e) {
      if (!mounted) return;
      setState(() => _errore = e);
      if (widget.attiva) _timer = Timer(const Duration(seconds: 30), _carica);
    }
  }

  void _dettagliCorriere(PosizioneCorriere p) => showModalBottomSheet(
        context: context,
        builder: (ctx) => ListView(shrinkWrap: true, padding: const EdgeInsets.all(16), children: [
          Text(p.nomeCompleto, style: Theme.of(ctx).textTheme.titleLarge),
          Text('Ultima posizione: ${fmtData(p.rilevataIl)} ${fmtOra(p.rilevataIl)}'),
          const SizedBox(height: 8),
          const Text('Spedizioni in consegna:'),
          for (final id in p.spedizioni)
            ListTile(
              leading: const Icon(Icons.inventory_2_outlined),
              title: Text('Spedizione $id'),
              trailing: const Icon(Icons.chevron_right),
              onTap: () {
                Navigator.of(ctx).pop();
                widget.alApriSpedizione(id);
              },
            ),
        ]),
      );

  @override
  Widget build(BuildContext context) {
    final p = _posizioni;
    return Padding(
      padding: const EdgeInsets.all(16),
      child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        Row(children: [
          Expanded(
            child: Text(
              p == null
                  ? 'Posizione dei corrieri in consegna'
                  : '${p.corrieri.length} corrieri in consegna · aggiornamento ogni ${p.intervalloSecondi} s'
                      '${_aggiornata == null ? '' : ' · ultimo alle ${fmtOra(_aggiornata!)}'}',
              style: Theme.of(context).textTheme.titleMedium,
            ),
          ),
          IconButton(tooltip: 'Aggiorna', onPressed: _carica, icon: const Icon(Icons.refresh)),
        ]),
        if (_errore != null) Text(testoErrore(_errore!), style: TextStyle(color: Theme.of(context).colorScheme.error)),
        const SizedBox(height: 8),
        Expanded(
          child: p == null
              // la rotellina gira solo a sezione visibile: nascosta non deve consumare nulla
              ? (widget.attiva ? const Center(child: CircularProgressIndicator()) : const SizedBox())
              : p.corrieri.isEmpty
                  ? const Center(child: Text('Nessun corriere con spedizioni in consegna ha inviato la posizione'))
                  : LayoutBuilder(builder: (_, vincoli) {
                      final mappa = MappaPosizioni(posizioni: p.corrieri, alTocco: _dettagliCorriere);
                      final elenco = ListView(children: [
                        for (final c in p.corrieri)
                          ListTile(
                            leading: const Icon(Icons.local_shipping_outlined),
                            title: Text(c.nomeCompleto),
                            subtitle: Text('${c.spedizioni.length} in consegna · ${fmtDataOra(c.rilevataIl)}'),
                            onTap: () => _dettagliCorriere(c),
                          ),
                      ]);
                      return vincoli.maxWidth >= 900
                          ? Row(children: [Expanded(child: mappa), SizedBox(width: 300, child: elenco)])
                          : Column(children: [Expanded(flex: 3, child: mappa), Expanded(flex: 2, child: elenco)]);
                    }),
        ),
      ]),
    );
  }
}
