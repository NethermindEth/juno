package db

import (
	"testing"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/hash"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

type testTendermintDB = TendermintDB[starknet.Value, hash.Hash, starknet.Address]

func newTestTMDB(t *testing.T) (testTendermintDB, db.KeyValueStore, string) {
	t.Helper()
	dbPath := t.TempDir()
	testDB, err := pebble.New(dbPath)
	require.NoError(t, err)

	tmState := NewTendermintDB[starknet.Value, hash.Hash, starknet.Address](testDB, types.Height(0))
	require.NotNil(t, tmState)

	return tmState, testDB, dbPath
}

func reopenTestTMDB(t *testing.T, oldDB db.KeyValueStore, dbPath string, testHeight types.Height) (testTendermintDB, db.KeyValueStore) {
	t.Helper()
	require.NoError(t, oldDB.Close())

	newDB, err := pebble.New(dbPath)
	require.NoError(t, err)

	tmState := NewTendermintDB[starknet.Value, hash.Hash, starknet.Address](newDB, testHeight)
	return tmState, newDB
}

func TestWALLifecycle(t *testing.T) {
	testHeight := types.Height(1000)
	testRound := types.Round(1)
	testStep := types.StepPrevote

	sender1 := starknet.Address(*new(felt.Felt).SetUint64(1))
	sender2 := starknet.Address(*new(felt.Felt).SetUint64(2))
	sender3 := starknet.Address(*new(felt.Felt).SetUint64(3))
	val1 := starknet.Value(felt.FromUint64(10))
	valHash1 := val1.Hash()

	proposal := starknet.Proposal{
		MessageHeader: starknet.MessageHeader{Height: testHeight, Round: testRound, Sender: sender1},
		ValidRound:    testRound,
		Value:         utils.HeapPtr(val1),
	}
	prevote := starknet.Prevote{
		MessageHeader: starknet.MessageHeader{Height: testHeight, Round: testRound, Sender: sender2},
		ID:            &valHash1,
	}
	precommit := starknet.Precommit{
		MessageHeader: starknet.MessageHeader{Height: testHeight, Round: testRound, Sender: sender3},
		ID:            &valHash1,
	}
	timeoutMsg := types.Timeout{Height: testHeight, Round: testRound, Step: testStep}

	expectedEntries := []WalEntry[starknet.Value, hash.Hash, starknet.Address]{
		{Type: types.MessageTypeProposal, Entry: proposal},
		{Type: types.MessageTypePrevote, Entry: prevote},
		{Type: types.MessageTypePrecommit, Entry: precommit},
		{Type: types.MessageTypeTimeout, Entry: timeoutMsg},
	}

	tmState, db, dbPath := newTestTMDB(t)

	t.Run("Write entries", func(t *testing.T) {
		for _, entry := range expectedEntries {
			require.NoError(t, tmState.SetWALEntry(entry.Entry))
		}
	})

	t.Run("Commit batch and get entries", func(t *testing.T) {
		require.NoError(t, tmState.Flush())
		retrieved, err := tmState.GetWALEntries(testHeight)
		require.NoError(t, err)
		require.ElementsMatch(t, expectedEntries, retrieved)
	})

	t.Run("Reload the db and get entries", func(t *testing.T) {
		tmState, _ = reopenTestTMDB(t, db, dbPath, testHeight)
		retrieved, err := tmState.GetWALEntries(testHeight)
		require.NoError(t, err)
		require.Equal(t, expectedEntries, retrieved)
	})

	t.Run("Delete entries", func(t *testing.T) {
		require.NoError(t, tmState.DeleteWALEntries(testHeight))
	})

	t.Run("Commit batch and get entries (after deletion)", func(t *testing.T) {
		require.NoError(t, tmState.Flush())
		_, err := tmState.GetWALEntries(testHeight)
		require.NoError(t, err)
	})
}
