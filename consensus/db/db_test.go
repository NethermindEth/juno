package db

import (
	"testing"

	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

type value uint64

func (v value) Hash() felt.Felt {
	return *new(felt.Felt).SetUint64(uint64(v))
}

func newTestTMDB(t *testing.T) (TendermintDB[value, felt.Felt, felt.Felt], db.KeyValueStore, string) {
	t.Helper()
	dbPath := t.TempDir()
	testDB, err := pebble.New(dbPath)
	require.NoError(t, err)

	tmState := NewTendermintDB[value, felt.Felt, felt.Felt](testDB, types.Height(0))
	require.NotNil(t, tmState)

	return tmState, testDB, dbPath
}

func reopenTestTMDB(t *testing.T, oldDB db.KeyValueStore, dbPath string, testHeight types.Height) (TendermintDB[value, felt.Felt, felt.Felt], db.KeyValueStore) {
	t.Helper()
	require.NoError(t, oldDB.Close())

	newDB, err := pebble.New(dbPath)
	require.NoError(t, err)

	tmState := NewTendermintDB[value, felt.Felt, felt.Felt](newDB, testHeight)
	return tmState, newDB
}

func TestWALLifecycle(t *testing.T) {
	testHeight := types.Height(1000)
	testRound := types.Round(1)
	testStep := types.StepPrevote

	sender1 := *new(felt.Felt).SetUint64(1)
	sender2 := *new(felt.Felt).SetUint64(2)
	sender3 := *new(felt.Felt).SetUint64(3)
	var val1 value = 10
	valHash1 := val1.Hash()

	proposal := types.Proposal[value, felt.Felt, felt.Felt]{
		MessageHeader: types.MessageHeader[felt.Felt]{Height: testHeight, Round: testRound, Sender: sender1},
		ValidRound:    testRound,
		Value:         utils.HeapPtr(val1),
	}
	prevote := types.Prevote[felt.Felt, felt.Felt]{
		MessageHeader: types.MessageHeader[felt.Felt]{Height: testHeight, Round: testRound, Sender: sender2},
		ID:            &valHash1,
	}
	precommit := types.Precommit[felt.Felt, felt.Felt]{
		MessageHeader: types.MessageHeader[felt.Felt]{Height: testHeight, Round: testRound, Sender: sender3},
		ID:            &valHash1,
	}
	timeoutMsg := types.Timeout{Height: testHeight, Round: testRound, Step: testStep}

	expectedEntries := []WalEntry[value, felt.Felt, felt.Felt]{
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
