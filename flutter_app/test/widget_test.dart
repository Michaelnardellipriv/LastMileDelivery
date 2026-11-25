// flutter test — avvio dell'app in modalità demo (API_BASE vuoto): si parte dal NUOVO login.
import 'dart:ui';

import 'package:flutter_test/flutter_test.dart';
import 'package:veloce24_app/auth/recupero_link.dart';
import 'package:veloce24_app/main.dart';
import 'package:veloce24_app/store.dart';
import 'package:veloce24_app/auth/session.dart';


void main() {
  setUp(() async {
    store.usaStorage(MemoriaSessioneStorage());
    apiBase = '';
    tokenRecuperoDaLink.value = null;
    await store.esci();
  });

  testWidgets("all'avvio compare il login con il recupero password, senza la vecchia pagina di prova", (tester) async {
    await tester.pumpWidget(const App());
    await tester.pumpAndSettle();
    expect(find.text('Veloce24'), findsOneWidget);
    expect(find.text('Accedi'), findsOneWidget);
    expect(find.text('Password dimenticata?'), findsOneWidget);
    expect(find.textContaining('non ha ancora un endpoint di login'), findsNothing);

    await tester.tap(find.text('Password dimenticata?'));
    await tester.pumpAndSettle();
    expect(find.text('Invia link di recupero'), findsOneWidget);
  });

  testWidgets('demo: scegliendo un utente di prova si entra nella Shell e dal profilo si esce', (tester) async {
    tester.view.physicalSize = const Size(1200, 900); // il profilo mostra "Cambia password" solo a schermo largo
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(const App());
    await tester.pumpAndSettle();
    await tester.tap(find.textContaining('Olga').first); // chip dell'utente demo: compila email e password
    await tester.pump();
    await tester.tap(find.text('Accedi'));
    await tester.pumpAndSettle();
    expect(find.textContaining('Operatore logistico'), findsWidgets);

    await tester.tap(find.byTooltip('Il mio profilo'));
    await tester.pumpAndSettle();
    expect(find.text('Cambia password'), findsOneWidget);
  });
}
