// Instradamento post-login: sceglie la schermata in base a `utente.ruolo` di POST /api/auth/login.
// I quattro valori sono quelli del backend (identita.Ruolo): operatore, corriere, cliente, amministratore.
import 'package:material_ui/material_ui.dart';
import '../api.dart';
import '../backend_page.dart';
import 'profile_page.dart';
import '../store.dart';

class HomePerRuolo extends StatefulWidget {
  final Sessione sessione;
  const HomePerRuolo({super.key, required this.sessione});

  @override
  State<HomePerRuolo> createState() => _HomePerRuoloState();
}

class _HomePerRuoloState extends State<HomePerRuolo> {
  // Un solo client per sessione: porta `Authorization: Bearer <JWT>` su ogni richiesta,
  // comprese quelle dell'area personale (profilo e cambio password).
  late final ApiClient _api = ApiClient.daSessione(apiBase, widget.sessione, onSessioneScaduta: _sessioneScaduta);

  /// Il backend ha risposto 401 al token (scaduto o revocato): si torna al login.
  void _sessioneScaduta() {
    if (store.sessione == null) return; // già uscito (più richieste in volo possono fallire insieme)
    // `store.esci()` sostituisce la home con il login, ma le schermate già impilate sopra
    // (area personale, dettagli) resterebbero: prima si chiudono.
    if (mounted) Navigator.of(context).popUntil((r) => r.isFirst);
    store.esci(avviso: 'Sessione scaduta: accedi di nuovo.');
  }

  void _esci() {
    Navigator.of(context).popUntil((r) => r.isFirst);
    store.esci();
  }

  /// Area personale: riusa lo stesso client (quindi lo stesso JWT) della home.
  void _apriProfilo() => Navigator.of(context).push(MaterialPageRoute<void>(
        builder: (_) => ProfilePage.backend(api: _api, sessione: widget.sessione, onLogout: _esci),
      ));

  Widget get _elenco => BackendElencoPage(api: _api, onLogout: _esci, onProfilo: _apriProfilo);

  @override
  Widget build(BuildContext context) {
    // Oggi esiste una sola vista collegata al backend (elenco spedizioni, già filtrato dal
    // backend per ruolo). Ogni ruolo ha il suo ramo: quando ci saranno le schermate dedicate
    // basta sostituire il widget del ramo, senza toccare login né store.
    return switch (widget.sessione.ruolo) {
      // ── ROUTING per ruolo: `RuoloApi` ha esattamente i quattro valori del backend, lo switch è esaustivo ──
      RuoloApi.operatore => _elenco, // → dashboard operatore
      RuoloApi.corriere => _elenco, // → pannello corriere
      RuoloApi.cliente => _elenco, // → area cliente
      RuoloApi.amministratore => _elenco, // → pannello admin
    };
  }
}
