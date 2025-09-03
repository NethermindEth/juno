package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

func TestPolkaAny(t *testing.T) {
	t.Run("Line 34: upon 2f + 1 {PREVOTE, h_p, round_p, *} while step_p = prevote schedule timeout", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)

		// Initialise state
		currentRound.start()

		// Receive a proposal and move to prevote step
		currentRound.validator(0).proposal(value(42), -1)

		// Receive 2 more prevotes combined with our own prevote, all in mixed value
		currentRound.validator(1).prevote(nil)
		currentRound.validator(2).prevote(utils.HeapPtr(value(44))).expectActions(
			currentRound.action().writeWALPrevote(2, utils.HeapPtr(value(44))),
			currentRound.action().scheduleTimeout(types.StepPrevote),
		)
		assert.True(t, stateMachine.state.timeoutPrevoteScheduled)

		// Assertions - We should still be in prevote step, but timeout should be scheduled
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrevote)
	})

	t.Run("Line 34: not enough prevotes (less than 2f + 1)", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)

		// Initialise state
		currentRound.start()

		// Receive a proposal and move to prevote step
		currentRound.validator(0).proposal(value(42), -1)

		// Receive 1 more prevote (not enough for 2f+1 where f=1)
		currentRound.validator(0).prevote(utils.HeapPtr(value(42)))

		// No action should be taken
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrevote)
	})

	t.Run("Line 34: enough prevotes but not in prevote step", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)

		// Initialise state (don't move to prevote step)
		currentRound.start()

		// Add enough prevotes, but since we're not in prevote step, nothing should happen
		currentRound.validator(0).prevote(utils.HeapPtr(value(42))).expectActions(
			currentRound.action().writeWALPrevote(0, utils.HeapPtr(value(42))),
		)
		currentRound.validator(1).prevote(utils.HeapPtr(value(43))).expectActions(
			currentRound.action().writeWALPrevote(1, utils.HeapPtr(value(43))),
		)
		currentRound.validator(2).prevote(utils.HeapPtr(value(44))).expectActions(
			currentRound.action().writeWALPrevote(2, utils.HeapPtr(value(44))),
		)

		// Assertions - still in propose step, no timeout should be scheduled
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPropose)
	})

	t.Run("Line 34: only schedule timeout the first time", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)

		// Initialise state
		currentRound.start()

		// Receive a proposal and move to prevote step
		currentRound.validator(0).proposal(value(42), -1)

		// Receive 2 prevotes, combined with our own prevote and schedule timeout for prevote
		currentRound.validator(0).prevote(nil).expectActions(
			currentRound.action().writeWALPrevote(0, nil),
		)
		currentRound.validator(1).prevote(utils.HeapPtr(value(43))).expectActions(
			currentRound.action().writeWALPrevote(1, utils.HeapPtr(value(43))),
			currentRound.action().scheduleTimeout(types.StepPrevote),
		)
		assert.True(t, stateMachine.state.timeoutPrevoteScheduled)

		// Receive 1 more prevote, no timeout should be scheduled
		currentRound.validator(2).prevote(utils.HeapPtr(value(44))).expectActions(
			currentRound.action().writeWALPrevote(2, utils.HeapPtr(value(44))),
		)
		assert.True(t, stateMachine.state.timeoutPrevoteScheduled)

		// Assertions - We should still be in prevote step
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrevote)
	})
}
