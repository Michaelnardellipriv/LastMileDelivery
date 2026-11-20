// Sezioni di sola lettura del dettaglio di una spedizione (dati, persone, indirizzi,
// note, storico), comuni al dettaglio dell'operatore e a quello dell'amministratore.
import 'package:flutter/material.dart';

import 'api.dart';
import 'comuni.dart';

/// Dati della spedizione, persone coinvolte e indirizzi.
List<Widget> sezioniSpedizione(Spedizione s) => [
      Sezione('Spedizione', [
        Riga('Creata il', fmtDataOra(s.creataIl)),
        Riga('Richiesta il', fmtDataOra(s.richiestaIl)),
        Riga('Ritirata il', s.ritirataIl == null ? 'non ancora ritirata' : fmtDataOra(s.ritirataIl!)),
        if (s.conclusaIl != null) Riga('Conclusa il', fmtDataOra(s.conclusaIl!)),
        if (s.annullataIl != null) Riga('Annullata il', fmtDataOra(s.annullataIl!)),
        Riga('Reso', s.isReso ? 'sì' : 'no'),
        Riga('Ritiri falliti', '${s.tentativiRitiro ?? 0}'),
        Riga('Consegne fallite', '${s.tentativiConsegna ?? 0}'),
      ]),
      Sezione('Persone', [
        Riga('Mittente', descriviPersona(s.mittente)),
        Riga('Destinatario', descriviPersona(s.destinatario)),
        Riga('Corriere', s.corriere == null ? 'non assegnata' : descriviPersona(s.corriere)),
        if (s.assegnataDa != null)
          Riga('Assegnata da', [
            nomeConId(s.assegnataDa),
            if (s.assegnataDa!.email != null) s.assegnataDa!.email!,
            if (s.assegnataDa!.telefono != null) s.assegnataDa!.telefono!,
          ].join(' · ')),
      ]),
      Sezione('Indirizzi', [
        Riga('Ritiro', s.ritiro.testo),
        Riga('Consegna', s.consegna.testo),
      ]),
    ];

/// Le note della spedizione, interne e pubbliche, con l'autore.
List<Widget> vociNote(List<Nota> note) => [
      if (note.isEmpty) const Text('Nessuna nota'),
      for (final n in note)
        ListTile(
          contentPadding: EdgeInsets.zero,
          leading: Icon(n.interna ? Icons.lock_outline : Icons.public, color: n.interna ? null : Colors.teal),
          title: Text(n.testo),
          subtitle: Text([
            n.interna ? 'interna' : 'pubblica · il cliente vede il motivo "${n.motivo ?? ''}"',
            fmtDataOra(n.creataIl),
            if (n.autore != null) n.autore!.testo,
          ].join(' · ')),
        ),
    ];

/// Storico delle modifiche, dalla più recente, con chi le ha fatte.
class SezioneStorico extends StatelessWidget {
  final List<VoceStorico> storico;
  const SezioneStorico(this.storico, {super.key});

  @override
  Widget build(BuildContext context) => Sezione('Storico', [
        if (storico.isEmpty) const Text('Nessuna modifica registrata'),
        for (final v in storico.reversed)
          ListTile(
            contentPadding: EdgeInsets.zero,
            dense: true,
            leading: Icon(switch (v.operazione) {
              'creazione' => Icons.add_circle_outline,
              'annullamento' => Icons.cancel_outlined,
              _ => Icons.edit_outlined,
            }),
            title: Text(descriviModifica(v)),
            subtitle: Text('${fmtDataOra(v.quando)} · ${v.autore.testo}'),
          ),
      ]);

  static String descriviModifica(VoceStorico v) {
    if (v.operazione == 'creazione') return 'Spedizione creata';
    final prima = v.prima ?? const {};
    final cambi = [
      for (final e in v.dopo.entries)
        if (e.key != 'modificata_il') '${e.key.replaceAll('_', ' ')}: ${prima[e.key] ?? '—'} → ${e.value ?? '—'}',
    ];
    return cambi.isEmpty ? v.operazione : cambi.join('\n');
  }
}
