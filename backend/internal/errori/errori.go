package errori

type ErroreCampo struct {
	Campo     string `json:"campo"`
	Messaggio string `json:"messaggio"`
}

type Validazione struct {
	Dettagli []ErroreCampo
}

func (e *Validazione) Error() string { return "dati non validi" }

func (e *Validazione) Aggiungi(campo, messaggio string) {
	e.Dettagli = append(e.Dettagli, ErroreCampo{campo, messaggio})
}

func (e *Validazione) Segnalato(campo string) bool {
	for _, d := range e.Dettagli {
		if d.Campo == campo {
			return true
		}
	}
	return false
}

func (e *Validazione) Err() error {
	if len(e.Dettagli) > 0 {
		return e
	}
	return nil
}

type RiferimentoNonTrovato struct {
	Campo     string
	Messaggio string
}

func (e *RiferimentoNonTrovato) Error() string { return e.Campo + ": " + e.Messaggio }

type NonAutorizzato struct {
	Messaggio string
}

func (e *NonAutorizzato) Error() string { return e.Messaggio }

type NonTrovato struct {
	Messaggio string
}

func (e *NonTrovato) Error() string { return e.Messaggio }

type Conflitto struct {
	Messaggio string
	Dettagli  []ErroreCampo
}

func (e *Conflitto) Error() string { return e.Messaggio }

type TroppoPresto struct {
	Messaggio         string
	RiprovaTraSecondi int
}

func (e *TroppoPresto) Error() string { return e.Messaggio }
