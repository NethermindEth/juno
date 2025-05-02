package tendermint

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProposalAndPolkaCurrent(t *testing.T) {
	t.Run("Line 36: lock and broadcast precommit when step is prevote", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)

		committedValue := value(42)

		// Initialise the round
		currentRound.start()

		// Prevotes are collected
		currentRound.validator(0).prevote(&committedValue)
		currentRound.validator(1).prevote(&committedValue)

		// Receives proposal, go straight to precommit
		currentRound.validator(0).proposal(committedValue, -1).expectActions(
			currentRound.action().scheduleTimeout(StepPrevote),
			currentRound.action().broadcastPrevote(&committedValue),
			currentRound.action().broadcastPrecommit(&committedValue),
		)

		assert.True(t, stateMachine.state.lockedValueAndOrValidValueSet)
		assertState(t, stateMachine, Height(0), Round(0), StepPrecommit)
	})

	t.Run("Line 36: record valid value even if we don't prevote it", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 1)
		currentRound := newTestRound(t, stateMachine, 0, 0)
		nextRound := newTestRound(t, stateMachine, 0, 1)
		committedValue := value(42)

		// Initialise the round
		currentRound.start()

		// Prevotes are collected but not proposal
		currentRound.processTimeout(StepPropose).expectActions(
			currentRound.action().broadcastPrevote(nil),
		)
		currentRound.validator(2).prevote(&committedValue)
		currentRound.validator(3).prevote(&committedValue).expectActions(
			currentRound.action().scheduleTimeout(StepPrevote),
		)

		currentRound.processTimeout(StepPrevote).expectActions(
			currentRound.action().broadcastPrecommit(nil),
		)

		// Receives proposal and prevote from validator 0, record valid value and round
		currentRound.validator(0).proposal(committedValue, -1)

		// Line 36 is triggered, record valid value and round but don't broadcast precommit
		currentRound.validator(0).prevote(&committedValue).expectActions()
		assertState(t, stateMachine, Height(0), Round(0), StepPrecommit)
		assert.Equal(t, &committedValue, stateMachine.state.validValue)
		assert.Equal(t, Round(0), stateMachine.state.validRound)

		// Use valid value on next round
		currentRound.validator(2).precommit(&committedValue)
		currentRound.validator(3).precommit(&committedValue).expectActions(
			currentRound.action().scheduleTimeout(StepPrecommit),
		)
		currentRound.processTimeout(StepPrecommit).expectActions(
			nextRound.action().broadcastProposal(committedValue, 0),
			nextRound.action().broadcastPrevote(&committedValue),
		)
		assertState(t, stateMachine, Height(0), Round(1), StepPrevote)
	})

	t.Run("Line 36: not trigger if not first time", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)

		committedValue := value(42)

		// Initialise the round
		currentRound.start()

		// Receive proposal and broadcast prevote
		currentRound.validator(0).proposal(committedValue, -1).expectActions(
			currentRound.action().broadcastPrevote(&committedValue),
		)
		assertState(t, stateMachine, Height(0), Round(0), StepPrevote)

		// 2 prevotes are collected
		currentRound.validator(0).prevote(&committedValue)
		currentRound.validator(1).prevote(&committedValue).expectActions(
			currentRound.action().scheduleTimeout(StepPrevote),
			currentRound.action().broadcastPrecommit(&committedValue),
		)
		assert.True(t, stateMachine.state.lockedValueAndOrValidValueSet)
		assertState(t, stateMachine, Height(0), Round(0), StepPrecommit)

		// Last prevote collected doesn't trigger the rule
		currentRound.validator(2).prevote(&committedValue).expectActions()
		assert.True(t, stateMachine.state.lockedValueAndOrValidValueSet)
		assertState(t, stateMachine, Height(0), Round(0), StepPrecommit)
	})
}
