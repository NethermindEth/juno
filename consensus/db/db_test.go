package db

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebblev2"
	"github.com/stretchr/testify/require"
)

type testTendermintDB = TendermintDB[starknet.Value, starknet.Hash, starknet.Address]

func newTestTMDB(t *testing.T) (testTendermintDB, db.KeyValueStore, string) {
	t.Helper()
	dbPath := t.TempDir()
	testDB, err := pebblev2.New(dbPath)
	require.NoError(t, err)

	tmState := NewTendermintDB[starknet.Value, starknet.Hash, starknet.Address](testDB)
	require.NotNil(t, tmState)

	return tmState, testDB, dbPath
}

func reopenTestTMDB(t *testing.T, oldDB db.KeyValueStore, dbPath string) (testTendermintDB, db.KeyValueStore) {
	t.Helper()
	require.NoError(t, oldDB.Close())

	newDB, err := pebblev2.New(dbPath)
	require.NoError(t, err)

	tmState := NewTendermintDB[starknet.Value, starknet.Hash, starknet.Address](newDB)
	return tmState, newDB
}

func buildExpectedEntries(testHeight types.Height) []starknet.WALEntry {
	testRound := types.Round(1)
	testStep := types.StepPrevote

	sender1 := felt.FromUint64[starknet.Address](1)
	sender2 := felt.FromUint64[starknet.Address](2)
	sender3 := felt.FromUint64[starknet.Address](3)
	val1 := felt.FromUint64[starknet.Value](10)
	valHash1 := val1.Hash()

	proposal := starknet.WALProposal{
		MessageHeader: starknet.MessageHeader{Height: testHeight, Round: testRound, Sender: sender1},
		ValidRound:    testRound,
		Value:         &val1,
	}
	prevote := starknet.WALPrevote{
		MessageHeader: starknet.MessageHeader{Height: testHeight, Round: testRound, Sender: sender2},
		ID:            &valHash1,
	}
	precommit := starknet.WALPrecommit{
		MessageHeader: starknet.MessageHeader{Height: testHeight, Round: testRound, Sender: sender3},
		ID:            &valHash1,
	}
	timeoutMsg := starknet.WALTimeout{Height: testHeight, Round: testRound, Step: testStep}

	return []starknet.WALEntry{
		&proposal,
		&prevote,
		&precommit,
		&timeoutMsg,
	}
}

// This utility function mixes the entries from the different batches into a single slice.
// The entries from the same batch are kept in order.
// The entries from the different batches are interleaved in a random order.
func mixEntries(entryList [][]starknet.WALEntry) []starknet.WALEntry {
	totalCount := 0
	for _, entries := range entryList {
		totalCount += len(entries)
	}

	mixed := make([]starknet.WALEntry, totalCount)
	for i := range mixed {
		selected := rand.IntN(len(entryList))
		mixed[i] = entryList[selected][0]
		entryList[selected] = entryList[selected][1:]
		if len(entryList[selected]) == 0 {
			entryList[selected] = entryList[len(entryList)-1]
			entryList = entryList[:len(entryList)-1]
		}
	}
	return mixed
}

func testWrite(t *testing.T, tmState testTendermintDB, entryList ...[]starknet.WALEntry) {
	t.Helper()
	for _, entry := range mixEntries(entryList) {
		require.NoError(t, tmState.SetWALEntry(entry))
	}
	require.NoError(t, tmState.Flush())
}

func testRead(t *testing.T, tmState testTendermintDB, entryList ...[]starknet.WALEntry) {
	t.Helper()
	index := 0
	entries := slices.Concat(entryList...)
	for entry, err := range tmState.LoadAllEntries() {
		require.NoError(t, err)
		require.Equal(t, entries[index], entry)
		index++
	}
}

func TestWALLifecycle(t *testing.T) {
	firstHeight := types.Height(1000)
	secondHeight := firstHeight + 1
	thirdHeight := secondHeight + 1
	firstHeightFirstBatch := buildExpectedEntries(firstHeight)
	firstHeightSecondBatch := buildExpectedEntries(firstHeight)
	secondHeightFirstBatch := buildExpectedEntries(secondHeight)
	secondHeightSecondBatch := buildExpectedEntries(secondHeight)
	thirdHeightFirstBatch := buildExpectedEntries(thirdHeight)
	thirdHeightSecondBatch := buildExpectedEntries(thirdHeight)
	tmState, db, dbPath := newTestTMDB(t)

	t.Run("Write entries from height 1 batch 1 and height 2 batch 1", func(t *testing.T) {
		testWrite(t, tmState, firstHeightFirstBatch, secondHeightFirstBatch)
	})

	t.Run("Reload the db and get entries from the first 2 heights", func(t *testing.T) {
		tmState, db = reopenTestTMDB(t, db, dbPath)
		testRead(t, tmState, firstHeightFirstBatch, secondHeightFirstBatch)
	})

	t.Run("Write entries from height 1 batch 2 and height 3 batch 1", func(t *testing.T) {
		testWrite(t, tmState, firstHeightSecondBatch, thirdHeightFirstBatch)
	})

	t.Run("Reload the db and get entries from 3 heights", func(t *testing.T) {
		tmState, db = reopenTestTMDB(t, db, dbPath)
		testRead(
			t,
			tmState,
			firstHeightFirstBatch,
			firstHeightSecondBatch,
			secondHeightFirstBatch,
			thirdHeightFirstBatch,
		)
	})

	t.Run("Delete entries", func(t *testing.T) {
		require.NoError(t, tmState.DeleteWALEntries(firstHeight))
	})

	t.Run("Write entries from height 2 batch 2 and height 3 batch 2", func(t *testing.T) {
		testWrite(t, tmState, secondHeightSecondBatch, thirdHeightSecondBatch)
	})

	t.Run("Reload the db and get entries from the last 2 heights", func(t *testing.T) {
		tmState, db = reopenTestTMDB(t, db, dbPath)
		testRead(
			t,
			tmState,
			secondHeightFirstBatch,
			secondHeightSecondBatch,
			thirdHeightFirstBatch,
			thirdHeightSecondBatch,
		)
	})
}
