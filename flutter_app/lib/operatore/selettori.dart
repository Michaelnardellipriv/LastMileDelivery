// Finestre per cercare e scegliere clienti, indirizzi e corrieri, con la creazione
// di clienti e indirizzi nuovi.
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import 'api.dart';
import 'comuni.dart';

Future<Persona?> scegliCorriere(BuildContext context, ApiOperatore api) => showDialog<Persona>(
      context: context,
      builder: (_) => _DialogoRicerca<Persona>(
        titolo: 'Scegli il corriere',
        cerca: api.corrieri,
        titoloVoce: (c) => c.nomeCompleto,
        sottotitoloVoce: (c) => [c.email, c.telefono].whereType<String>().join(' · '),
      ),
    );

Future<Persona?> scegliCliente(BuildContext context, ApiOperatore api, String titolo) => showDialog<Persona>(
      context: context,
      builder: (_) => _DialogoRicerca<Persona>(
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
      builder: (_) => _DialogoRicerca<Indirizzo>(
        titolo: titolo,
        cerca: api.indirizzi,
        titoloVoce: (i) => i.riga1,
        sottotitoloVoce: (i) => i.riga2,
        etichettaNuovo: 'Nuovo indirizzo',
        creaNuovo: (ctx) => showDialog<Indirizzo>(context: ctx, builder: (_) => _NuovoIndirizzo(api: api)),
      ),
    );

class _DialogoRicerca<T> extends StatefulWidget {
  final String titolo;
  final Future<List<T>> Function(String q) cerca;
  final String Function(T) titoloVoce;
  final String Function(T) sottotitoloVoce;
  final String? etichettaNuovo;
  final Future<T?> Function(BuildContext)? creaNuovo;
  const _DialogoRicerca({
    super.key,
    required this.titolo,
    required this.cerca,
    required this.titoloVoce,
    required this.sottotitoloVoce,
    this.etichettaNuovo,
    this.creaNuovo,
  });
  @override
  State<_DialogoRicerca<T>> createState() => _DialogoRicercaState<T>();
}

class _DialogoRicercaState<T> extends State<_DialogoRicerca<T>> {
  Timer? _ritardo;
  List<T> _risultati = [];
  Object? _errore;
  bool _attesa = false;
  int _richiesta = 0;

  @override
  void initState() {
    super.initState();
    _carica('');
  }

  @override
  void dispose() {
    _ritardo?.cancel();
    super.dispose();
  }

  Future<void> _carica(String q) async {
    final n = ++_richiesta;
    setState(() => _attesa = true);
    try {
      final r = await widget.cerca(q);
      if (mounted && n == _richiesta)
        setState(() {
          _risultati = r;
          _errore = null;
        });
    } catch (e) {
      if (mounted && n == _richiesta) setState(() => _errore = e);
    } finally {
      if (mounted && n == _richiesta) setState(() => _attesa = false);
    }
  }

  @override
  Widget build(BuildContext context) => AlertDialog(
        title: Text(widget.titolo),
        content: SizedBox(
          width: 520,
          height: 440,
          child: Column(children: [
            TextField(
              autofocus: true,
              decoration: const InputDecoration(prefixIcon: Icon(Icons.search), hintText: 'Cerca', border: OutlineInputBorder()),
              onChanged: (q) {
                _ritardo?.cancel();
                _ritardo = Timer(const Duration(milliseconds: 300), () => _carica(q));
              },
            ),
            const SizedBox(height: 8),
            if (_attesa) const LinearProgressIndicator(),
            Expanded(
              child: _errore != null
                  ? Center(child: Text(testoErrore(_errore!)))
                  : _risultati.isEmpty && !_attesa
                      ? const Center(child: Text('Nessun risultato'))
                      : ListView.separated(
                          itemCount: _risultati.length,
                          separatorBuilder: (_, __) => const Divider(height: 1),
                          itemBuilder: (_, i) {
                            final v = _risultati[i];
                            return ListTile(
                              title: Text(widget.titoloVoce(v)),
                              subtitle: Text(widget.sottotitoloVoce(v)),
                              onTap: () => Navigator.of(context).pop(v),
                            );
                          },
                        ),
            ),
          ]),
        ),
        actions: [
          if (widget.creaNuovo != null)
            OutlinedButton.icon(
              icon: const Icon(Icons.add),
              label: Text(widget.etichettaNuovo!),
              onPressed: () async {
                final nuovo = await widget.creaNuovo!(context);
                if (nuovo != null && context.mounted) Navigator.of(context).pop(nuovo);
              },
            ),
          TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Annulla')),
        ],
      );
}

/// Campi di testo con l'errore restituito dal backend per quel campo.
class _Campi {
  final Map<String, TextEditingController> c;
  _Campi(List<String> nomi) : c = {for (final n in nomi) n: TextEditingController()};
  String operator [](String n) => c[n]!.text;
  Widget campo(String nome, String etichetta, Object? errore,
          {bool password = false, String? suggerimento, double larghezza = 240, List<TextInputFormatter>? formati, TextInputType? tastiera}) =>
      SizedBox(
        width: larghezza,
        child: TextField(
          controller: c[nome],
          obscureText: password,
          inputFormatters: formati,
          keyboardType: tastiera,
          decoration: InputDecoration(
            labelText: etichetta,
            helperText: suggerimento,
            errorText: erroreCampo(errore, nome),
            errorMaxLines: 4, // nei campi stretti (civico, CAP) il messaggio va a capo invece di essere tagliato
            helperMaxLines: 2,
            border: const OutlineInputBorder(),
          ),
        ),
      );
}

class _NuovoCliente extends StatefulWidget {
  final ApiOperatore api;
  const _NuovoCliente({required this.api});
  @override
  State<_NuovoCliente> createState() => _NuovoClienteState();
}

class _NuovoClienteState extends State<_NuovoCliente> {
  final _campi = _Campi(['nome', 'cognome', 'prefisso', 'nr_telefono', 'email', 'password']);
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
              _campi.campo('prefisso', 'Prefisso', _errore,
                  larghezza: 90, tastiera: TextInputType.phone, formati: [FilteringTextInputFormatter.allow(RegExp(r'[+0-9]'))]),
              // si possono scrivere solo cifre; il formato completo lo controlla comunque il backend
              _campi.campo('nr_telefono', 'Telefono', _errore,
                  larghezza: 390, tastiera: TextInputType.phone, formati: [FilteringTextInputFormatter.digitsOnly]),
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
  final _campi = _Campi(['via', 'civico', 'citta', 'cap', 'regione', 'nazione']);
  late String _chiave = widget.api.nuovaChiave();
  Object? _errore;
  bool _attesa = false;

  @override
  void initState() {
    super.initState();
    _campi.c['nazione']!.text = 'Italia';
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
              _campi.campo('citta', 'Città', _errore, larghezza: 390),
              _campi.campo('cap', 'CAP', _errore, larghezza: 90),
              _campi.campo('regione', 'Regione', _errore),
              _campi.campo('nazione', 'Nazione', _errore),
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
