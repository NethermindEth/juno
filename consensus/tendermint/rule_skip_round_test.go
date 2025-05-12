package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/consensus/types"
)

func TestSkipRound(t *testing.T) {
	t.Run("Line 55 (Proposal): Start round r' when f+1 future round messages are received from round r'", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3, true)
		expectedHeight := types.Height(0)
		rPrime, rPrimeVal := types.Round(4), value(10)
		futureRound := newTestRound(t, stateMachine, expectedHeight, rPrime)

		stateMachine.ProcessStart(types.Round(0))
		futureRound.validator(1).prevote(&rPrimeVal)
		futureRound.validator(0).proposal(rPrimeVal, -1)

		// The step is not propose because the proposal which is received in round r' leads to consensus
		// engine broadcasting prevote to the proposal which changes the step from propose to prevote.
		assertState(t, stateMachine, expectedHeight, rPrime, types.StepPrevote)
	})

	t.Run("Line 55 (Prevote): Start round r' when f+1 future round messages are received from round r'", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3, true)
		expectedHeight := types.Height(0)
		rPrime, rPrimeVal := types.Round(4), value(10)
		futureRound := newTestRound(t, stateMachine, expectedHeight, rPrime)

		stateMachine.ProcessStart(types.Round(0))
		futureRound.validator(0).prevote(&rPrimeVal)
		futureRound.validator(1).prevote(&rPrimeVal)

		// The step here remains propose because a proposal is yet to be received to allow the node to send the
		// prevote for it.
		assertState(t, stateMachine, expectedHeight, rPrime, types.StepPropose)
	})

	t.Run("Line 55 (Precommit): Start round r' when f+1 future round messages are received from round r'", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3, true)
		expectedHeight := types.Height(0)
		rPrime := types.Round(4)
		round4Value := value(10)
		futureRound := newTestRound(t, stateMachine, expectedHeight, rPrime)

		stateMachine.ProcessStart(types.Round(0))
		futureRound.validator(1).prevote(&round4Value)
		futureRound.validator(0).precommit(&round4Value)

		// The step here remains propose because a proposal is yet to be received to allow the node to send the
		// prevote for it.
		assertState(t, stateMachine, expectedHeight, rPrime, types.StepPropose)
	})
}
