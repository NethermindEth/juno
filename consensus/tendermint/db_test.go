package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

func newTestTMDB(t *testing.T) (TendermintDB[value, felt.Felt, felt.Felt], db.KeyValueStore, string) {
	t.Helper()
	dbPath := t.TempDir()
	testDB, err := pebble.New(dbPath)
	require.NoError(t, err)

	tmState := NewTendermintDB[value, felt.Felt, felt.Felt](testDB, height(0))
	require.NotNil(t, tmState)

	return tmState, testDB, dbPath
}

func reopenTestTMDB(t *testing.T, oldDB db.KeyValueStore, dbPath string, testHeight height) (TendermintDB[value, felt.Felt, felt.Felt], db.KeyValueStore) {
	t.Helper()
	require.NoError(t, oldDB.Close())

	newDB, err := pebble.New(dbPath)
	require.NoError(t, err)

	tmState := NewTendermintDB[value, felt.Felt, felt.Felt](newDB, testHeight)
	return tmState, newDB
}

func TestWALLifecycle(t *testing.T) {
	testHeight := height(1000)
	testRound := round(1)
	testStep := prevote

	sender1 := *new(felt.Felt).SetUint64(1)
	sender2 := *new(felt.Felt).SetUint64(2)
	sender3 := *new(felt.Felt).SetUint64(3)
	var val1 value = 10
	valHash1 := val1.Hash()

	proposal := Proposal[value, felt.Felt, felt.Felt]{
		MessageHeader: MessageHeader[felt.Felt]{Height: testHeight, Round: testRound, Sender: sender1},
		ValidRound:    testRound,
		Value:         utils.HeapPtr(val1),
	}
	prevote := Prevote[felt.Felt, felt.Felt]{
		MessageHeader: MessageHeader[felt.Felt]{Height: testHeight, Round: testRound, Sender: sender2},
		ID:            &valHash1,
	}
	precommit := Precommit[felt.Felt, felt.Felt]{
		MessageHeader: MessageHeader[felt.Felt]{Height: testHeight, Round: testRound, Sender: sender3},
		ID:            &valHash1,
	}
	timeoutMsg := timeout{Height: testHeight, Round: testRound, Step: testStep}

	expectedEntries := []walEntry[value, felt.Felt, felt.Felt]{
		{Type: MessageTypeProposal, Entry: proposal},
		{Type: MessageTypePrevote, Entry: prevote},
		{Type: MessageTypePrecommit, Entry: precommit},
		{Type: MessageTypeTimeout, Entry: timeoutMsg},
	}

	tmState, db, dbPath := newTestTMDB(t)

	t.Run("Write entries", func(t *testing.T) {
		for _, entry := range expectedEntries {
			require.NoError(t, tmState.SetWALEntry(entry.Entry))
		}
	})

	t.Run("Commit batch and get entries", func(t *testing.T) {
		require.NoError(t, tmState.CommitBatch())
		retrieved, err := tmState.GetWALMsgs(testHeight)
		require.NoError(t, err)
		require.ElementsMatch(t, expectedEntries, retrieved)
	})

	t.Run("Reload the db and get entries", func(t *testing.T) {
		tmState, _ = reopenTestTMDB(t, db, dbPath, testHeight)
		retrieved, err := tmState.GetWALMsgs(testHeight)
		require.NoError(t, err)
		require.ElementsMatch(t, expectedEntries, retrieved)
	})

	t.Run("Delete entries", func(t *testing.T) {
		require.NoError(t, tmState.DeleteWALMsgs(testHeight))
	})

	t.Run("Commit batch and get entries (after deletion)", func(t *testing.T) {
		require.NoError(t, tmState.CommitBatch())
		_, err := tmState.GetWALMsgs(testHeight)
		require.NoError(t, err)
	})
}
