package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper to get a TMDBInterface instance
func newTestTMDB(t *testing.T) TMDBInterface[value, felt.Felt, felt.Felt] {
	testDB := memory.New()
	tmState := NewTMDB[value, felt.Felt, felt.Felt](testDB)
	require.NotNil(t, tmState)
	return tmState
}

func TestCommitBatch(t *testing.T) {
	tmState := newTestTMDB(t)
	require.NoError(t, tmState.CommitBatch())
}

func TestGetWALCount(t *testing.T) {
	tmState := newTestTMDB(t)

	height := height(1000)
	expectedNumMsgs := uint32(42)

	// call get function when no data has been set
	numMsgs, err := tmState.GetWALCount(height)
	require.ErrorIs(t, err, db.ErrKeyNotFound)
	require.Equal(t, uint32(0), numMsgs)

	// Cast to underlying *TMDB to access internal method (only for testing)
	tmdbImpl, ok := tmState.(*TMDB[value, felt.Felt, felt.Felt])
	require.True(t, ok, "Failed to cast TMDBInterface to *TMDB")
	require.NoError(t, tmdbImpl.setNumMsgsAtHeight(height, expectedNumMsgs))

	// get NumMsgsAtHeight - from batch
	numMsgs, err = tmState.GetWALCount(height)
	require.NoError(t, err)
	require.Equal(t, expectedNumMsgs, numMsgs)

	// commit to disk
	require.NoError(t, tmState.CommitBatch())

	// get NumMsgsAtHeight - from db
	numMsgs, err = tmState.GetWALCount(height)
	require.NoError(t, err)
	require.Equal(t, expectedNumMsgs, numMsgs)
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

	// 1. Create Messages
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
		H: testHeight,
		R: testRound,
		S: testStep,
	}

	// 2. Store Messages using SetWALEntry
	require.NoError(t, tmState.SetWALEntry(proposalMessage, testHeight))
	require.NoError(t, tmState.SetWALEntry(prevoteMessage, testHeight))
	require.NoError(t, tmState.SetWALEntry(precommitMessage, testHeight))
	require.NoError(t, tmState.SetWALEntry(timeoutEvent, testHeight))

	// 3. Commit the Batch
	require.NoError(t, tmState.CommitBatch())

	// 4. Verify Number of Messages
	numMsgs, err := tmState.GetWALCount(testHeight)
	require.NoError(t, err)
	require.Equal(t, uint32(4), numMsgs, "Expected 4 messages stored at height")

	// 5. Retrieve all WAL messages
	retrievedEntries, err := tmState.GetWALMsgs(testHeight)
	require.NoError(t, err, "Error getting WAL messages")
	require.Len(t, retrievedEntries, 4, "Expected 4 total entries (1 proposal, 1 prevote, 1 precommit, 1 timeout)")

	// 6. Assert Messages by Type
	var (
		proposalFound  *Proposal[value, felt.Felt, felt.Felt]
		prevoteFound   *Prevote[felt.Felt, felt.Felt]
		precommitFound *Precommit[felt.Felt, felt.Felt]
		timeoutFound   *timeout
	)

	for _, entry := range retrievedEntries {
		switch msg := entry.(type) {
		case Proposal[value, felt.Felt, felt.Felt]:
			require.Nil(t, proposalFound, "Found multiple proposals")
			// Need to copy msg because the loop variable 'msg' will be reused
			found := msg
			proposalFound = &found
		case Prevote[felt.Felt, felt.Felt]:
			require.Nil(t, prevoteFound, "Found multiple prevotes")
			found := msg
			prevoteFound = &found
		case Precommit[felt.Felt, felt.Felt]:
			require.Nil(t, precommitFound, "Found multiple precommits")
			found := msg
			precommitFound = &found
		case *timeout:
			require.Nil(t, timeoutFound, "Found multiple timeouts")
			timeoutFound = msg // msg is already a pointer here
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

	// 1. Add some messages for the test height using SetWALEntry
	proposal := Proposal[value, felt.Felt, felt.Felt]{
		MessageHeader: MessageHeader[felt.Felt]{Height: testHeight, Round: testRound, Sender: sender1},
		Value:         utils.HeapPtr(val1),
	}
	prevote := Prevote[felt.Felt, felt.Felt]{
		MessageHeader: MessageHeader[felt.Felt]{Height: testHeight, Round: testRound, Sender: sender1},
		ID:            &valHash1,
	}
	timeout := &timeout{H: testHeight, R: testRound, S: propose}

	require.NoError(t, tmState.SetWALEntry(proposal, testHeight))
	require.NoError(t, tmState.SetWALEntry(prevote, testHeight))
	require.NoError(t, tmState.SetWALEntry(timeout, testHeight))

	// 2. Commit the initial messages
	require.NoError(t, tmState.CommitBatch(), "Failed to commit initial messages")

	// 3. Verify messages exist before deletion
	numMsgsBefore, err := tmState.GetWALCount(testHeight)
	require.NoError(t, err, "Failed to get num messages before delete")
	require.Equal(t, uint32(3), numMsgsBefore, "Incorrect number of messages before delete")

	retrievedBefore, err := tmState.GetWALMsgs(testHeight)
	require.NoError(t, err, "Failed to get WAL messages before delete")
	require.Len(t, retrievedBefore, 3, "Incorrect number of WAL messages retrieved before delete")

	// 4. Call DeleteMsgsAtHeight
	require.NoError(t, tmState.DeleteWALMsgs(testHeight), "Failed to call DeleteMsgsAtHeight")

	// 5. Verify deletion is staged in batch (optional check, GetNumMsgsAtHeight reads batch first)
	_, err = tmState.GetWALCount(testHeight)
	require.ErrorIs(t, err, db.ErrKeyNotFound, "Count key should not be found in batch after DeleteMsgsAtHeight")

	// 6. Commit the deletion batch
	require.NoError(t, tmState.CommitBatch(), "Failed to commit deletion batch")

	// 7. Verify messages are gone from the DB
	numMsgsAfter, err := tmState.GetWALCount(testHeight)
	require.ErrorIs(t, err, db.ErrKeyNotFound, "NumMsgsAtHeight should return ErrKeyNotFound after delete and commit")
	require.Equal(t, uint32(0), numMsgsAfter) // Check zero value on error

	_, err = tmState.GetWALMsgs(testHeight)
	require.Error(t, err)
}
