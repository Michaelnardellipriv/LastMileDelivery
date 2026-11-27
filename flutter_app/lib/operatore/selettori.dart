// Finestre per cercare e scegliere clienti e indirizzi, con la creazione di clienti e
// indirizzi nuovi. La scelta del corriere è in comune/ricerca.dart.
import 'package:flutter/material.dart';

import 'api.dart';
import '../comune/comuni.dart';
import '../comune/ricerca.dart';

export '../comune/ricerca.dart' show scegliCorriere;

Future<Persona?> scegliCliente(BuildContext context, ApiOperatore api, String titolo) => showDialog<Persona>(
      context: context,
      builder: (_) => DialogoRicerca<Persona>(
        titolo: titolo,
        cerca: api.clienti,
        titoloVoce: (c) => c.nomeCompleto,
        sottotitoloVoce: (c) => [c.email, c.telefono].whereType<String>().join(' · '),
        etichettaNuovo: 'Nuovo cliente',
        creaNuovo: (ctx) => showDialog<Persona>(context: ctx, builder: (_) => _NuovoCliente(api: api)),
      ),
    );

Future<Indirizzo?> scegliIndirizzo(BuildContext context, ApiOperatore api, String titolo) => showDialog<Indirizzo>(
      context: context,
      builder: (_) => DialogoRicerca<Indirizzo>(
        titolo: titolo,
        cerca: api.indirizzi,
        titoloVoce: (i) => i.riga1,
        sottotitoloVoce: (i) => i.riga2,
        etichettaNuovo: 'Nuovo indirizzo',
        creaNuovo: (ctx) => showDialog<Indirizzo>(context: ctx, builder: (_) => _NuovoIndirizzo(api: api)),
      ),
    );

class _NuovoCliente extends StatefulWidget {
  final ApiOperatore api;
  const _NuovoCliente({required this.api});
  @override
  State<_NuovoCliente> createState() => _NuovoClienteState();
}

class _NuovoClienteState extends State<_NuovoCliente> {
  final _campi = Campi(['nome', 'cognome', 'prefisso', 'nr_telefono', 'email', 'password']);
  late String _chiave = widget.api.nuovaChiave();
  Object? _errore;
  bool _attesa = false;

  @override
  void initState() {
    super.initState();
    _campi.c['prefisso']!.text = '+39';
  }

  Future<void> _salva() async {
    setState(() {
      _attesa = true;
      _errore = null;
    });
    try {
      final (cliente, mailInviata) = await widget.api.creaCliente(
        nome: _campi['nome'],
        cognome: _campi['cognome'],
        prefisso: _campi['prefisso'],
        telefono: _campi['nr_telefono'],
        email: _campi['email'],
        password: _campi.c['password']!.text,
        chiave: _chiave,
      );
      if (!mounted) return;
      avviso(
          context,
          mailInviata
              ? 'Cliente creato: le credenziali sono state inviate a ${cliente.email}.'
              : 'Cliente creato, ma la mail con le credenziali NON è partita: comunicagliele tu.');
      Navigator.of(context).pop(cliente);
    } catch (e) {
      // Risposta di errore dal backend: non è stato creato nulla, il prossimo invio
      // (con i dati corretti) è una richiesta nuova e usa una chiave nuova.
      if (e is ErroreServer && e.status != null) _chiave = widget.api.nuovaChiave();
      setState(() => _errore = e);
    } finally {
      if (mounted) setState(() => _attesa = false);
    }
  }

  @override
  Widget build(BuildContext context) => AlertDialog(
        title: const Text('Nuovo cliente'),
        content: SizedBox(
          width: 520,
          child: SingleChildScrollView(
            child: Wrap(spacing: 12, runSpacing: 12, children: [
              _campi.campo('nome', 'Nome', _errore),
              _campi.campo('cognome', 'Cognome', _errore),
              // nessun filtro su ciò che si scrive: prefisso e telefono li controlla il backend
              _campi.campo('prefisso', 'Prefisso', _errore, larghezza: 90, tastiera: TextInputType.phone),
              _campi.campo('nr_telefono', 'Telefono', _errore, larghezza: 390, tastiera: TextInputType.phone),
              _campi.campo('email', 'Email', _errore, larghezza: 492),
              _campi.campo('password', 'Password iniziale', _errore,
                  password: true, larghezza: 492, suggerimento: 'Almeno 8 caratteri: verrà inviata al cliente per mail'),
              if (_errore != null && (_errore is! ErroreServer || (_errore as ErroreServer).campi.isEmpty))
                Text(testoErrore(_errore!), style: TextStyle(color: Theme.of(context).colorScheme.error)),
            ]),
          ),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Annulla')),
          FilledButton(onPressed: _attesa ? null : _salva, child: const Text('Crea cliente')),
        ],
      );
}

class _NuovoIndirizzo extends StatefulWidget {
  final ApiOperatore api;
  const _NuovoIndirizzo({required this.api});
  @override
  State<_NuovoIndirizzo> createState() => _NuovoIndirizzoState();
}

class _NuovoIndirizzoState extends State<_NuovoIndirizzo> {
  final _campi = Campi(['via', 'civico', 'citta', 'cap', 'regione', 'nazione']);
  late String _chiave = widget.api.nuovaChiave();
  Object? _errore;
  bool _attesa = false;

  /// Città della sede dell'operatore: ritiro e consegna restano lì, quindi città,
  /// regione e nazione si prendono dalla sede e non si cambiano.
  Sede? _sede;

  @override
  void initState() {
    super.initState();
    _campi.c['nazione']!.text = 'Italia';
    _caricaSede();
  }

  Future<void> _caricaSede() async {
    final idSede = widget.api.utente?.idSede;
    if (idSede == null) return;
    try {
      final sede = await widget.api.sede(idSede);
      if (!mounted) return;
      setState(() {
        _sede = sede;
        _campi.c['citta']!.text = sede.indirizzo.citta ?? '';
        _campi.c['regione']!.text = sede.indirizzo.regione ?? '';
        _campi.c['nazione']!.text = sede.indirizzo.nazione ?? '';
      });
    } catch (e) {
      // senza la sede i campi restano liberi: il backend rifiuta comunque un'altra città
      registraErrore(e);
    }
  }

  Future<void> _salva() async {
    setState(() {
      _attesa = true;
      _errore = null;
    });
    try {
      final ind = await widget.api.creaIndirizzo(
        via: _campi['via'],
        civico: _campi['civico'],
        citta: _campi['citta'],
        cap: _campi['cap'],
        regione: _campi['regione'],
        nazione: _campi['nazione'],
        chiave: _chiave,
      );
      if (mounted) Navigator.of(context).pop(ind);
    } catch (e) {
      // Risposta di errore dal backend: non è stato creato nulla, il prossimo invio
      // (con i dati corretti) è una richiesta nuova e usa una chiave nuova.
      if (e is ErroreServer && e.status != null) _chiave = widget.api.nuovaChiave();
      setState(() => _errore = e);
    } finally {
      if (mounted) setState(() => _attesa = false);
    }
  }

  @override
  Widget build(BuildContext context) => AlertDialog(
        title: const Text('Nuovo indirizzo'),
        content: SizedBox(
          width: 520,
          child: SingleChildScrollView(
            child: Wrap(spacing: 12, runSpacing: 12, children: [
              _campi.campo('via', 'Via', _errore, larghezza: 390),
              _campi.campo('civico', 'Civico', _errore, larghezza: 90),
              _campi.campo('citta', 'Città', _errore,
                  larghezza: 390, soloLettura: _sede != null, suggerimento: _sede == null ? null : 'La città della tua sede'),
              _campi.campo('cap', 'CAP', _errore, larghezza: 90),
              _campi.campo('regione', 'Regione', _errore, soloLettura: _sede != null),
              _campi.campo('nazione', 'Nazione', _errore, soloLettura: _sede != null),
              if (_errore != null && (_errore is! ErroreServer || (_errore as ErroreServer).campi.isEmpty))
                Text(testoErrore(_errore!), style: TextStyle(color: Theme.of(context).colorScheme.error)),
            ]),
          ),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Annulla')),
          FilledButton(onPressed: _attesa ? null : _salva, child: const Text('Salva indirizzo')),
        ],
      );
}
