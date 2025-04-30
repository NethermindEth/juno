package integ

import (
	"github.com/NethermindEth/juno/consensus/tendermint"
	"github.com/NethermindEth/juno/core/felt"
)

type listener[M tendermint.Message[value, felt.Felt, felt.Felt]] struct {
	ch chan M
}

func newListener[M tendermint.Message[value, felt.Felt, felt.Felt]](buffer int) listener[M] {
	return listener[M]{
		ch: make(chan M, buffer),
	}
}

func (l listener[M]) Listen() <-chan M {
	return l.ch
}
