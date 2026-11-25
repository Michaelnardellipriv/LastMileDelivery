// Anagrafica dei clienti per l'operatore: ricerca, pagine e modifica dei dati (nome,
// cognome, telefono, email). Disattivare un cliente o cambiargli la password spetta
// all'amministratore. La pagina non valida nulla: manda ciò che è scritto e mostra gli
// errori del backend sotto i campi.
import 'package:flutter/material.dart';

import '../comune/comuni.dart';
import '../comune/ricerca.dart';
import 'api.dart';

class ClientiPage extends StatefulWidget {
  final ApiOperatore api;
  const ClientiPage({super.key, required this.api});
  @override
  State<ClientiPage> createState() => _ClientiPageState();
}

class _ClientiPageState extends State<ClientiPage> {
  final _q = TextEditingController();
  int _pagina = 1;
  Pagina<ClienteAnagrafica>? _risultato;
  Object? _errore;
  bool _attesa = false;

  @override
  void initState() {
    super.initState();
    _carica();
  }

  Future<void> _carica({int pagina = 1}) async {
    setState(() {
      _attesa = true;
      _errore = null;
    });
    try {
      final r = await widget.api.anagraficaClienti(_q.text, pagina: pagina);
      if (!mounted) return;
      if (r.oltreLaFine) return await _carica(pagina: r.totalePagine);
      setState(() {
        _risultato = r;
        _pagina = pagina;
      });
    } catch (e) {
      if (mounted) setState(() => _errore = e);
    } finally {
      if (mounted) setState(() => _attesa = false);
    }
  }

  Future<void> _modifica(ClienteAnagrafica c) async {
    final fatto = await showDialog<ClienteAnagrafica>(context: context, builder: (_) => DialogoCliente(api: widget.api, cliente: c));
    if (fatto == null || !mounted) return;
    avviso(context, 'Dati di ${fatto.nomeCompleto} aggiornati');
    _carica(pagina: _pagina);
  }

  Widget _voce(ClienteAnagrafica c) {
    final colori = Theme.of(context).colorScheme;
    return ListTile(
      leading: const CircleAvatar(child: Icon(Icons.person_outline)),
      title: Wrap(spacing: 8, crossAxisAlignment: WrapCrossAlignment.center, children: [
        Text(c.nomeCompleto, style: TextStyle(fontWeight: FontWeight.w600, color: c.attivo ? null : colori.outline)),
        // destinatario: ha ricevuto almeno una spedizione; altrimenti mittente (anche appena creato)
        Chip(
          avatar: Icon(c.destinatario ? Icons.move_to_inbox_outlined : Icons.outbox_outlined, size: 16),
          label: Text(c.destinatario ? 'Destinatario' : 'Mittente', style: const TextStyle(fontSize: 11)),
          visualDensity: VisualDensity.compact,
          side: BorderSide.none,
          backgroundColor: c.destinatario ? colori.primaryContainer : colori.tertiaryContainer,
        ),
        if (!c.attivo)
          Chip(
            label: const Text('disattivato', style: TextStyle(fontSize: 11)),
            visualDensity: VisualDensity.compact,
            side: BorderSide.none,
            backgroundColor: colori.errorContainer,
          ),
      ]),
      subtitle: Text('${c.email} · ${c.prefisso} ${c.nrTelefono}\nid cliente ${c.id}'),
      isThreeLine: true,
      trailing: IconButton(tooltip: 'Modifica dati', onPressed: _attesa ? null : () => _modifica(c), icon: const Icon(Icons.edit_outlined)),
    );
  }

  @override
  Widget build(BuildContext context) {
    final r = _risultato;
    return Column(children: [
      Padding(
        padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
        child: Wrap(spacing: 8, runSpacing: 8, crossAxisAlignment: WrapCrossAlignment.center, children: [
          SizedBox(
            width: 280,
            child: TextField(
              controller: _q,
              decoration: const InputDecoration(
                  labelText: 'Cerca nome, email o telefono', isDense: true, border: OutlineInputBorder(), prefixIcon: Icon(Icons.search)),
              onSubmitted: (_) => _carica(),
            ),
          ),
          FilledButton.tonalIcon(onPressed: _attesa ? null : () => _carica(), icon: const Icon(Icons.search), label: const Text('Cerca')),
        ]),
      ),
      if (_attesa) const LinearProgressIndicator() else const SizedBox(height: 4),
      Expanded(
        child: _errore != null
            ? Center(
                child: Column(mainAxisSize: MainAxisSize.min, children: [
                  Padding(padding: const EdgeInsets.all(16), child: Text(testoErrore(_errore!), textAlign: TextAlign.center)),
                  OutlinedButton(onPressed: () => _carica(pagina: _pagina), child: const Text('Riprova')),
                ]),
              )
            : r == null
                ? const SizedBox()
                : r.dati.isEmpty
                    ? const Center(child: Text('Nessun cliente trovato'))
                    : ListView.separated(
                        itemCount: r.dati.length,
                        separatorBuilder: (_, __) => const Divider(height: 1),
                        itemBuilder: (_, i) => _voce(r.dati[i]),
                      ),
      ),
      if (r != null) BarraPagine(pagina: r, attesa: _attesa, vaiA: (p) => _carica(pagina: p)),
    ]);
  }
}

/// Modifica dei dati di un cliente: partono solo i campi cambiati.
class DialogoCliente extends StatefulWidget {
  final ApiOperatore api;
  final ClienteAnagrafica cliente;
  const DialogoCliente({super.key, required this.api, required this.cliente});
  @override
  State<DialogoCliente> createState() => _DialogoClienteState();
}

class _DialogoClienteState extends State<DialogoCliente> {
  static const _nomiCampi = ['nome', 'cognome', 'prefisso', 'nr_telefono', 'email'];
  final _campi = Campi(_nomiCampi);
  late String _chiave = widget.api.nuovaChiave();
  Object? _errore;
  bool _attesa = false;

  @override
  void initState() {
    super.initState();
    for (final e in widget.cliente.dati.entries) {
      _campi.c[e.key]!.text = e.value;
    }
  }

  Future<void> _salva() async {
    setState(() {
      _attesa = true;
      _errore = null;
    });
    final modifiche = {
      for (final e in widget.cliente.dati.entries)
        if (_campi[e.key] != e.value) e.key: _campi[e.key],
    };
    try {
      final c = await widget.api.modificaCliente(widget.cliente, modifiche, chiave: _chiave);
      if (mounted) Navigator.of(context).pop(c);
    } catch (e) {
      // rifiutata dal backend: non è cambiato nulla, il prossimo invio usa una chiave nuova
      if (e is ErroreServer && e.status != null) _chiave = widget.api.nuovaChiave();
      if (mounted) setState(() => _errore = e);
    } finally {
      if (mounted) setState(() => _attesa = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final errore = erroreGenerale(context, _errore, campiMostrati: _nomiCampi.toSet());
    return AlertDialog(
      title: Text('Modifica ${widget.cliente.nomeCompleto}'),
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
            const SizedBox(
              width: 492,
              child: Text('Disattivare il cliente o cambiargli la password spetta all\'amministratore.'),
            ),
            if (errore != null) SizedBox(width: 492, child: errore),
          ]),
        ),
      ),
      actions: [
        TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Annulla')),
        FilledButton(
          onPressed: _attesa ? null : _salva,
          child: _attesa
              ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
              : const Text('Salva modifiche'),
        ),
      ],
    );
  }
}
