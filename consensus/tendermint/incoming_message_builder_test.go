package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
)

// incomingMessageBuilder is a helper struct to build incoming messages received from other nodes for the state machine.
// Each method builds a message, processes it, assert if it's stored in the state machine, and returns an actionAsserter to assert the result actions.
type incomingMessageBuilder struct {
	testing      *testing.T
	stateMachine *stateMachine[value, felt.Felt, felt.Felt]
	header       MessageHeader[felt.Felt]
}

// proposal builds and processes a Proposal message, asserts it's stored in the state machine,
// and returns an actionAsserter to check resulting actions.
func (t incomingMessageBuilder) proposal(val value, validRound Round) actionAsserter[Proposal[value, felt.Felt, felt.Felt]] {
	t.testing.Helper()
	proposal := Proposal[value, felt.Felt, felt.Felt]{
		MessageHeader: t.header,
		ValidRound:    validRound,
		Value:         &val,
	}
	actions := t.stateMachine.ProcessProposal(proposal)

	assertProposal(t.testing, t.stateMachine, proposal)

	return actionAsserter[Proposal[value, felt.Felt, felt.Felt]]{
		testing:      t.testing,
		stateMachine: t.stateMachine,
		inputMessage: proposal,
		actions:      actions,
	}
}

// prevote builds and processes a Prevote message, asserts it's stored in the state machine,
// and returns an actionAsserter to check resulting actions.
func (t incomingMessageBuilder) prevote(val *value) actionAsserter[Prevote[felt.Felt, felt.Felt]] {
	t.testing.Helper()
	prevote := Prevote[felt.Felt, felt.Felt]{
		MessageHeader: t.header,
		ID:            getHash(val),
	}
	actions := t.stateMachine.ProcessPrevote(prevote)

	assertPrevote(t.testing, t.stateMachine, prevote)

	return actionAsserter[Prevote[felt.Felt, felt.Felt]]{
		testing:      t.testing,
		stateMachine: t.stateMachine,
		inputMessage: prevote,
		actions:      actions,
	}
}

// precommit builds and processes a Precommit message, asserts it's stored in the state machine,
// and returns an actionAsserter to check resulting actions.
func (t incomingMessageBuilder) precommit(val *value) actionAsserter[Precommit[felt.Felt, felt.Felt]] {
	t.testing.Helper()
	precommit := Precommit[felt.Felt, felt.Felt]{
		MessageHeader: t.header,
		ID:            getHash(val),
	}
	actions := t.stateMachine.ProcessPrecommit(precommit)

	assertPrecommit(t.testing, t.stateMachine, precommit)

	return actionAsserter[Precommit[felt.Felt, felt.Felt]]{
		testing:      t.testing,
		stateMachine: t.stateMachine,
		inputMessage: precommit,
		actions:      actions,
	}
}
