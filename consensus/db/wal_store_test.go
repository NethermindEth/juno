package db_test

import (
	"context"
	"os"
	"strings"
	"testing"

	consensusdb "github.com/NethermindEth/juno/consensus/db"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/types/wal"
	"github.com/NethermindEth/juno/db/remote"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestNewTendermintWALStoreRejectsDatabaseWithoutLocalPath(t *testing.T) {
	testDB, err := remote.New(
		"localhost:1234",
		context.Background(),
		log.NewNopZapLogger(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, testDB.Close())
	}()

	walStore, err := consensusdb.NewTendermintWALStore[
		starknet.Value,
		starknet.Hash,
		starknet.Address,
	](testDB)
	require.Error(t, err)
	require.Nil(t, walStore)
}

func TestSetWALEntryRejectsNilEntries(t *testing.T) {
	walStore, testDB, _ := newTestTendermintWALStore(t)
	defer func() {
		require.NoError(t, walStore.Close())
		require.NoError(t, testDB.Close())
	}()

	tests := map[string]starknet.WALEntry{
		"nil interface": nil,
		"nil start":     (*wal.Start)(nil),
		"nil proposal":  (*starknet.WALProposal)(nil),
		"nil prevote":   (*starknet.WALPrevote)(nil),
		"nil precommit": (*starknet.WALPrecommit)(nil),
		"nil timeout":   (*starknet.WALTimeout)(nil),
	}

	for name, entry := range tests {
		t.Run(name, func(t *testing.T) {
			require.Error(t, walStore.SetWALEntry(entry))
		})
	}
}

func TestWALReopenLoadsInterleavedEntries(t *testing.T) {
	firstHeight := types.Height(1000)
	secondHeight := firstHeight + 1
	thirdHeight := secondHeight + 1
	firstHeightFirstBatch := makeWALEntriesForHeight(firstHeight)
	firstHeightSecondBatch := makeWALEntriesForHeight(firstHeight)
	secondHeightFirstBatch := makeWALEntriesForHeight(secondHeight)
	thirdHeightFirstBatch := makeWALEntriesForHeight(thirdHeight)

	walStore, testDB, dbPath := newTestTendermintWALStore(t)
	defer func() {
		require.NoError(t, walStore.Close())
		require.NoError(t, testDB.Close())
	}()

	for entryIndex := range firstHeightFirstBatch {
		require.NoError(t, walStore.SetWALEntry(firstHeightFirstBatch[entryIndex]))
		require.NoError(t, walStore.SetWALEntry(secondHeightFirstBatch[entryIndex]))
	}
	require.NoError(t, walStore.Flush())
	walStore, testDB = reopenTestTendermintWALStore(t, walStore, testDB, dbPath)
	assertLoadedEntries(t, walStore, firstHeightFirstBatch, secondHeightFirstBatch)

	writeAndFlushEntries(t, walStore, firstHeightSecondBatch, thirdHeightFirstBatch)
	walStore, testDB = reopenTestTendermintWALStore(t, walStore, testDB, dbPath)
	assertLoadedEntries(
		t,
		walStore,
		firstHeightFirstBatch,
		firstHeightSecondBatch,
		secondHeightFirstBatch,
		thirdHeightFirstBatch,
	)
}

func TestWALReopenSkipsPrunedHeight(t *testing.T) {
	firstHeight := types.Height(1000)
	secondHeight := firstHeight + 1
	thirdHeight := secondHeight + 1
	firstHeightEntries := makeWALEntriesForHeight(firstHeight)
	secondHeightEntries := makeWALEntriesForHeight(secondHeight)
	thirdHeightEntries := makeWALEntriesForHeight(thirdHeight)

	walStore, testDB, dbPath := newTestTendermintWALStore(t)
	defer func() {
		require.NoError(t, walStore.Close())
		require.NoError(t, testDB.Close())
	}()

	writeAndFlushEntries(t, walStore, firstHeightEntries, secondHeightEntries, thirdHeightEntries)
	walStore, testDB = reopenTestTendermintWALStore(t, walStore, testDB, dbPath)

	require.NoError(t, walStore.DeleteWALEntries(firstHeight))
	require.NoError(t, walStore.Flush())

	walStore, testDB = reopenTestTendermintWALStore(t, walStore, testDB, dbPath)
	assertLoadedEntries(t, walStore, secondHeightEntries, thirdHeightEntries)
}

func TestWALCloseFlushesPendingEntries(t *testing.T) {
	committedHeight := types.Height(1000)
	pendingHeight := committedHeight + 1
	committedEntries := makeWALEntriesForHeight(committedHeight)
	pendingEntries := makeWALEntriesForHeight(pendingHeight)

	walStore, testDB, dbPath := newTestTendermintWALStore(t)
	writeAndFlushEntries(t, walStore, committedEntries)
	for _, entry := range pendingEntries {
		require.NoError(t, walStore.SetWALEntry(entry))
	}

	require.NoError(t, walStore.Close())
	require.NoError(t, testDB.Close())

	walStore, testDB = openTestTendermintWALStore(t, dbPath)
	defer func() {
		require.NoError(t, walStore.Close())
		require.NoError(t, testDB.Close())
	}()

	assertLoadedEntries(
		t,
		walStore,
		committedEntries,
		pendingEntries,
	)
}

func TestPruningAllHeightsRemovesWALFiles(t *testing.T) {
	// Must match cleanupPruneRecordInterval to trigger WAL file cleanup.
	const heightsToDelete = 256

	firstHeight := types.Height(1000)
	entryBatches := make([][]starknet.WALEntry, heightsToDelete)
	for i := range entryBatches {
		entryBatches[i] = makeWALEntriesForHeight(firstHeight + types.Height(i))
	}

	walStore, testDB, dbPath := newTestTendermintWALStore(t)
	defer func() {
		require.NoError(t, walStore.Close())
		require.NoError(t, testDB.Close())
	}()

	writeAndFlushEntries(t, walStore, entryBatches...)

	for i := range entryBatches {
		require.NoError(t, walStore.DeleteWALEntries(firstHeight+types.Height(i)))
		require.NoError(t, walStore.Flush())
	}

	assertNoEntries(t, walStore)
	assertNoWALLogFiles(t, dbPath)

	walStore, testDB = reopenTestTendermintWALStore(t, walStore, testDB, dbPath)
	writeAndFlushEntries(t, walStore, makeWALEntriesForHeight(firstHeight))
	assertNoEntries(t, walStore)
}

func TestWALReopenRejectsWritesAtPrunedHeight(t *testing.T) {
	prunedHeight := types.Height(1000)
	retainedHeight := prunedHeight + 1
	retainedEntries := makeWALEntriesForHeight(retainedHeight)

	walStore, testDB, dbPath := newTestTendermintWALStore(t)
	defer func() {
		require.NoError(t, walStore.Close())
		require.NoError(t, testDB.Close())
	}()

	writeAndFlushEntries(t, walStore, retainedEntries)
	require.NoError(t, walStore.DeleteWALEntries(prunedHeight))
	require.NoError(t, walStore.Flush())

	walStore, testDB = reopenTestTendermintWALStore(t, walStore, testDB, dbPath)
	writeAndFlushEntries(t, walStore, makeWALEntriesForHeight(prunedHeight))

	assertLoadedEntries(t, walStore, retainedEntries)
}

func assertNoWALLogFiles(t *testing.T, dbPath string) {
	t.Helper()

	dirEntries, err := os.ReadDir(consensusdb.DefaultWALDir(dbPath))
	require.NoError(t, err)
	for _, entry := range dirEntries {
		require.False(t, strings.HasSuffix(entry.Name(), ".log"), entry.Name())
	}
}
