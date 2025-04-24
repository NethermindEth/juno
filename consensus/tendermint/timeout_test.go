package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

func TestTimeout(t *testing.T) {
	t.Run("OnTimeoutPropose: round zero the node is not the proposer thus send a prevote nil", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)

		currentRound.start().expectActions(currentRound.action().scheduleTimeout(StepPropose))
		currentRound.processTimeout(StepPropose).expectActions(currentRound.action().broadcastPrevote(nil))

		assertState(t, stateMachine, Height(0), Round(0), StepPrevote)
	})

	t.Run("OnTimeoutPrevote: move to next round", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)

		currentRound := newTestRound(t, stateMachine, 0, 0)

		currentRound.start()
		currentRound.validator(0).proposal(value(42), -1)
		currentRound.validator(1).prevote(utils.HeapPtr(value(42)))
		currentRound.validator(2).prevote(nil).expectActions(currentRound.action().scheduleTimeout(StepPrevote))
		assert.True(t, stateMachine.state.timeoutPrevoteScheduled)
		assertState(t, stateMachine, Height(0), Round(0), StepPrevote)

		currentRound.processTimeout(StepPrevote).expectActions(currentRound.action().broadcastPrecommit(nil))
		assertState(t, stateMachine, Height(0), Round(0), StepPrecommit)
	})

	t.Run("OnTimeoutPrecommit: move to next round", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)

		currentRound := newTestRound(t, stateMachine, 0, 0)
		nextRound := newTestRound(t, stateMachine, 0, 1)

		currentRound.start()
		currentRound.validator(0).precommit(utils.HeapPtr(value(10)))
		currentRound.validator(1).precommit(nil)
		currentRound.validator(2).precommit(nil).expectActions(currentRound.action().scheduleTimeout(StepPrecommit))
		assert.True(t, stateMachine.state.timeoutPrecommitScheduled)

		currentRound.processTimeout(StepPrecommit).expectActions(nextRound.action().scheduleTimeout(StepPropose))
		assert.False(t, stateMachine.state.timeoutPrecommitScheduled)

		assertState(t, stateMachine, Height(0), Round(1), StepPropose)

		// Do nothing on old timeout
		currentRound.processTimeout(StepPropose).expectActions()
		assertState(t, stateMachine, Height(0), Round(1), StepPropose)

		currentRound.processTimeout(StepPrevote).expectActions()
		assertState(t, stateMachine, Height(0), Round(1), StepPropose)
	})

	t.Run("Ignore old timeout on new height", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)

		currentRound := newTestRound(t, stateMachine, 0, 0)

		committedValue := value(10)

		currentRound.start().expectActions(currentRound.action().scheduleTimeout(StepPropose))

		// Honest validators quickly agree on a value
		currentRound.validator(0).proposal(committedValue, -1)
		currentRound.validator(0).prevote(&committedValue)
		currentRound.validator(1).prevote(&committedValue).expectActions(
			currentRound.action().scheduleTimeout(StepPrevote),
			currentRound.action().broadcastPrecommit(&committedValue),
		)
		currentRound.validator(0).precommit(&committedValue)
		currentRound.validator(2).precommit(nil).expectActions(
			currentRound.action().scheduleTimeout(StepPrecommit),
		)
		currentRound.validator(1).precommit(&committedValue)

		// All old timeout should do nothing
		assertState(t, stateMachine, Height(1), Round(0), StepPropose)
		currentRound.processTimeout(StepPropose).expectActions()
		assertState(t, stateMachine, Height(1), Round(0), StepPropose)
		currentRound.processTimeout(StepPrevote).expectActions()
		assertState(t, stateMachine, Height(1), Round(0), StepPropose)
		currentRound.processTimeout(StepPrecommit).expectActions()
		assertState(t, stateMachine, Height(1), Round(0), StepPropose)
	})
}
