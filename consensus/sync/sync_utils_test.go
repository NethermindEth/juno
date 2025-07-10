package sync_test

import (
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/consensus/db"
	"github.com/NethermindEth/juno/consensus/driver"
	"github.com/NethermindEth/juno/consensus/p2p"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/stretchr/testify/require"
)

type (
	listeners    = p2p.Listeners[starknet.Value, starknet.Hash, starknet.Address]
	broadcasters = p2p.Broadcasters[starknet.Value, starknet.Hash, starknet.Address]
	tendermintDB = db.TendermintDB[starknet.Value, starknet.Hash, starknet.Address]
	blockchain   = driver.Blockchain[starknet.Value, starknet.Hash]
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

type mockBlockchain struct {
	t *testing.T
}

func (m *mockBlockchain) Commit(height types.Height, value starknet.Value) {
}

func newMockBlockchain(t *testing.T) blockchain {
	return &mockBlockchain{
		t: t,
	}
}

func mockListeners(
	proposalCh chan starknet.Proposal,
	prevoteCh chan starknet.Prevote,
	precommitCh chan starknet.Precommit,
) listeners {
	return listeners{
		ProposalListener:  newMockListener(proposalCh),
		PrevoteListener:   newMockListener(prevoteCh),
		PrecommitListener: newMockListener(precommitCh),
	}
}

func mockBroadcasters() broadcasters {
	return broadcasters{
		ProposalBroadcaster:  &mockBroadcaster[starknet.Proposal]{},
		PrevoteBroadcaster:   &mockBroadcaster[starknet.Prevote]{},
		PrecommitBroadcaster: &mockBroadcaster[starknet.Precommit]{},
	}
}

func mockTimeoutFn(step types.Step, round types.Round) time.Duration {
	return 10 * time.Second // Don't want to trigger timeouts for this test
}

func newTendermintDB(t *testing.T) tendermintDB {
	t.Helper()
	dbPath := t.TempDir()
	pebbleDB, err := pebble.New(dbPath)
	require.NoError(t, err)

	return db.NewTendermintDB[starknet.Value, starknet.Hash, starknet.Address](pebbleDB, types.Height(0))
}
