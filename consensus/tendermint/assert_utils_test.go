package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
)

func value(value uint64) starknet.Value {
	return starknet.Value(felt.FromUint64(value))
}

// assertState asserts that the state machine is in the expected state.
func assertState(t *testing.T, stateMachine *testStateMachine, expectedHeight types.Height, expectedRound types.Round, expectedStep types.Step) {
	t.Helper()
	assert.Equal(t, expectedHeight, stateMachine.state.height, "height not equal")
	assert.Equal(t, expectedRound, stateMachine.state.round, "round not equal")
	assert.Equal(t, expectedStep, stateMachine.state.step, "step not equal")
}

func assertMessage[T starknet.Message](t *testing.T, messages map[types.Height]map[types.Round]map[starknet.Address]T, expectedMsgHeader starknet.MessageHeader, expectedMsg T) {
	t.Helper()
	assert.Contains(t, messages, expectedMsgHeader.Height, "height not found")
	assert.Contains(t, messages[expectedMsgHeader.Height], expectedMsgHeader.Round, "round not found")
	assert.Contains(t, messages[expectedMsgHeader.Height][expectedMsgHeader.Round], expectedMsgHeader.Sender, "sender not found")
	assert.Equal(t, expectedMsg, messages[expectedMsgHeader.Height][expectedMsgHeader.Round][expectedMsgHeader.Sender], "message not equal")
}

// assertProposal asserts that the proposal message is in the state machine, except when the state machine advanced to the next height.
func assertProposal(t *testing.T, stateMachine *testStateMachine, expectedMsg starknet.Proposal) {
	t.Helper()
	// New height will discard the previous height messages.
	if stateMachine.state.height != expectedMsg.Height {
		return
	}
	assertMessage(t, stateMachine.messages.Proposals, expectedMsg.MessageHeader, expectedMsg)
}

// assertPrevote asserts that the prevote message is in the state machine, except when the state machine advanced to the next height.
func assertPrevote(t *testing.T, stateMachine *testStateMachine, expectedMsg starknet.Prevote) {
	t.Helper()
	// New height will discard the previous height messages.
	if stateMachine.state.height != expectedMsg.Height {
		return
	}
	assertMessage(t, stateMachine.messages.Prevotes, expectedMsg.MessageHeader, expectedMsg)
}

// assertPrecommit asserts that the precommit message is in the state machine, except when the state machine advanced to the next height.
func assertPrecommit(t *testing.T, stateMachine *testStateMachine, expectedMsg starknet.Precommit) {
	t.Helper()
	// New height will discard the previous height messages.
	if stateMachine.state.height != expectedMsg.Height {
		return
	}
	assertMessage(t, stateMachine.messages.Precommits, expectedMsg.MessageHeader, expectedMsg)
}
