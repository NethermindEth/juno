package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/utils"
)

func TestStartRound(t *testing.T) {
	t.Run("node is the proposer", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 0)

		currentRound := newTestRound(t, stateMachine, 0, 0)

		val := value(1)
		currentRound.start().expectActions(
			currentRound.action().broadcastProposal(val, -1),
			currentRound.action().broadcastPrevote(utils.HeapPtr(val)),
		)

		assertState(t, stateMachine, Height(0), Round(0), StepPrevote)
	})

	t.Run("node is not the proposer: schedule timeoutPropose", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)

		currentRound.start().expectActions(currentRound.action().scheduleTimeout(StepPropose))

		assertState(t, stateMachine, Height(0), Round(0), StepPropose)
	})
}
