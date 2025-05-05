package tendermint

import "testing"

func TestSkipRound(t *testing.T) {
	t.Run("Line 55 (Proposal): Start round r' when f+1 future round messages are received from round r'", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		expectedHeight := height(0)
		rPrime, rPrimeVal := round(4), value(10)
		futureRound := newTestRound(t, stateMachine, expectedHeight, rPrime)

		stateMachine.processStart(round(0))
		futureRound.validator(1).prevote(&rPrimeVal)
		futureRound.validator(0).proposal(rPrimeVal, -1)

		// The step is not propose because the proposal which is received in round r' leads to consensus
		// engine broadcasting prevote to the proposal which changes the step from propose to prevote.
		assertState(t, stateMachine, expectedHeight, rPrime, prevote)
	})

	t.Run("Line 55 (Prevote): Start round r' when f+1 future round messages are received from round r'", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		expectedHeight := height(0)
		rPrime, rPrimeVal := round(4), value(10)
		futureRound := newTestRound(t, stateMachine, expectedHeight, rPrime)

		stateMachine.processStart(round(0))
		futureRound.validator(0).prevote(&rPrimeVal)
		futureRound.validator(1).prevote(&rPrimeVal)

		// The step here remains propose because a proposal is yet to be received to allow the node to send the
		// prevote for it.
		assertState(t, stateMachine, expectedHeight, rPrime, propose)
	})

	t.Run("Line 55 (Precommit): Start round r' when f+1 future round messages are received from round r'", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		expectedHeight := height(0)
		rPrime := round(4)
		round4Value := value(10)
		futureRound := newTestRound(t, stateMachine, expectedHeight, rPrime)

		stateMachine.processStart(round(0))
		futureRound.validator(1).prevote(&round4Value)
		futureRound.validator(0).precommit(&round4Value)

		// The step here remains propose because a proposal is yet to be received to allow the node to send the
		// prevote for it.
		assertState(t, stateMachine, expectedHeight, rPrime, propose)
	})
}
