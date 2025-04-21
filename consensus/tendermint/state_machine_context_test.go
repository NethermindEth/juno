package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
)

// stateMachineContext is a build struct to build test scenarios for the state machine.
// Sample usages:
//
//	// Create a new state machine with 4 validators and this node is the validator with index 3 (0-based).
//	sm := setupStateMachine(t, 4, 3)
//	// Create a builder for height 1 and round 0.
//	currentRound := newTestRound(t, sm, 1, 0)
//	// Start the new round.
//	currentRound.start()
//	// Receive a prevote from the 1st validator.
//	currentRound.validator(0).prevote(&val)
//	// Trigger a timeout for prevote step and expect to broadcast a precommit for nil value.
//	currentRound.processTimeout(StepPrevote).expectActions(currentRound.action().broadcastPrecommit(nil))
//
// NOTE: `builderHeight` and `builderRound` are NOT the current height and round of the state machine,
// but the height and round of the test construct that is being built.
type stateMachineContext struct {
	testing       *testing.T
	stateMachine  *Tendermint[value, felt.Felt, felt.Felt]
	builderHeight height
	builderRound  round
}

func newTestRound(t *testing.T, stateMachine *Tendermint[value, felt.Felt, felt.Felt], h height, r round) stateMachineContext {
	return stateMachineContext{
		testing:       t,
		stateMachine:  stateMachine,
		builderHeight: h,
		builderRound:  r,
	}
}

// start triggers the start of a new round in the state machine using t.round,
// and returns a list of the resulting actions (wrapped in actionAsserter).
func (t stateMachineContext) start() actionAsserter[any] {
	return actionAsserter[any]{
		testing:      t.testing,
		stateMachine: t.stateMachine,
		inputMessage: nil,
		actions:      t.stateMachine.processStart(t.builderRound),
	}
}

// processTimeout triggers a timeout and returns an actionAsserter to assert the result actions.
func (t stateMachineContext) processTimeout(s step) actionAsserter[any] {
	return actionAsserter[any]{
		testing:      t.testing,
		stateMachine: t.stateMachine,
		inputMessage: nil,
		actions:      t.stateMachine.processTimeout(timeout{s: s, h: t.builderHeight, r: t.builderRound}),
	}
}

// validator returns an incomingMessageBuilder to build incoming messages from a specific validator.
func (t stateMachineContext) validator(idx int) incomingMessageBuilder {
	return incomingMessageBuilder{
		testing:      t.testing,
		stateMachine: t.stateMachine,
		header:       MessageHeader[felt.Felt]{Height: t.builderHeight, Round: t.builderRound, Sender: *getVal(idx)},
	}
}

// action returns an actionBuilder to build expected actions as the result of processing messages and timeouts for the state machine.
func (t stateMachineContext) action() actionBuilder {
	return actionBuilder{
		thisNodeAddr: t.stateMachine.nodeAddr,
		actionHeight: t.builderHeight,
		actionRound:  t.builderRound,
	}
}
