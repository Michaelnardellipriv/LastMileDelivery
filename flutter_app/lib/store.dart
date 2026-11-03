// Modello, regole di business e client API (rispecchiano db/schema_LastMileDb.sql).
import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;

/// Avvio con: flutter run --dart-define=API_BASE=http://localhost:8080
/// (emulatore Android: http://10.0.2.2:8080). Vuoto = solo dati demo locali.
const apiBase = String.fromEnvironment('API_BASE');

enum Ruolo { cliente, corriere, operatore, admin }

const ruoloNome = {
  Ruolo.cliente: 'Cliente',
  Ruolo.corriere: 'Corriere',
  Ruolo.operatore: 'Operatore logistico',
  Ruolo.admin: 'Amministratore',
};
const tuttiStati = ['creata', 'assegnata', 'ritirata', 'in consegna', 'consegnata', 'non consegnata', 'persa', 'annullata'];
const _ops = {Ruolo.operatore, Ruolo.admin};
const _cou = {Ruolo.corriere, Ruolo.operatore, Ruolo.admin};
// Transizioni di stato: solo l'annullamento è imposto dal DB, le altre sono una proposta.
final transizioni = <String, Map<String, Set<Ruolo>>>{
  'creata': {'assegnata': _ops, 'annullata': _ops},
  'assegnata': {'ritirata': _cou, 'annullata': _ops},
  'ritirata': {'in consegna': _cou, 'persa': _ops},
  'in consegna': {'consegnata': _cou, 'non consegnata': _cou, 'persa': _ops},
  'non consegnata': {'assegnata': _ops},
};

DateTime ago(double giorni) => DateTime.now().subtract(Duration(minutes: (giorni * 1440).round()));
String fd(DateTime d) {
  String p(int n) => n.toString().padLeft(2, '0');
  return '${p(d.day)}/${p(d.month)}/${d.year} ${p(d.hour)}:${p(d.minute)}';
}

class Utente {
  final int id;
  final String nome, cognome;
  final Ruolo ruolo;
  Utente(this.id, this.nome, this.cognome, this.ruolo);
  String get nomeCompleto => '$nome $cognome';
  String get email => '$nome.$cognome@veloce24.test'.toLowerCase();
  String get tel => '+39 33300000$id';
}

class Indirizzo {
  final int id;
  final String via, civico, citta, cap;
  Indirizzo(this.id, this.via, this.civico, this.citta, this.cap);
  String get testo => '$via $civico, $cap $citta';
}

class Spedizione {
  final int id, idOpCreato, idMittente, idDestinatario, idRitiro, idConsegna;
  final String codOrdine;
  final bool isReso;
  final DateTime creataIl, richiestaIl;
  String stato;
  int? idAssegnazione, idOpAnnullato;
  DateTime? annullataIl;
  Spedizione({
    required this.id, required this.codOrdine, required this.isReso, required this.creataIl,
    required this.richiestaIl, required this.idOpCreato, required this.idMittente,
    required this.idDestinatario, required this.idRitiro, required this.idConsegna,
    this.stato = 'creata', this.idAssegnazione,
  });
}

class Assegnazione {
  final int id, idOperatore, idCorriere;
  Assegnazione(this.id, this.idOperatore, this.idCorriere);
}

class Nota {
  final int id, idSpedizione, idAutore;
  final String testo;
  final bool interna;
  final String? motivo;
  final DateTime il;
  Nota(this.id, this.idSpedizione, this.idAutore, this.testo, this.interna, this.motivo, this.il);
}

class Cambiamento {
  final String operazione; // creazione | modifica | annullamento
  final Map<String, dynamic>? vecchio;
  final Map<String, dynamic> nuovo;
  final DateTime il;
  final int idAutore, idSpedizione;
  Cambiamento(this.operazione, this.vecchio, this.nuovo, this.il, this.idAutore, this.idSpedizione);
}

class Tentativo {
  final int idSpedizione;
  final String tipo;
  final DateTime il;
  Tentativo(this.idSpedizione, this.tipo, this.il);
}

class Store extends ChangeNotifier {
  int _seq = 100;
  int? meId;
  final utenti = <Utente>[];
  final indirizzi = <Indirizzo>[];
  final sped = <Spedizione>[];
  final ass = <Assegnazione>[];
  final note = <Nota>[];
  final tent = <Tentativo>[];
  final camb = <Cambiamento>[];

  Store() {
    utenti.addAll([
      Utente(1, 'Giulia', 'Admin', Ruolo.admin), Utente(2, 'Olga', 'Operatore', Ruolo.operatore),
      Utente(3, 'Marco', 'Bianchi', Ruolo.corriere), Utente(4, 'Sara', 'Neri', Ruolo.corriere),
      Utente(5, 'Rossi', 'Store', Ruolo.cliente), Utente(6, 'Anna', 'Verdi', Ruolo.cliente),
      Utente(7, 'Bruno', 'Gallo', Ruolo.cliente),
    ]);
    indirizzi.addAll([
      Indirizzo(1, 'Via Brennero', '10', 'Trento', '38122'), Indirizzo(2, 'Via Verdi', '12', 'Milano', '20121'),
      Indirizzo(3, 'Corso Francia', '5', 'Torino', '10138'), Indirizzo(4, 'Via Roma', '3', 'Bolzano', '39100'),
      Indirizzo(5, 'Via Po', '9', 'Verona', '37121'),
    ]);
    void mk(int id, String cod, List<String> path, int m, int d, int r, int c, int? cour, double giorni, {bool reso = false}) {
      final s = Spedizione(id: id, codOrdine: cod, isReso: reso, creataIl: ago(giorni), richiestaIl: ago(giorni),
          idOpCreato: 2, idMittente: m, idDestinatario: d, idRitiro: r, idConsegna: c);
      sped.add(s);
      camb.add(Cambiamento('creazione', null, {'stato': 'creata'}, ago(giorni), 2, id));
      for (var i = 0; i < path.length; i++) {
        final st = path[i];
        if (st == 'assegnata' && cour != null) {
          final a = Assegnazione(_seq++, 2, cour);
          ass.add(a);
          s.idAssegnazione = a.id;
        }
        camb.add(Cambiamento(st == 'annullata' ? 'annullamento' : 'modifica', {'stato': s.stato}, {'stato': st},
            ago(giorni - (i + 1) * 0.2), st == 'assegnata' || st == 'annullata' ? 2 : (cour ?? 2), id));
        s.stato = st;
      }
      if (s.stato == 'annullata') s.annullataIl = ago(giorni - 1);
    }
    const run = ['assegnata', 'ritirata', 'in consegna'];
    mk(1, 'ORD-0001', [], 5, 6, 1, 2, null, 0);
    mk(2, 'ORD-0002', ['assegnata'], 5, 7, 1, 4, 4, 1);
    mk(3, 'ORD-0003', run, 5, 6, 1, 2, 3, 2);
    mk(4, 'ORD-0004', [...run, 'non consegnata'], 7, 6, 1, 5, 3, 4);
    mk(5, 'ORD-0005', [...run, 'consegnata'], 5, 7, 1, 3, 4, 9);
    mk(6, 'ORD-0006', [...run, 'consegnata'], 5, 6, 1, 3, 4, 45);
    mk(7, 'ORD-0007', ['annullata'], 6, 5, 2, 1, null, 3, reso: true);
    note.add(Nota(_seq++, 4, 3, 'Citofono guasto, destinatario irraggiungibile.', true, 'destinatario assente', ago(3.5)));
    tent.add(Tentativo(4, 'consegna', ago(3.5)));
  }

  Utente? get me => utenti.where((u) => u.id == meId).firstOrNull;
  Utente? utente(int? id) => utenti.where((u) => u.id == id).firstOrNull;
  String nome(int? id) => utente(id)?.nomeCompleto ?? '—';
  String indirizzo(int id) => indirizzi.where((a) => a.id == id).firstOrNull?.testo ?? '—';
  Spedizione? byId(int id) => sped.where((s) => s.id == id).firstOrNull;
  int? corriereDi(Spedizione s) => ass.where((a) => a.id == s.idAssegnazione).firstOrNull?.idCorriere;

  void login(int? id) { meId = id; notifyListeners(); }

  List<Spedizione> visibili(Utente u) => sped.where((s) {
        if (u.ruolo == Ruolo.corriere) return corriereDi(s) == u.id;
        if (u.ruolo != Ruolo.cliente) return true;
        if (s.idMittente != u.id && s.idDestinatario != u.id) return false;
        final fine = camb.where((c) => c.idSpedizione == s.id && c.nuovo['stato'] == 'consegnata').lastOrNull;
        return fine == null || DateTime.now().difference(fine.il).inDays < 30; // regola dei 30 giorni
      }).toList()
        ..sort((a, b) => b.id.compareTo(a.id));

  List<String> prossimiStati(Spedizione s, Utente u) {
    if (u.ruolo == Ruolo.corriere && corriereDi(s) != u.id) return [];
    return (transizioni[s.stato] ?? {}).entries.where((e) => e.value.contains(u.ruolo)).map((e) => e.key).toList();
  }

  /// Ritorna un messaggio di errore oppure null se riuscito.
  String? cambiaStato(int id, String a, int? corriere, String testo, bool interna) {
    final u = me!, s = byId(id);
    if (s == null) return 'Spedizione inesistente';
    if (!prossimiStati(s, u).contains(a)) return 'Passaggio non consentito: ${s.stato} → $a';
    if (a == 'assegnata' && corriere == null) return 'Seleziona un corriere';
    if (a == 'non consegnata' && testo.trim().isEmpty) return 'La nota è obbligatoria se la consegna non riesce';
    if (testo.trim().isNotEmpty) {
      note.add(Nota(_seq++, id, u.id, testo.trim(), interna, a == 'non consegnata' ? 'consegna non riuscita' : null, ago(0)));
    }
    if (a == 'non consegnata') tent.add(Tentativo(id, 'consegna', ago(0)));
    final vecchio = {'stato': s.stato}, nuovo = <String, dynamic>{'stato': a};
    if (a == 'assegnata') {
      final x = Assegnazione(_seq++, u.id, corriere!);
      ass.add(x);
      s.idAssegnazione = x.id;
      nuovo['id_assegnazione_corrente'] = x.id;
    }
    if (a == 'annullata') {
      s.annullataIl = ago(0);
      s.idOpAnnullato = u.id;
    }
    s.stato = a;
    camb.add(Cambiamento(a == 'annullata' ? 'annullamento' : 'modifica', vecchio, nuovo, ago(0), u.id, id));
    notifyListeners();
    return null;
  }

  void aggiungiNota(int id, String testo, bool interna) {
    note.add(Nota(_seq++, id, me!.id, testo.trim(), interna, null, ago(0)));
    notifyListeners();
  }

  /// Valida come il backend Go; se API_BASE è impostato chiama POST /api/spedizioni.
  Future<({int? id, String? errore})> creaSpedizione({
    required String cod, required bool reso, required int mittente, required int destinatario,
    required DateTime richiesta, required int ritiro, required int consegna,
  }) async {
    if (cod.trim().isEmpty) return (id: null, errore: 'cod_ordine è obbligatorio');
    if (cod.length > 50) return (id: null, errore: 'cod_ordine può contenere al massimo 50 caratteri');
    if (sped.any((s) => s.codOrdine == cod)) return (id: null, errore: 'cod_ordine già presente (409)');
    var id = _seq++;
    if (apiBase.isNotEmpty) {
      try {
        final r = await http
            .post(Uri.parse('$apiBase/api/spedizioni'),
                headers: {'Content-Type': 'application/json'},
                body: jsonEncode({
                  'cod_ordine': cod, 'is_reso': reso, 'id_cliente_mittente': mittente,
                  'id_cliente_destinatario': destinatario,
                  'dataora_richiesta_spedizione': richiesta.toUtc().toIso8601String(),
                  'id_indirizzo_ritiro': ritiro, 'id_indirizzo_spedizione': consegna,
                }))
            .timeout(const Duration(seconds: 5));
        if (r.statusCode != 201) return (id: null, errore: '${r.body.trim()} (HTTP ${r.statusCode})');
        id = (jsonDecode(r.body) as Map)['id'] as int;
      } catch (_) {
        return (id: null, errore: 'Backend non raggiungibile');
      }
    }
    sped.add(Spedizione(id: id, codOrdine: cod, isReso: reso, creataIl: ago(0), richiestaIl: richiesta,
        idOpCreato: me!.id, idMittente: mittente, idDestinatario: destinatario, idRitiro: ritiro, idConsegna: consegna));
    camb.add(Cambiamento('creazione', null, {'stato': 'creata', 'cod_ordine': cod}, ago(0), me!.id, id));
    notifyListeners();
    return (id: id, errore: null);
  }
}

final store = Store();
