package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

func TestCommitBatch(t *testing.T) {
	testDB := pebble.NewMemTest(t)
	tmState := NewTMState(testDB)
	require.NoError(t, tmState.CommitBatch())
}

func TestGetNumMsgsAtHeight(t *testing.T) {
	testDB := pebble.NewMemTest(t)
	tmState := NewTMState(testDB)

	height := height(1000)
	expectedNumMsgs := uint32(42)

	// call get function when no data has been set
	numMsgs, err := tmState.GetNumMsgsAtHeight(height)
	require.ErrorIs(t, err, db.ErrKeyNotFound)
	require.Equal(t, uint32(0), numMsgs)

	// set data
	tmState.SetNumMsgsAtHeight(height, expectedNumMsgs)
	require.NoError(t, tmState.CommitBatch())

	// get data from batch
	numMsgs, err = tmState.GetNumMsgsAtHeight(height)
	require.NoError(t, err)
	require.Equal(t, expectedNumMsgs, numMsgs)

	require.NoError(t, tmState.CommitBatch())

	// get data from db
	numMsgs, err = tmState.GetNumMsgsAtHeight(height)
	require.NoError(t, err)
	require.Equal(t, expectedNumMsgs, numMsgs)
}

func TestSetAndGetWAL(t *testing.T) {
	testDB := pebble.NewMemTest(t)
	tmState := NewTMState(testDB)

	app := newApp()

	proposalMessage := Proposal[value, felt.Felt, felt.Felt]{
		MessageHeader: MessageHeader[felt.Felt]{
			Height: height(10),
			Round:  round(2),
			Sender: *new(felt.Felt).SetUint64(123),
		},
		ValidRound: round(2),
		Value:      utils.HeapPtr(app.cur + 1),
	}
	height := height(1000)
	require.NoError(t, SetWAL[value, felt.Felt, felt.Felt](&tmState, proposalMessage, nil, height))
	msgs, err := GetWALMsgs[value, felt.Felt, felt.Felt, Proposal[value, felt.Felt, felt.Felt]](&tmState, height)
	require.NoError(t, err)
	require.NotNil(t, msgs)
}
