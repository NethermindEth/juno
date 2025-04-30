package integ

import (
	"github.com/NethermindEth/juno/consensus/tendermint"
	"github.com/NethermindEth/juno/core/felt"
)

type broadcaster[M tendermint.Message[value, felt.Felt, felt.Felt]] struct {
	addr      felt.Felt
	listeners map[felt.Felt]listener[M]
}

func newBroadcaster[M tendermint.Message[value, felt.Felt, felt.Felt]](
	addr felt.Felt,
	listeners map[felt.Felt]listener[M],
) *broadcaster[M] {
	return &broadcaster[M]{
		addr:      addr,
		listeners: listeners,
	}
}

func (b broadcaster[M]) Broadcast(msg M) {
	for addr, listener := range b.listeners {
		if addr != b.addr {
			go func() {
				listener.ch <- msg
			}()
		}
	}
}
