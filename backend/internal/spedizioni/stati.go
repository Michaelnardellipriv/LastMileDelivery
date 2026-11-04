package spedizioni

type Stato string

const (
	Creata        Stato = "creata"
	Assegnata     Stato = "assegnata"
	Ritirata      Stato = "ritirata"
	Persa         Stato = "persa"
	InConsegna    Stato = "in consegna"
	Consegnata    Stato = "consegnata"
	NonConsegnata Stato = "non consegnata"
	Annullata     Stato = "annullata"
)

var Stati = []Stato{
	Creata, Assegnata, Ritirata, Persa, InConsegna, Consegnata, NonConsegnata, Annullata,
}

func (s Stato) Valido() bool {
	for _, x := range Stati {
		if s == x {
			return true
		}
	}
	return false
}

func (s Stato) Finale() bool {
	switch s {
	case Persa, Consegnata, NonConsegnata, Annullata:
		return true
	}
	return false
}

func (s Stato) Modificabile() bool {
	return s == Creata || s == Assegnata
}

func (s Stato) AmmetteReso() bool {
	return s == Annullata || s == Persa || s == NonConsegnata
}

func (s Stato) String() string { return string(s) }

type TipoTentativo string

const (
	TentativoRitiro   TipoTentativo = "ritiro"
	TentativoConsegna TipoTentativo = "consegna"
)

func (t TipoTentativo) Valido() bool {
	return t == TentativoRitiro || t == TentativoConsegna
}
