package integtest

import (
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
)

// Mock broadcaster for testing purposes. It has the map of all listeners and the node's own address
// to which it will not broadcast.
type broadcaster[M types.Message[value, felt.Felt, felt.Felt]] struct {
	addr      *felt.Felt
	listeners map[felt.Felt]listener[M]
}

func newBroadcaster[M types.Message[value, felt.Felt, felt.Felt]](
	addr *felt.Felt,
	listeners map[felt.Felt]listener[M],
) broadcaster[M] {
	return broadcaster[M]{
		addr:      addr,
		listeners: listeners,
	}
}

func (b broadcaster[M]) Broadcast(msg M) {
	for addr, listener := range b.listeners {
		if !addr.Equal(b.addr) {
			go func() {
				listener.ch <- msg
			}()
		}
	}
}
