import 'dart:async';

import 'package:flutter/material.dart';

import 'api.dart';

String due(int n) => n.toString().padLeft(2, '0');
String fmtData(DateTime d) => '${due(d.day)}/${due(d.month)}/${d.year}';
String fmtDataOra(DateTime d) => '${fmtData(d)} ${due(d.hour)}:${due(d.minute)}';
String fmtOra(DateTime d) => '${due(d.hour)}:${due(d.minute)}:${due(d.second)}';

Color coloreStato(String s) => switch (s) {
      'consegnata' => Colors.green,
      'non consegnata' || 'persa' || 'annullata' => Colors.red,
      'ritirata' || 'in consegna' => Colors.orange,
      'assegnata' => Colors.blue,
      _ => Colors.grey,
    };

class ChipStato extends StatelessWidget {
  final String stato;
  const ChipStato(this.stato, {super.key});
  @override
  Widget build(BuildContext context) {
    final c = coloreStato(stato);
    return Chip(
      label: Text(stato, style: TextStyle(color: c, fontWeight: FontWeight.w600, fontSize: 12)),
      backgroundColor: c.withValues(alpha: .15),
      side: BorderSide.none,
      visualDensity: VisualDensity.compact,
    );
  }
}

void avviso(BuildContext context, String messaggio) => ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(messaggio)));

final _giaRegistrati = Expando<bool>();

/// Scrive l'errore completo nella console del browser (una volta sola per errore):
/// a schermo va solo il messaggio breve di [testoErrore].
void registraErrore(Object e, [StackTrace? traccia]) {
  if (_giaRegistrati[e] == true) return;
  _giaRegistrati[e] = true;
  traccia ??= e is Error ? e.stackTrace : null;
  debugPrint('[Veloce24] errore: $e${traccia == null ? '' : '\n$traccia'}');
}

/// Messaggio breve per l'utente; i dettagli tecnici finiscono in console.
String testoErrore(Object e) {
  registraErrore(e);
  final errore = primoErrore(e);
  return errore is ErroreServer
      ? errore.perUtente
      : 'Si è verificato un errore imprevisto. Riprova; se succede ancora avvisa l\'assistenza.';
}

/// Con due chiamate in parallelo ((a, b).wait) l'errore arriva dentro un
/// ParallelWaitError: per il messaggio all'utente conta il primo errore del backend.
Object primoErrore(Object e) {
  if (e is ParallelWaitError) {
    final errori = e.errors;
    if (errori is (AsyncError?, AsyncError?)) return errori.$1?.error ?? errori.$2?.error ?? e;
  }
  return e;
}

void mostraErrore(BuildContext context, Object e) {
  if (!context.mounted) return;
  ScaffoldMessenger.of(context).showSnackBar(SnackBar(
    content: Text(testoErrore(e)),
    backgroundColor: Theme.of(context).colorScheme.error,
  ));
}

/// Errore del campo, se il backend l'ha segnalato (risposte 400 e 409 con "dettagli").
String? erroreCampo(Object? e, String campo) => e is ErroreServer ? e.campi[campo] : null;

class Sezione extends StatelessWidget {
  final String titolo;
  final List<Widget> figli;
  final Widget? azione;
  const Sezione(this.titolo, this.figli, {super.key, this.azione});
  @override
  Widget build(BuildContext context) => Card(
        margin: const EdgeInsets.only(bottom: 12),
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Row(children: [
              Expanded(child: Text(titolo, style: Theme.of(context).textTheme.titleMedium)),
              if (azione != null) azione!,
            ]),
            const SizedBox(height: 8),
            ...figli,
          ]),
        ),
      );
}

class Riga extends StatelessWidget {
  final String etichetta;
  final String? valore;
  const Riga(this.etichetta, this.valore, {super.key});
  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.symmetric(vertical: 3),
        child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
          SizedBox(width: 140, child: Text(etichetta, style: TextStyle(color: Theme.of(context).colorScheme.outline))),
          Expanded(child: SelectableText(valore == null || valore!.isEmpty ? '—' : valore!)),
        ]),
      );
}

/// Barra delle pagine: "Pagina 2 di 14 · 21–40 di 271", con prima, precedente,
/// successiva e ultima. I pulsanti si spengono ai bordi: niente pagine vuote oltre la fine.
class BarraPagine extends StatelessWidget {
  final Pagina<Object?> pagina;
  final bool attesa;
  final void Function(int pagina) vaiA;
  const BarraPagine({super.key, required this.pagina, required this.attesa, required this.vaiA});

  @override
  Widget build(BuildContext context) {
    final p = pagina;
    final stile = Theme.of(context).textTheme.bodySmall;
    final elementi = p.totaleElementi == 0
        ? 'nessun elemento'
        : '${p.da}–${p.a} di ${p.totaleElementi} ${p.totaleElementi == 1 ? 'elemento' : 'elementi'}';
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      child: Row(mainAxisAlignment: MainAxisAlignment.center, children: [
        IconButton(
          tooltip: 'Prima pagina',
          onPressed: attesa || !p.haPrecedente ? null : () => vaiA(1),
          icon: const Icon(Icons.first_page),
        ),
        IconButton(
          tooltip: 'Pagina precedente',
          onPressed: attesa || !p.haPrecedente ? null : () => vaiA(p.pagina - 1),
          icon: const Icon(Icons.chevron_left),
        ),
        Flexible(
          child: Column(mainAxisSize: MainAxisSize.min, children: [
            Text(p.totalePagine == 0 ? 'Nessuna pagina' : 'Pagina ${p.pagina} di ${p.totalePagine}', textAlign: TextAlign.center),
            Text(elementi, style: stile, textAlign: TextAlign.center),
          ]),
        ),
        IconButton(
          tooltip: 'Pagina successiva',
          onPressed: attesa || !p.haSuccessiva ? null : () => vaiA(p.pagina + 1),
          icon: const Icon(Icons.chevron_right),
        ),
        IconButton(
          tooltip: 'Ultima pagina',
          onPressed: attesa || !p.haSuccessiva ? null : () => vaiA(p.totalePagine),
          icon: const Icon(Icons.last_page),
        ),
      ]),
    );
  }
}

/// Persona con l'id, per chi ha fatto un'azione: "Oscar Operatore (id 1)".
String nomeConId(Persona? p) => p == null ? '—' : '${p.nomeCompleto}${p.id == null ? '' : ' (id ${p.id})'}';

String descriviPersona(Persona? p) {
  if (p == null) return '—';
  return [p.nomeCompleto, if (p.email != null) p.email!, if (p.telefono != null) p.telefono!].join(' · ');
}
