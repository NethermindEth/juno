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
	fromBlock, err := f.FromBlock()
	require.NoError(t, err)
	require.Equal(t, uint64(0), fromBlock)
	toBlock, err := f.ToBlock()
	require.NoError(t, err)
	require.Equal(t, filter.ToBlock(), toBlock)
	nextBlock, err := f.NextBlock()
	require.NoError(t, err)
	require.Equal(t, uint64(0), nextBlock)
}

func TestRunningEventFilter_LazyInitialization_EmptyDB(t *testing.T) {
	testDB := memory.New()
	rf := core.NewRunningEventFilterLazy(testDB, core.InitializeRunningEventFilter)
	fromBlock, err := rf.FromBlock()
	require.NoError(t, err)
	require.Equal(t, uint64(0), fromBlock)
	toBlock, err := rf.ToBlock()
	require.NoError(t, err)
	require.Equal(t, core.NumBlocksPerFilter-1, toBlock)
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
		fromBlock, err := rf.FromBlock()
		require.NoError(t, err)
		require.Equal(t, uint64(0), fromBlock)
		toBlock, err := rf.ToBlock()
		require.NoError(t, err)
		require.Equal(t, core.NumBlocksPerFilter-1, toBlock)
		nextBlock, err := rf.NextBlock()
		require.NoError(t, err)
		require.Equal(t, expectedNext, nextBlock)
	})

	t.Run("Should return an error when couldn't initialise", func(t *testing.T) {
		closedDB := memory.New()
		closedDB.Close()
		rf := core.NewRunningEventFilterLazy(closedDB, core.InitializeRunningEventFilter)
		_, err := rf.FromBlock()
		require.Error(t, err)
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
			nextBlock, err := rf.NextBlock()
			require.NoError(t, err)
			require.Equal(t, expectedNext, nextBlock, "must catch up to chain height + 1")
			fromBlock, err := rf.FromBlock()
			require.NoError(t, err)
			require.Equal(t, uint64(0), fromBlock)
			toBlock, err := rf.ToBlock()
			require.NoError(t, err)
			require.Equal(t, core.NumBlocksPerFilter-1, toBlock)

			preserved, err := rf.BlocksForKeys(snapshotKeys)
			require.NoError(t, err)
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
			fromBlock, err := rf.FromBlock()
			require.NoError(t, err)
			require.Equal(t, core.NumBlocksPerFilter, fromBlock,
				"window rotated to second window during fill")
			toBlock, err := rf.ToBlock()
			require.NoError(t, err)
			require.Equal(t, 2*core.NumBlocksPerFilter-1, toBlock)
			nextBlock, err := rf.NextBlock()
			require.NoError(t, err)
			require.Equal(t, latest+1, nextBlock)

			// Second window: every position in [N, latest] hit.
			filled, err := rf.BlocksForKeys(headerKeys)
			require.NoError(t, err)
			for i := core.NumBlocksPerFilter; i <= latest; i++ {
				require.True(t, filled.Test(uint(i-fromBlock)),
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

func TestRunningEventFilter_InsertWithBatch_AtWindowBoundary(t *testing.T) {
	// Regression for the deterministic failure at block k*NumBlocksPerFilter-1.
	// statebackend used to commit chain height in one batch and then call
	// runningFilter.Insert separately. On the first call after restart at a
	// boundary, ensureInit's GetChainHeight saw the just-committed boundary
	// block, fillRunningEventFilter consumed it and rotated the inner window,
	// and the outer Insert then tried to add the same block to the new
	// (empty) window — "block X doesn't belong to range [X+1, Y]". The fix
	// wires the insert into the same batch as the chain-height update, so
	// GetChainHeight reads the pre-batch value during ensureInit.

	boundary := core.NumBlocksPerFilter - 1
	headerKeys := [][]byte{{0xAB}}

	setup := func(t *testing.T) (db.KeyValueStore, *core.Header) {
		t.Helper()
		testDB := memory.New()
		// Snapshot caught up through block boundary-1 (next = boundary), so
		// when GetChainHeight catches up to the boundary the initializer
		// takes the same-window-resume branch and fills exactly one block —
		// the boundary block — which is what tipped the fill over the edge.
		storedFilter := core.NewAggregatedFilter(0)
		snap := core.NewRunningEventFilterHot(testDB, &storedFilter, boundary)
		require.NoError(t, core.WriteRunningEventFilter(testDB, snap))
		header := &core.Header{
			Number:      boundary,
			Hash:        felt.NewRandom[felt.Felt](),
			EventsBloom: testBloomWithKeys(t, headerKeys),
		}
		return testDB, header
	}

	t.Run("non-batched Insert on lazy filter fails at the boundary", func(t *testing.T) {
		// Documents why InsertWithBatch is the only safe entry point for
		// lazy filters during block ingestion. When chain height has been
		// committed before Insert, ensureInit's GetChainHeight sees the
		// boundary block, fill rotates the inner window, and the outer
		// insert then can't place the same block in the rolled-over range.
		testDB, header := setup(t)
		require.NoError(t, core.WriteBlockHeaderByNumber(testDB, header))
		require.NoError(t, core.WriteChainHeight(testDB, boundary))

		rf := core.NewRunningEventFilterLazy(testDB, core.InitializeRunningEventFilter)
		err := rf.Insert(header.EventsBloom, boundary)
		require.ErrorContains(t, err,
			"inserting block 8191 into window [8192,16383]: block number is not within range")
	})

	t.Run(
		"fix: header and chain height buffered in same batch as InsertWithBatch",
		func(t *testing.T) {
			testDB, header := setup(t)
			require.NoError(t, core.WriteChainHeight(testDB, boundary-1))

			rf := core.NewRunningEventFilterLazy(testDB, core.InitializeRunningEventFilter)
			require.NoError(t, testDB.Write(func(batch db.Batch) error {
				if err := core.WriteBlockHeaderByNumber(batch, header); err != nil {
					return err
				}
				if err := core.WriteChainHeight(batch, boundary); err != nil {
					return err
				}
				return rf.InsertWithBatch(batch, header.EventsBloom, boundary)
			}))

			nextBlock, err := rf.NextBlock()
			require.NoError(t, err)
			require.Equal(t, boundary+1, nextBlock)
			fromBlock, err := rf.FromBlock()
			require.NoError(t, err)
			require.Equal(t, boundary+1, fromBlock)
			toBlock, err := rf.ToBlock()
			require.NoError(t, err)
			require.Equal(t, 2*core.NumBlocksPerFilter-1, toBlock)

			stored, err := core.GetAggregatedBloomFilter(testDB, 0, boundary)
			require.NoError(t, err)
			require.True(t, stored.BlocksForKeys(headerKeys).Test(uint(boundary)),
				"completed window persisted via the batch with boundary bit set")
		},
	)
}

func TestRunningEventFilter_OnReorgWithBatch_AtWindowBoundary(t *testing.T) {
	// Regression for OnReorg + lazy-init at a window boundary. If the
	// chain-height revert is committed before OnReorg runs on a still-lazy
	// filter, ensureInit reads the post-revert height and advances the
	// filter as if caught up; OnReorg then reverts one block too many, and
	// at a boundary that extra revert crosses into the previous window and
	// clears a bit for a block still on chain.
	//
	// OnReorgWithBatch inside the revert's writeFn keeps ensureInit on the
	// pre-revert height, so the subsequent clear targets exactly the
	// reverted block.

	boundary := core.NumBlocksPerFilter - 1
	headerKeys := [][]byte{{0xCD}}

	setup := func(t *testing.T) (db.KeyValueStore, *core.Header) {
		t.Helper()
		testDB := memory.New()

		// Persisted previous window with bits for headerKeys at every block.
		// This is the window the running filter has already rolled over.
		prevWindow := core.NewAggregatedFilter(0)
		for i := uint64(0); i <= boundary; i++ {
			require.NoError(t, prevWindow.Insert(testBloomWithKeys(t, headerKeys), i))
		}
		require.NoError(t, core.WriteAggregatedBloomFilter(testDB, &prevWindow))

		// Stored running filter snapshot: caught up through `boundary`,
		// inner is the fresh empty next window. boundary+1 is the block
		// about to be reverted.
		nextInner := core.NewAggregatedFilter(boundary + 1)
		snap := core.NewRunningEventFilterHot(testDB, &nextInner, boundary+1)
		require.NoError(t, core.WriteRunningEventFilter(testDB, snap))

		header := &core.Header{
			Number:      boundary + 1,
			Hash:        felt.NewRandom[felt.Felt](),
			EventsBloom: testBloomWithKeys(t, headerKeys),
		}
		return testDB, header
	}

	t.Run(
		"non-batched OnReorg on lazy filter silently over-clears at the boundary",
		func(t *testing.T) {
			testDB, header := setup(t)
			require.NoError(t, core.WriteBlockHeaderByNumber(testDB, header))
			// Post-revert chain height already committed — pre-fix would commit
			// this in a batch and then call OnReorg separately.
			require.NoError(t, core.WriteChainHeight(testDB, boundary))

			rf := core.NewRunningEventFilterLazy(testDB, core.InitializeRunningEventFilter)
			require.NoError(t, rf.OnReorg())

			// ensureInit saw post-revert chain height = boundary, took the
			// "caught up" branch (stored.next == latest+1), and left the filter
			// at f.next = boundary+1, f.inner = [boundary+1, 2N-1]. onReorg
			// then crossed back into [0, boundary], loaded it, and cleared bit
			// `boundary` — a block that's still on chain.
			nextBlock, err := rf.NextBlock()
			require.NoError(t, err)
			require.Equal(t, boundary, nextBlock)
			fromBlock, err := rf.FromBlock()
			require.NoError(t, err)
			require.Equal(t, uint64(0), fromBlock)
			toBlock, err := rf.ToBlock()
			require.NoError(t, err)
			require.Equal(t, boundary, toBlock)
			matches, err := rf.BlocksForKeys(headerKeys)
			require.NoError(t, err)
			require.False(t, matches.Test(uint(boundary)),
				"filter wrongly cleared the bit for a block still on chain")
		},
	)

	t.Run("fix: OnReorgWithBatch inside writeFn preserves on-chain block bits", func(t *testing.T) {
		testDB, header := setup(t)
		require.NoError(t, core.WriteBlockHeaderByNumber(testDB, header))
		require.NoError(t, core.WriteChainHeight(testDB, boundary+1))

		rf := core.NewRunningEventFilterLazy(testDB, core.InitializeRunningEventFilter)
		require.NoError(t, testDB.Write(func(batch db.Batch) error {
			// Mirror statebackend.RevertHead: delete reverted block's header,
			// revert chain height, OnReorgWithBatch — all in one batch.
			if err := core.DeleteBlockHeaderByNumber(batch, header.Number); err != nil {
				return err
			}
			if err := core.WriteChainHeight(batch, boundary); err != nil {
				return err
			}
			return rf.OnReorgWithBatch(batch)
		}))

		// ensureInit saw pre-revert chain height = boundary+1, filled one
		// block ([boundary+1, boundary+1]) into the empty next window, and
		// onReorg then cleared exactly the reverted block. No boundary
		// cross, no over-clear of [0, boundary].
		nextBlock, err := rf.NextBlock()
		require.NoError(t, err)
		require.Equal(t, boundary+1, nextBlock)
		fromBlock, err := rf.FromBlock()
		require.NoError(t, err)
		require.Equal(t, boundary+1, fromBlock)
		matches, err := rf.BlocksForKeys(headerKeys)
		require.NoError(t, err)
		require.False(t, matches.Test(0),
			"reverted block's bit (relative position 0) must be cleared")

		// Previous window on disk is untouched.
		stored, err := core.GetAggregatedBloomFilter(testDB, 0, boundary)
		require.NoError(t, err)
		require.True(t, stored.BlocksForKeys(headerKeys).Test(uint(boundary)),
			"previous window's boundary block bit must remain on disk")
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
	fromBlock, err := rf.FromBlock()
	require.NoError(t, err)
	require.Equal(t, filter.ToBlock()+1, fromBlock)

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
	got, err := rf.BlocksForKeys(keys)
	require.NoError(t, err)
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
	clone, err := rf.Clone()
	require.NoError(t, err)
	require.NotSame(t, rf, clone)
	rfInner, err := rf.InnerFilter()
	require.NoError(t, err)
	cloneInner, err := clone.InnerFilter()
	require.NoError(t, err)
	require.NotSame(t, rfInner, cloneInner)
	rfFrom, err := rf.FromBlock()
	require.NoError(t, err)
	cloneFrom, err := clone.FromBlock()
	require.NoError(t, err)
	require.Equal(t, rfFrom, cloneFrom)
	require.Equal(t, rf, clone)
}

func TestRunningEventFilter_Reorg(t *testing.T) {
	requireNextBlock := func(t *testing.T, rf *core.RunningEventFilter, expected uint64) {
		t.Helper()
		got, err := rf.NextBlock()
		require.NoError(t, err)
		require.Equal(t, expected, got)
	}
	requireInnerBounds := func(t *testing.T, rf *core.RunningEventFilter, from, to uint64) {
		t.Helper()
		inner, err := rf.InnerFilter()
		require.NoError(t, err)
		require.Equal(t, from, inner.FromBlock())
		require.Equal(t, to, inner.ToBlock())
	}

	t.Run("Reorg within current running filter", func(t *testing.T) {
		testDB := memory.New()
		rf := populateRunningFilter(t, testDB, 5)
		requireNextBlock(t, rf, 6)
		require.NoError(t, rf.OnReorg())
		requireNextBlock(t, rf, 5)
	})
	t.Run("Reorg spanning previous aggregation range", func(t *testing.T) {
		testDB := memory.New()
		rf := populateRunningFilter(t, testDB, core.NumBlocksPerFilter-1)
		requireNextBlock(t, rf, core.NumBlocksPerFilter)
		requireInnerBounds(t, rf, core.NumBlocksPerFilter, 2*core.NumBlocksPerFilter-1)

		require.NoError(t, rf.OnReorg())
		requireNextBlock(t, rf, core.NumBlocksPerFilter-1)
		requireInnerBounds(t, rf, 0, core.NumBlocksPerFilter-1)
	})

	t.Run("Reorg spanning multiple aggregation range", func(t *testing.T) {
		testDB := memory.New()
		rf := populateRunningFilter(t, testDB, 2*core.NumBlocksPerFilter-1)
		requireNextBlock(t, rf, 2*core.NumBlocksPerFilter)
		requireInnerBounds(t, rf, 2*core.NumBlocksPerFilter, 3*core.NumBlocksPerFilter-1)

		for range core.NumBlocksPerFilter + 1 {
			require.NoError(t, rf.OnReorg())
		}
		requireNextBlock(t, rf, core.NumBlocksPerFilter-1)
		requireInnerBounds(t, rf, 0, core.NumBlocksPerFilter-1)
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

	rfInner, err := rf.InnerFilter()
	require.NoError(t, err)
	decodedInner, err := decoded.InnerFilter()
	require.NoError(t, err)
	require.Equal(t, rfInner, decodedInner)
	rfNext, err := rf.NextBlock()
	require.NoError(t, err)
	decodedNext, err := decoded.NextBlock()
	require.NoError(t, err)
	require.Equal(t, rfNext, decodedNext)
}
