package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
)

// incomingMessageBuilder is a helper struct to build incoming messages received from other nodes for the state machine.
// Each method builds a message, processes it, assert if it's stored in the state machine, and returns an actionAsserter to assert the result actions.
type incomingMessageBuilder struct {
	testing      *testing.T
	stateMachine *testStateMachine
	header       types.MessageHeader
}

// proposal builds and processes a Proposal message, asserts it's stored in the state machine,
// and returns an actionAsserter to check resulting actions.
func (t incomingMessageBuilder) proposal(val starknet.Value, validRound types.Round) actionAsserter[starknet.Proposal] {
	t.testing.Helper()
	proposal := starknet.Proposal{
		MessageHeader: t.header,
		ValidRound:    validRound,
		Value:         &val,
	}
	actions := t.stateMachine.ProcessProposal(proposal)

	assertProposal(t.testing, t.stateMachine, proposal)

	return actionAsserter[starknet.Proposal]{
		testing:      t.testing,
		stateMachine: t.stateMachine,
		actions:      actions,
	}
}

// prevote builds and processes a types.Prevote message, asserts it's stored in the state machine,
// and returns an actionAsserter to check resulting actions.
func (t incomingMessageBuilder) prevote(val *starknet.Value) actionAsserter[types.Prevote] {
	t.testing.Helper()
	prevote := types.Prevote{
		MessageHeader: t.header,
		ID:            getHash(val),
	}
	actions := t.stateMachine.ProcessPrevote(prevote)

	assertPrevote(t.testing, t.stateMachine, prevote)

	return actionAsserter[types.Prevote]{
		testing:      t.testing,
		stateMachine: t.stateMachine,
		actions:      actions,
	}
}

// precommit builds and processes a Precommit message, asserts it's stored in the state machine,
// and returns an actionAsserter to check resulting actions.
func (t incomingMessageBuilder) precommit(val *starknet.Value) actionAsserter[types.Precommit] {
	t.testing.Helper()
	precommit := types.Precommit{
		MessageHeader: t.header,
		ID:            getHash(val),
	}
	actions := t.stateMachine.ProcessPrecommit(precommit)

	assertPrecommit(t.testing, t.stateMachine, precommit)

	return actionAsserter[types.Precommit]{
		testing:      t.testing,
		stateMachine: t.stateMachine,
		actions:      actions,
	}
}
