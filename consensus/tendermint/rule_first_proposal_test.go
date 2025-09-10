package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

func TestFirstProposal(t *testing.T) {
	t.Run("Line 22: valid proposal with lockedRound = -1 should prevote for proposal", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)

		// Initialise state (already in propose step)
		currentRound.start()

		// Receive a valid proposal with lockedRound = -1
		proposalValue := value(42)
		currentRound.validator(0).proposal(proposalValue, -1).expectActions(
			currentRound.action().writeWALProposal(0, proposalValue, -1),
			currentRound.action().broadcastPrevote(&proposalValue),
		)

		// Assertions - We should be in prevote step
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrevote)
	})

	t.Run("Line 22: valid proposal with lockedValue matching proposal should prevote for proposal", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		previousRound := newTestRound(t, stateMachine, 0, 0)
		nextRound := newTestRound(t, stateMachine, 0, 1)

		expectedValue := value(42)

		// Simulate the previous round such that it times out in the precommit step
		// Validator 0 is a faulty proposer, proposing an invalid value to validator 1
		previousRound.start()
		previousRound.validator(0).proposal(expectedValue, -1).expectActions(
			previousRound.action().writeWALProposal(0, expectedValue, -1),
			previousRound.action().broadcastPrevote(&expectedValue),
		)
		previousRound.validator(0).prevote(utils.HeapPtr(expectedValue))
		previousRound.validator(1).prevote(nil)
		previousRound.validator(2).prevote(utils.HeapPtr(expectedValue))
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrecommit)

		// Validator 0 stays silent in the precommit step
		previousRound.validator(1).precommit(nil)
		previousRound.validator(2).precommit(utils.HeapPtr(expectedValue))
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrecommit)
		assert.Equal(t, &expectedValue, stateMachine.state.lockedValue)
		assert.Equal(t, types.Round(0), stateMachine.state.lockedRound)

		// Precommit timeout
		previousRound.processTimeout(types.StepPrecommit)

		// Validator 1 coincidentally proposes the same expected value with lockedRound = -1,
		// even if it hasn't received a proposal in the previous round.
		nextRound.validator(1).proposal(expectedValue, -1).expectActions(
			nextRound.action().writeWALProposal(1, expectedValue, -1),
			nextRound.action().broadcastPrevote(&expectedValue),
		)
		assertState(t, stateMachine, types.Height(0), types.Round(1), types.StepPrevote)
	})

	t.Run("Line 22: valid proposal with lockedValue not matching proposal should prevote nil", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		previousRound := newTestRound(t, stateMachine, 0, 0)
		nextRound := newTestRound(t, stateMachine, 0, 1)

		expectedValue := value(42)

		// Simulate the previous round such that it times out in the precommit step
		// Validator 0 is a faulty proposer, proposing an invalid value to validator 1
		previousRound.start()
		previousRound.validator(0).proposal(expectedValue, -1)
		previousRound.validator(0).prevote(utils.HeapPtr(expectedValue))
		previousRound.validator(1).prevote(nil)
		previousRound.validator(2).prevote(utils.HeapPtr(expectedValue))
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrecommit)

		// Validator 0 stays silent in the precommit step
		previousRound.validator(1).precommit(nil)
		previousRound.validator(2).precommit(utils.HeapPtr(expectedValue))
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrecommit)
		assert.Equal(t, &expectedValue, stateMachine.state.lockedValue)
		assert.Equal(t, types.Round(0), stateMachine.state.lockedRound)

		// Precommit timeout
		previousRound.processTimeout(types.StepPrecommit)

		// Validator 1 proposes an unexpected value, because it hasn't received a proposal in the previous round.
		nextRound.validator(1).proposal(value(43), -1).expectActions(
			nextRound.action().writeWALProposal(1, value(43), -1),
			nextRound.action().broadcastPrevote(nil),
		)
		assertState(t, stateMachine, types.Height(0), types.Round(1), types.StepPrevote)
	})

	t.Run("Line 22: invalid proposal should prevote nil", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)

		// Initialise state
		currentRound.start()

		// Receive an invalid proposal
		currentRound.validator(0).proposal(value(0), -1).expectActions(
			currentRound.action().writeWALProposal(0, value(0), -1),
			currentRound.action().broadcastPrevote(nil),
		)

		// Assertions - We should be in prevote step
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrevote)
	})

	t.Run("Line 22: proposal received when not in propose step should do nothing", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)

		// Initialise state and move to prevote step to avoid being in propose step
		currentRound.start()

		// Proposal timeout
		currentRound.processTimeout(types.StepPropose)

		// Receive 2 more prevotes, move to precommit step
		currentRound.validator(1).prevote(nil)
		currentRound.validator(2).prevote(nil)
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrecommit)

		// Receive a proposal while not in propose step
		currentRound.validator(0).proposal(value(42), -1).expectActions(
			currentRound.action().writeWALProposal(0, value(42), -1),
		)

		// Assertions - We should still be in precommit step, nothing changed
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrecommit)
	})
}
