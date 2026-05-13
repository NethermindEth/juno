package core_test

import (
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/testutils"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/db/pebblev2"
	"github.com/NethermindEth/juno/encoder"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/bits-and-blooms/bitset"
	"github.com/bits-and-blooms/bloom/v3"
	"github.com/stretchr/testify/require"
)

func testBloomWithRandomKeys(t *testing.T, numKeys uint) *bloom.BloomFilter {
	t.Helper()
	filter := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
	for range numKeys {
		key := felt.NewRandom[felt.Felt]()
		filter.Add(key.Marshal())
	}
	return filter
}

func testBloomWithKeys(t *testing.T, keys [][]byte) *bloom.BloomFilter {
	t.Helper()
	filter := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
	for _, key := range keys {
		filter.Add(key)
	}
	return filter
}

func populateRunningFilter(t *testing.T, db db.KeyValueStore, toBlock uint64) *core.RunningEventFilter {
	t.Helper()
	filter := core.NewAggregatedFilter(0)
	rf := core.NewRunningEventFilterHot(db, &filter, 0)
	for i := range toBlock + 1 {
		require.NoError(t, rf.Insert(testBloomWithRandomKeys(t, 3), i))
	}
	return rf
}

func TestRunningEventFilter_HotInitialization(t *testing.T) {
	testDB := memory.New()
	filter := core.NewAggregatedFilter(0)
	f := core.NewRunningEventFilterHot(testDB, &filter, 0)
	require.Equal(t, uint64(0), f.FromBlock())
	require.Equal(t, filter.ToBlock(), f.ToBlock())
	require.Equal(t, uint64(0), f.NextBlock())
}

func TestRunningEventFilter_LazyInitialization_EmptyDB(t *testing.T) {
	testDB := memory.New()
	rf := core.NewRunningEventFilterLazy(testDB, core.InitializeRunningEventFilter)
	require.Equal(t, uint64(0), rf.FromBlock())
	require.Equal(t, core.NumBlocksPerFilter-1, rf.ToBlock())
	require.NoError(t, rf.Insert(testBloomWithRandomKeys(t, 1), 0))
}

func TestRunningEventFilter_LazyInitialization_Preload(t *testing.T) {
	testDB := memory.New()
	n := &networks.Sepolia
	chain := blockchain.New(
		testDB,
		n,
		blockchain.WithNewState(statetestutils.UseNewState()),
	)
	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)

	var expectedNext uint64 = 7
	for i := range expectedNext {
		b, err := gw.BlockByNumber(t.Context(), i)
		require.NoError(t, err)
		s, err := gw.StateUpdate(t.Context(), i)
		require.NoError(t, err)
		require.NoError(t, chain.Store(b, &core.BlockCommitments{}, s, nil))
	}

	t.Run("Load from DB when running filter upto date", func(t *testing.T) {
		require.NoError(t, chain.WriteRunningEventFilter())

		rf := core.NewRunningEventFilterLazy(testDB, core.InitializeRunningEventFilter)
		require.Equal(t, uint64(0), rf.FromBlock())
		require.Equal(t, core.NumBlocksPerFilter-1, rf.ToBlock())
		require.Equal(t, expectedNext, rf.NextBlock())
	})

	t.Run("Should panic when couldn't initialise", func(t *testing.T) {
		testDB.Close()
		rf := core.NewRunningEventFilterLazy(testDB, core.InitializeRunningEventFilter)
		require.Panics(t, func() {
			rf.FromBlock()
		})
	})

	t.Run("Rebuild filters when out-dated or not persisted", func(t *testing.T) {
		// Same-window resume keeps the snapshot's bitmap and only fills the
		// gap. We stamp the snapshot's [0, staleNext) bits with synthetic
		// keys not present in the real Sepolia headers — if a rebuild ran
		// instead, those positions would be refilled from header blooms and
		// the synthetic keys would no longer hit.
		t.Run("Single-window resume preserves bitmap", func(t *testing.T) {
			const staleNext uint64 = 3
			snapshotKeys := [][]byte{{0x77, 0x88}}
			filter := core.NewAggregatedFilter(0)
			snap := core.NewRunningEventFilterHot(testDB, &filter, 0)
			for i := range staleNext {
				require.NoError(t, snap.Insert(testBloomWithKeys(t, snapshotKeys), i))
			}
			require.NoError(t, core.WriteRunningEventFilter(testDB, snap))

			rf := core.NewRunningEventFilterLazy(testDB, core.InitializeRunningEventFilter)
			require.Equal(t, expectedNext, rf.NextBlock(), "must catch up to chain height + 1")
			require.Equal(t, uint64(0), rf.FromBlock())
			require.Equal(t, core.NumBlocksPerFilter-1, rf.ToBlock())

			preserved := rf.BlocksForKeys(snapshotKeys)
			for i := range staleNext {
				require.True(t, preserved.Test(uint(i)),
					"snapshot bit at block %d not preserved", i)
			}
		})

		t.Run("Multi-window rebuild rotates and persists full windows", func(t *testing.T) {
			const targetBatchByteSize = 96 * db.Megabyte
			testDB, err := pebblev2.New(t.TempDir())
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, testDB.Close()) })
			latest := core.NumBlocksPerFilter + 5

			headerKeys := [][]byte{{0xEE}}
			batch := testDB.NewBatch()
			for i := uint64(0); i <= latest; i++ {
				header := &core.Header{
					Number:      i,
					Hash:        felt.NewRandom[felt.Felt](),
					EventsBloom: testBloomWithKeys(t, headerKeys),
				}
				require.NoError(t, core.WriteBlockHeaderByNumber(batch, header))
				if batch.Size() >= targetBatchByteSize {
					require.NoError(t, batch.Write())
					batch = testDB.NewBatch()
				}
			}
			require.NoError(t, batch.Write())
			require.NoError(t, core.WriteChainHeight(testDB, latest))

			rf := core.NewRunningEventFilterLazy(testDB, core.InitializeRunningEventFilter)
			require.Equal(t, core.NumBlocksPerFilter, rf.FromBlock(),
				"window rotated to second window during fill")
			require.Equal(t, 2*core.NumBlocksPerFilter-1, rf.ToBlock())
			require.Equal(t, latest+1, rf.NextBlock())

			// Second window: every position in [N, latest] hit.
			filled := rf.BlocksForKeys(headerKeys)
			for i := core.NumBlocksPerFilter; i <= latest; i++ {
				require.True(t, filled.Test(uint(i-rf.FromBlock())),
					"block %d (second window) not filled", i)
			}
			// First window persisted on rotation: every position [0, N-1] hit.
			stored, err := core.GetAggregatedBloomFilter(testDB, 0, core.NumBlocksPerFilter-1)
			require.NoError(t, err)
			storedMatches := stored.BlocksForKeys(headerKeys)
			for i := range core.NumBlocksPerFilter {
				require.True(t, storedMatches.Test(uint(i)),
					"persisted block %d (first window) not filled", i)
			}
		})
	})
}

func TestRunningEventFilter_InsertAndPersist(t *testing.T) {
	testDB := memory.New()
	filter := core.NewAggregatedFilter(0)
	rf := core.NewRunningEventFilterHot(testDB, &filter, 0)
	for block := uint64(0); block <= filter.ToBlock(); block++ {
		err := rf.Insert(testBloomWithRandomKeys(t, 1), block)
		require.NoError(t, err)
	}
	require.Equal(t, filter.ToBlock()+1, rf.FromBlock())

	fetchedFilter, err := core.GetAggregatedBloomFilter(testDB, filter.FromBlock(), filter.ToBlock())
	require.NoError(t, err)
	require.Equal(t, filter, fetchedFilter)
}

func TestRunningEventFilter_BlocksForKeys(t *testing.T) {
	keys := [][]byte{{0x1}, {0x2}}
	testDB := memory.New()
	filter := core.NewAggregatedFilter(0)
	rf := core.NewRunningEventFilterHot(testDB, &filter, 0)
	// Add bloom with event for block 0
	bloom := testBloomWithKeys(t, keys)
	require.NoError(t, rf.Insert(bloom, 0))
	got := rf.BlocksForKeys(keys)
	require.True(t, got.Test(0))
}

func TestRunningEventFilter_BlocksForKeysInto(t *testing.T) {
	keys := [][]byte{{0x1}, {0x2}}
	testDB := memory.New()
	filter := core.NewAggregatedFilter(0)
	rf := core.NewRunningEventFilterHot(testDB, &filter, 0)
	// Add bloom with event for block 0
	bloom := testBloomWithKeys(t, keys)
	require.NoError(t, rf.Insert(bloom, 0))
	matchesBuf := *bitset.New(uint(core.NumBlocksPerFilter))
	require.NoError(t, rf.BlocksForKeysInto(keys, &matchesBuf))
	require.True(t, matchesBuf.Test(0))
}

func TestRunningEventFilter_Clone(t *testing.T) {
	testDB := memory.New()
	filter := core.NewAggregatedFilter(0)
	require.NoError(t, filter.Insert(testBloomWithRandomKeys(t, 5), 0))
	rf := core.NewRunningEventFilterHot(testDB, &filter, 0)
	clone := rf.Clone()
	require.NotSame(t, rf, clone)
	require.NotSame(t, rf.InnerFilter(), clone.InnerFilter())
	require.Equal(t, rf.FromBlock(), clone.FromBlock())
	require.Equal(t, rf, clone)
}

func TestRunningEventFilter_Reorg(t *testing.T) {
	t.Run("Reorg within current running filter", func(t *testing.T) {
		testDB := memory.New()
		rf := populateRunningFilter(t, testDB, 5)
		require.Equal(t, uint64(6), rf.NextBlock())
		require.NoError(t, rf.OnReorg())
		require.Equal(t, uint64(5), rf.NextBlock())
	})
	t.Run("Reorg spanning previous aggregation range", func(t *testing.T) {
		testDB := memory.New()
		rf := populateRunningFilter(t, testDB, core.NumBlocksPerFilter-1)
		require.Equal(t, core.NumBlocksPerFilter, rf.NextBlock())
		require.Equal(t, core.NumBlocksPerFilter, rf.InnerFilter().FromBlock())
		require.Equal(t, 2*core.NumBlocksPerFilter-1, rf.InnerFilter().ToBlock())

		require.NoError(t, rf.OnReorg())
		require.Equal(t, core.NumBlocksPerFilter-1, rf.NextBlock())
		require.Equal(t, core.NumBlocksPerFilter-1, rf.InnerFilter().ToBlock())
		require.Equal(t, uint64(0), rf.InnerFilter().FromBlock())
	})

	t.Run("Reorg spanning multiple aggregation range", func(t *testing.T) {
		testDB := memory.New()
		rf := populateRunningFilter(t, testDB, 2*core.NumBlocksPerFilter-1)
		require.Equal(t, 2*core.NumBlocksPerFilter, rf.NextBlock())
		require.Equal(t, 2*core.NumBlocksPerFilter, rf.InnerFilter().FromBlock())
		require.Equal(t, 3*core.NumBlocksPerFilter-1, rf.InnerFilter().ToBlock())

		for range core.NumBlocksPerFilter + 1 {
			require.NoError(t, rf.OnReorg())
		}
		require.Equal(t, core.NumBlocksPerFilter-1, rf.NextBlock())
		require.Equal(t, core.NumBlocksPerFilter-1, rf.InnerFilter().ToBlock())
		require.Equal(t, uint64(0), rf.InnerFilter().FromBlock())
	})
}

func TestMarshalling(t *testing.T) {
	testDB := memory.New()
	filter := core.NewAggregatedFilter(core.NumBlocksPerFilter)
	rf := core.NewRunningEventFilterHot(testDB, &filter, core.NumBlocksPerFilter)
	err := rf.Insert(testBloomWithRandomKeys(t, 1), core.NumBlocksPerFilter)
	require.NoError(t, err)

	rfBytes, err := encoder.Marshal(rf)
	require.NoError(t, err)

	var decoded core.RunningEventFilter
	require.NoError(t, encoder.Unmarshal(rfBytes, &decoded))

	require.Equal(t, rf.InnerFilter(), decoded.InnerFilter())
	require.Equal(t, rf.NextBlock(), decoded.NextBlock())
}
