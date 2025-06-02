package integtest

import (
	"github.com/NethermindEth/juno/consensus/p2p"
	"github.com/NethermindEth/juno/consensus/starknet"
)

type (
	Broadcaster[M starknet.Message] = p2p.Broadcaster[M, starknet.Value, starknet.Hash, starknet.Address]
	Broadcasters                    = p2p.Broadcasters[starknet.Value, starknet.Hash, starknet.Address]
)

// Mock broadcaster for testing purposes. It has the map of all listeners and the node's own address
// to which it will not broadcast.
type broadcaster[M starknet.Message] struct {
	addr      *starknet.Address
	listeners map[starknet.Address]listener[M]
}

func newBroadcaster[M starknet.Message](
	addr *starknet.Address,
	listeners map[starknet.Address]listener[M],
) broadcaster[M] {
	return broadcaster[M]{
		addr:      addr,
		listeners: listeners,
	}
}

func (b broadcaster[M]) Broadcast(msg M) {
	for addr, listener := range b.listeners {
		if !addr.AsFelt().Equal(b.addr.AsFelt()) {
			go func() {
				listener.ch <- msg
			}()
		}
	}
}
