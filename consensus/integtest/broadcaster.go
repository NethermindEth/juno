package integtest

import (
	"github.com/NethermindEth/juno/consensus/p2p"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
)

type (
	Broadcaster[M starknet.Message] = p2p.Broadcaster[M, starknet.Value]
	Broadcasters                    = p2p.Broadcasters[starknet.Value]
)

// Mock broadcaster for testing purposes. It has the map of all listeners and the node's own address
// to which it will not broadcast.
type broadcaster[M starknet.Message] struct {
	addr      *types.Addr
	listeners map[types.Addr]listener[M]
}

func newBroadcaster[M starknet.Message](
	addr *types.Addr,
	listeners map[types.Addr]listener[M],
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
