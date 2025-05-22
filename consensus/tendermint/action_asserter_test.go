package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/hash"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

// actionAsserter is a helper struct to assert the actions of the state machine.
// inputMessage is the message that was processed to produce the actions.
// actions is the list of actions that were produced by the state machine after processing the input message.
type actionAsserter[T any] struct {
	testing      *testing.T
	stateMachine *testStateMachine
	inputMessage T
	actions      []starknet.Action
}

// expectActions asserts the expected actions are in the result actions.
// It also asserts additional conditions for each of the expected actions.
func (a actionAsserter[T]) expectActions(expected ...starknet.Action) actionAsserter[T] {
	a.testing.Helper()
	assert.ElementsMatch(a.testing, expected, a.actions)
	for _, action := range expected {
		switch action := action.(type) {
		case *starknet.BroadcastProposal:
			assertProposal(a.testing, a.stateMachine, starknet.Proposal(*action))
		case *starknet.BroadcastPrevote:
			assertPrevote(a.testing, a.stateMachine, starknet.Prevote(*action))
		case *starknet.BroadcastPrecommit:
			// If the post state is not in new height and a value is committed, assert the valid and locked tuples.
			if a.stateMachine.state.height == action.Height && action.ID != nil {
				assert.Equal(a.testing, a.stateMachine.state.lockedRound, action.Round)
				assert.Equal(a.testing, getHash(a.stateMachine.state.lockedValue), action.ID)
				assert.Equal(a.testing, a.stateMachine.state.validRound, action.Round)
				assert.Equal(a.testing, getHash(a.stateMachine.state.validValue), action.ID)
			}
			assertPrecommit(a.testing, a.stateMachine, starknet.Precommit(*action))
		case *types.ScheduleTimeout:
			// ScheduleTimeout doesn't come with any state change, so there's nothing to assert here.
		}
	}
	return a
}

func getHash(val *starknet.Value) *hash.Hash {
	if val != nil {
		return utils.HeapPtr(val.Hash())
	}
	return nil
}
