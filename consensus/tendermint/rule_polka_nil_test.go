package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/consensus/types"
)

func TestPolkaNil(t *testing.T) {
	t.Run("Line 44: upon 2f + 1 {PREVOTE, h_p, round_p, nil} broadcast nil precommit", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)

		// Initialise state
		currentRound.start()

		// Receive a proposal and move to prevote step
		currentRound.validator(0).proposal(value(42), -1)

		// Receive 3 prevotes
		currentRound.validator(0).prevote(nil)
		currentRound.validator(1).prevote(nil)
		currentRound.validator(2).prevote(nil).expectActions(
			currentRound.action().writeWALPrevote(2, nil),
			currentRound.action().broadcastPrecommit(nil),
		)

		// Assertions - We should be in precommit step
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrecommit)
	})

	t.Run("Line 44: upon 2f + 1 {PREVOTE, h_p, round_p, nil} from other nodes broadcast nil precommit", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)

		// Initialise state
		currentRound.start()

		// Timeout proposal
		currentRound.processTimeout(types.StepPropose)

		// Receive 2 prevotes, combined with our own prevote due to proposal timeout
		currentRound.validator(0).prevote(nil)
		currentRound.validator(1).prevote(nil).expectActions(
			currentRound.action().writeWALPrevote(1, nil),
			currentRound.action().scheduleTimeout(types.StepPrevote),
			currentRound.action().broadcastPrecommit(nil),
		)

		// Assertions - We should be in precommit step
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrecommit)
	})

	t.Run("Line 44: not enough nil prevotes (less than 2f + 1)", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)

		// Initialise state
		currentRound.start()

		// Receive a proposal and move to prevote step
		currentRound.validator(0).proposal(value(42), -1)

		// Receive 2 prevotes
		currentRound.validator(0).prevote(nil)
		currentRound.validator(1).prevote(nil).expectActions(
			currentRound.action().writeWALPrevote(1, nil),
			currentRound.action().scheduleTimeout(types.StepPrevote),
		)

		// Assertions - Only 2 validators prevote nil (not enough for 2f+1 where f=1)
		// No expected precommit action should occur
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrevote)
	})

	t.Run("Line 44: enough nil prevotes but not in prevote step", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)

		// Initialise state
		currentRound.start()

		// Now add enough nil prevotes, but since we're not in prevote step, nothing should happen
		currentRound.validator(0).prevote(nil)
		currentRound.validator(1).prevote(nil)
		currentRound.validator(2).prevote(nil)

		// Assertions - we should still be in propose step
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPropose)
	})

	t.Run("Line 44: mixed prevotes (some nil, some with value)", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)

		// Initialise state
		currentRound.start()

		// Receive a proposal and move to prevote step
		currentRound.validator(0).proposal(value(42), -1)

		// Receive 2 prevotes
		currentRound.validator(0).prevote(nil)
		currentRound.validator(1).prevote(nil)

		// This validator votes for a value instead of nil
		val := value(42)
		currentRound.validator(2).prevote(&val)

		// No precommit action should occur with mixed votes
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrevote)
	})
}
