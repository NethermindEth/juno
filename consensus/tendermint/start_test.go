package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/consensus/types"
)

func TestStartRound(t *testing.T) {
	t.Run("node is the proposer", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 0)

		currentRound := newTestRound(t, stateMachine, 0, 0)

		val := value(1)
		currentRound.start().expectActions(
			currentRound.action().writeWALStart(),
			currentRound.action().broadcastProposal(val, -1),
			currentRound.action().broadcastPrevote(new(val)),
		)

		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrevote)
	})

	t.Run("node is not the proposer: schedule timeoutPropose", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)

		currentRound.start().expectActions(
			currentRound.action().writeWALStart(),
			currentRound.action().scheduleTimeout(types.StepPropose),
		)

		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPropose)
	})
}
