package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/consensus/types"
	"github.com/stretchr/testify/assert"
)

func TestProposalAndPolkaPrevious(t *testing.T) {
	t.Run("Line 28: locked value is unchanged", func(t *testing.T) {
		// In this test, we're validator 1. Validator 0 is faulty.
		stateMachine := setupStateMachine(t, 4, 3)

		firstRound := newTestRound(t, stateMachine, 0, 0)
		secondRound := newTestRound(t, stateMachine, 0, 1)

		wrongValue := value(42)
		correctValue := value(43)

		// In the first round, validator 0 sent us a different (valid) value from all the other peers
		firstRound.start()
		firstRound.validator(0).proposal(wrongValue, -1).expectActions(
			firstRound.action().writeWALProposal(0, wrongValue, -1),
			firstRound.action().broadcastPrevote(&wrongValue),
		)
		firstRound.validator(0).prevote(&correctValue)
		firstRound.validator(1).prevote(&correctValue)
		firstRound.validator(2).prevote(&correctValue)
		firstRound.validator(1).precommit(&correctValue)
		firstRound.validator(2).precommit(&correctValue)
		firstRound.processTimeout(types.StepPrevote)
		firstRound.processTimeout(types.StepPrecommit)

		assertState(t, stateMachine, types.Height(0), types.Round(1), types.StepPropose)

		// In the 2nd round, validator 1 proposes the correct value with valid round set to the 1st round.
		// We accept and prevote it because we receive quorum of prevotes in the 1st round.
		secondRound.validator(1).proposal(correctValue, 0).expectActions(
			secondRound.action().writeWALProposal(1, correctValue, 0),
			secondRound.action().broadcastPrevote(&correctValue),
		)
	})

	t.Run("Line 28: locked value change", func(t *testing.T) {
		// This test is to test the behaviour of prevoting a value even if it doesn't match the locked value of the previous round.
		// In this test, we're validator 3. Validator 0 is faulty.
		// 1st round:
		// - Faulty validator 0 is the proposer, sends `firstValue` to validator 1 and 3 (us) and ignores validator 2.
		// - Validator 2 prevotes and precommits nil in this round, because it doesn't receive the proposal.
		// - We receive prevotes from validator 0 and 1, so we lock and precommit `firstValue`.
		// - Faulty validator 0 doesn't send prevote to validator 1, so validator 1 doesn't have enough quorum, so it precommits nil.
		// - Not enough precommits, so we don't commit and move to the next round.
		// 2nd round:
		// - In the 2nd round, validator 1 proposes `secondValue`, because it didn't lock to `firstValue`.
		// - We reject it because we locked to the previous value, so we prevote and precommit nil.
		// - Validator 0, 1 and 2 form a quorum, so validator 1 and 2 prevote and precommit `secondValue`.
		// - Faulty validator 0 delays sending prevote to us, so we receive it after timeout.
		// - Faulty validator 0 stays silent during the precommit phase.
		// - Precommit timeout is triggered, we move to the next round.
		// 3rd round:
		// - Validator 2 proposes `secondValue`, because it locked to `secondValue` in the 2nd round. Valid round is set to the 2nd round.
		// - We saw prevotes in the 2nd round (proposal's valid round) from validator 0 (delayed), 1 and 2.
		// - We accept it even if it doesn't match the locked value.
		stateMachine := setupStateMachine(t, 4, 3)

		firstRound := newTestRound(t, stateMachine, 0, 0)
		secondRound := newTestRound(t, stateMachine, 0, 1)
		thirdRound := newTestRound(t, stateMachine, 0, 2)

		firstValue := value(42)
		secondValue := value(43)

		// In the first round, validator 0 "tricked" us to lock to a value but sent a different value to validator 2.
		firstRound.start()
		firstRound.validator(0).proposal(firstValue, -1).expectActions(
			firstRound.action().writeWALProposal(0, firstValue, -1),
			firstRound.action().broadcastPrevote(&firstValue),
		)
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrevote)
		firstRound.validator(0).prevote(&firstValue)

		// Validator 2 receives no proposal
		firstRound.validator(2).prevote(nil)

		// Line 36 is triggered
		firstRound.validator(1).prevote(&firstValue).expectActions(
			firstRound.action().writeWALPrevote(1, &firstValue),
			firstRound.action().broadcastPrecommit(&firstValue),
		)
		assert.Equal(t, stateMachine.state.lockedValue, &firstValue)
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrecommit)

		// Validator 0 sends prevote nil to validator 1, so validator 1 will precommit nil.
		firstRound.validator(1).precommit(nil)
		firstRound.validator(2).precommit(nil)
		firstRound.processTimeout(types.StepPrecommit)

		assert.Equal(t, stateMachine.state.lockedValue, &firstValue)
		assertState(t, stateMachine, types.Height(0), types.Round(1), types.StepPropose)

		// In the second round, validator 1 proposes a different value, because it didn't lock to the previous value.
		// We reject it because we locked to the previous value.
		secondRound.validator(1).proposal(secondValue, -1).expectActions(
			secondRound.action().writeWALProposal(1, secondValue, -1),
			secondRound.action().broadcastPrevote(nil),
		)
		secondRound.validator(1).prevote(&secondValue)
		secondRound.validator(2).prevote(&secondValue).expectActions(
			secondRound.action().writeWALPrevote(2, &secondValue),
			secondRound.action().scheduleTimeout(types.StepPrevote),
		)
		secondRound.processTimeout(types.StepPrevote).expectActions(
			secondRound.action().writeWALTimeout(types.StepPrevote),
			secondRound.action().broadcastPrecommit(nil),
		)
		// Validator 0 delays sending the prevote so we can only receives after timeout.
		secondRound.validator(0).prevote(&secondValue)
		// Validator 0 only send prevotes to the other 2 validators, so they will precommit it.
		// Then validator 0 stays silent during the precommit phase.
		secondRound.validator(1).precommit(&secondValue)
		secondRound.validator(2).precommit(&secondValue)
		secondRound.processTimeout(types.StepPrecommit)

		assert.Equal(t, stateMachine.state.lockedValue, &firstValue)
		assertState(t, stateMachine, types.Height(0), types.Round(2), types.StepPropose)

		// In the third round, validator 2 proposes value from second round. We accept it
		thirdRound.validator(2).proposal(secondValue, 1).expectActions(
			thirdRound.action().writeWALProposal(2, secondValue, 1),
			thirdRound.action().broadcastPrevote(&secondValue),
		)
	})
}
