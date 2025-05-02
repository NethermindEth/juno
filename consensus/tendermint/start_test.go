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

		assertState(t, stateMachine, height(0), round(0), prevote)
	})

	t.Run("node is not the proposer: schedule timeoutPropose", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)

		currentRound.start().expectActions(currentRound.action().scheduleTimeout(propose))

		assertState(t, stateMachine, height(0), round(0), propose)
	})
}
