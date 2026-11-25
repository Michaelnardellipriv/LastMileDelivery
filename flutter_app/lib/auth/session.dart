// Persistenza della sessione di login: tutto il corpo di POST /api/auth/login (token JWT, scade_il,
// utente.id_utente, utente.id, ruolo, nome, cognome, email). Il backend NON emette refresh token.
//
// Il token NON va in shared_preferences (testo in chiaro): si usa flutter_secure_storage,
// che su Android usa EncryptedSharedPreferences/Keystore, su iOS il Keychain.
// Su Web salva cifrato in localStorage (richiede https o localhost): protegge da letture
// casuali, non da un XSS. In produzione web è preferibile un cookie HttpOnly emesso dal backend.
import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import '../api.dart';
import 'package:flutter/widgets.dart';     // Widget, BuildContext, State, Navigator, ...
// ValueNotifier, Listenable, ...

abstract class SessioneStorage {
  Future<Sessione?> leggi();
  Future<void> salva(Sessione s);
  Future<void> cancella();
}

/// Implementazione di produzione. Ogni errore di storage è assorbito: nel peggiore dei casi
/// la sessione resta solo in memoria e al prossimo avvio si rifà il login.
class SecureSessioneStorage implements SessioneStorage {
  // v2: formato con tutti i campi del backend; i salvataggi vecchi (v1) non si leggono e si ignorano.
  static const _chiave = 'veloce24.sessione.v2';
  static const _chiaveVecchia = 'veloce24.sessione';
  final FlutterSecureStorage _s;

  SecureSessioneStorage({FlutterSecureStorage? storage})
      : _s = storage ?? const FlutterSecureStorage(aOptions: AndroidOptions(encryptedSharedPreferences: true));

  @override
  Future<Sessione?> leggi() async {
    try {
      final testo = await _s.read(key: _chiave);
      if (testo == null) return null;
      final j = jsonDecode(testo);
      return j is Map<String, dynamic> ? Sessione.daJsonSalvato(j) : null;
    } catch (e) {
      debugPrint('Sessione non leggibile, la ignoro: ${e.runtimeType}'); // mai loggare il token
      return null;
    }
  }

  @override
  Future<void> salva(Sessione s) async {
    try {
      // ── JWT: salvataggio cifrato ──
      await _s.write(key: _chiave, value: jsonEncode(s.toJson()));
    } catch (e) {
      debugPrint('Sessione non salvata: ${e.runtimeType}');
    }
  }

  @override
  Future<void> cancella() async {
    try {
      await _s.delete(key: _chiave);
      await _s.delete(key: _chiaveVecchia);
    } catch (e) {
      debugPrint('Sessione non cancellata: ${e.runtimeType}');
    }
  }
}

/// Solo per i test: niente plugin, niente disco.
class MemoriaSessioneStorage implements SessioneStorage {
  Sessione? _s;
  @override
  Future<Sessione?> leggi() async => _s;
  @override
  Future<void> salva(Sessione s) async => _s = s;
  @override
  Future<void> cancella() async => _s = null;
}
