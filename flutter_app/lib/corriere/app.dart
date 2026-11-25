// Area corrieri (pagina pensata per il telefono), aperta dopo il login unico di
// area_riservata.dart: spedizioni assegnate al corriere, con ricerca, dettaglio,
// cambio di stato, tentativi non riusciti e note. Finché è aperta, manda al backend la
// posizione simulata del corriere.
import 'package:flutter/material.dart';

import 'api.dart';
import 'posizione.dart';
import 'spedizioni.dart';

class HomeCorriere extends StatefulWidget {
  final ApiCorriere api;
  final VoidCallback alLogout;

  /// Apre il profilo e il cambio password (ProfilePage); assente = nessun pulsante.
  final VoidCallback? alProfilo;
  const HomeCorriere({super.key, required this.api, required this.alLogout, this.alProfilo});
  @override
  State<HomeCorriere> createState() => _HomeCorriereState();
}

class _HomeCorriereState extends State<HomeCorriere> {
  late final InvioPosizione _posizione = InvioPosizione(widget.api);

  @override
  void initState() {
    super.initState();
    _posizione.avvia();
  }

  @override
  void dispose() {
    _posizione.dispose(); // al logout il telefono smette di mandare la posizione
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => Scaffold(
    appBar: AppBar(
      title: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        const Text('Le mie spedizioni', style: TextStyle(fontSize: 18)),
        Text(widget.api.utente!.nomeCompleto, style: Theme.of(context).textTheme.bodySmall),
      ]),
      actions: [
        if (widget.alProfilo != null) IconButton(tooltip: 'Il mio profilo', icon: const Icon(Icons.account_circle_outlined), onPressed: widget.alProfilo),
        IconButton(tooltip: 'Esci', icon: const Icon(Icons.logout), onPressed: widget.alLogout),
      ],
    ),
    body: SafeArea(
      child: Align(
        alignment: Alignment.topCenter,
        // Su schermi larghi (tablet, browser) la colonna resta leggibile.
        child: ConstrainedBox(constraints: const BoxConstraints(maxWidth: 640), child: SpedizioniCorrierePage(api: widget.api)),
      ),
    ),
    bottomNavigationBar: BarraPosizione(invio: _posizione),
  );
}
