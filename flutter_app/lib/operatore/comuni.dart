import 'package:flutter/material.dart';

import 'api.dart';

String due(int n) => n.toString().padLeft(2, '0');
String fmtData(DateTime d) => '${due(d.day)}/${due(d.month)}/${d.year}';
String fmtDataOra(DateTime d) => '${fmtData(d)} ${due(d.hour)}:${due(d.minute)}';

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

String testoErrore(Object e) => e is ErroreServer ? e.perUtente : 'Errore inatteso: $e';

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

String descriviPersona(Persona? p) {
  if (p == null) return '—';
  return [p.nomeCompleto, if (p.email != null) p.email!, if (p.telefono != null) p.telefono!].join(' · ');
}
