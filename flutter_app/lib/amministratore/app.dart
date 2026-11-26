// Area amministratori (pagina web), aperta dopo il login unico di area_riservata.dart:
// le spedizioni di una sede alla volta (la sede si sceglie sempre) con dettaglio e
// storico in sola lettura, gestione di utenti e sedi, configurazione (parametri di ogni sede e
// motivi).
import 'package:flutter/material.dart';

import '../comune/spedizioni.dart';
import 'api.dart';
import 'configurazione.dart';
import 'sedi.dart';
import 'spedizioni.dart';
import 'utenti.dart';

class HomeAmministratore extends StatefulWidget {
  final ApiAmministratore api;
  final VoidCallback alLogout;

  /// Apre il profilo e il cambio password (ProfilePage); assente = nessun pulsante.
  final VoidCallback? alProfilo;
  const HomeAmministratore({super.key, required this.api, required this.alLogout, this.alProfilo});
  @override
  State<HomeAmministratore> createState() => _HomeAmministratoreState();
}

class _HomeAmministratoreState extends State<HomeAmministratore> {
  int _sezione = 0;

  /// Cresce a ogni modifica delle sedi: spedizioni, utenti e configurazione rileggono l'elenco.
  int _versioneSedi = 0;

  @override
  Widget build(BuildContext context) {
    final api = widget.api;
    final pagine = [
      SpedizioniPage(
          api: api,
          filtroSede: true,
          versioneSedi: _versioneSedi,
          dettaglio: (id, _) => DettaglioSolaLettura(api: api, id: id)),
      UtentiPage(api: api, versioneSedi: _versioneSedi),
      SediPage(api: api, alCambio: () => setState(() => _versioneSedi++)),
      ConfigurazionePage(api: api, versioneSedi: _versioneSedi),
    ];
    const voci = [
      (Icons.local_shipping_outlined, 'Spedizioni'),
      (Icons.manage_accounts_outlined, 'Utenti'),
      (Icons.store_mall_directory_outlined, 'Sedi'),
      (Icons.tune, 'Configurazione'),
    ];
    final largo = MediaQuery.sizeOf(context).width >= 900;
    final corpo = IndexedStack(index: _sezione, children: pagine);

    return Scaffold(
      appBar: AppBar(
        title: Text('Veloce24 · Amministrazione · ${api.utente!.nomeCompleto}', style: const TextStyle(fontSize: 16)),
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
