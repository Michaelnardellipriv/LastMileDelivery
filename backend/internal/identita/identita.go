package identita

import "context"

type Ruolo string

const (
	Operatore      Ruolo = "operatore"
	Corriere       Ruolo = "corriere"
	Cliente        Ruolo = "cliente"
	Amministratore Ruolo = "amministratore"
)

func (r Ruolo) Nome() string {
	switch r {
	case Operatore:
		return "operatore logistico"
	case Corriere:
		return "corriere"
	case Cliente:
		return "cliente"
	}
	return "amministratore"
}

type Richiedente struct {
	Ruolo Ruolo
	ID    int32
}

type chiaveContesto struct{}

func NelContesto(ctx context.Context, r Richiedente) context.Context {
	return context.WithValue(ctx, chiaveContesto{}, r)
}

func DalContesto(ctx context.Context) Richiedente {
	r, _ := ctx.Value(chiaveContesto{}).(Richiedente)
	return r
}
