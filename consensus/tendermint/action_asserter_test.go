package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

// actionAsserter is a helper struct to assert the actions of the state machine.
// inputMessage is the message that was processed to produce the actions.
// actions is the list of actions that were produced by the state machine after processing the input message.
type actionAsserter[T any] struct {
	testing      *testing.T
	stateMachine *Tendermint[value, felt.Felt, felt.Felt]
	inputMessage T
	actions      []Action[value, felt.Felt, felt.Felt]
}

// expectActions asserts the expected actions are in the result actions.
// It also asserts additional conditions for each of the expected actions.
func (a actionAsserter[T]) expectActions(expected ...Action[value, felt.Felt, felt.Felt]) actionAsserter[T] {
	a.testing.Helper()
	assert.ElementsMatch(a.testing, expected, a.actions)
	for _, action := range expected {
		switch action := action.(type) {
		case *BroadcastProposal[value, felt.Felt, felt.Felt]:
			assertProposal(a.testing, a.stateMachine, Proposal[value, felt.Felt, felt.Felt](*action))
		case *BroadcastPrevote[felt.Felt, felt.Felt]:
			assertPrevote(a.testing, a.stateMachine, Prevote[felt.Felt, felt.Felt](*action))
		case *BroadcastPrecommit[felt.Felt, felt.Felt]:
			// If the post state is not in new height and a value is committed, assert the valid and locked tuples.
			if a.stateMachine.state.height == action.Height && action.ID != nil {
				assert.Equal(a.testing, a.stateMachine.state.lockedRound, action.Round)
				assert.Equal(a.testing, getHash(a.stateMachine.state.lockedValue), action.ID)
				assert.Equal(a.testing, a.stateMachine.state.validRound, action.Round)
				assert.Equal(a.testing, getHash(a.stateMachine.state.validValue), action.ID)
			}
			assertPrecommit(a.testing, a.stateMachine, Precommit[felt.Felt, felt.Felt](*action))
		case *ScheduleTimeout:
			// ScheduleTimeout doesn't come with any state change, so there's nothing to assert here.
		}
	}
	return a
}

func getHash(val *value) *felt.Felt {
	if val != nil {
		return utils.HeapPtr(val.Hash())
	}
	return nil
}
