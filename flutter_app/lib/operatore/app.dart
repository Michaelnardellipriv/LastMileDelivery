// Area operatori (pagina web), aperta dopo il login unico di area_riservata.dart:
// spedizioni con filtri e dettaglio, nuova spedizione, mappa dei corrieri in consegna,
// anagrafica dei clienti (modifica dei dati).
import 'package:flutter/material.dart';

import '../comune/spedizioni.dart';
import 'api.dart';
import 'clienti.dart';
import 'dettaglio.dart';
import 'mappa.dart';
import 'nuova_spedizione.dart';

class HomeOperatore extends StatefulWidget {
  final ApiOperatore api;
  final VoidCallback alLogout;

  /// Apre il profilo e il cambio password (ProfilePage); assente = nessun pulsante.
  final VoidCallback? alProfilo;
  const HomeOperatore({super.key, required this.api, required this.alLogout, this.alProfilo});
  @override
  State<HomeOperatore> createState() => _HomeOperatoreState();
}

class _HomeOperatoreState extends State<HomeOperatore> {
  int _sezione = 0;
  final _spedizioni = GlobalKey<SpedizioniPageState>();

  void _apriSpedizione(int id) {
    setState(() => _sezione = 0);
    WidgetsBinding.instance.addPostFrameCallback((_) => _spedizioni.currentState?.apri(id, aggiornaElenco: true));
  }

  @override
  Widget build(BuildContext context) {
    final api = widget.api;
    final pagine = [
      SpedizioniPage(
        key: _spedizioni,
        api: api,
        dettaglio: (id, alCambio) => DettaglioSpedizione(api: api, id: id, alCambio: alCambio),
      ),
      NuovaSpedizionePage(api: api, alCreata: _apriSpedizione),
      // la mappa resta in memoria ma si aggiorna solo quando è la sezione a schermo
      MappaCorrieriPage(api: api, alApriSpedizione: _apriSpedizione, attiva: _sezione == 2),
      ClientiPage(api: api),
    ];
    const voci = [
      (Icons.local_shipping_outlined, 'Spedizioni'),
      (Icons.add_box_outlined, 'Nuova spedizione'),
      (Icons.map_outlined, 'Corrieri'),
      (Icons.people_outline, 'Clienti'),
    ];
    final largo = MediaQuery.sizeOf(context).width >= 900;
    final corpo = IndexedStack(index: _sezione, children: pagine);

    return Scaffold(
      appBar: AppBar(
        title: Text('Veloce24 · ${api.utente!.nomeCompleto}', style: const TextStyle(fontSize: 16)),
        actions: [
          if (widget.alProfilo != null) IconButton(tooltip: 'Il mio profilo', icon: const Icon(Icons.account_circle_outlined), onPressed: widget.alProfilo),
          IconButton(tooltip: 'Esci', icon: const Icon(Icons.logout), onPressed: widget.alLogout),
        ],
      ),
      body: largo
          ? Row(children: [
        NavigationRail(
          selectedIndex: _sezione,
          labelType: NavigationRailLabelType.all,
          onDestinationSelected: (i) => setState(() => _sezione = i),
          destinations: [for (final v in voci) NavigationRailDestination(icon: Icon(v.$1), label: Text(v.$2))],
        ),
        const VerticalDivider(width: 1),
        Expanded(child: corpo),
      ])
          : corpo,
      bottomNavigationBar: largo
          ? null
          : NavigationBar(
        selectedIndex: _sezione,
        onDestinationSelected: (i) => setState(() => _sezione = i),
        destinations: [for (final v in voci) NavigationDestination(icon: Icon(v.$1), label: v.$2)],
      ),
    );
  }
}
