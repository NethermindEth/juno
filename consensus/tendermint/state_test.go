package tendermint

import (
	"encoding/binary"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to convert height to bytes (assuming it's not exported or easily accessible)
// Based on unexported heightToBytes in state.go
func heightToBytesHelper(height height) []byte {
	heightBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(heightBytes, uint32(height))
	return heightBytes
}

func TestCommitBatch(t *testing.T) {
	testDB := memory.New()
	tmState := NewTMState(testDB)
	require.NoError(t, tmState.CommitBatch())
}

func TestGetNumMsgsAtHeight(t *testing.T) {
	testDB := memory.New()
	tmState := NewTMState(testDB)

	height := height(1000)
	expectedNumMsgs := uint32(42)

	// call get function when no data has been set
	numMsgs, err := tmState.GetNumMsgsAtHeight(height)
	require.ErrorIs(t, err, db.ErrKeyNotFound)
	require.Equal(t, uint32(0), numMsgs)

	// set NumMsgsAtHeight
	require.NoError(t, tmState.setNumMsgsAtHeight(height, expectedNumMsgs))

	// get NumMsgsAtHeight - from batch
	numMsgs, err = tmState.GetNumMsgsAtHeight(height)
	require.NoError(t, err)
	require.Equal(t, expectedNumMsgs, numMsgs)

	// commit to disk
	require.NoError(t, tmState.CommitBatch())

	// get NumMsgsAtHeight - from db
	numMsgs, err = tmState.GetNumMsgsAtHeight(height)
	require.NoError(t, err)
	require.Equal(t, expectedNumMsgs, numMsgs)
}

func TestSetAndGetWAL(t *testing.T) {
	testDB := memory.New()
	tmState := NewTMState(testDB)
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
		h: testHeight,
		r: testRound,
		s: testStep,
	}

	// 2. Store Messages using SetWALMsg and SetWALTimeout
	err := SetWALMsg[value, felt.Felt, felt.Felt](&tmState, proposalMessage, testHeight)
	require.NoError(t, err)
	err = SetWALMsg[value, felt.Felt, felt.Felt](&tmState, prevoteMessage, testHeight)
	require.NoError(t, err)
	err = SetWALMsg[value, felt.Felt, felt.Felt](&tmState, precommitMessage, testHeight)
	require.NoError(t, err)

	// Store the timeout directly using SetWALTimeout
	err = SetWALTimeout(&tmState, timeoutEvent, testHeight)
	require.NoError(t, err)

	// 3. Commit the Batch
	require.NoError(t, tmState.CommitBatch())

	// 4. Verify Number of Messages
	numMsgs, err := tmState.GetNumMsgsAtHeight(testHeight)
	require.NoError(t, err)
	require.Equal(t, uint32(4), numMsgs, "Expected 4 messages stored at height") // Updated count

	// 5. Retrieve all WAL messages
	retrievedMsgs, err := GetWALMsgs[value, felt.Felt, felt.Felt](&tmState, testHeight) // Call once without M
	require.NoError(t, err, "Error getting WAL messages")
	require.Len(t, retrievedMsgs, 4, "Expected 4 total entries (1 proposal, 1 prevote, 1 precommit, 1 timeout)") // Updated count

	// 6. Assert Messages by Type
	var (
		proposalFound  *Proposal[value, felt.Felt, felt.Felt]
		prevoteFound   *Prevote[felt.Felt, felt.Felt]
		precommitFound *Precommit[felt.Felt, felt.Felt]
		timeoutFound   *timeout
	)

	for _, retrieved := range retrievedMsgs {
		if retrieved.Timeout != nil {
			require.Nil(t, timeoutFound, "Found multiple timeouts")
			timeoutFound = retrieved.Timeout
			continue
		}

		switch msg := retrieved.Msg.(type) {
		case Proposal[value, felt.Felt, felt.Felt]:
			require.Nil(t, proposalFound, "Found multiple proposals")
			proposalFound = &msg // Store pointer to the message
		case Prevote[felt.Felt, felt.Felt]:
			require.Nil(t, prevoteFound, "Found multiple prevotes")
			prevoteFound = &msg
		case Precommit[felt.Felt, felt.Felt]:
			require.Nil(t, precommitFound, "Found multiple precommits")
			precommitFound = &msg
		default:
			t.Fatalf("Found unexpected message type in WAL: %T", retrieved.Msg)
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
