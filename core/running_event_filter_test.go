package core_test

import (
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/statetestutils"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/encoder"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
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
	rf := core.NewRunningEventFilterLazy(testDB)
	require.Equal(t, uint64(0), rf.FromBlock())
	require.Equal(t, core.NumBlocksPerFilter-1, rf.ToBlock())
	require.NoError(t, rf.Insert(testBloomWithRandomKeys(t, 1), 0))
}

func TestRunningEventFilter_LazyInitialization_Preload(t *testing.T) {
	testDB := memory.New()
	n := &utils.Sepolia
	chain := blockchain.New(testDB, n, statetestutils.UseNewState())
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

	t.Run("Rebuild when running fitler not persisted", func(t *testing.T) {
		rf := core.NewRunningEventFilterLazy(testDB)
		require.Equal(t, uint64(0), rf.FromBlock())
		require.Equal(t, core.NumBlocksPerFilter-1, rf.ToBlock())
		require.Equal(t, expectedNext, rf.NextBlock())
	})

	t.Run("Load from DB when running filter upto date", func(t *testing.T) {
		require.NoError(t, chain.WriteRunningEventFilter())

		rf := core.NewRunningEventFilterLazy(testDB)
		require.Equal(t, uint64(0), rf.FromBlock())
		require.Equal(t, core.NumBlocksPerFilter-1, rf.ToBlock())
		require.Equal(t, expectedNext, rf.NextBlock())
	})

	t.Run("Should panic when couldn't initialise", func(t *testing.T) {
		testDB.Close()
		rf := core.NewRunningEventFilterLazy(testDB)
		require.Panics(t, func() {
			rf.FromBlock()
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
