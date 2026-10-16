package historyprunner_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
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
	layout testutils.Layout,
) (db.KeyValueStore, []*testutils.StoredBlock) {
	t.Helper()
	database := testutils.NewPebbleTestDB(t)
	blocks := make([]*testutils.StoredBlock, totalBlocks)
	for i := range totalBlocks {
		blocks[i] = testutils.StoreBlockWithLayout(t, database, i, 0, layout)
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

var layouts = []testutils.Layout{testutils.DeprecatedLayout, testutils.NewStateLayout}

// TestMigrate_FullRun is the migration-side mirror of TestPruneUpto from
// the pruner package: same DB layout, same expected post-state. The
// historyprunner migrator is a different code path (range-tombstone wipe
// + scratch-stage + restore) but converges on the same on-disk shape as
// PruneUpto for the keep window and the carve-outs. Every history answer at or
// above the floor must survive, and a re-run must change nothing.
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

	for _, layout := range layouts {
		for _, tc := range tests {
			t.Run(layout.String()+"/"+tc.name, func(t *testing.T) {
				require.GreaterOrEqual(t, tc.oldestBlockKept, lag)

				database, blocks := setupChain(t, totalBlocks, layout)
				floor := tc.oldestBlockKept - 1
				want := testutils.HistoryAnswers(t, database, blocks, floor, totalBlocks-1)

				m := historyprunner.New(tc.retainedBlocks, 0)
				state, err := m.Migrate(t.Context(), database, &networks.Mainnet, log.NewNopZapLogger())
				require.NoError(t, err)
				require.Nil(t, state, "fully completed migration must not return intermediate state")

				testutils.AssertPostPruneState(t, database, blocks, tc.oldestBlockKept, lag)
				assert.Equal(t, want, testutils.HistoryAnswers(t, database, blocks, floor, totalBlocks-1))
				assert.False(t, bucketHasKeys(t, database, db.Temporary), "scratch space must be wiped")

				_, err = historyprunner.New(tc.retainedBlocks, 0).
					Migrate(t.Context(), database, &networks.Mainnet, log.NewNopZapLogger())
				require.NoError(t, err)
				testutils.AssertPostPruneState(t, database, blocks, tc.oldestBlockKept, lag)
				assert.Equal(t, want, testutils.HistoryAnswers(t, database, blocks, floor, totalBlocks-1))
			})
		}
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

	for _, layout := range layouts {
		t.Run(layout.String(), func(t *testing.T) {
			database := testutils.NewPebbleTestDB(t)
			blocks := make([]*testutils.StoredBlock, totalBlocks)
			for i := range totalBlocks {
				ts := oldTS
				if i >= youngFrom {
					ts = youngTS
				}
				blocks[i] = testutils.StoreBlockWithLayout(t, database, i, ts, layout)
			}
			tip := totalBlocks - 1
			require.NoError(t, core.WriteChainHeight(database, tip))
			require.NoError(t, core.WriteL1Head(database, &core.L1Head{
				BlockNumber: tip,
				BlockHash:   blocks[tip].Header.Hash,
				StateRoot:   felt.NewRandom[felt.Felt](),
			}))

			m := historyprunner.New(retainedBlocks, minAge)
			state, err := m.Migrate(t.Context(), database, &networks.Mainnet, log.NewNopZapLogger())
			require.NoError(t, err)
			require.Nil(t, state)

			testutils.AssertPostPruneState(t, database, blocks, oldestBlockKept, lag)
		})
	}
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

	for _, layout := range layouts {
		t.Run(layout.String(), func(t *testing.T) {
			database := testutils.NewPebbleTestDB(t)
			blocks := make([]*testutils.StoredBlock, totalBlocks)
			for i := range totalBlocks {
				blocks[i] = testutils.StoreBlockWithLayout(t, database, i, staleTS, layout)
			}
			tip := totalBlocks - 1
			require.NoError(t, core.WriteChainHeight(database, tip))
			require.NoError(t, core.WriteL1Head(database, &core.L1Head{
				BlockNumber: tip,
				BlockHash:   blocks[tip].Header.Hash,
				StateRoot:   felt.NewRandom[felt.Felt](),
			}))

			m := historyprunner.New(retainedBlocks, minAge)
			state, err := m.Migrate(t.Context(), database, &networks.Mainnet, log.NewNopZapLogger())
			require.NoError(t, err)
			require.Nil(t, state)

			testutils.AssertPostPruneState(t, database, blocks, oldestBlockKept, lag)
		})
	}
}

// TestMigrate_NoOpWhenChainShorterThanRetention covers the early exit when
// minHead < retainedBlocks: nothing to prune yet. The rolling pruner takes
// over once blocks arrive past the threshold.
func TestMigrate_NoOpWhenChainShorterThanRetention(t *testing.T) {
	const totalBlocks uint64 = 5
	const retainedBlocks uint64 = 10

	for _, layout := range layouts {
		t.Run(layout.String(), func(t *testing.T) {
			database, blocks := setupChain(t, totalBlocks, layout)

			m := historyprunner.New(retainedBlocks, 0)
			state, err := m.Migrate(t.Context(), database, &networks.Mainnet, log.NewNopZapLogger())
			require.NoError(t, err)
			require.Nil(t, state)

			// All blocks intact.
			for _, b := range blocks {
				testutils.AssertBlockExists(t, database, b)
			}
		})
	}
}

// TestMigrate_NoOpOnEmptyDB covers the early exit when the chain head
// pointer is missing. This is the cold-start path on a fresh node.
func TestMigrate_NoOpOnEmptyDB(t *testing.T) {
	database := testutils.NewPebbleTestDB(t)
	m := historyprunner.New(10, 0)
	state, err := m.Migrate(t.Context(), database, &networks.Mainnet, log.NewNopZapLogger())
	require.NoError(t, err)
	require.Nil(t, state)
}

// cancelAfterReads wraps a KeyValueStore and cancels the supplied context on
// the n-th read: a block header Get or an iterator Next or Seek. Those are the
// units every step advances by, history entries in the scans and blocks in the
// lookup restore, so each run gets some work done before it is cancelled.
// First is not counted, since the layout detection on every run probes the
// buckets with it. The lookup restore reads from concurrent workers, hence the
// atomic counter.
type cancelAfterReads struct {
	db.KeyValueStore
	cancel context.CancelFunc
	after  int64
	count  atomic.Int64
}

func (c *cancelAfterReads) read() {
	if c.count.Add(1) == c.after {
		c.cancel()
	}
}

func (c *cancelAfterReads) Get(key []byte, cb func([]byte) error) error {
	if len(key) > 0 && key[0] == byte(db.BlockHeadersByNumber) {
		c.read()
	}
	return c.KeyValueStore.Get(key, cb)
}

func (c *cancelAfterReads) NewIterator(prefix []byte, withUpperBound bool) (db.Iterator, error) {
	it, err := c.KeyValueStore.NewIterator(prefix, withUpperBound)
	if err != nil {
		return nil, err
	}
	return &countingIterator{Iterator: it, read: c.read}, nil
}

type countingIterator struct {
	db.Iterator
	read func()
}

func (i *countingIterator) Next() bool {
	i.read()
	return i.Iterator.Next()
}

func (i *countingIterator) Seek(key []byte) bool {
	i.read()
	return i.Iterator.Seek(key)
}

// runCancellable runs Migrate with a wrapped DB that cancels the context
// after `cancelAfter` reads. Returns the intermediate state blob the migrator
// persisted, or nil if it ran to completion.
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
	wrapped := &cancelAfterReads{KeyValueStore: database, cancel: cancel, after: int64(cancelAfter)}

	m := historyprunner.New(retainedBlocks, 0)
	require.NoError(t, m.Before(prevState))
	state, err := m.Migrate(ctx, wrapped, &networks.Mainnet, log.NewNopZapLogger())
	require.NoError(t, err)
	return state
}

// TestMigrate_CancelAndResume verifies that a migration interrupted again and
// again, in every phase, ends in the same database as a single-shot run, in
// both history layouts.
func TestMigrate_CancelAndResume(t *testing.T) {
	const totalBlocks uint64 = 30
	const retainedBlocks uint64 = 10
	const oldestBlockKept = totalBlocks - retainedBlocks - 1 // 19
	lag := core.BlockHashLag

	for _, layout := range layouts {
		t.Run(layout.String(), func(t *testing.T) {
			database, blocks := setupChain(t, totalBlocks, layout)
			want := testutils.HistoryAnswers(t, database, blocks, oldestBlockKept-1, totalBlocks-1)

			var (
				state    []byte
				attempts int
			)
			for {
				attempts++
				require.Less(t, attempts, 1000, "every run must make progress")
				state = runCancellable(t, state, database, 2, retainedBlocks)
				if state == nil {
					break
				}
			}
			assert.Greater(t, attempts, 10,
				"the test should actually exercise cancellation in every phase")

			testutils.AssertPostPruneState(t, database, blocks, oldestBlockKept, lag)
			assert.Equal(t, want,
				testutils.HistoryAnswers(t, database, blocks, oldestBlockKept-1, totalBlocks-1))
		})
	}
}

// TestMigrate_RejectsMixedHistoryLayouts covers a new-state migration that was
// interrupted midway: some history is converted, the rest is not.
func TestMigrate_RejectsMixedHistoryLayouts(t *testing.T) {
	database, _ := setupChain(t, 30, testutils.DeprecatedLayout)
	addr, value := felt.FromUint64[felt.Felt](1), felt.FromUint64[felt.Felt](1)
	require.NoError(t, state.WriteNonceHistory(database, &addr, 0, &value))

	_, err := historyprunner.New(10, 0).
		Migrate(t.Context(), database, &networks.Mainnet, log.NewNopZapLogger())
	require.ErrorContains(t, err, "history is partly migrated to the new state")
}

func bucketHasKeys(t *testing.T, r db.KeyValueReader, bucket db.Bucket) bool {
	t.Helper()
	it, err := r.NewIterator(bucket.Key(), true)
	require.NoError(t, err)
	defer it.Close()
	return it.First()
}
