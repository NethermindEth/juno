package proposal_test

import (
	"math/rand/v2"
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

func createTestBuildResult() *builder.BuildResult {
	return buildResultAtHeight(rand.Uint64())
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
			value: createTestBuildResult(),
		},
		{
			name:  "store and get multiple values",
			key:   felt.FromUint64[starknet.Hash](2),
			value: createTestBuildResult(),
		},
		{
			name:  "store with zero hash key",
			key:   felt.FromUint64[starknet.Hash](0),
			value: createTestBuildResult(),
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

func TestProposalStore_Delete(t *testing.T) {
	store := &proposal.ProposalStore[starknet.Hash]{}
	key := felt.FromUint64[starknet.Hash](1)

	store.Store(key, buildResultAtHeight(42))
	require.NotNil(t, store.Get(key))

	store.Delete(key)
	require.Nil(t, store.Get(key))
}

func TestProposalStore_DeleteByHeight(t *testing.T) {
	store := &proposal.ProposalStore[starknet.Hash]{}

	keyA1 := felt.FromUint64[starknet.Hash](10)
	keyA2 := felt.FromUint64[starknet.Hash](11)
	keyB := felt.FromUint64[starknet.Hash](20)

	store.Store(keyA1, buildResultAtHeight(7))
	store.Store(keyA2, buildResultAtHeight(7))
	store.Store(keyB, buildResultAtHeight(8))

	store.DeleteByHeight(types.Height(7))

	require.Nil(t, store.Get(keyA1))
	require.Nil(t, store.Get(keyA2))
	require.NotNil(t, store.Get(keyB))
}

func TestProposalStore_DeleteByHeight_UnknownHeightIsNoop(t *testing.T) {
	store := &proposal.ProposalStore[starknet.Hash]{}
	key := felt.FromUint64[starknet.Hash](1)
	store.Store(key, buildResultAtHeight(5))

	store.DeleteByHeight(types.Height(99))

	require.NotNil(t, store.Get(key))
}

func TestProposalStore_DeleteByHeight_AllowsReuseOfHeight(t *testing.T) {
	store := &proposal.ProposalStore[starknet.Hash]{}
	key := felt.FromUint64[starknet.Hash](1)

	store.Store(key, buildResultAtHeight(5))
	store.DeleteByHeight(types.Height(5))
	require.Nil(t, store.Get(key))

	store.Store(key, buildResultAtHeight(5))
	require.NotNil(t, store.Get(key))
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
		value := createTestBuildResult()

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

// TestProposalStore_ConcurrentStoreAndDeleteByHeight exercises the cross-map invariant
// between byHash and byHeight under concurrent writes, height-level deletes, and reads.
// Intended to be run with -race; the runtime catches map corruption and the race detector
// catches unsynchronised access to the two internal maps.
func TestProposalStore_ConcurrentStoreAndDeleteByHeight(t *testing.T) {
	const (
		heights       = 50
		keysPerHeight = 20
		readers       = 200
		readOps       = 100
	)

	store := &proposal.ProposalStore[starknet.Hash]{}
	wg := conc.NewWaitGroup()

	for h := range heights {
		for k := range keysPerHeight {
			wg.Go(func() {
				key := felt.FromUint64[starknet.Hash](uint64(h*keysPerHeight + k))
				store.Store(key, buildResultAtHeight(uint64(h)))
			})
		}
	}

	for h := range heights {
		wg.Go(func() {
			store.DeleteByHeight(types.Height(h))
		})
	}

	for range readers {
		wg.Go(func() {
			for range readOps {
				key := felt.FromUint64[starknet.Hash](rand.Uint64N(heights * keysPerHeight))
				_ = store.Get(key)
			}
		})
	}

	wg.Wait()
}
