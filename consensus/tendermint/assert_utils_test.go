package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/votecounter"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
)

func value(value uint64) starknet.Value {
	return starknet.Value(felt.FromUint64(value))
}

// assertState asserts that the state machine is in the expected state.
func assertState(
	t *testing.T,
	stateMachine *testStateMachine,
	expectedHeight types.Height,
	expectedRound types.Round,
	expectedStep types.Step,
) {
	t.Helper()
	assert.Equal(t, expectedHeight, stateMachine.state.height, "height not equal")
	assert.Equal(t, expectedRound, stateMachine.state.round, "round not equal")
	assert.Equal(t, expectedStep, stateMachine.state.step, "step not equal")
}

// assertProposal asserts that the proposal message is in the state machine, except when the state machine advanced to the next height.
func assertProposal(t *testing.T, stateMachine *testStateMachine, expectedMsg starknet.Proposal) {
	t.Helper()
	// New height will discard the previous height messages.
	if stateMachine.state.height > expectedMsg.Height {
		return
	}
	stateMachine.voteCounter.AssertProposal(t, expectedMsg)
}

// assertPrevote asserts that the prevote message is in the state machine, except when the state machine advanced to the next height.
func assertPrevote(t *testing.T, stateMachine *testStateMachine, expectedMsg starknet.Prevote) {
	t.Helper()
	// New height will discard the previous height messages.
	if stateMachine.state.height > expectedMsg.Height {
		return
	}
	stateMachine.voteCounter.AssertVote(t, (*starknet.Vote)(&expectedMsg), votecounter.Prevote)
}

// assertPrecommit asserts that the precommit message is in the state machine, except when the state machine advanced to the next height.
func assertPrecommit(t *testing.T, stateMachine *testStateMachine, expectedMsg starknet.Precommit) {
	t.Helper()
	// New height will discard the previous height messages.
	if stateMachine.state.height > expectedMsg.Height {
		return
	}
	stateMachine.voteCounter.AssertVote(t, (*starknet.Vote)(&expectedMsg), votecounter.Precommit)
}
