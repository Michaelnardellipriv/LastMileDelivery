// Posizione dei corrieri con spedizioni in consegna, su mappa OpenStreetMap.
// Si aggiorna da sola all'intervallo impostato dall'amministratore.
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_map/flutter_map.dart';
import 'package:latlong2/latlong.dart';

import 'api.dart';
import 'comuni.dart';

class MappaPosizioni extends StatelessWidget {
  final List<PosizioneCorriere> posizioni;
  final void Function(PosizioneCorriere)? alTocco;
  const MappaPosizioni({super.key, required this.posizioni, this.alTocco});

  @override
  Widget build(BuildContext context) {
    final punti = [for (final p in posizioni) LatLng(p.latitudine, p.longitudine)];
    final opzioni = punti.length > 1
        ? MapOptions(initialCameraFit: CameraFit.coordinates(coordinates: punti, padding: const EdgeInsets.all(48), maxZoom: 15))
        : MapOptions(initialCenter: punti.isEmpty ? const LatLng(46.0679, 11.1211) : punti.first, initialZoom: 14);
    return ClipRRect(
      borderRadius: BorderRadius.circular(12),
      child: FlutterMap(
        options: opzioni,
        children: [
          TileLayer(
            urlTemplate: 'https://tile.openstreetmap.org/{z}/{x}/{y}.png',
            userAgentPackageName: 'it.veloce24.app',
          ),
          MarkerLayer(markers: [
            for (final p in posizioni)
              Marker(
                point: LatLng(p.latitudine, p.longitudine),
                width: 180,
                height: 64,
                child: GestureDetector(
                  onTap: alTocco == null ? null : () => alTocco!(p),
                  child: Column(mainAxisSize: MainAxisSize.min, children: [
                    Container(
                      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                      decoration: BoxDecoration(
                        color: Theme.of(context).colorScheme.surface,
                        borderRadius: BorderRadius.circular(6),
                        boxShadow: const [BoxShadow(blurRadius: 3, color: Colors.black26)],
                      ),
                      child: Text(p.nomeCompleto, style: const TextStyle(fontSize: 12), overflow: TextOverflow.ellipsis),
                    ),
                    const Icon(Icons.local_shipping, color: Colors.deepOrange, size: 30),
                  ]),
                ),
              ),
          ]),
          const RichAttributionWidget(attributions: [TextSourceAttribution('© OpenStreetMap contributors')]),
        ],
      ),
    );
  }
}

class MappaCorrieriPage extends StatefulWidget {
  final ApiOperatore api;
  final void Function(int idSpedizione) alApriSpedizione;
  const MappaCorrieriPage({super.key, required this.api, required this.alApriSpedizione});
  @override
  State<MappaCorrieriPage> createState() => _MappaCorrieriPageState();
}

class _MappaCorrieriPageState extends State<MappaCorrieriPage> {
  Posizioni? _posizioni;
  Object? _errore;
  DateTime? _aggiornata;
  Timer? _timer;
  int _versione = 0;

  @override
  void initState() {
    super.initState();
    _carica();
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
        _versione++;
      });
      _timer = Timer(Duration(seconds: p.intervalloSecondi.clamp(5, 3600)), _carica);
    } catch (e) {
      if (!mounted) return;
      setState(() => _errore = e);
      _timer = Timer(const Duration(seconds: 30), _carica);
    }
  }

  void _dettagliCorriere(PosizioneCorriere p) => showModalBottomSheet(
        context: context,
        builder: (ctx) => ListView(shrinkWrap: true, padding: const EdgeInsets.all(16), children: [
          Text(p.nomeCompleto, style: Theme.of(ctx).textTheme.titleLarge),
          Text('Ultima posizione: ${fmtDataOra(p.rilevataIl)}'),
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
                      '${_aggiornata == null ? '' : ' · ultimo ${fmtDataOra(_aggiornata!)}'}',
              style: Theme.of(context).textTheme.titleMedium,
            ),
          ),
          IconButton(tooltip: 'Aggiorna', onPressed: _carica, icon: const Icon(Icons.refresh)),
        ]),
        if (_errore != null) Text(testoErrore(_errore!), style: TextStyle(color: Theme.of(context).colorScheme.error)),
        const SizedBox(height: 8),
        Expanded(
          child: p == null
              ? const Center(child: CircularProgressIndicator())
              : p.corrieri.isEmpty
                  ? const Center(child: Text('Nessun corriere con spedizioni in consegna ha inviato la posizione'))
                  : LayoutBuilder(builder: (_, vincoli) {
                      final mappa = MappaPosizioni(key: ValueKey(_versione), posizioni: p.corrieri, alTocco: _dettagliCorriere);
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
