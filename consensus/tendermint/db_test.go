package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

// Test helper to get a TMDBInterface instance
func newTestTMDB(t *testing.T) TendermintDB[value, felt.Felt, felt.Felt] {
	t.Helper()
	dbPath := t.TempDir()
	testDB, err := pebble.New(dbPath)
	require.NoError(t, err)
	tmState := NewTendermintDB[value, felt.Felt, felt.Felt](testDB, height(0))
	require.NotNil(t, tmState)
	return tmState
}

func TestSetAndGetWAL(t *testing.T) {
	tmState := newTestTMDB(t)
	testHeight := height(1000)
	testRound := round(1)
	testStep := prevote
	sender1 := *new(felt.Felt).SetUint64(1)
	sender2 := *new(felt.Felt).SetUint64(2)
	sender3 := *new(felt.Felt).SetUint64(3)
	var val1 value = 10
	valHash1 := val1.Hash()

	// Create Messages
	proposalMessage := Proposal[value, felt.Felt, felt.Felt]{
		MessageHeader: MessageHeader[felt.Felt]{Height: testHeight, Round: testRound, Sender: sender1},
		ValidRound:    testRound,
		Value:         utils.HeapPtr(val1),
	}
	prevoteMessage := Prevote[felt.Felt, felt.Felt]{
		MessageHeader: MessageHeader[felt.Felt]{Height: testHeight, Round: testRound, Sender: sender2},
		ID:            &valHash1,
	}
	precommitMessage := Precommit[felt.Felt, felt.Felt]{
		MessageHeader: MessageHeader[felt.Felt]{Height: testHeight, Round: testRound, Sender: sender3},
		ID:            &valHash1,
	}
	timeoutEvent := &timeout{
		Height: testHeight,
		Round:  testRound,
		Step:   testStep,
	}

	expectedEntries := []walEntry[value, felt.Felt, felt.Felt]{
		{
			Type:  MessageTypeProposal,
			Entry: proposalMessage,
		},
		{
			Type:  MessageTypePrevote,
			Entry: prevoteMessage,
		},
		{
			Type:  MessageTypePrecommit,
			Entry: precommitMessage,
		},
		{
			Type:  MessageTypeTimeout,
			Entry: *timeoutEvent,
		},
	}

	for _, entry := range expectedEntries {
		require.NoError(t, tmState.SetWALEntry(entry.Entry))
	}

	// Commit the Batch
	require.NoError(t, tmState.CommitBatch())

	// Retrieve all WAL messages
	retrievedEntries, err := tmState.GetWALMsgs(testHeight)
	require.NoError(t, err, "Error getting WAL messages")
	require.Len(t, retrievedEntries, 4, "Expected 4 total entries (1 proposal, 1 prevote, 1 precommit, 1 timeout)")

	require.ElementsMatch(t, expectedEntries, retrievedEntries, "Mismatch between expected and retrieved WAL entries")
}

func TestDeleteMsgsAtHeight(t *testing.T) {
	tmState := newTestTMDB(t)
	testHeight := height(2000)
	testRound := round(5)
	sender1 := *new(felt.Felt).SetUint64(101)
	var val1 value = 99
	valHash1 := val1.Hash()

	// Add some messages for the test height using SetWALEntry
	proposal := Proposal[value, felt.Felt, felt.Felt]{
		MessageHeader: MessageHeader[felt.Felt]{Height: testHeight, Round: testRound, Sender: sender1},
		Value:         &val1,
	}
	prevote := Prevote[felt.Felt, felt.Felt]{
		MessageHeader: MessageHeader[felt.Felt]{Height: testHeight, Round: testRound, Sender: sender1},
		ID:            &valHash1,
	}
	timeout := &timeout{Height: testHeight, Round: testRound, Step: propose}

	expectedEntries := []walEntry[value, felt.Felt, felt.Felt]{
		{
			Type:  MessageTypeProposal,
			Entry: proposal,
		},
		{
			Type:  MessageTypePrevote,
			Entry: prevote,
		},
		{
			Type:  MessageTypeTimeout,
			Entry: *timeout,
		},
	}

	// Set the msgs
	for _, entry := range expectedEntries {
		require.NoError(t, tmState.SetWALEntry(entry.Entry))
	}

	// Commit the initial messages
	require.NoError(t, tmState.CommitBatch(), "Failed to commit initial messages")

	retrievedBefore, err := tmState.GetWALMsgs(testHeight)
	require.NoError(t, err, "Failed to get WAL messages before delete")
	require.ElementsMatch(t, expectedEntries, retrievedBefore, "Mismatch between expected and retrieved WAL entries")

	// Call DeleteMsgsAtHeight
	require.NoError(t, tmState.DeleteWALMsgs(testHeight), "Failed to call DeleteMsgsAtHeight")

	// Commit the deletion batch
	require.NoError(t, tmState.CommitBatch(), "Failed to commit deletion batch")

	_, err = tmState.GetWALMsgs(testHeight)
	require.Error(t, err)
}
