// Validatori dei form di autenticazione (login, recupero password, cambio password).
//
// SCELTA DI PROGETTO: la validazione dei dati (formato email, lunghezza e regole della password,
// "diversa da quella attuale", token valido...) la fa SOLO il backend. Il client non blocca mai
// l'invio per questi motivi: manda i dati così come sono e mostra l'errore 400 del backend (con i
// `dettagli` per campo) sotto il campo giusto. Così le regole hanno una sola fonte di verità
// (backend/internal/autenticazione/password.go) e non possono divergere dal client.
//
// Questo file resta solo per non rompere gli import esistenti: i validatori per i dati sono
// "passanti" (restituiscono sempre null, cioè nessun errore).
//
// Unica eccezione, voluta: [validaConferma]. Il campo "conferma password" non viene mai inviato
// al backend, quindi il backend non può controllarlo; è un controllo di digitazione, non una
// validazione del dato.

/// Non blocca: il formato dell'email lo valuta il backend.
String? validaEmail(String? v) => null;

/// Non blocca: lunghezza e regole della nuova password le valuta il backend.
/// [attuale] è accettato solo per compatibilità con i chiamanti esistenti.
String? validaNuovaPassword(String? v, {String? attuale}) => null;

/// Controllo di digitazione (non di dominio): la conferma deve coincidere con la nuova password.
/// Il backend non riceve questo campo, quindi non può verificarlo.
String? validaConferma(String? v, String nuova) => v == nuova ? null : 'Le password non coincidono';
