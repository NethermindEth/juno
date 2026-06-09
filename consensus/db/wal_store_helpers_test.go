package db_test

import (
	"slices"
	"testing"

	consensusdb "github.com/NethermindEth/juno/consensus/db"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
	kvdb "github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebblev2"
	"github.com/stretchr/testify/require"
)

type testTendermintWALStore = consensusdb.TendermintWALStore[
	starknet.Value,
	starknet.Hash,
	starknet.Address,
]

func openTestTendermintWALStore(
	tb testing.TB,
	dbPath string,
) (testTendermintWALStore, kvdb.KeyValueStore) {
	tb.Helper()

	testDB, err := pebblev2.New(dbPath)
	require.NoError(tb, err)
	walStore, err := consensusdb.NewTendermintWALStore[
		starknet.Value,
		starknet.Hash,
		starknet.Address,
	](testDB)
	require.NoError(tb, err)

	return walStore, testDB
}

func newTestTendermintWALStore(
	tb testing.TB,
) (testTendermintWALStore, kvdb.KeyValueStore, string) {
	tb.Helper()
	dbPath := tb.TempDir()

	walStore, testDB := openTestTendermintWALStore(tb, dbPath)

	return walStore, testDB, dbPath
}

func reopenTestTendermintWALStore(
	t *testing.T,
	oldWALStore testTendermintWALStore,
	oldDB kvdb.KeyValueStore,
	dbPath string,
) (testTendermintWALStore, kvdb.KeyValueStore) {
	t.Helper()
	require.NoError(t, oldWALStore.Close())
	require.NoError(t, oldDB.Close())

	return openTestTendermintWALStore(t, dbPath)
}

func makeWALEntriesForHeight(height types.Height) []starknet.WALEntry {
	const round = types.Round(1)
	const timeoutStep = types.StepPrevote

	proposalSender := felt.FromUint64[starknet.Address](1)
	prevoteSender := felt.FromUint64[starknet.Address](2)
	precommitSender := felt.FromUint64[starknet.Address](3)
	proposalValue := felt.FromUint64[starknet.Value](10)
	proposalValueHash := proposalValue.Hash()

	proposal := starknet.WALProposal{
		MessageHeader: starknet.MessageHeader{Height: height, Round: round, Sender: proposalSender},
		ValidRound:    round,
		Value:         &proposalValue,
	}
	prevote := starknet.WALPrevote{
		MessageHeader: starknet.MessageHeader{Height: height, Round: round, Sender: prevoteSender},
		ID:            &proposalValueHash,
	}
	precommit := starknet.WALPrecommit{
		MessageHeader: starknet.MessageHeader{Height: height, Round: round, Sender: precommitSender},
		ID:            &proposalValueHash,
	}
	timeout := starknet.WALTimeout{Height: height, Round: round, Step: timeoutStep}

	return []starknet.WALEntry{
		&proposal,
		&prevote,
		&precommit,
		&timeout,
	}
}

func writeAndFlushEntries(
	t *testing.T,
	walStore testTendermintWALStore,
	entryList ...[]starknet.WALEntry,
) {
	t.Helper()
	for _, entries := range entryList {
		for _, entry := range entries {
			require.NoError(t, walStore.SetWALEntry(entry))
		}
	}
	require.NoError(t, walStore.Flush())
}

func assertLoadedEntries(
	t *testing.T,
	walStore testTendermintWALStore,
	entryList ...[]starknet.WALEntry,
) {
	t.Helper()
	index := 0
	entries := slices.Concat(entryList...)
	for entry, err := range walStore.LoadAllEntries() {
		require.NoError(t, err)
		require.Equal(t, entries[index], entry)
		index++
	}
	require.Equal(t, len(entries), index)
}

func assertNoEntries(t *testing.T, walStore testTendermintWALStore) {
	t.Helper()

	for entry, err := range walStore.LoadAllEntries() {
		require.NoError(t, err)
		require.FailNowf(t, "unexpected entry", "%v", entry)
	}
}
