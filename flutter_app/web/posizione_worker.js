// Web Worker dell'area corrieri (solo versione web): simula la posizione del corriere
// e la manda al backend (POST /api/corrieri/posizione) all'intervallo deciso
// dall'amministratore.
//
// Gira in un thread separato dalla pagina: quando il corriere cambia scheda o riduce il
// browser, il browser rallenta la pagina ma non questo worker, quindi gli invii
// continuano. A scheda chiusa si ferma: nell'app Android lo stesso lavoro lo farà un
// servizio in background (vedi lib/corriere/posizione.dart, InvioPosizioneTimer).
//
// Lo spostamento simulato è lo stesso di PercorsoSimulato in lib/corriere/posizione.dart:
// se si cambia uno, va cambiato anche l'altro.
//
// Messaggi dalla pagina:  {tipo: 'avvia', base, token, lat, lon, direzione, intervallo}
//                         {tipo: 'token', token}   token rinnovato (dura 15 minuti)
//                         {tipo: 'ferma'}
// Messaggi alla pagina:   {ok, status, messaggio, tecnico, lat, lon, intervallo}

let stato = null;   // null = fermo
let timer = null;

self.onmessage = (evento) => {
  const m = evento.data;
  if (m.tipo === 'avvia') {
    clearTimeout(timer);
    stato = {
      base: m.base, token: m.token,
      lat: m.lat, lon: m.lon, direzione: m.direzione,
      intervallo: m.intervallo,
    };
    invia();
  } else if (m.tipo === 'token') {
    if (stato) stato.token = m.token;
  } else if (m.tipo === 'ferma') {
    clearTimeout(timer);
    stato = null;
  }
};

// Circa 150 metri per invio, con una piccola curva.
function muovi() {
  stato.direzione += (Math.random() - 0.5) * 0.8;
  stato.lat += Math.cos(stato.direzione) * 0.0014;
  stato.lon += Math.sin(stato.direzione) * 0.0020;
}

async function invia() {
  if (!stato) return;
  const corrente = stato;
  muovi();
  const url = corrente.base + '/api/corrieri/posizione';
  let esito;
  try {
    const risposta = await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json',
        'Authorization': 'Bearer ' + corrente.token,
      },
      body: JSON.stringify({ latitudine: corrente.lat, longitudine: corrente.lon }),
    });
    const testo = await risposta.text();
    let json = {};
    try { json = JSON.parse(testo); } catch (_) { /* risposta non JSON */ }
    if (risposta.ok) {
      if (json.intervallo_secondi > 0) corrente.intervallo = json.intervallo_secondi;
      esito = { ok: true, status: risposta.status };
    } else {
      esito = {
        ok: false,
        status: risposta.status,
        messaggio: json.errore || ('errore HTTP ' + risposta.status),
        tecnico: 'POST ' + url + ' → ' + risposta.status + ': ' + testo,
      };
    }
  } catch (errore) {
    // rete assente, backend spento o CORS: nessuno status
    esito = { ok: false, status: null, messaggio: String(errore), tecnico: 'POST ' + url + ': ' + String(errore) };
  }
  if (stato !== corrente) return; // fermato (o riavviato) durante l'invio
  self.postMessage({ ...esito, lat: corrente.lat, lon: corrente.lon, intervallo: corrente.intervallo });
  if (esito.status === 401 || esito.status === 403) {
    // sessione scaduta o account disattivato: la pagina verifica con il rinnovo e decide
    // se riportare al login o ripartire
    stato = null;
    return;
  }
  timer = setTimeout(invia, corrente.intervallo * 1000);
}
