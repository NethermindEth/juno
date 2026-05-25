package proposal_test

import (
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/proposal"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/sourcegraph/conc"
	"github.com/stretchr/testify/require"
)

const (
	keyCount = 1000
	opCount  = 100
)

func buildResultAtHeight(blockNumber uint64) *builder.BuildResult {
	return &builder.BuildResult{
		PreConfirmed: &pending.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number: blockNumber,
				},
			},
		},
		SimulateResult: &blockchain.SimulateResult{
			BlockCommitments: &core.BlockCommitments{
				TransactionCommitment: new(felt.Felt).SetUint64(1),
				EventCommitment:       new(felt.Felt).SetUint64(2),
				ReceiptCommitment:     new(felt.Felt).SetUint64(3),
				StateDiffCommitment:   new(felt.Felt).SetUint64(4),
			},
		},
		L2GasConsumed: 5,
	}
}

func TestProposalStore_StoreAndGet(t *testing.T) {
	store := &proposal.ProposalStore[starknet.Hash]{}

	tests := []struct {
		name  string
		key   starknet.Hash
		value *builder.BuildResult
	}{
		{
			name:  "store and get single value",
			key:   felt.FromUint64[starknet.Hash](1),
			value: buildResultAtHeight(100),
		},
		{
			name:  "store and get multiple values",
			key:   felt.FromUint64[starknet.Hash](2),
			value: buildResultAtHeight(101),
		},
		{
			name:  "store with zero hash key",
			key:   felt.FromUint64[starknet.Hash](0),
			value: buildResultAtHeight(102),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Nil(t, store.Get(tt.key))

			store.Store(tt.key, tt.value)

			result := store.Get(tt.key)
			require.NotNil(t, result)
			require.Equal(t, result, tt.value)
		})
	}
}

func TestProposalStore_StoreNilValue(t *testing.T) {
	store := &proposal.ProposalStore[starknet.Hash]{}
	key := felt.FromUint64[starknet.Hash](1)

	store.Store(key, nil)

	require.Nil(t, store.Get(key))
}

func TestProposalStore_FinalizeHeight(t *testing.T) {
	store := &proposal.ProposalStore[starknet.Hash]{}

	keyA1 := felt.FromUint64[starknet.Hash](10)
	keyA2 := felt.FromUint64[starknet.Hash](11)
	keyB := felt.FromUint64[starknet.Hash](20)

	store.Store(keyA1, buildResultAtHeight(7))
	store.Store(keyA2, buildResultAtHeight(7))
	store.Store(keyB, buildResultAtHeight(8))

	store.FinalizeHeight(types.Height(7))

	require.Nil(t, store.Get(keyA1))
	require.Nil(t, store.Get(keyA2))
	require.NotNil(t, store.Get(keyB))
}

func TestProposalStore_FinalizedGuard(t *testing.T) {
	store := &proposal.ProposalStore[starknet.Hash]{}

	currentKey := felt.FromUint64[starknet.Hash](1)
	stragglerKey := felt.FromUint64[starknet.Hash](2)
	belowKey := felt.FromUint64[starknet.Hash](3)
	futureKey := felt.FromUint64[starknet.Hash](4)

	store.Store(currentKey, buildResultAtHeight(7))
	store.FinalizeHeight(types.Height(7))

	store.Store(stragglerKey, buildResultAtHeight(7))
	store.Store(belowKey, buildResultAtHeight(5))
	require.Nil(t, store.Get(stragglerKey))
	require.Nil(t, store.Get(belowKey))

	store.Store(futureKey, buildResultAtHeight(9))
	require.NotNil(t, store.Get(futureKey))
}

func TestProposalStore_IsFinalized(t *testing.T) {
	store := &proposal.ProposalStore[starknet.Hash]{}

	// Cursor zero-value means height 0 is finalized from the start (genesis).
	require.True(t, store.IsFinalized(types.Height(0)))
	require.False(t, store.IsFinalized(types.Height(1)))
	require.False(t, store.IsFinalized(types.Height(10)))

	store.FinalizeHeight(types.Height(5))

	require.True(t, store.IsFinalized(types.Height(5)))
	require.False(t, store.IsFinalized(types.Height(6)))
}

func TestProposalStore_FinalizeHeight_CursorMonotonic(t *testing.T) {
	store := &proposal.ProposalStore[starknet.Hash]{}

	store.FinalizeHeight(types.Height(10))
	store.FinalizeHeight(types.Height(5))

	stragglerAt8 := felt.FromUint64[starknet.Hash](1)
	store.Store(stragglerAt8, buildResultAtHeight(8))
	require.Nil(t, store.Get(stragglerAt8))
}

func doNTimes(n int, f func(i int)) {
	wg := conc.NewWaitGroup()
	for i := range n {
		wg.Go(func() {
			f(i)
		})
	}
	wg.Wait()
}

func TestProposalStore_ConcurrentAccess(t *testing.T) {
	store := &proposal.ProposalStore[starknet.Hash]{}

	doNTimes(keyCount, func(i int) {
		key := felt.FromUint64[starknet.Hash](uint64(i))
		value := buildResultAtHeight(uint64(i) + 1)

		doNTimes(opCount, func(_ int) {
			require.Nil(t, store.Get(key))
		})

		store.Store(key, value)

		doNTimes(opCount, func(_ int) {
			result := store.Get(key)
			require.NotNil(t, result)
			require.Equal(t, result, value)
		})
	})
}

func TestProposalStore_ConcurrentStoreAndFinalizeHeight(t *testing.T) {
	store := &proposal.ProposalStore[starknet.Hash]{}

	const (
		writers      = 8
		writesEach   = 500
		heightWindow = 32
	)

	wg := conc.NewWaitGroup()
	for w := range writers {
		wg.Go(func() {
			for i := range writesEach {
				h := uint64(i % heightWindow)
				k := felt.FromUint64[starknet.Hash](uint64(w*writesEach + i))
				store.Store(k, buildResultAtHeight(h))
			}
		})
	}
	wg.Go(func() {
		for i := range writesEach {
			store.FinalizeHeight(types.Height(uint64(i % heightWindow)))
		}
	})
	wg.Wait()

	store.FinalizeHeight(types.Height(heightWindow))
	for w := range writers {
		for i := range writesEach {
			k := felt.FromUint64[starknet.Hash](uint64(w*writesEach + i))
			require.Nil(t, store.Get(k))
		}
	}
}
