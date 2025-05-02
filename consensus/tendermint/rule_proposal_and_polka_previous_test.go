package tendermint

import (
	"testing"

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

		// In the first round, validator 0 "tricked" us with invalid value.
		firstRound.start()
		firstRound.validator(0).proposal(wrongValue, -1).expectActions(
			firstRound.action().broadcastPrevote(&wrongValue),
		)
		firstRound.validator(0).prevote(&correctValue)
		firstRound.validator(1).prevote(&correctValue)
		firstRound.validator(2).prevote(&correctValue)
		firstRound.validator(1).precommit(&correctValue)
		firstRound.validator(2).precommit(&correctValue)
		firstRound.processTimeout(precommit)

		assertState(t, stateMachine, height(0), round(1), propose)

		secondRound.validator(1).proposal(correctValue, 0).expectActions(
			secondRound.action().broadcastPrevote(&correctValue),
		)
	})

	t.Run("Line 28: locked value change", func(t *testing.T) {
		// In this test, we're validator 1. Validator 0 is faulty.
		stateMachine := setupStateMachine(t, 4, 3)

		firstRound := newTestRound(t, stateMachine, 0, 0)
		secondRound := newTestRound(t, stateMachine, 0, 1)
		thirdRound := newTestRound(t, stateMachine, 0, 2)

		wrongValue := value(42)
		correctValue := value(43)

		// In the first round, validator 0 "tricked" us with invalid value.
		firstRound.start()
		firstRound.validator(0).proposal(wrongValue, -1).expectActions(
			firstRound.action().broadcastPrevote(&wrongValue),
		)
		firstRound.validator(0).prevote(&wrongValue)
		// Validator 2 receives an invalid proposal
		firstRound.validator(2).prevote(nil)
		firstRound.validator(1).prevote(&wrongValue).expectActions(
			firstRound.action().broadcastPrecommit(&wrongValue),
		)
		// Validator 0 sends prevote nil to validator 1, so validator 1 will precommit nil.
		firstRound.validator(1).precommit(nil)
		firstRound.validator(2).precommit(nil)
		firstRound.processTimeout(precommit)

		assert.Equal(t, stateMachine.state.lockedValue, &wrongValue)
		assertState(t, stateMachine, height(0), round(1), propose)

		// In the second round, validator 1 proposes a different value, because it didn't lock to the previous value.
		// We reject it because we locked to the previous value.
		secondRound.validator(1).proposal(correctValue, -1).expectActions(
			secondRound.action().broadcastPrevote(nil),
		)
		secondRound.validator(1).prevote(&correctValue)
		secondRound.validator(2).prevote(&correctValue).expectActions(
			secondRound.action().scheduleTimeout(prevote),
		)
		secondRound.processTimeout(prevote).expectActions(
			secondRound.action().broadcastPrecommit(nil),
		)
		// Validator 0 delays sending the prevote so we can only receives after timeout.
		secondRound.validator(0).prevote(&correctValue)
		// Validator 0 only send prevotes to the other 2 validators, so they will precommit it.
		// Then validator 0 stays silent during the precommit phase.
		secondRound.validator(1).precommit(&correctValue)
		secondRound.validator(2).precommit(&correctValue)
		secondRound.processTimeout(precommit)

		assert.Equal(t, stateMachine.state.lockedValue, &wrongValue)
		assertState(t, stateMachine, height(0), round(2), propose)

		// In the third round, validator 2 proposes value from second round. We accept it
		thirdRound.validator(2).proposal(correctValue, 1).expectActions(
			thirdRound.action().broadcastPrevote(&correctValue),
		)
	})
}
