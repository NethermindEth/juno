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

		currentRound.start().expectActions(currentRound.action().scheduleTimeout(propose))
		currentRound.processTimeout(propose).expectActions(currentRound.action().broadcastPrevote(nil))

		assertState(t, stateMachine, height(0), round(0), prevote)
	})

	t.Run("OnTimeoutPrevote: move to next round", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)

		currentRound := newTestRound(t, stateMachine, 0, 0)

		currentRound.start()
		currentRound.validator(0).proposal(value(42), -1)
		currentRound.validator(1).prevote(utils.HeapPtr(value(42)))
		currentRound.validator(2).prevote(nil).expectActions(currentRound.action().scheduleTimeout(prevote))
		assert.True(t, stateMachine.state.timeoutPrevoteScheduled)
		assertState(t, stateMachine, height(0), round(0), prevote)

		currentRound.processTimeout(prevote).expectActions(currentRound.action().broadcastPrecommit(nil))
		assertState(t, stateMachine, height(0), round(0), precommit)
	})

	t.Run("OnTimeoutPrecommit: move to next round", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)

		currentRound := newTestRound(t, stateMachine, 0, 0)
		nextRound := newTestRound(t, stateMachine, 0, 1)

		currentRound.start()
		currentRound.validator(0).precommit(utils.HeapPtr(value(10)))
		currentRound.validator(1).precommit(nil)
		currentRound.validator(2).precommit(nil).expectActions(currentRound.action().scheduleTimeout(precommit))
		assert.True(t, stateMachine.state.timeoutPrecommitScheduled)

		currentRound.processTimeout(precommit).expectActions(nextRound.action().scheduleTimeout(propose))
		assert.False(t, stateMachine.state.timeoutPrecommitScheduled)

		assertState(t, stateMachine, height(0), round(1), propose)

		// Do nothing on old timeout
		currentRound.processTimeout(propose).expectActions()
		assertState(t, stateMachine, height(0), round(1), propose)

		currentRound.processTimeout(prevote).expectActions()
		assertState(t, stateMachine, height(0), round(1), propose)
	})

	t.Run("Ignore old timeout on new height", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)

		currentRound := newTestRound(t, stateMachine, 0, 0)

		committedValue := value(10)

		currentRound.start().expectActions(currentRound.action().scheduleTimeout(propose))

		// Honest validators quickly agree on a value
		currentRound.validator(0).proposal(committedValue, -1)
		currentRound.validator(0).prevote(&committedValue)
		currentRound.validator(1).prevote(&committedValue).expectActions(
			currentRound.action().scheduleTimeout(prevote),
			currentRound.action().broadcastPrecommit(&committedValue),
		)
		currentRound.validator(0).precommit(&committedValue)
		currentRound.validator(2).precommit(nil).expectActions(
			currentRound.action().scheduleTimeout(precommit),
		)
		currentRound.validator(1).precommit(&committedValue)

		// All old timeout should do nothing
		assertState(t, stateMachine, height(1), round(0), propose)
		currentRound.processTimeout(propose).expectActions()
		assertState(t, stateMachine, height(1), round(0), propose)
		currentRound.processTimeout(prevote).expectActions()
		assertState(t, stateMachine, height(1), round(0), propose)
		currentRound.processTimeout(precommit).expectActions()
		assertState(t, stateMachine, height(1), round(0), propose)
	})
}
