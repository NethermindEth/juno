package historyprunner_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/historyprunner"
	"github.com/NethermindEth/juno/pruner/testutils"
	_ "github.com/NethermindEth/juno/utils/cbor/registry"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupChain stores totalBlocks blocks plus the chain-height and L1-head
// pointers the migrator reads to compute its cutoff. The L1 head is set to
// the tip so that minHead = chainHeight and oldestBlockKept reflects
// retainedBlocks alone, matching the simple PruneUpto setup in the pruner
// tests.
func setupChain(
	t *testing.T,
	totalBlocks uint64,
) (db.KeyValueStore, []*testutils.StoredBlock) {
	t.Helper()
	return setupChainWith(t, totalBlocks, testutils.StoreBlock)
}

func setupChainWith(
	t *testing.T,
	totalBlocks uint64,
	store func(*testing.T, db.KeyValueStore, uint64) *testutils.StoredBlock,
) (db.KeyValueStore, []*testutils.StoredBlock) {
	t.Helper()
	database := testutils.NewPebbleTestDB(t)
	blocks := make([]*testutils.StoredBlock, totalBlocks)
	for i := range totalBlocks {
		blocks[i] = store(t, database, i)
	}
	tip := totalBlocks - 1
	require.NoError(t, core.WriteChainHeight(database, tip))
	require.NoError(t, core.WriteL1Head(database, &core.L1Head{
		BlockNumber: tip,
		BlockHash:   blocks[tip].Header.Hash,
		StateRoot:   felt.NewRandom[felt.Felt](),
	}))
	return database, blocks
}

// TestMigrate_FullRun is the migration-side mirror of TestPruneUpto from
// the pruner package: same DB layout, same expected post-state. The
// historyprunner migrator is a different code path (range-tombstone wipe
// + scratch-stage + restore) but converges on the same on-disk shape as
// PruneUpto for the keep window and the carve-outs.
func TestMigrate_FullRun(t *testing.T) {
	const totalBlocks uint64 = 30
	lag := core.BlockHashLag

	tests := []struct {
		name            string
		retainedBlocks  uint64
		oldestBlockKept uint64
	}{
		{name: "standard", retainedBlocks: 10, oldestBlockKept: totalBlocks - 10 - 1},
		{name: "zero retained", retainedBlocks: 0, oldestBlockKept: totalBlocks - 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.GreaterOrEqual(t, tc.oldestBlockKept, lag)

			database, blocks := setupChain(t, totalBlocks)

			m := historyprunner.New(tc.retainedBlocks, 0, false)
			state, err := m.Migrate(t.Context(), database, &networks.Mainnet, log.NewNopZapLogger())
			require.NoError(t, err)
			require.Nil(t, state, "fully completed migration must not return intermediate state")

			testutils.AssertPostPruneState(t, database, blocks, tc.oldestBlockKept, lag)
		})
	}
}

// TestMigrate_MinAgeFloorTightensCutoff verifies the wallclock floor
// shrinks the pruned set when it's tighter than the block-count floor:
// blocks whose Timestamp is within minAge of now must be retained even
// if standardFloor would discard them.
func TestMigrate_MinAgeFloorTightensCutoff(t *testing.T) {
	const totalBlocks uint64 = 60
	const retainedBlocks uint64 = 5
	const minAge = 1 * time.Hour
	lag := core.BlockHashLag

	// Blocks 0..39 are 2h old (outside minAge); 40..59 are 30min old (inside).
	// minAgeFloor = 40; standardFloor = 60-1-5 = 54; combined = 40.
	const youngFrom uint64 = 40
	const oldestBlockKept = youngFrom
	now := time.Now()
	oldTS := uint64(now.Add(-2 * time.Hour).Unix())      //nolint:gosec // test fixture
	youngTS := uint64(now.Add(-30 * time.Minute).Unix()) //nolint:gosec // test fixture

	database := testutils.NewPebbleTestDB(t)
	blocks := make([]*testutils.StoredBlock, totalBlocks)
	for i := range totalBlocks {
		ts := oldTS
		if i >= youngFrom {
			ts = youngTS
		}
		blocks[i] = testutils.StoreBlockWithTimestamp(t, database, i, ts)
	}
	tip := totalBlocks - 1
	require.NoError(t, core.WriteChainHeight(database, tip))
	require.NoError(t, core.WriteL1Head(database, &core.L1Head{
		BlockNumber: tip,
		BlockHash:   blocks[tip].Header.Hash,
		StateRoot:   felt.NewRandom[felt.Felt](),
	}))

	m := historyprunner.New(retainedBlocks, minAge, false)
	state, err := m.Migrate(t.Context(), database, &networks.Mainnet, log.NewNopZapLogger())
	require.NoError(t, err)
	require.Nil(t, state)

	testutils.AssertPostPruneState(t, database, blocks, oldestBlockKept, lag)
}

// TestMigrate_MinAgeFloorIgnoredInDeepCatchUp verifies that when every
// stored block is older than minAge (deep catch-up: chain tip's timestamp
// reflects historical sync, not wallclock), the wallclock floor is
// suppressed and the standard block-count floor wins. Mirrors the
// withinTimeWindow check in the running pruner.
func TestMigrate_MinAgeFloorIgnoredInDeepCatchUp(t *testing.T) {
	const totalBlocks uint64 = 30
	const retainedBlocks uint64 = 10
	const oldestBlockKept = totalBlocks - retainedBlocks - 1 // 19
	const minAge = 1 * time.Hour
	lag := core.BlockHashLag

	// All blocks 2h old → no block clears the wallclock cutoff
	staleTS := uint64(time.Now().Add(-2 * time.Hour).Unix()) //nolint:gosec // test fixture

	database := testutils.NewPebbleTestDB(t)
	blocks := make([]*testutils.StoredBlock, totalBlocks)
	for i := range totalBlocks {
		blocks[i] = testutils.StoreBlockWithTimestamp(t, database, i, staleTS)
	}
	tip := totalBlocks - 1
	require.NoError(t, core.WriteChainHeight(database, tip))
	require.NoError(t, core.WriteL1Head(database, &core.L1Head{
		BlockNumber: tip,
		BlockHash:   blocks[tip].Header.Hash,
		StateRoot:   felt.NewRandom[felt.Felt](),
	}))

	m := historyprunner.New(retainedBlocks, minAge, false)
	state, err := m.Migrate(t.Context(), database, &networks.Mainnet, log.NewNopZapLogger())
	require.NoError(t, err)
	require.Nil(t, state)

	testutils.AssertPostPruneState(t, database, blocks, oldestBlockKept, lag)
}

// TestMigrate_NoOpWhenChainShorterThanRetention covers the early exit when
// minHead < retainedBlocks: nothing to prune yet. The rolling pruner takes
// over once blocks arrive past the threshold.
func TestMigrate_NoOpWhenChainShorterThanRetention(t *testing.T) {
	const totalBlocks uint64 = 5
	const retainedBlocks uint64 = 10

	database, blocks := setupChain(t, totalBlocks)

	m := historyprunner.New(retainedBlocks, 0, false)
	state, err := m.Migrate(t.Context(), database, &networks.Mainnet, log.NewNopZapLogger())
	require.NoError(t, err)
	require.Nil(t, state)

	// All blocks intact.
	for _, b := range blocks {
		testutils.AssertBlockExists(t, database, b)
	}
}

// TestMigrate_NoOpOnEmptyDB covers the early exit when the chain head
// pointer is missing. This is the cold-start path on a fresh node.
func TestMigrate_NoOpOnEmptyDB(t *testing.T) {
	database := testutils.NewPebbleTestDB(t)
	m := historyprunner.New(10, 0, false)
	state, err := m.Migrate(t.Context(), database, &networks.Mainnet, log.NewNopZapLogger())
	require.NoError(t, err)
	require.Nil(t, state)
}

// cancelAfterBlocks wraps a KeyValueStore and cancels the supplied context
// after the n-th state-update Get. Both stager and restorer call
// GetStateUpdateByBlockNum exactly once at the top of each block, so
// counting only Gets against the StateUpdatesByBlockNumber bucket gives a
// per-block cancellation point that is decoupled from how many state-diff
// entries (storage / nonce / replaced-class) any given block happens to
// have. Other Gets (chain height, l1 head, parent header, transactions)
// are ignored.
//
// The pipeline runs maxWorkers Get-callers concurrently, so the counter
// uses atomic.Int64 — a non-atomic ++/== pair would let the trigger value
// be skipped or fired multiple times under racing increments.
type cancelAfterBlocks struct {
	db.KeyValueStore
	cancel context.CancelFunc
	after  int64
	count  atomic.Int64
}

func (c *cancelAfterBlocks) Get(key []byte, cb func([]byte) error) error {
	if len(key) > 0 && key[0] == byte(db.StateUpdatesByBlockNumber) {
		if c.count.Add(1) == c.after {
			c.cancel()
		}
	}
	return c.KeyValueStore.Get(key, cb)
}

// runCancellable runs Migrate with a wrapped DB that cancels the context
// after `cancelAfter` per-block state-update Gets. Returns the
// intermediate state blob the migrator persisted, or nil if it ran to
// completion.
func runCancellable(
	t *testing.T,
	prevState []byte,
	database db.KeyValueStore,
	cancelAfter int,
	retainedBlocks uint64,
) []byte {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	wrapped := &cancelAfterBlocks{KeyValueStore: database, cancel: cancel, after: int64(cancelAfter)}

	m := historyprunner.New(retainedBlocks, 0, false)
	require.NoError(t, m.Before(prevState))
	state, err := m.Migrate(ctx, wrapped, &networks.Mainnet, log.NewNopZapLogger())
	require.NoError(t, err)
	return state
}

// TestMigrate_CancelAndResume verifies that a migration interrupted partway
// through can be resumed by reconstructing a Migrator with the persisted
// intermediate state. The end-state must be identical to a single-shot
// run: blocks below the lag floor gone, lag-window headers kept,
// hash→number carve-out at oldestBlockKept-1, blocks ≥ oldestBlockKept
// fully restored.
//
// The test cancels MULTIPLE times to also exercise cancellation crossing
// the stager→restorer phase boundary. The keeper window has 10 blocks;
// stager and restorer each visit it once, so there are 20 per-block
// state-update Gets total, and cancelling every 2 forces several resumes.
func TestMigrate_CancelAndResume(t *testing.T) {
	const totalBlocks uint64 = 30
	const retainedBlocks uint64 = 10
	const oldestBlockKept = totalBlocks - retainedBlocks - 1 // 19
	lag := core.BlockHashLag

	database, blocks := setupChain(t, totalBlocks)

	// Cancel every 2 blocks until the migration completes. Each iteration
	// must either return a non-nil intermediate state (still going) or
	// nil (done). The number of attempts is not deterministic — concurrent
	// workers in the pipeline race against ctx cancellation. We only assert that cancellation
	// was actually exercised (>= 2 attempts) and that the loop terminates.
	var (
		state    []byte
		attempts int
	)
	for {
		attempts++
		state = runCancellable(t, state, database, 2, retainedBlocks)
		if state == nil {
			break
		}
	}
	assert.GreaterOrEqual(t, attempts, 2,
		"the test should actually exercise cancellation, not complete on the first try")

	// End-state after multiple cancel/resume cycles must be byte-for-byte
	// identical to a single-shot run.
	testutils.AssertPostPruneState(t, database, blocks, oldestBlockKept, lag)
}

// TestMigrate_NewState checks that the migration on new-state history keeps
// every answer at or above the retention floor and leaves each key only its
// latest entry below it.
func TestMigrate_NewState(t *testing.T) {
	const totalBlocks uint64 = 30
	const retainedBlocks uint64 = 10
	const oldestBlockKept = totalBlocks - retainedBlocks - 1
	floor := oldestBlockKept - 1

	database, blocks := setupChainWith(t, totalBlocks, testutils.StoreNewStateBlock)
	want := testutils.NewStateHistoryAnswers(t, database, blocks, floor, totalBlocks-1)

	m := historyprunner.New(retainedBlocks, 0, true)
	state, err := m.Migrate(t.Context(), database, &networks.Mainnet, log.NewNopZapLogger())
	require.NoError(t, err)
	require.Nil(t, state)

	testutils.AssertPostPruneState(t, database, blocks, oldestBlockKept, core.BlockHashLag)
	assert.Equal(t, want, testutils.NewStateHistoryAnswers(t, database, blocks, floor, totalBlocks-1))

	has := func(key []byte) bool {
		ok, err := database.Has(key)
		require.NoError(t, err)
		return ok
	}
	for b := range totalBlocks {
		kept := b >= floor
		assert.Equal(t, kept, has(db.ContractStorageHistoryAtBlockKey(
			testutils.SharedAddr1, testutils.SharedSlot, b)), "shared slot at %d", b)
		assert.Equal(t, kept, has(db.ContractNonceHistoryAtBlockKey(testutils.SharedAddr1, b)))
		assert.Equal(t, kept, has(db.ContractClassHashHistoryAtBlockKey(testutils.SharedAddr2, b)))
	}
	assert.False(t, bucketHasKeys(t, database, db.Temporary), "scratch space must be wiped")

	// Re-running on the pruned history must not change any answer
	_, err = historyprunner.New(retainedBlocks, 0, true).
		Migrate(t.Context(), database, &networks.Mainnet, log.NewNopZapLogger())
	require.NoError(t, err)
	assert.Equal(t, want, testutils.NewStateHistoryAnswers(t, database, blocks, floor, totalBlocks-1))
}

func bucketHasKeys(t *testing.T, r db.KeyValueReader, bucket db.Bucket) bool {
	t.Helper()
	it, err := r.NewIterator(bucket.Key(), true)
	require.NoError(t, err)
	defer it.Close()
	return it.First()
}
