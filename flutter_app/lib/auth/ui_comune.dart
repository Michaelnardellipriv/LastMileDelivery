// Pezzi di UI condivisi da recupero password e area personale.
// Stessi criteri visivi di login_page.dart (card centrata sopra i 600 px, a tutto schermo sotto).
import 'package:material_ui/material_ui.dart';

/// Sotto questa larghezza il layout è "mobile" (a tutto schermo); sopra, card centrata.
const larghezzaCard = 600.0;
const larghezzaMassimaCard = 450.0;

/// Snackbar di esito: verde/primario per il successo, rosso per l'errore.
/// Su schermi larghi ha una larghezza fissa, per non stirarsi su tutto il browser.
void mostraEsito(BuildContext context, String testo, {bool errore = false}) {
  final cs = Theme.of(context).colorScheme;
  final sfondo = errore ? cs.errorContainer : cs.primaryContainer;
  final primoPiano = errore ? cs.onErrorContainer : cs.onPrimaryContainer;
  final largo = MediaQuery.sizeOf(context).width >= larghezzaCard;
  ScaffoldMessenger.of(context)
    ..hideCurrentSnackBar()
    ..showSnackBar(SnackBar(
      content: Text(testo, style: TextStyle(color: primoPiano)),
      backgroundColor: sfondo,
      behavior: SnackBarBehavior.floating,
      width: largo ? larghezzaMassimaCard : null,
      showCloseIcon: true,
      closeIconColor: primoPiano,
      duration: Duration(seconds: errore ? 6 : 4),
    ));
}

/// Contenitore delle pagine "da non loggati" (recupero password): contenuto centrato,
/// card con ombra su web/desktop, a tutto schermo su mobile, scorrevole con la tastiera aperta.
class AuthCardLayout extends StatelessWidget {
  const AuthCardLayout({super.key, required this.child});
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return LayoutBuilder(builder: (context, vincoli) {
      final largo = vincoli.maxWidth >= larghezzaCard;
      // Margini fluidi su mobile: 6% della larghezza, tra 16 e 32 px.
      final double margine = largo ? 24.0 : (vincoli.maxWidth * 0.06).clamp(16.0, 32.0).toDouble();
      return SingleChildScrollView(
        keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
        child: ConstrainedBox(
          constraints: BoxConstraints(minHeight: vincoli.maxHeight),
          child: Center(
            child: Padding(
              padding: EdgeInsets.symmetric(horizontal: margine, vertical: 24),
              child: ConstrainedBox(
                constraints: BoxConstraints(maxWidth: largo ? larghezzaMassimaCard : double.infinity),
                child: largo ? _Card(colore: cs.surface, bordo: cs.outlineVariant, child: child) : child,
              ),
            ),
          ),
        ),
      );
    });
  }
}

class _Card extends StatelessWidget {
  const _Card({required this.colore, required this.bordo, required this.child});
  final Color colore, bordo;
  final Widget child;

  @override
  Widget build(BuildContext context) => Container(
        padding: const EdgeInsets.fromLTRB(32, 36, 32, 28),
        decoration: BoxDecoration(
          color: colore,
          borderRadius: BorderRadius.circular(20),
          border: Border.all(color: bordo.withValues(alpha: .5)),
          boxShadow: [
            BoxShadow(color: Colors.black.withValues(alpha: .10), blurRadius: 32, offset: const Offset(0, 12)),
            BoxShadow(color: Colors.black.withValues(alpha: .06), blurRadius: 6, offset: const Offset(0, 2)),
          ],
        ),
        child: child,
      );
}

/// Campo password con l'occhio per mostrare/nascondere il testo.
/// Ogni istanza ricorda da sola se è visibile: i tre campi del cambio password sono indipendenti.
class CampoPassword extends StatefulWidget {
  const CampoPassword({
    super.key,
    required this.controller,
    required this.etichetta,
    this.focusNode,
    this.fieldKey,
    this.validator,
    this.onChanged,
    this.onFieldSubmitted,
    this.textInputAction,
    this.autofillHints,
    this.readOnly = false,
  });

  final TextEditingController controller;
  final String etichetta;
  final FocusNode? focusNode;

  /// Serve a rivalidare il campo "da fuori" (es. la conferma quando cambia la nuova password).
  final GlobalKey<FormFieldState<String>>? fieldKey;
  final String? Function(String?)? validator;
  final ValueChanged<String>? onChanged;
  final ValueChanged<String>? onFieldSubmitted;
  final TextInputAction? textInputAction;

  /// `AutofillHints.password` per quella attuale, `AutofillHints.newPassword` per la nuova:
  /// i gestori di password capiscono quando proporre di generarne una.
  final Iterable<String>? autofillHints;
  final bool readOnly;

  @override
  State<CampoPassword> createState() => _CampoPasswordState();
}

class _CampoPasswordState extends State<CampoPassword> {
  bool _nascosta = true;
  // L'icona occhio non deve rubare il focus al campo (il cursore resta dov'è).
  final _occhioFocus = FocusNode(canRequestFocus: false);

  @override
  void dispose() {
    _occhioFocus.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => TextFormField(
        key: widget.fieldKey,
        controller: widget.controller,
        focusNode: widget.focusNode,
        readOnly: widget.readOnly,
        obscureText: _nascosta,
        keyboardType: TextInputType.visiblePassword,
        textInputAction: widget.textInputAction,
        autofillHints: widget.autofillHints,
        autocorrect: false,
        enableSuggestions: false,
        autovalidateMode: AutovalidateMode.onUserInteraction,
        validator: widget.validator,
        onChanged: widget.onChanged,
        onFieldSubmitted: widget.onFieldSubmitted,
        decoration: InputDecoration(
          labelText: widget.etichetta,
          prefixIcon: const Icon(Icons.lock_outline),
          border: const OutlineInputBorder(),
          suffixIcon: IconButton(
            focusNode: _occhioFocus,
            tooltip: _nascosta ? 'Mostra password' : 'Nascondi password',
            icon: Icon(_nascosta ? Icons.visibility_outlined : Icons.visibility_off_outlined),
            onPressed: () => setState(() => _nascosta = !_nascosta),
          ),
        ),
      );
}
