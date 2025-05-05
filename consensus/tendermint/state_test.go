package tendermint

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

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
	require.NoError(t, tmState.CommitBatch())

	// get NumMsgsAtHeight
	numMsgs, err = tmState.GetNumMsgsAtHeight(height)
	require.NoError(t, err)
	require.Equal(t, expectedNumMsgs, numMsgs)
}

func TestSetAndGetWAL(t *testing.T) {
	testDB := memory.New()
	tmState := NewTMState(testDB)

	app := newApp()

	// set msg
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

	// commit
	require.NoError(t, tmState.CommitBatch())

	// get NumMsgsAtHeight
	numMsgs, err := tmState.GetNumMsgsAtHeight(height)
	require.NoError(t, err)
	require.Equal(t, uint32(1), numMsgs)

	// get msg
	msgs, err := GetWALMsgs[value, felt.Felt, felt.Felt, Proposal[value, felt.Felt, felt.Felt]](&tmState, height)
	require.NoError(t, err)
	fmt.Println("msgs", msgs)
	require.NotNil(t, msgs)
}
