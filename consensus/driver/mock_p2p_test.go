package driver_test

import (
	"context"
	"sync"

	"github.com/NethermindEth/juno/consensus/p2p"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
)

type mockListener[M starknet.Message] struct {
	ch chan M
}

func newMockListener[M starknet.Message](ch chan M) *mockListener[M] {
	return &mockListener[M]{
		ch: ch,
	}
}

func (m *mockListener[M]) Listen() <-chan M {
	return m.ch
}

type mockBroadcaster[M starknet.Message] struct {
	wg                  sync.WaitGroup
	mu                  sync.Mutex
	broadcastedMessages []M
}

func (m *mockBroadcaster[M]) Broadcast(msg M) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.broadcastedMessages = append(m.broadcastedMessages, msg)
	m.wg.Done()
}

type mockP2P struct {
	listeners    listeners
	broadcasters broadcasters
}

func newMockP2P(
	proposalCh chan starknet.Proposal,
	prevoteCh chan starknet.Prevote,
	precommitCh chan starknet.Precommit,
) p2p.P2P[starknet.Value, starknet.Hash, starknet.Address] {
	return &mockP2P{
		listeners: listeners{
			ProposalListener:  newMockListener(proposalCh),
			PrevoteListener:   newMockListener(prevoteCh),
			PrecommitListener: newMockListener(precommitCh),
		},
		broadcasters: broadcasters{
			ProposalBroadcaster:  &mockBroadcaster[starknet.Proposal]{},
			PrevoteBroadcaster:   &mockBroadcaster[starknet.Prevote]{},
			PrecommitBroadcaster: &mockBroadcaster[starknet.Precommit]{},
		},
	}
}

func (m *mockP2P) Run(ctx context.Context) error {
	return nil
}

func (m *mockP2P) Listeners() p2p.Listeners[starknet.Value, starknet.Hash, starknet.Address] {
	return m.listeners
}

func (m *mockP2P) Broadcasters() p2p.Broadcasters[starknet.Value, starknet.Hash, starknet.Address] {
	return m.broadcasters
}

func (m *mockP2P) OnCommit(ctx context.Context, height types.Height, value starknet.Value) {
}
