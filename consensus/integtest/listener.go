package integtest

import (
	"github.com/NethermindEth/juno/consensus/p2p"
	"github.com/NethermindEth/juno/consensus/starknet"
)

type (
	Listener[M starknet.Message] = p2p.Listener[M, starknet.Value, starknet.Hash, starknet.Address]
	Listeners                    = p2p.Listeners[starknet.Value, starknet.Hash, starknet.Address]
)

type listener[M starknet.Message] struct {
	ch chan M
}

func newListener[M starknet.Message](buffer int) listener[M] {
	return listener[M]{
		ch: make(chan M, buffer),
	}
}

func (l listener[M]) Listen() <-chan M {
	return l.ch
}
