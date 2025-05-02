package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
)

// assertState asserts that the state machine is in the expected state.
func assertState(t *testing.T, stateMachine *Tendermint[value, felt.Felt, felt.Felt], expectedHeight height, expectedRound round, expectedStep step) {
	t.Helper()
	assert.Equal(t, expectedHeight, stateMachine.state.height, "height not equal")
	assert.Equal(t, expectedRound, stateMachine.state.round, "round not equal")
	assert.Equal(t, expectedStep, stateMachine.state.step, "step not equal")
}

func assertMessage[T Message[value, felt.Felt, felt.Felt]](t *testing.T, messages map[height]map[round]map[felt.Felt]T, expectedMsgHeader MessageHeader[felt.Felt], expectedMsg T) {
	t.Helper()
	assert.Contains(t, messages, expectedMsgHeader.Height, "height not found")
	assert.Contains(t, messages[expectedMsgHeader.Height], expectedMsgHeader.Round, "round not found")
	assert.Contains(t, messages[expectedMsgHeader.Height][expectedMsgHeader.Round], expectedMsgHeader.Sender, "sender not found")
	assert.Equal(t, expectedMsg, messages[expectedMsgHeader.Height][expectedMsgHeader.Round][expectedMsgHeader.Sender], "message not equal")
}

// assertProposal asserts that the proposal message is in the state machine, except when the state machine advanced to the next height.
func assertProposal(t *testing.T, stateMachine *Tendermint[value, felt.Felt, felt.Felt], expectedMsg Proposal[value, felt.Felt, felt.Felt]) {
	t.Helper()
	// New height will discard the previous height messages.
	if stateMachine.state.height != expectedMsg.Height {
		return
	}
	assertMessage(t, stateMachine.messages.proposals, expectedMsg.MessageHeader, expectedMsg)
}

// assertPrevote asserts that the prevote message is in the state machine, except when the state machine advanced to the next height.
func assertPrevote(t *testing.T, stateMachine *Tendermint[value, felt.Felt, felt.Felt], expectedMsg Prevote[felt.Felt, felt.Felt]) {
	t.Helper()
	// New height will discard the previous height messages.
	if stateMachine.state.height != expectedMsg.Height {
		return
	}
	assertMessage(t, stateMachine.messages.prevotes, expectedMsg.MessageHeader, expectedMsg)
}

// assertPrecommit asserts that the precommit message is in the state machine, except when the state machine advanced to the next height.
func assertPrecommit(t *testing.T, stateMachine *Tendermint[value, felt.Felt, felt.Felt], expectedMsg Precommit[felt.Felt, felt.Felt]) {
	t.Helper()
	// New height will discard the previous height messages.
	if stateMachine.state.height != expectedMsg.Height {
		return
	}
	assertMessage(t, stateMachine.messages.precommits, expectedMsg.MessageHeader, expectedMsg)
}
