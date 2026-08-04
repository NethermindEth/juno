package statedifflength_test

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/NethermindEth/juno/migration/statedifflength"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
)

// writeBlock stores a state update with storageDiffs entries plus a commitments
// record
func writeBlock(
	t *testing.T,
	database db.KeyValueStore,
	blockNum, storageDiffs uint64,
	length *uint64,
) {
	t.Helper()

	diffs := make(map[felt.Felt]map[felt.Felt]*felt.Felt)
	inner := make(map[felt.Felt]*felt.Felt, storageDiffs)
	for i := range storageDiffs {
		inner[*new(felt.Felt).SetUint64(i)] = new(felt.Felt).SetUint64(i)
	}
	diffs[*new(felt.Felt).SetUint64(blockNum)] = inner

	require.NoError(t, core.WriteStateUpdateByBlockNum(database, blockNum, &core.StateUpdate{
		StateDiff: &core.StateDiff{StorageDiffs: diffs},
	}))
	require.NoError(t, core.WriteBlockCommitment(database, blockNum, &core.BlockCommitments{
		TransactionCommitment: new(felt.Felt).SetUint64(blockNum),
		StateDiffLength:       length,
	}))
}

func storedLength(t *testing.T, database db.KeyValueStore, blockNum uint64) *uint64 {
	t.Helper()
	commitments, err := core.GetBlockCommitmentByBlockNum(database, blockNum)
	require.NoError(t, err)
	return commitments.StateDiffLength
}

func run(t *testing.T, database db.KeyValueStore, m *statedifflength.Migrator) []byte {
	t.Helper()
	state, err := migrate(t.Context(), database, m)
	require.NoError(t, err)
	return state
}

func migrate(
	ctx context.Context,
	database db.KeyValueStore,
	m *statedifflength.Migrator,
) ([]byte, error) {
	return m.Migrate(ctx, database, &networks.Mainnet, log.NewNopZapLogger())
}

// cancelAfter reports a cancelled context once Err has been polled n times, so a
// mid-loop cancellation lands on a known block instead of on a timer.
type cancelAfter struct {
	context.Context
	polls *int
	n     int
}

func (c cancelAfter) Err() error {
	if *c.polls >= c.n {
		return context.Canceled
	}
	*c.polls++
	return nil
}

func TestMigrateBackfillsMissingLengths(t *testing.T) {
	database := memory.New()
	for blockNum := range uint64(3) {
		writeBlock(t, database, blockNum, blockNum+1, nil)
	}
	require.NoError(t, core.WriteChainHeight(database, 2))

	m := &statedifflength.Migrator{}
	require.NoError(t, m.Before(nil))
	require.Nil(t, run(t, database, m), "migration should not ask to re-run")

	for blockNum := range uint64(3) {
		length := storedLength(t, database, blockNum)
		require.NotNil(t, length, "block %d was not backfilled", blockNum)
		require.Equal(t, blockNum+1, *length, "block %d", blockNum)
	}
}

func TestMigrateLeavesExistingLengthsAlone(t *testing.T) {
	database := memory.New()
	stale := uint64(999)
	writeBlock(t, database, 0, 5, &stale)
	require.NoError(t, core.WriteChainHeight(database, 0))

	for range 2 {
		m := &statedifflength.Migrator{}
		require.NoError(t, m.Before(nil))
		run(t, database, m)
		require.Equal(t, uint64(999), *storedLength(t, database, 0))
	}
}

func TestMigrateSkipsBlocksWithoutCommitments(t *testing.T) {
	database := memory.New()
	// Block 0 pruned: neither commitments nor state update exist.
	writeBlock(t, database, 1, 2, nil)
	require.NoError(t, core.WriteChainHeight(database, 1))

	m := &statedifflength.Migrator{}
	require.NoError(t, m.Before(nil))
	run(t, database, m)

	require.Equal(t, uint64(2), *storedLength(t, database, 1))
}

func TestMigrateEmptyDatabase(t *testing.T) {
	database := memory.New()

	m := &statedifflength.Migrator{}
	require.NoError(t, m.Before(nil))
	require.Nil(t, run(t, database, m))
}

func TestMigrateResumesFromIntermediateState(t *testing.T) {
	database := memory.New()
	for blockNum := range uint64(3) {
		writeBlock(t, database, blockNum, blockNum+1, nil)
	}
	require.NoError(t, core.WriteChainHeight(database, 2))

	var state [8]byte
	binary.BigEndian.PutUint64(state[:], 2)

	m := &statedifflength.Migrator{}
	require.NoError(t, m.Before(state[:]))
	run(t, database, m)

	require.Nil(t, storedLength(t, database, 0), "block 0 should have been skipped")
	require.Nil(t, storedLength(t, database, 1), "block 1 should have been skipped")
	require.Equal(t, uint64(3), *storedLength(t, database, 2))
}

func TestBeforeRejectsMalformedState(t *testing.T) {
	m := &statedifflength.Migrator{}
	require.Error(t, m.Before([]byte{1, 2, 3}))
}

func TestMigrateCancellationFlushesAndResumes(t *testing.T) {
	database := memory.New()
	for blockNum := range uint64(4) {
		writeBlock(t, database, blockNum, blockNum+1, nil)
	}
	require.NoError(t, core.WriteChainHeight(database, 3))

	// Let blocks 0 and 1 through, then cancel before block 2.
	polls := 0
	ctx := cancelAfter{Context: t.Context(), polls: &polls, n: 2}

	m := &statedifflength.Migrator{}
	require.NoError(t, m.Before(nil))
	state, err := migrate(ctx, database, m)
	require.ErrorIs(t, err, context.Canceled)

	// The partial batch is flushed, so the work already done survives.
	require.Equal(t, uint64(1), *storedLength(t, database, 0))
	require.Equal(t, uint64(2), *storedLength(t, database, 1))
	require.Nil(t, storedLength(t, database, 2))

	// The state points at the first unprocessed block, not past it.
	require.Len(t, state, 8)
	require.Equal(t, uint64(2), binary.BigEndian.Uint64(state))

	resumed := &statedifflength.Migrator{}
	require.NoError(t, resumed.Before(state))
	require.Nil(t, run(t, database, resumed))

	for blockNum := range uint64(4) {
		require.Equal(t, blockNum+1, *storedLength(t, database, blockNum), "block %d", blockNum)
	}
}

func TestMigrateRotatesBatches(t *testing.T) {
	database := memory.New()
	for blockNum := range uint64(6) {
		writeBlock(t, database, blockNum, blockNum+1, nil)
	}
	require.NoError(t, core.WriteChainHeight(database, 5))

	m := &statedifflength.Migrator{BatchByteSize: 1}
	require.NoError(t, m.Before(nil))
	require.Nil(t, run(t, database, m))

	for blockNum := range uint64(6) {
		require.Equal(t, blockNum+1, *storedLength(t, database, blockNum), "block %d", blockNum)
	}
}

func TestMigrateStartsAfterPrunedPrefix(t *testing.T) {
	database := memory.New()
	// Blocks 0..4 pruned away; only 5 and 6 remain.
	for blockNum := range uint64(2) {
		writeBlock(t, database, 5+blockNum, blockNum+1, nil)
	}
	require.NoError(t, core.WriteChainHeight(database, 6))

	m := &statedifflength.Migrator{}
	require.NoError(t, m.Before(nil))
	require.Nil(t, run(t, database, m))

	require.Equal(t, uint64(1), *storedLength(t, database, 5))
	require.Equal(t, uint64(2), *storedLength(t, database, 6))
}
