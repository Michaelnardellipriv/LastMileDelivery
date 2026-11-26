// Package interno è il contratto tra i due microservizi:
//
//   - utenti (database globale): login, utenti, clienti, sedi (con il loro indirizzo);
//   - spedizioni (database delle spedizioni): spedizioni, indirizzi di ritiro e consegna,
//     note, storico, impostazioni delle sedi, motivi e posizioni dei corrieri.
//
// I due database sono separati, quindi niente JOIN né chiavi esterne tra i due: quando
// un servizio ha bisogno di un dato dell'altro lo chiede qui, con una chiamata HTTP.
// Le rotte /interno/* non passano dall'Ingress (che espone solo /api) e ogni chiamata
// porta la chiave condivisa CHIAVE_INTERNA nell'intestazione X-Chiave-Interna.
package interno

import (
	"crypto/subtle"
	"net/http"
	"time"

	"lastmiledelivery/backend/internal/httpx"
)

const IntestazioneChiave = "X-Chiave-Interna"

// Percorsi del servizio utenti.
const (
	PercorsoRisolvi      = "/interno/risolvi"
	PercorsoDestinatario = "/interno/clienti/{id}/destinatario"
)

// Percorsi del servizio spedizioni.
const (
	PercorsoSpedizioniAperte = "/interno/corrieri/{id}/spedizioni-aperte"
	PercorsoSessioniRevocate = "/interno/sessioni/revocate"
	// PercorsoGruppi: i gruppi di sedi che hanno un database delle spedizioni (mappa
	// sede → database). Il servizio utenti lo chiede prima di creare una sede.
	PercorsoGruppi = "/interno/gruppi"
)

// Gruppi è la risposta di PercorsoGruppi.
type Gruppi struct {
	Codici []int `json:"codici"`
}

// SessioneRevocata: una sessione chiusa dal servizio utenti. Il servizio spedizioni la
// tiene finché il token non potrebbe più valere comunque (ScadeIl, fine della sessione).
//
// Ruolo e IDSede dicono dove il servizio spedizioni controlla quel token, quindi dove
// registrare la revoca: operatori e corrieri nel database del gruppo della sede del login
// (quella del token), clienti e amministratori nel principale. Una sessione di operatore o
// corriere senza sede (aperta prima della mappa delle sedi) va in tutti i gruppi.
type SessioneRevocata struct {
	ID      string    `json:"id"`
	ScadeIl time.Time `json:"scade_il"`
	Motivo  string    `json:"motivo"`
	Ruolo   string    `json:"ruolo"`
	IDSede  int64     `json:"id_sede,omitempty"`
}

type RichiestaRevoca struct {
	Sessioni []SessioneRevocata `json:"sessioni"`
}

// Richiesta elenca gli id da risolvere, per tipo. Gli id dei ruoli sono quelli delle
// tabelle clienti, corrieri, operatori_logistici e amministratori (quelli usati nelle
// spedizioni), non l'id dell'utente.
type Richiesta struct {
	Sedi           []int64 `json:"sedi,omitempty"`
	Indirizzi      []int64 `json:"indirizzi,omitempty"`
	Clienti        []int64 `json:"clienti,omitempty"`
	Corrieri       []int64 `json:"corrieri,omitempty"`
	Operatori      []int64 `json:"operatori,omitempty"`
	Amministratori []int64 `json:"amministratori,omitempty"`
}

func (r Richiesta) Vuota() bool {
	return len(r.Sedi)+len(r.Indirizzi)+len(r.Clienti)+len(r.Corrieri)+len(r.Operatori)+len(r.Amministratori) == 0
}

type Sede struct {
	Nome    string `json:"nome"`
	Attiva  bool   `json:"attiva"`
	IDCitta int64  `json:"id_citta"`
	// città della sede: è anche quella di tutti gli indirizzi di ritiro e consegna della
	// sede (che stanno nel database delle spedizioni)
	Citta   string `json:"citta"`
	Regione string `json:"regione"`
	Nazione string `json:"nazione"`
	// Gruppo: il database delle spedizioni della sede (sedi.gruppo_spedizioni). Puntatore:
	// un servizio utenti di una versione precedente non lo manda, e il servizio spedizioni
	// non deve scambiare il campo mancante per il gruppo 0 (lo terrebbe in memoria).
	Gruppo *int `json:"gruppo"`
}

type Indirizzo struct {
	Via     string `json:"via"`
	Civico  string `json:"civico"`
	CAP     string `json:"cap"`
	IDCitta int64  `json:"id_citta"`
	Citta   string `json:"citta"`
	Regione string `json:"regione"`
	Nazione string `json:"nazione"`
}

type Persona struct {
	Nome     string `json:"nome"`
	Cognome  string `json:"cognome"`
	Email    string `json:"email"`
	Telefono string `json:"telefono"` // prefisso e numero, es. "+39 3330000001"
	Attivo   bool   `json:"attivo"`
	IDSede   int64  `json:"id_sede,omitempty"` // corrieri e operatori
}

// Risposta: un id che non esiste semplicemente non c'è nella mappa.
type Risposta struct {
	Sedi           map[int64]Sede      `json:"sedi"`
	Indirizzi      map[int64]Indirizzo `json:"indirizzi"`
	Clienti        map[int64]Persona   `json:"clienti"`
	Corrieri       map[int64]Persona   `json:"corrieri"`
	Operatori      map[int64]Persona   `json:"operatori"`
	Amministratori map[int64]Persona   `json:"amministratori"`
}

// SpedizioniAperte è la risposta di PercorsoSpedizioniAperte.
type SpedizioniAperte struct {
	Aperte bool `json:"aperte"`
}

// Protetto lascia passare solo le chiamate con la chiave interna giusta. Senza chiave
// configurata le rotte interne non rispondono a nessuno.
func Protetto(chiave string) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			data := r.Header.Get(IntestazioneChiave)
			if chiave == "" || subtle.ConstantTimeCompare([]byte(data), []byte(chiave)) != 1 {
				httpx.ScriviErrore(w, http.StatusNotFound, "risorsa non trovata")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
