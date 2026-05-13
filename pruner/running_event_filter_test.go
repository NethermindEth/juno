package pruner_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/pruner"
	"github.com/NethermindEth/juno/pruner/testutils"
	"github.com/bits-and-blooms/bloom/v3"
	"github.com/stretchr/testify/require"
)

func testBloomWithRandomKey(t *testing.T) *bloom.BloomFilter {
	t.Helper()
	filter := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
	key := felt.NewRandom[felt.Felt]()
	filter.Add(key.Marshal())
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

// storeBlockWithBloom writes a full block via testutils.StoreBlock and then
// overlays EventsBloom on its header so the rebuild path actually inserts
// data into the running filter when it visits the block.
func storeBlockWithBloom(
	t *testing.T,
	database db.KeyValueStore,
	blockNum uint64,
	bloomFilter *bloom.BloomFilter,
) {
	t.Helper()
	block := testutils.StoreBlock(t, database, blockNum)
	block.Header.EventsBloom = bloomFilter
	require.NoError(t, core.WriteBlockHeaderByNumber(database, block.Header))
}

func TestRunningEventFilter_LazyInitialization_EmptyDB(t *testing.T) {
	testDB := memory.New()
	rf := core.NewRunningEventFilterLazy(testDB, pruner.InitializeRunningEventFilter)
	require.Equal(t, uint64(0), rf.FromBlock())
	require.Equal(t, core.NumBlocksPerFilter-1, rf.ToBlock())
	require.NoError(t, rf.Insert(testBloomWithRandomKey(t), 0))
}

// TestRunningEventFilter_LazyInitialization_CaughtUp covers the
// short-circuit branch: snapshot.next == latest+1 → init returns the
// on-disk snapshot without filling or rebuilding.
func TestRunningEventFilter_LazyInitialization_CaughtUp(t *testing.T) {
	const latest uint64 = 50
	database := testutils.NewPebbleTestDB(t)

	sharedBloom := testBloomWithRandomKey(t)
	for i := uint64(0); i <= latest; i++ {
		storeBlockWithBloom(t, database, i, sharedBloom)
	}
	require.NoError(t, core.WriteChainHeight(database, latest))

	filter := core.NewAggregatedFilter(0)
	snap := core.NewRunningEventFilterHot(database, &filter, 0)
	for i := uint64(0); i <= latest; i++ {
		require.NoError(t, snap.Insert(sharedBloom, i))
	}
	require.NoError(t, core.WriteRunningEventFilter(database, snap))

	rf := core.NewRunningEventFilterLazy(database, pruner.InitializeRunningEventFilter)
	require.Equal(t, uint64(0), rf.FromBlock())
	require.Equal(t, core.NumBlocksPerFilter-1, rf.ToBlock())
	require.Equal(t, latest+1, rf.NextBlock())
}

// TestRunningEventFilter_LazyInitialization_RebuildAnchorless_FillFromFloor
// exercises the rebuild fallthrough: no persisted snapshot and no
// aggregated-filter anchor, so the window roots at floorAligned and
// filling starts at floor itself. The pruned prefix [0, pruneTo) stays
// zero in the bitmap.
func TestRunningEventFilter_LazyInitialization_RebuildAnchorless_FillFromFloor(t *testing.T) {
	const (
		latest  uint64 = 50
		pruneTo uint64 = 30
	)
	database := testutils.NewPebbleTestDB(t)

	retainedKeys := [][]byte{{0xEE}}
	for blockNum := uint64(0); blockNum <= latest; blockNum++ {
		b := testBloomWithRandomKey(t)
		if blockNum >= pruneTo {
			b = testBloomWithKeys(t, retainedKeys)
		}
		storeBlockWithBloom(t, database, blockNum, b)
	}
	require.NoError(t, core.WriteChainHeight(database, latest))

	_, _, err := pruner.PruneUpto(t.Context(), database, pruneTo, testTargetBatchByteSize)
	require.NoError(t, err)

	rf := core.NewRunningEventFilterLazy(database, pruner.InitializeRunningEventFilter)
	require.Equal(t, uint64(0), rf.FromBlock(),
		"window rooted at floorAligned (= 0 since floor < NumBlocksPerFilter)")
	require.Equal(t, core.NumBlocksPerFilter-1, rf.ToBlock())
	require.Equal(t, latest+1, rf.NextBlock())

	matches := rf.BlocksForKeys(retainedKeys)
	for i := pruneTo; i <= latest; i++ {
		require.True(t, matches.Test(uint(i)),
			"retained block %d must be hit by the rebuilt filter", i)
	}
	for i := range pruneTo {
		require.False(t, matches.Test(uint(i)),
			"pruned block %d must remain unset", i)
	}
}

// setupSameWindowResume builds the shared fixture for the same-window
// resume tests and returns the lazy filter plus both key sets.
func setupSameWindowResume(
	t *testing.T,
	latest,
	pruneTo,
	snapshotNext uint64,
) (rf *core.RunningEventFilter, headerKeys, snapshotKeys [][]byte) {
	t.Helper()
	database := testutils.NewPebbleTestDB(t)

	// Two orthogonal probe tags. headerKeys is stamped into on-disk
	// headers (fill path sets these bits); snapshotKeys is stamped into
	// the persisted snapshot (resume path preserves these bits). Distinct
	// values so each assertion isolates one path.
	headerKeys = [][]byte{{0xAA, 0xBB}}
	snapshotKeys = [][]byte{{0x11, 0x22}}

	// Headers at and past snapshotNext carry headerKeys so an unclamped
	// fill would deterministically set bits in [snapshotNext, pruneTo).
	for i := uint64(0); i <= latest; i++ {
		var b *bloom.BloomFilter
		if i >= snapshotNext {
			b = testBloomWithKeys(t, headerKeys)
		} else {
			b = testBloomWithRandomKey(t)
		}

		storeBlockWithBloom(t, database, i, b)
	}
	require.NoError(t, core.WriteChainHeight(database, latest))

	// Persisted snapshot stamped with snapshotKeys for [0, snapshotNext);
	// init's resume branch keeps these bits, rebuild would refill them.
	filter := core.NewAggregatedFilter(0)
	snap := core.NewRunningEventFilterHot(database, &filter, 0)
	for i := range snapshotNext {
		require.NoError(t, snap.Insert(testBloomWithKeys(t, snapshotKeys), i))
	}
	require.NoError(t, core.WriteRunningEventFilter(database, snap))

	_, _, err := pruner.PruneUpto(t.Context(), database, pruneTo, testTargetBatchByteSize)
	require.NoError(t, err)

	rf = core.NewRunningEventFilterLazy(database, pruner.InitializeRunningEventFilter)
	return rf, headerKeys, snapshotKeys
}

// TestRunningEventFilter_LazyInitialization_SameWindowResume covers
// the no-clamp same-window resume: floor (= pruneTo) sits below
// snapshot.next, so the snapshot bitmap is kept and [snapshotNext, latest]
// is filled from on-disk headers.
func TestRunningEventFilter_LazyInitialization_SameWindowResume(t *testing.T) {
	const (
		latest       uint64 = 50
		pruneTo      uint64 = 10
		snapshotNext uint64 = 20
	)
	rf, headerKeys, snapshotKeys := setupSameWindowResume(t, latest, pruneTo, snapshotNext)

	require.Equal(t, latest+1, rf.NextBlock())
	require.Equal(t, uint64(0), rf.FromBlock())
	require.Equal(t, core.NumBlocksPerFilter-1, rf.ToBlock())

	preserved := rf.BlocksForKeys(snapshotKeys)
	for i := range snapshotNext {
		require.True(t, preserved.Test(uint(i)),
			"snapshot bit at block %d not preserved", i)
	}
	filled := rf.BlocksForKeys(headerKeys)
	for i := snapshotNext; i <= latest; i++ {
		require.True(t, filled.Test(uint(i)),
			"retained block %d must be filled from header bloom", i)
	}
}

// TestRunningEventFilter_LazyInitialization_SameWindowResumeClamped covers
// the clamp branch: floor (= pruneTo) sits above snapshot.next, so the
// max(next, floor) clamp kicks in. Snapshot bits in [0, snapshotNext) still
// survive on the inner bitmap; positions [snapshotNext, floor) stay zero;
// [floor, latest] is filled.
func TestRunningEventFilter_LazyInitialization_SameWindowResumeClamped(t *testing.T) {
	const (
		latest       uint64 = 50
		pruneTo      uint64 = 30
		snapshotNext uint64 = 20
	)
	rf, headerKeys, snapshotKeys := setupSameWindowResume(t, latest, pruneTo, snapshotNext)

	require.Equal(t, latest+1, rf.NextBlock())
	require.Equal(t, uint64(0), rf.FromBlock())

	preserved := rf.BlocksForKeys(snapshotKeys)
	for i := range snapshotNext {
		require.True(t, preserved.Test(uint(i)),
			"snapshot bit at block %d not preserved across clamp", i)
	}
	filled := rf.BlocksForKeys(headerKeys)
	for i := pruneTo; i <= latest; i++ {
		require.True(t, filled.Test(uint(i)),
			"retained block %d must be filled from header bloom", i)
	}
	for i := snapshotNext; i < pruneTo; i++ {
		require.False(t, filled.Test(uint(i)),
			"clamped gap block %d must remain unset", i)
	}
}

// TestRunningEventFilter_LazyInitialization_MultiWindowRebuildAfterPrune
// covers the rebuild path when the chain spans past a filter-window
// boundary. Rebuild starts at floor (= pruneTo), walks through the first
// window, rotates at N-1, and lands in the second window. Full blocks
// only for [0, pruneTo] (so PruneUpto's per-block sweep has real
// hash-keyed data to delete); headers-only for the rest, batched into
// a single pebble write.
func TestRunningEventFilter_LazyInitialization_MultiWindowRebuildAfterPrune(t *testing.T) {
	const pruneTo uint64 = 50
	latest := core.NumBlocksPerFilter + 5
	database := testutils.NewPebbleTestDB(t)

	headerKeys := [][]byte{{0xEE}}
	sharedBloom := testBloomWithKeys(t, headerKeys)
	// Full block fixture only for blocks PruneUpto will iterate; the
	// per-block sweep reads hash-keyed data so it needs the full shape.
	for i := uint64(0); i <= pruneTo; i++ {
		storeBlockWithBloom(t, database, i, sharedBloom)
	}
	// Past pruneTo, only the header (for rebuild fill) and the
	// commitment (so OldestRetainedBlock keeps reflecting retained
	// state) are written.
	batch := database.NewBatch()
	for i := pruneTo + 1; i <= latest; i++ {
		header := &core.Header{
			Number:      i,
			Hash:        felt.NewRandom[felt.Felt](),
			EventsBloom: sharedBloom,
		}
		require.NoError(t, core.WriteBlockHeaderByNumber(batch, header))
		require.NoError(t, core.WriteBlockCommitment(batch, i, &core.BlockCommitments{}))
		if batch.Size() >= testTargetBatchByteSize {
			require.NoError(t, batch.Write())
			batch = database.NewBatch()
		}
	}
	require.NoError(t, batch.Write())
	require.NoError(t, core.WriteChainHeight(database, latest))

	_, _, err := pruner.PruneUpto(t.Context(), database, pruneTo, testTargetBatchByteSize)
	require.NoError(t, err)

	rf := core.NewRunningEventFilterLazy(database, pruner.InitializeRunningEventFilter)
	require.Equal(t, core.NumBlocksPerFilter, rf.FromBlock(),
		"window rotated to second window after fill crossed N-1")
	require.Equal(t, 2*core.NumBlocksPerFilter-1, rf.ToBlock())
	require.Equal(t, latest+1, rf.NextBlock())

	// Second (current) window: positions [N, latest] hit.
	filled := rf.BlocksForKeys(headerKeys)
	for i := core.NumBlocksPerFilter; i <= latest; i++ {
		require.True(t, filled.Test(uint(i-rf.FromBlock())),
			"block %d (second window) not filled", i)
	}
	// First window persisted on rotation: positions [pruneTo, N-1] hit
	// (filled from headers), [0, pruneTo) zero (pruned prefix skipped).
	stored, err := core.GetAggregatedBloomFilter(database, 0, core.NumBlocksPerFilter-1)
	require.NoError(t, err)
	storedMatches := stored.BlocksForKeys(headerKeys)
	for i := pruneTo; i < core.NumBlocksPerFilter; i++ {
		require.True(t, storedMatches.Test(uint(i)),
			"block %d (past floor, first window) not filled", i)
	}
	for i := range pruneTo {
		require.False(t, storedMatches.Test(uint(i)),
			"pruned block %d must remain unset", i)
	}
}
