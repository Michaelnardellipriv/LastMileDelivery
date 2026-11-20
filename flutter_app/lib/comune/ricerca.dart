// Pezzi comuni alle aree operatori e amministratori: finestra di ricerca in
// un'anagrafica, scelta del corriere e campi di testo che mostrano l'errore del
// backend sotto il campo.
import 'dart:async';

import 'package:flutter/material.dart';

import 'api.dart';
import 'comuni.dart';

/// Sceglie un corriere attivo (GET /api/corrieri: operatori e amministratori).
Future<Persona?> scegliCorriere(BuildContext context, ApiSessione api) => showDialog<Persona>(
      context: context,
      builder: (_) => DialogoRicerca<Persona>(
        titolo: 'Scegli il corriere',
        cerca: api.corrieri,
        titoloVoce: (c) => c.nomeCompleto,
        sottotitoloVoce: (c) => [c.email, c.telefono].whereType<String>().join(' · '),
      ),
    );

class DialogoRicerca<T> extends StatefulWidget {
  final String titolo;
  final Future<Pagina<T>> Function(String q) cerca;
  final String Function(T) titoloVoce;
  final String Function(T) sottotitoloVoce;
  final String? etichettaNuovo;
  final Future<T?> Function(BuildContext)? creaNuovo;
  const DialogoRicerca({
    super.key,
    required this.titolo,
    required this.cerca,
    required this.titoloVoce,
    required this.sottotitoloVoce,
    this.etichettaNuovo,
    this.creaNuovo,
  });
  @override
  State<DialogoRicerca<T>> createState() => _DialogoRicercaState<T>();
}

class _DialogoRicercaState<T> extends State<DialogoRicerca<T>> {
  Timer? _ritardo;
  List<T> _risultati = [];
  int _totale = 0;
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
          _risultati = r.dati;
          _totale = r.totaleElementi;
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
            if (!_attesa && _errore == null && _totale > _risultati.length)
              Padding(
                padding: const EdgeInsets.only(bottom: 4),
                child: Text('Mostrati ${_risultati.length} di $_totale: scrivi per restringere la ricerca',
                    style: Theme.of(context).textTheme.bodySmall),
              ),
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
class Campi {
  final Map<String, TextEditingController> c;
  Campi(List<String> nomi) : c = {for (final n in nomi) n: TextEditingController()};
  String operator [](String n) => c[n]!.text;
  Widget campo(String nome, String etichetta, Object? errore,
          {bool password = false, String? suggerimento, double larghezza = 240, TextInputType? tastiera}) =>
      SizedBox(
        width: larghezza,
        child: TextField(
          controller: c[nome],
          obscureText: password,
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

/// Errore da mostrare in fondo al modulo: quello senza dettagli per campo (rete, 403,
/// 409...) e quelli sui campi che non sono a schermo ([campiMostrati] hanno già il
/// messaggio sotto il campo).
Widget? erroreGenerale(BuildContext context, Object? errore, {Set<String> campiMostrati = const {}}) {
  if (errore == null) return null;
  final stile = TextStyle(color: Theme.of(context).colorScheme.error);
  if (errore is! ErroreServer || errore.campi.isEmpty) return Text(testoErrore(errore), style: stile);
  final altri = [
    for (final e in errore.campi.entries)
      if (!campiMostrati.contains(e.key)) '${e.key}: ${e.value}',
  ];
  return altri.isEmpty ? null : Text(altri.join('\n'), style: stile);
}
