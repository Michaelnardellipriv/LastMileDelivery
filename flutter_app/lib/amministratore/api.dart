// Client HTTP dell'area amministratori, secondo backend/api/openapi.yaml: alla sessione
// comune (spedizioni, dettaglio, storico, corrieri: solo lettura) aggiunge le
// impostazioni, i motivi e la gestione degli utenti (corrieri, operatori e clienti).
//
// Nessun controllo sui valori: partono come l'amministratore li scrive e li valida il
// backend, che risponde con l'errore per campo.
import '../comune/api.dart';

export '../comune/api.dart';

DateTime? _data(Object? v) => v == null ? null : DateTime.parse(v as String).toLocal();

/// Parametri della gestione delle spedizioni (GET /api/impostazioni).
class Impostazioni {
  final int giorniVisibilitaCliente, maxTentativiRitiro, maxTentativiConsegna, intervalloPosizioneSec;
  final DateTime? modificateIl;
  final int? idAmministratoreModifica;
  Impostazioni(this.giorniVisibilitaCliente, this.maxTentativiRitiro, this.maxTentativiConsegna, this.intervalloPosizioneSec,
      this.modificateIl, this.idAmministratoreModifica);
  factory Impostazioni.daJson(Map<String, dynamic> j) => Impostazioni(
        j['giorni_visibilita_cliente'] as int,
        j['max_tentativi_ritiro'] as int,
        j['max_tentativi_consegna'] as int,
        j['intervallo_posizione_sec'] as int,
        _data(j['modificate_il']),
        j['id_amministratore_modifica'] as int?,
      );

  /// Valori per nome del campo JSON, nello stesso ordine della pagina.
  Map<String, int> get valori => {
        'giorni_visibilita_cliente': giorniVisibilitaCliente,
        'max_tentativi_ritiro': maxTentativiRitiro,
        'max_tentativi_consegna': maxTentativiConsegna,
        'intervallo_posizione_sec': intervalloPosizioneSec,
      };
}

/// Motivo pubblico con il suo stato (GET /api/impostazioni/motivi).
class MotivoGestito {
  final int id;
  final String descrizione;
  final bool attivo;
  final DateTime? creatoIl;
  MotivoGestito(this.id, this.descrizione, this.attivo, this.creatoIl);
  factory MotivoGestito.daJson(Map<String, dynamic> j) =>
      MotivoGestito(j['id'] as int, j['descrizione'] as String? ?? '', j['attivo'] as bool? ?? true, _data(j['creato_il']));
}

/// Corriere, operatore o cliente (GET /api/utenti). [id] è l'id dell'utente, da usare
/// in /api/utenti/{id}; [idRuolo] quello del ruolo, usato nelle spedizioni.
/// [destinatario] c'è solo per i clienti: true se hanno ricevuto almeno una spedizione
/// (accedono all'app), false se sono solo mittenti.
class UtenteGestito {
  final int id, idRuolo;
  final String ruolo, nome, cognome, email, prefisso, nrTelefono;
  final bool attivo;
  final bool? destinatario;
  UtenteGestito(this.id, this.idRuolo, this.ruolo, this.nome, this.cognome, this.email, this.prefisso, this.nrTelefono, this.attivo,
      [this.destinatario]);
  factory UtenteGestito.daJson(Map<String, dynamic> j) => UtenteGestito(
        j['id'] as int,
        j['id_ruolo'] as int? ?? 0,
        j['ruolo'] as String? ?? '',
        j['nome'] as String? ?? '',
        j['cognome'] as String? ?? '',
        j['email'] as String? ?? '',
        j['prefisso'] as String? ?? '',
        j['nr_telefono'] as String? ?? '',
        j['attivo'] as bool? ?? true,
        j['destinatario'] as bool?,
      );
  String get nomeCompleto => '$nome $cognome'.trim();
  String get telefono => '$prefisso $nrTelefono'.trim();

  /// "Corriere", "Operatore logistico", "Cliente destinatario", "Cliente mittente"
  /// (mittente: non ha mai ricevuto spedizioni, anche se appena creato).
  String get descrizioneRuolo => ruolo == 'cliente'
      ? (destinatario == true ? 'Cliente destinatario' : 'Cliente mittente')
      : etichetteRuoli[ruolo] ?? ruolo;

  /// Valori modificabili per nome del campo JSON.
  Map<String, String> get dati => {'nome': nome, 'cognome': cognome, 'prefisso': prefisso, 'nr_telefono': nrTelefono, 'email': email};
}

/// Ruoli che l'amministratore crea (i clienti li crea l'operatore).
const ruoliGestiti = {'corriere': 'Corriere', 'operatore': 'Operatore logistico'};

/// Ruoli dell'elenco utenti, con l'etichetta da mostrare.
const etichetteRuoli = {...ruoliGestiti, 'cliente': 'Cliente'};

class FiltroUtenti {
  final String q;
  final String? ruolo;
  final bool? attivo;
  const FiltroUtenti({this.q = '', this.ruolo, this.attivo});
}

class ApiAmministratore extends ApiSessione {
  ApiAmministratore({required super.base, super.client});

  /// Continua la sessione aperta dal login unico.
  ApiAmministratore.da(super.sessione) : super.da();

  // ───── impostazioni ─────

  Future<Impostazioni> impostazioni() async =>
      Impostazioni.daJson(await chiama('GET', '/api/impostazioni') as Map<String, dynamic>);

  /// PATCH /api/impostazioni con i soli campi in [modifiche] (anche nessuno: lo rifiuta il backend).
  Future<Impostazioni> modificaImpostazioni(Map<String, Object?> modifiche, {required String chiave}) async =>
      Impostazioni.daJson(await chiama('PATCH', '/api/impostazioni', chiave: chiave, corpo: modifiche) as Map<String, dynamic>);

  // ───── motivi ─────

  /// Tutti i motivi, anche quelli disattivati.
  Future<List<MotivoGestito>> motiviGestiti() async {
    final j = await chiama('GET', '/api/impostazioni/motivi') as Map<String, dynamic>;
    return [for (final m in j['dati'] as List) MotivoGestito.daJson(m as Map<String, dynamic>)];
  }

  Future<MotivoGestito> creaMotivo(String descrizione, {required String chiave}) async => MotivoGestito.daJson(
      await chiama('POST', '/api/impostazioni/motivi', chiave: chiave, corpo: {'descrizione': descrizione}) as Map<String, dynamic>);

  /// Un motivo non si cancella né si rinomina: si attiva o disattiva.
  Future<MotivoGestito> attivaMotivo(int id, bool attivo) async => MotivoGestito.daJson(
      await chiama('PATCH', '/api/impostazioni/motivi/$id', chiave: nuovaChiave(), corpo: {'attivo': attivo}) as Map<String, dynamic>);

  // ───── corrieri e operatori ─────

  static const perPaginaUtenti = 20;

  Future<Pagina<UtenteGestito>> utenti(FiltroUtenti f, {int pagina = 1, int perPagina = perPaginaUtenti}) async {
    final j = await chiama('GET', '/api/utenti', query: {
      if (f.q.trim().isNotEmpty) 'q': f.q.trim(), // come le altre ricerche
      if (f.ruolo != null) 'ruolo': f.ruolo!,
      if (f.attivo != null) 'attivo': '${f.attivo}',
      'pagina': '$pagina',
      'per_pagina': '$perPagina',
    }) as Map<String, dynamic>;
    return Pagina.daJson(j, UtenteGestito.daJson, pagina: pagina, perPagina: perPagina);
  }

  /// Crea un corriere o un operatore; restituisce l'utente e se la mail con le credenziali è partita.
  Future<(UtenteGestito, bool)> creaUtente(Map<String, Object?> dati, {required String chiave}) async {
    final j = await chiama('POST', '/api/utenti', chiave: chiave, corpo: dati) as Map<String, dynamic>;
    return (UtenteGestito.daJson(j['utente'] as Map<String, dynamic>), j['mail_inviata'] as bool? ?? false);
  }

  /// PATCH /api/utenti/{id} con i soli campi in [modifiche]: dati, password o attivo.
  Future<UtenteGestito> modificaUtenteGestito(int id, Map<String, Object?> modifiche, {String? chiave}) async =>
      UtenteGestito.daJson(await modificaUtente(id, modifiche, chiave: chiave));
}
