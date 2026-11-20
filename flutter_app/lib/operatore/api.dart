// Client HTTP dell'area operatori: alla sessione comune aggiunge le chiamate che il
// backend riserva all'operatore (creazione, assegnazione, reso, posizioni dei
// corrieri, clienti e indirizzi).
import '../comune/api.dart';

export '../comune/api.dart';

/// Cliente di GET /api/clienti: [id] è quello usato nelle spedizioni, [idUtente] quello
/// di PATCH /api/utenti/{id}. [destinatario]: ha ricevuto almeno una spedizione;
/// altrimenti è un mittente (anche se appena creato).
class ClienteAnagrafica {
  final int id, idUtente;
  final String nome, cognome, email, prefisso, nrTelefono;
  final bool attivo, destinatario;
  ClienteAnagrafica(this.id, this.idUtente, this.nome, this.cognome, this.email, this.prefisso, this.nrTelefono, this.attivo,
      [this.destinatario = false]);
  factory ClienteAnagrafica.daJson(Map<String, dynamic> j) => ClienteAnagrafica(
        j['id'] as int,
        j['id_utente'] as int? ?? 0,
        j['nome'] as String? ?? '',
        j['cognome'] as String? ?? '',
        j['email'] as String? ?? '',
        j['prefisso'] as String? ?? '',
        j['nr_telefono'] as String? ?? '',
        j['attivo'] as bool? ?? true,
        j['destinatario'] as bool? ?? false,
      );
  String get nomeCompleto => '$nome $cognome'.trim();

  /// Valori modificabili per nome del campo JSON.
  Map<String, String> get dati => {'nome': nome, 'cognome': cognome, 'prefisso': prefisso, 'nr_telefono': nrTelefono, 'email': email};
}

class ApiOperatore extends ApiSessione {
  ApiOperatore({required super.base, super.client});

  /// Continua la sessione aperta dal login unico.
  ApiOperatore.da(super.sessione) : super.da();

  /// Crea la spedizione. La risposta di POST /api/spedizioni è la spedizione "piatta"
  /// (solo id, senza persone e indirizzi): da qui servono id, codice e stato, il
  /// dettaglio completo si legge poi con [dettaglio]. La data richiesta la sceglie
  /// l'operatore; la data del ritiro la registra il sistema, quindi non si manda.
  Future<({int id, String codOrdine, String stato})> creaSpedizione({
    required String codOrdine,
    int? idMittente,
    int? idDestinatario,
    int? idRitiro,
    int? idConsegna,
    DateTime? richiestaIl,
    int? idCorriere,
    required String chiave,
  }) async {
    // I campi non scelti non vengono mandati: è il backend a dire quali mancano.
    final j = await chiama('POST', '/api/spedizioni', chiave: chiave, corpo: {
      'cod_ordine': codOrdine,
      if (idMittente != null) 'id_cliente_mittente': idMittente,
      if (idDestinatario != null) 'id_cliente_destinatario': idDestinatario,
      if (idRitiro != null) 'id_indirizzo_ritiro': idRitiro,
      if (idConsegna != null) 'id_indirizzo_spedizione': idConsegna,
      if (richiestaIl != null) 'dataora_richiesta_spedizione': richiestaIl.toUtc().toIso8601String(),
      if (idCorriere != null) 'id_corriere': idCorriere,
    }) as Map<String, dynamic>;
    return (id: j['id'] as int, codOrdine: j['cod_ordine'] as String, stato: j['stato'] as String);
  }

  Future<void> assegna(int id, int idCorriere) =>
      chiama('POST', '/api/spedizioni/$id/assegnazioni', chiave: nuovaChiave(), corpo: {'id_corriere': idCorriere});

  /// Segna (o toglie) il reso: PATCH /api/spedizioni/{id} con is_reso. In quali stati è
  /// ammesso (annullata, persa, non consegnata) lo decide il backend.
  Future<void> impostaReso(int id, bool reso) =>
      chiama('PATCH', '/api/spedizioni/$id', chiave: nuovaChiave(), corpo: {'is_reso': reso});

  Future<Posizioni> posizioni() async {
    final j = await chiama('GET', '/api/corrieri/posizioni') as Map<String, dynamic>;
    return Posizioni(j['intervallo_secondi'] as int? ?? 30,
        [for (final c in j['corrieri'] as List) PosizioneCorriere.daJson(c as Map<String, dynamic>)]);
  }

  // ───── anagrafiche ─────

  Future<Pagina<Persona>> clienti(String q) => elenco('/api/clienti', q, Persona.daJson);

  /// Anagrafica dei clienti, una pagina alla volta (sezione Clienti).
  Future<Pagina<ClienteAnagrafica>> anagraficaClienti(String q, {int pagina = 1, int perPagina = 20}) async {
    final j = await chiama('GET', '/api/clienti', query: {
      if (q.trim().isNotEmpty) 'q': q.trim(),
      'pagina': '$pagina',
      'per_pagina': '$perPagina',
    }) as Map<String, dynamic>;
    return Pagina.daJson(j, ClienteAnagrafica.daJson, pagina: pagina, perPagina: perPagina);
  }

  /// L'operatore cambia solo i dati del cliente (PATCH /api/utenti/{id_utente}): stato e
  /// password li gestisce l'amministratore, e il backend lo fa rispettare.
  Future<ClienteAnagrafica> modificaCliente(ClienteAnagrafica c, Map<String, Object?> modifiche, {required String chiave}) async {
    final u = await modificaUtente(c.idUtente, modifiche, chiave: chiave);
    return ClienteAnagrafica(c.id, c.idUtente, u['nome'] as String? ?? '', u['cognome'] as String? ?? '', u['email'] as String? ?? '',
        u['prefisso'] as String? ?? '', u['nr_telefono'] as String? ?? '', u['attivo'] as bool? ?? true,
        u['destinatario'] as bool? ?? c.destinatario);
  }
  Future<Pagina<Indirizzo>> indirizzi(String q) => elenco('/api/indirizzi', q, Indirizzo.daJson);

  /// Restituisce il cliente creato e se la mail con le credenziali è partita.
  Future<(Persona, bool)> creaCliente({
    required String nome,
    required String cognome,
    required String prefisso,
    required String telefono,
    required String email,
    required String password,
    required String chiave,
  }) async {
    final j = await chiama('POST', '/api/clienti', chiave: chiave, corpo: {
      'nome': nome,
      'cognome': cognome,
      'prefisso': prefisso,
      'nr_telefono': telefono,
      'email': email,
      'password': password,
    }) as Map<String, dynamic>;
    return (Persona.daJson(j['cliente'] as Map<String, dynamic>), j['mail_inviata'] as bool? ?? false);
  }

  Future<Indirizzo> creaIndirizzo({
    required String via,
    required String civico,
    required String citta,
    required String cap,
    required String regione,
    required String nazione,
    required String chiave,
  }) async =>
      Indirizzo.daJson(await chiama('POST', '/api/indirizzi', chiave: chiave, corpo: {
        'via': via,
        'civico': civico,
        'citta': citta,
        'cap': cap,
        'regione': regione,
        'nazione': nazione,
      }) as Map<String, dynamic>);
}
