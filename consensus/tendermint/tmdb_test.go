package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper to get a TMDBInterface instance
func newTestTMDB(t *testing.T) TendermintDB[value, felt.Felt, felt.Felt] {
	dbPath := t.TempDir()
	testDB, err := pebble.New(dbPath)
	require.NoError(t, err)
	tmState := NewTendermintDB[value, felt.Felt, felt.Felt](testDB, height(0))
	require.NotNil(t, tmState)
	return tmState
}

func TestCommitBatch(t *testing.T) {
	tmState := newTestTMDB(t)
	require.NoError(t, tmState.CommitBatch())
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

	// Store Messages using SetWALEntry
	require.NoError(t, tmState.SetWALEntry(proposalMessage, testHeight))
	require.NoError(t, tmState.SetWALEntry(prevoteMessage, testHeight))
	require.NoError(t, tmState.SetWALEntry(precommitMessage, testHeight))
	require.NoError(t, tmState.SetWALEntry(timeoutEvent, testHeight))

	// Commit the Batch
	require.NoError(t, tmState.CommitBatch())

	// Retrieve all WAL messages
	retrievedEntries, err := tmState.GetWALMsgs(testHeight)
	require.NoError(t, err, "Error getting WAL messages")
	require.Len(t, retrievedEntries, 4, "Expected 4 total entries (1 proposal, 1 prevote, 1 precommit, 1 timeout)")

	// Assert Messages by Type
	var (
		proposalFound  *Proposal[value, felt.Felt, felt.Felt]
		prevoteFound   *Prevote[felt.Felt, felt.Felt]
		precommitFound *Precommit[felt.Felt, felt.Felt]
		timeoutFound   *timeout
	)

	for _, entry := range retrievedEntries {
		switch entry.Type {
		case MessageTypeProposal:
			require.Nil(t, proposalFound, "Found multiple proposals")
			// Need to copy msg because the loop variable 'msg' will be reused
			found := (entry.Entry).(Proposal[value, felt.Felt, felt.Felt])
			proposalFound = &found
		case MessageTypePrevote:
			require.Nil(t, prevoteFound, "Found multiple prevotes")
			found := (entry.Entry).(Prevote[felt.Felt, felt.Felt])
			prevoteFound = &found
		case MessageTypePrecommit:
			require.Nil(t, precommitFound, "Found multiple precommits")
			found := (entry.Entry).(Precommit[felt.Felt, felt.Felt])
			precommitFound = &found
		case MessageTypeTimeout:
			require.Nil(t, timeoutFound, "Found multiple timeouts")
			msg := (entry.Entry).(timeout)
			timeoutFound = &msg
		default:
			t.Fatalf("Found unexpected message type in WAL: %T", entry)
		}
	}

	// Assert that each expected message type was found exactly once
	require.NotNil(t, proposalFound, "Proposal message not found")
	assert.Equal(t, proposalMessage, *proposalFound, "Retrieved proposal mismatch")

	require.NotNil(t, prevoteFound, "Prevote message not found")
	assert.Equal(t, prevoteMessage, *prevoteFound, "Retrieved prevote mismatch")

	require.NotNil(t, precommitFound, "Precommit message not found")
	assert.Equal(t, precommitMessage, *precommitFound, "Retrieved precommit mismatch")

	require.NotNil(t, timeoutFound, "Timeout message not found")
	assert.Equal(t, *timeoutEvent, *timeoutFound, "Retrieved timeout mismatch")
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
		Value:         utils.HeapPtr(val1),
	}
	prevote := Prevote[felt.Felt, felt.Felt]{
		MessageHeader: MessageHeader[felt.Felt]{Height: testHeight, Round: testRound, Sender: sender1},
		ID:            &valHash1,
	}
	timeout := &timeout{Height: testHeight, Round: testRound, Step: propose}

	require.NoError(t, tmState.SetWALEntry(proposal, testHeight))
	require.NoError(t, tmState.SetWALEntry(prevote, testHeight))
	require.NoError(t, tmState.SetWALEntry(timeout, testHeight))

	// Commit the initial messages
	require.NoError(t, tmState.CommitBatch(), "Failed to commit initial messages")

	retrievedBefore, err := tmState.GetWALMsgs(testHeight)
	require.NoError(t, err, "Failed to get WAL messages before delete")
	require.Len(t, retrievedBefore, 3, "Incorrect number of WAL messages retrieved before delete")

	// Call DeleteMsgsAtHeight
	require.NoError(t, tmState.DeleteWALMsgs(testHeight), "Failed to call DeleteMsgsAtHeight")

	// Commit the deletion batch
	require.NoError(t, tmState.CommitBatch(), "Failed to commit deletion batch")

	_, err = tmState.GetWALMsgs(testHeight)
	require.Error(t, err)
}
