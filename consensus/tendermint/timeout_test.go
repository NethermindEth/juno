package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

func TestTimeout(t *testing.T) {
	t.Run("OnTimeoutPropose: round zero the node is not the proposer thus send a prevote nil", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)

		currentRound.start().expectActions(
			currentRound.action().writeWALStart(),
			currentRound.action().scheduleTimeout(types.StepPropose),
		)
		currentRound.processTimeout(types.StepPropose).expectActions(
			currentRound.action().writeWALTimeout(types.StepPropose),
			currentRound.action().broadcastPrevote(nil),
		)

		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrevote)
	})

	t.Run("OnTimeoutPrevote: move to next round", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)

		currentRound := newTestRound(t, stateMachine, 0, 0)

		currentRound.start()
		currentRound.validator(0).proposal(value(42), -1)
		currentRound.validator(1).prevote(utils.HeapPtr(value(42)))
		currentRound.validator(2).prevote(nil).expectActions(
			currentRound.action().writeWALPrevote(2, nil),
			currentRound.action().scheduleTimeout(types.StepPrevote),
		)
		assert.True(t, stateMachine.state.timeoutPrevoteScheduled)
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrevote)

		currentRound.processTimeout(types.StepPrevote).expectActions(
			currentRound.action().writeWALTimeout(types.StepPrevote),
			currentRound.action().broadcastPrecommit(nil),
		)
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrecommit)
	})

	t.Run("OnTimeoutPrecommit: move to next round", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)

		currentRound := newTestRound(t, stateMachine, 0, 0)
		nextRound := newTestRound(t, stateMachine, 0, 1)

		currentRound.start()
		currentRound.validator(0).precommit(utils.HeapPtr(value(10)))
		currentRound.validator(1).precommit(nil)
		currentRound.validator(2).precommit(nil).expectActions(
			currentRound.action().writeWALPrecommit(2, nil),
			currentRound.action().scheduleTimeout(types.StepPrecommit),
		)
		assert.True(t, stateMachine.state.timeoutPrecommitScheduled)

		currentRound.processTimeout(types.StepPrecommit).expectActions(
			currentRound.action().writeWALTimeout(types.StepPrecommit),
			nextRound.action().scheduleTimeout(types.StepPropose),
		)
		assert.False(t, stateMachine.state.timeoutPrecommitScheduled)

		assertState(t, stateMachine, types.Height(0), types.Round(1), types.StepPropose)

		// Do nothing on old timeout
		currentRound.processTimeout(types.StepPropose).expectActions()
		assertState(t, stateMachine, types.Height(0), types.Round(1), types.StepPropose)

		currentRound.processTimeout(types.StepPrevote).expectActions()
		assertState(t, stateMachine, types.Height(0), types.Round(1), types.StepPropose)
	})

	t.Run("Ignore old timeout on new height", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)

		currentRound := newTestRound(t, stateMachine, 0, 0)

		committedValue := value(10)

		currentRound.start().expectActions(
			currentRound.action().writeWALStart(),
			currentRound.action().scheduleTimeout(types.StepPropose),
		)

		// Honest validators quickly agree on a value
		currentRound.validator(0).proposal(committedValue, -1)
		currentRound.validator(0).prevote(&committedValue)
		currentRound.validator(1).prevote(&committedValue).expectActions(
			currentRound.action().writeWALPrevote(1, &committedValue),
			currentRound.action().scheduleTimeout(types.StepPrevote),
			currentRound.action().broadcastPrecommit(&committedValue),
		)
		currentRound.validator(0).precommit(&committedValue)
		currentRound.validator(2).precommit(nil).expectActions(
			currentRound.action().writeWALPrecommit(2, nil),
			currentRound.action().scheduleTimeout(types.StepPrecommit),
		)
		currentRound.validator(1).precommit(&committedValue)

		// All old timeout should do nothing
		assertState(t, stateMachine, types.Height(1), types.Round(0), types.StepPropose)
		currentRound.processTimeout(types.StepPropose).expectActions()
		assertState(t, stateMachine, types.Height(1), types.Round(0), types.StepPropose)
		currentRound.processTimeout(types.StepPrevote).expectActions()
		assertState(t, stateMachine, types.Height(1), types.Round(0), types.StepPropose)
		currentRound.processTimeout(types.StepPrecommit).expectActions()
		assertState(t, stateMachine, types.Height(1), types.Round(0), types.StepPropose)
	})
}
