// Validatori dei form (login, recupero password, cambio password).
//
// Funzioni pure, senza widget: si testano senza UI (vedi test/profilo_test.dart).
import 'dart:convert';

/// Stesso criterio del CHECK `ck_utenti_email` del database: così il client non rifiuta
/// email che il backend accetterebbe, e non ne accetta che il database rifiuterebbe.
final formatoEmail = RegExp(r'^[^@\s]+@[^@\s]+\.[^@\s]+$');

String? validaEmail(String? v) {
  final t = (v ?? '').trim();
  if (t.isEmpty) return 'Inserisci la tua email';
  if (t.length > 254 || !formatoEmail.hasMatch(t)) return 'Formato email non valido';
  return null;
}

// ───────── Regole della nuova password ─────────
// Identiche a controllaPassword() in backend/internal/autenticazione/password.go:
//   obbligatoria · non solo spazi · almeno 8 caratteri (rune) · al massimo 72 byte (limite di bcrypt)
// e, nel cambio password, diversa da quella attuale. NON esistono requisiti di lettere/numeri:
// il client non deve essere più severo del backend. Se il backend cambia, allineare qui.

const passwordMinima = 8;
const passwordMassimaByte = 72;

class RequisitoPassword {
  const RequisitoPassword(this.etichetta, this.errore, this.soddisfatto);

  /// Testo nell'elenco dei requisiti sotto il campo.
  final String etichetta;

  /// Testo dell'errore di validazione quando il requisito manca.
  final String errore;
  final bool Function(String password) soddisfatto;
}

final requisitiPassword = <RequisitoPassword>[
  RequisitoPassword('Almeno $passwordMinima caratteri', 'Usa almeno $passwordMinima caratteri',
      (p) => p.runes.length >= passwordMinima),
  RequisitoPassword('Al massimo $passwordMassimaByte byte', 'Troppo lunga (massimo $passwordMassimaByte byte)',
      (p) => utf8.encode(p).length <= passwordMassimaByte),
];

/// [attuale]: se la nuova password coincide con quella attuale, il backend risponde 400.
String? validaNuovaPassword(String? v, {String? attuale}) {
  if (v == null || v.isEmpty) return 'Inserisci la nuova password';
  if (v.trim().isEmpty) return 'Non può essere vuota';
  for (final r in requisitiPassword) {
    if (!r.soddisfatto(v)) return r.errore;
  }
  if (attuale != null && attuale.isNotEmpty && v == attuale) return 'Deve essere diversa da quella attuale';
  return null;
}

String? validaConferma(String? v, String nuova) {
  if (v == null || v.isEmpty) return 'Conferma la nuova password';
  if (v != nuova) return 'Le password non coincidono';
  return null;
}
