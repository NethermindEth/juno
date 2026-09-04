package statedifflength_test

import (
	"context"
	"encoding/binary"
	"sync/atomic"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/migration/statedifflength"
	_ "github.com/NethermindEth/juno/utils/cbor/registry"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
)

// writeBlock stores a state update with storageDiffs entries plus a commitments
// record whose StateDiffLength is left unset, mimicking a pre-migration block.
func writeBlock(t *testing.T, database db.KeyValueStore, blockNum, storageDiffs uint64) {
	t.Helper()

	diffs := make(map[felt.Felt]map[felt.Felt]*felt.Felt)
	inner := make(map[felt.Felt]*felt.Felt, storageDiffs)
	for i := range storageDiffs {
		inner[felt.FromUint64[felt.Felt](i)] = felt.NewFromUint64[felt.Felt](i)
	}
	diffs[felt.FromUint64[felt.Felt](blockNum)] = inner

	require.NoError(t, core.WriteStateUpdateByBlockNum(database, blockNum, &core.StateUpdate{
		StateDiff: &core.StateDiff{StorageDiffs: diffs},
	}))
	require.NoError(t, core.WriteBlockCommitment(database, blockNum, &core.BlockCommitments{
		TransactionCommitment: felt.NewFromUint64[felt.Felt](blockNum),
	}))
}

func storedLength(t *testing.T, database db.KeyValueStore, blockNum uint64) uint64 {
	t.Helper()
	commitments, err := core.GetBlockCommitmentByBlockNum(database, blockNum)
	require.NoError(t, err)
	return commitments.StateDiffLength
}

func TestMigrateBackfillsMissingLengths(t *testing.T) {
	database := memory.New()
	const blocks = 200
	for blockNum := range uint64(blocks) {
		writeBlock(t, database, blockNum, blockNum+1)
	}
	require.NoError(t, core.WriteChainHeight(database, blocks-1))

	m := &statedifflength.Migrator{}
	require.NoError(t, m.Before(nil))
	state, err := m.Migrate(t.Context(), database, &networks.Mainnet, log.NewNopZapLogger())
	require.NoError(t, err)
	require.Nil(t, state)

	for blockNum := range uint64(blocks) {
		require.Equal(t, blockNum+1, storedLength(t, database, blockNum), "block %d", blockNum)
	}
}

func TestMigrateEmptyDatabase(t *testing.T) {
	database := memory.New()

	m := &statedifflength.Migrator{}
	require.NoError(t, m.Before(nil))
	state, err := m.Migrate(t.Context(), database, &networks.Mainnet, log.NewNopZapLogger())
	require.NoError(t, err)
	require.Nil(t, state)
}

type cancelAfterReads struct {
	db.KeyValueStore
	remaining atomic.Int64
	cancel    context.CancelFunc
}

func (c *cancelAfterReads) Get(key []byte, cb func(value []byte) error) error {
	if c.remaining.Add(-1) == 0 {
		c.cancel()
	}
	return c.KeyValueStore.Get(key, cb)
}

func TestMigrateCancelledResumesWithoutGaps(t *testing.T) {
	database := memory.New()
	const blocks = 3000
	for blockNum := range uint64(blocks) {
		writeBlock(t, database, blockNum, 1)
	}
	require.NoError(t, core.WriteChainHeight(database, blocks-1))

	ctx, cancel := context.WithCancel(t.Context())
	interrupting := &cancelAfterReads{KeyValueStore: database, cancel: cancel}
	interrupting.remaining.Store(1000)

	m := &statedifflength.Migrator{}
	require.NoError(t, m.Before(nil))
	state, err := m.Migrate(ctx, interrupting, &networks.Mainnet, log.NewNopZapLogger())
	require.NoError(t, err)

	// The cancel lands on the 1000th read and the migration spends two per block, so
	// the run is always interrupted mid-range with a checkpoint to resume from.
	require.NotNil(t, state, "an interrupted run must ask to re-run")
	resumeFrom := binary.BigEndian.Uint64(state)
	require.NotZero(t, resumeFrom, "the cancel must land after some blocks were committed")
	require.LessOrEqual(t, resumeFrom, uint64(blocks), "checkpoint past the chain height")
	t.Logf("cancelled with checkpoint at %d of %d", resumeFrom, blocks)

	for blockNum := range resumeFrom {
		require.Equal(t, uint64(1), storedLength(t, database, blockNum),
			"block %d below the checkpoint must be backfilled", blockNum)
	}
	for blockNum := resumeFrom; blockNum < blocks; blockNum++ {
		require.Zero(t, storedLength(t, database, blockNum),
			"block %d at or above the checkpoint must be untouched", blockNum)
	}

	resumed := &statedifflength.Migrator{}
	require.NoError(t, resumed.Before(state))
	resumedState, err := resumed.Migrate(
		t.Context(), database, &networks.Mainnet, log.NewNopZapLogger(),
	)
	require.NoError(t, err)
	require.Nil(t, resumedState)

	for blockNum := range uint64(blocks) {
		require.Equal(t, uint64(1), storedLength(t, database, blockNum), "block %d", blockNum)
	}
}

func TestMigrateResumesFromCheckpointWithoutGaps(t *testing.T) {
	database := memory.New()
	const blocks = 300
	for blockNum := range uint64(blocks) {
		writeBlock(t, database, blockNum, blockNum+1)
	}
	require.NoError(t, core.WriteChainHeight(database, blocks-1))

	// Simulate a checkpoint left by a previous graceful shutdown: blocks below it
	// are already done, the migration must cover exactly [checkpoint, height].
	const checkpoint = 120
	var state [8]byte
	binary.BigEndian.PutUint64(state[:], checkpoint)

	m := &statedifflength.Migrator{}
	require.NoError(t, m.Before(state[:]))
	migrated, err := m.Migrate(t.Context(), database, &networks.Mainnet, log.NewNopZapLogger())
	require.NoError(t, err)
	require.Nil(t, migrated)

	// Below the checkpoint stays untouched (still 0); from it up every block is set,
	// with no gaps.
	for blockNum := range uint64(checkpoint) {
		require.Zero(t, storedLength(t, database, blockNum), "block %d below checkpoint", blockNum)
	}
	for blockNum := uint64(checkpoint); blockNum < blocks; blockNum++ {
		require.Equal(t, blockNum+1, storedLength(t, database, blockNum), "block %d", blockNum)
	}
}

func TestBeforeRejectsMalformedState(t *testing.T) {
	m := &statedifflength.Migrator{}
	require.Error(t, m.Before([]byte{1, 2, 3}))
}

func TestMigrateStartsAfterPrunedPrefix(t *testing.T) {
	database := memory.New()
	// Blocks 0..4 pruned away; only 5 and 6 remain. The migration must start at 5
	// rather than failing on the missing records below it.
	for blockNum := range uint64(2) {
		writeBlock(t, database, 5+blockNum, blockNum+1)
	}
	require.NoError(t, core.WriteChainHeight(database, 6))

	m := &statedifflength.Migrator{}
	require.NoError(t, m.Before(nil))
	state, err := m.Migrate(t.Context(), database, &networks.Mainnet, log.NewNopZapLogger())
	require.NoError(t, err)
	require.Nil(t, state)

	require.Equal(t, uint64(1), storedLength(t, database, 5))
	require.Equal(t, uint64(2), storedLength(t, database, 6))
}

func TestMigrateFailsOnMissingRecordInRange(t *testing.T) {
	database := memory.New()
	writeBlock(t, database, 0, 1)
	require.NoError(t, core.WriteBlockCommitment(database, 1, &core.BlockCommitments{
		TransactionCommitment: new(felt.Felt).SetUint64(1),
	}))
	require.NoError(t, core.WriteChainHeight(database, 1))

	m := &statedifflength.Migrator{}
	require.NoError(t, m.Before(nil))
	_, err := m.Migrate(t.Context(), database, &networks.Mainnet, log.NewNopZapLogger())
	require.ErrorIs(t, err, db.ErrKeyNotFound)
}
