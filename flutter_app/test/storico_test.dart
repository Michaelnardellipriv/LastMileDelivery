// flutter test — storico della spedizione: i tentativi falliti del corriere (operazione
// "tentativo") compaiono accanto ai cambi di stato, con motivo, nota e autore.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:veloce24_app/comune/api.dart';
import 'package:veloce24_app/comune/scheda_spedizione.dart';

VoceStorico _voce(Map<String, dynamic> j) => VoceStorico.daJson(j);

final _corriere = {'ruolo': 'corriere', 'id': 7, 'nome': 'Max', 'cognome': 'Rossi'};

void main() {
  final storico = [
    _voce({
      'id': 1,
      'operazione': 'modifica',
      'valore_vecchio': {'stato': 'ritirata'},
      'valore_nuovo': {'stato': 'in consegna'},
      'modificato_il': '2026-11-20T09:00:00Z',
      'autore': _corriere,
    }),
    _voce({
      'id': 5,
      'operazione': 'tentativo',
      'valore_vecchio': null,
      'valore_nuovo': {'tipo': 'consegna', 'esito': 'fallito', 'motivo': 'destinatario assente', 'nota': 'citofono rotto'},
      'modificato_il': '2026-11-20T10:00:00Z',
      'autore': _corriere,
    }),
    _voce({
      'id': 6,
      'operazione': 'tentativo',
      'valore_vecchio': null,
      'valore_nuovo': {'tipo': 'ritiro', 'esito': 'fallito', 'motivo': null, 'nota': ''},
      'modificato_il': '2026-11-20T11:00:00Z',
      'autore': _corriere,
    }),
  ];

  test('il tentativo si descrive con tipo, motivo e nota', () {
    expect(SezioneStorico.descriviModifica(storico[1]), 'Tentativo di consegna fallito: destinatario assente\ncitofono rotto');
    expect(SezioneStorico.descriviModifica(storico[2]), 'Tentativo di ritiro fallito');
    expect(SezioneStorico.descriviModifica(storico[0]), 'stato: ritirata → in consegna');
  });

  testWidgets('nello storico il tentativo sta accanto al cambio di stato, con la sua icona', (tester) async {
    await tester.pumpWidget(MaterialApp(home: Scaffold(body: SingleChildScrollView(child: SezioneStorico(storico)))));

    final tentativo = find.text('Tentativo di consegna fallito: destinatario assente\ncitofono rotto');
    expect(tentativo, findsOneWidget);
    expect(find.text('stato: ritirata → in consegna'), findsOneWidget);
    expect(find.byIcon(Icons.report_problem_outlined), findsNWidgets(2));
    expect(find.textContaining('Max Rossi (corriere · id 7)'), findsNWidgets(3));
    // dalla più recente: il tentativo di consegna sta sopra il passaggio a "in consegna"
    expect(tester.getTopLeft(tentativo).dy, lessThan(tester.getTopLeft(find.text('stato: ritirata → in consegna')).dy));
  });
}
