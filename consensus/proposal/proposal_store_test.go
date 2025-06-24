package proposal

import (
	"math/rand/v2"
	"testing"

	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	syncpkg "github.com/NethermindEth/juno/sync"
	"github.com/sourcegraph/conc"
	"github.com/stretchr/testify/require"
)

const (
	keyCount = 1000
	opCount  = 100
)

// createTestHash creates a test Hash for testing purposes
func createTestHash(value uint64) starknet.Hash {
	f := new(felt.Felt).SetUint64(value)
	return starknet.Hash(*f)
}

// createTestBuildResult creates a test BuildResult for testing purposes
func createTestBuildResult() *builder.BuildResult {
	blockNumber := rand.Uint64()
	return &builder.BuildResult{
		Pending: syncpkg.Pending{
			Block: &core.Block{
				Header: &core.Header{
					Number: blockNumber,
				},
			},
		},
		ProposalCommitment: types.ProposalCommitment{
			BlockNumber: blockNumber,
		},
	}
}

func TestProposalStore_StoreAndGet(t *testing.T) {
	store := &ProposalStore[starknet.Hash]{}

	tests := []struct {
		name     string
		key      starknet.Hash
		value    *builder.BuildResult
		expected *builder.BuildResult
	}{
		{
			name:  "store and get single value",
			key:   createTestHash(1),
			value: createTestBuildResult(),
		},
		{
			name:  "store and get multiple values",
			key:   createTestHash(2),
			value: createTestBuildResult(),
		},
		{
			name:  "store with zero hash key",
			key:   starknet.Hash{},
			value: createTestBuildResult(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Nil(t, store.Get(tt.key))

			// Store the value
			store.Store(tt.key, tt.value)

			// Get the value
			result := store.Get(tt.key)
			require.NotNil(t, result)
			require.Equal(t, result, tt.value)
		})
	}
}

func TestProposalStore_StoreNilValue(t *testing.T) {
	store := &ProposalStore[starknet.Hash]{}
	key := createTestHash(1)

	// Store nil value
	store.Store(key, nil)

	// Get the value
	require.Nil(t, store.Get(key))
}

func TestProposalStore_EmptyBuildResult(t *testing.T) {
	store := &ProposalStore[starknet.Hash]{}
	key := createTestHash(1)

	// Create an empty BuildResult
	emptyResult := &builder.BuildResult{}

	// Store the empty result
	store.Store(key, emptyResult)

	// Get the result
	result := store.Get(key)
	require.NotNil(t, result)
	require.Equal(t, result, emptyResult)
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
	store := &ProposalStore[starknet.Hash]{}

	doNTimes(keyCount, func(i int) {
		key := createTestHash(uint64(i))
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
