package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/consensus/types"
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
			currentRound.action().writeWALProposal(0, committedValue, -1),
			currentRound.action().scheduleTimeout(types.StepPrevote),
			currentRound.action().broadcastPrevote(&committedValue),
			currentRound.action().broadcastPrecommit(&committedValue),
		)

		assert.True(t, stateMachine.state.lockedValueAndOrValidValueSet)
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrecommit)
	})

	t.Run("Line 36: lock and broadcast precommit when step is precommit", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)
		nextRound := newTestRound(t, stateMachine, 1, 0)

		committedValue := value(42)

		// Initialise the round
		currentRound.start()

		// Proposal timeout
		currentRound.processTimeout(types.StepPropose).expectActions(
			currentRound.action().writeWALTimeout(types.StepPropose),
			currentRound.action().broadcastPrevote(nil),
		)

		// Prevotes are collected
		currentRound.validator(1).prevote(&committedValue).expectActions(
			currentRound.action().writeWALPrevote(1, &committedValue),
		)
		currentRound.validator(2).prevote(&committedValue).expectActions(
			currentRound.action().writeWALPrevote(2, &committedValue),
			currentRound.action().scheduleTimeout(types.StepPrevote),
		)
		currentRound.processTimeout(types.StepPrevote).expectActions(
			currentRound.action().writeWALTimeout(types.StepPrevote),
			currentRound.action().broadcastPrecommit(nil),
		)
		currentRound.validator(0).prevote(&committedValue).expectActions(
			currentRound.action().writeWALPrevote(0, &committedValue),
		)

		// Receives precommits, this rule shouldn't block the commit value rule
		currentRound.validator(1).precommit(&committedValue)
		currentRound.validator(2).precommit(&committedValue)
		currentRound.validator(0).precommit(&committedValue)
		currentRound.validator(0).proposal(committedValue, -1).expectActions(
			currentRound.action().writeWALProposal(0, committedValue, -1),
			currentRound.action().commit(committedValue, -1, 0),
		)

		assertState(t, stateMachine, types.Height(1), types.Round(0), types.StepPropose)

		nextRound.start().expectActions(
			nextRound.action().writeWALStart(),
			nextRound.action().scheduleTimeout(types.StepPropose),
		)
		assertState(t, stateMachine, types.Height(1), types.Round(0), types.StepPropose)
	})

	t.Run("Line 36: record valid value even if we don't prevote it", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 1)
		currentRound := newTestRound(t, stateMachine, 0, 0)
		nextRound := newTestRound(t, stateMachine, 0, 1)
		committedValue := value(42)

		// Initialise the round
		currentRound.start()

		// Prevotes are collected but not proposal
		currentRound.processTimeout(types.StepPropose).expectActions(
			currentRound.action().writeWALTimeout(types.StepPropose),
			currentRound.action().broadcastPrevote(nil),
		)
		currentRound.validator(2).prevote(&committedValue)
		currentRound.validator(3).prevote(&committedValue).expectActions(
			currentRound.action().writeWALPrevote(3, &committedValue),
			currentRound.action().scheduleTimeout(types.StepPrevote),
		)

		currentRound.processTimeout(types.StepPrevote).expectActions(
			currentRound.action().writeWALTimeout(types.StepPrevote),
			currentRound.action().broadcastPrecommit(nil),
		)

		// Receives proposal and prevote from validator 0, record valid value and round
		currentRound.validator(0).proposal(committedValue, -1)

		// Line 36 is triggered, record valid value and round but don't broadcast precommit
		currentRound.validator(0).prevote(&committedValue).expectActions(
			currentRound.action().writeWALPrevote(0, &committedValue),
		)
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrecommit)
		assert.Equal(t, &committedValue, stateMachine.state.validValue)
		assert.Equal(t, types.Round(0), stateMachine.state.validRound)

		// Use valid value on next round
		currentRound.validator(2).precommit(&committedValue)
		currentRound.validator(3).precommit(&committedValue).expectActions(
			currentRound.action().writeWALPrecommit(3, &committedValue),
			currentRound.action().scheduleTimeout(types.StepPrecommit),
		)
		currentRound.processTimeout(types.StepPrecommit).expectActions(
			currentRound.action().writeWALTimeout(types.StepPrecommit),
			nextRound.action().broadcastProposal(committedValue, 0),
			nextRound.action().broadcastPrevote(&committedValue),
		)
		assertState(t, stateMachine, types.Height(0), types.Round(1), types.StepPrevote)
	})

	t.Run("Line 36: not trigger if not first time", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)

		committedValue := value(42)

		// Initialise the round
		currentRound.start()

		// Receive proposal and broadcast prevote
		currentRound.validator(0).proposal(committedValue, -1).expectActions(
			currentRound.action().writeWALProposal(0, committedValue, -1),
			currentRound.action().broadcastPrevote(&committedValue),
		)
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrevote)

		// 2 prevotes are collected
		currentRound.validator(0).prevote(&committedValue)
		currentRound.validator(1).prevote(&committedValue).expectActions(
			currentRound.action().writeWALPrevote(1, &committedValue),
			currentRound.action().scheduleTimeout(types.StepPrevote),
			currentRound.action().broadcastPrecommit(&committedValue),
		)
		assert.True(t, stateMachine.state.lockedValueAndOrValidValueSet)
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrecommit)

		// Last prevote collected doesn't trigger the rule
		currentRound.validator(2).prevote(&committedValue).expectActions(
			currentRound.action().writeWALPrevote(2, &committedValue),
		)
		assert.True(t, stateMachine.state.lockedValueAndOrValidValueSet)
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrecommit)
	})
}
