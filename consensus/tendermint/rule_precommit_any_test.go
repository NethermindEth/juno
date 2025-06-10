package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

func TestPrecommitAny(t *testing.T) {
	t.Run("Line 47: upon 2f + 1 {PRECOMMIT, h_p, round_p, *} for the first time schedule timeout", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)

		// Initialise state
		currentRound.start()

		// We need to get to precommit step, so first go through proposal and prevote
		currentRound.validator(0).proposal(starknet.Value(*new(felt.Felt).SetUint64(42)), -1)
		currentRound.validator(0).prevote(utils.HeapPtr(starknet.Value(*new(felt.Felt).SetUint64(42))))
		currentRound.validator(1).prevote(nil)
		currentRound.validator(2).prevote(utils.HeapPtr(starknet.Value(*new(felt.Felt).SetUint64(42))))

		// Receive 2 more precommits combined with our own precommit, all with mixed values
		currentRound.validator(0).precommit(utils.HeapPtr(starknet.Value(*new(felt.Felt).SetUint64(42))))
		currentRound.validator(1).precommit(nil).expectActions(currentRound.action().scheduleTimeout(types.StepPrecommit))

		assert.True(t, stateMachine.state.timeoutPrecommitScheduled)
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrecommit)
	})

	t.Run("Line 47: upon 2f + 1 {PRECOMMIT, h_p, round_p, *} without receiving proposal", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)

		// Initialise state
		currentRound.start()

		// Receive 3 precommits even though we haven't received a proposal.
		currentRound.validator(0).precommit(utils.HeapPtr(starknet.Value(*new(felt.Felt).SetUint64(42))))
		currentRound.validator(1).precommit(nil)
		currentRound.validator(2).precommit(utils.HeapPtr(starknet.Value(*new(felt.Felt).SetUint64(43)))).expectActions(currentRound.action().scheduleTimeout(types.StepPrecommit))

		assert.True(t, stateMachine.state.timeoutPrecommitScheduled)
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPropose)
	})

	t.Run("Line 47: not enough precommits (less than 2f + 1)", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)

		// Initialise state
		currentRound.start()

		// Set up to reach precommit step
		currentRound.validator(0).proposal(starknet.Value(*new(felt.Felt).SetUint64(42)), -1)
		currentRound.validator(0).prevote(utils.HeapPtr(starknet.Value(*new(felt.Felt).SetUint64(42))))
		currentRound.validator(1).prevote(utils.HeapPtr(starknet.Value(*new(felt.Felt).SetUint64(42))))

		// Receive 1 more precommit (not enough for 2f+1 where f=1)
		currentRound.validator(1).precommit(utils.HeapPtr(starknet.Value(*new(felt.Felt).SetUint64(42)))).expectActions()

		// No timeout should be scheduled
		assert.False(t, stateMachine.state.timeoutPrecommitScheduled)
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrecommit)
	})

	t.Run("Line 47: only schedule timeout the first time", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)

		// Initialise state
		currentRound.start()

		// Set up to reach precommit step
		// Validator 0 is a faulty proposer
		currentRound.validator(0).proposal(starknet.Value(*new(felt.Felt).SetUint64(42)), -1).expectActions(
			currentRound.action().broadcastPrevote(utils.HeapPtr(starknet.Value(*new(felt.Felt).SetUint64(42)))),
		)
		currentRound.validator(0).prevote(utils.HeapPtr(starknet.Value(*new(felt.Felt).SetUint64(42))))
		currentRound.validator(1).prevote(utils.HeapPtr(starknet.Value(*new(felt.Felt).SetUint64(42)))).expectActions(
			currentRound.action().scheduleTimeout(types.StepPrevote),
			currentRound.action().broadcastPrecommit(utils.HeapPtr(starknet.Value(*new(felt.Felt).SetUint64(42)))),
		)
		assert.False(t, stateMachine.state.timeoutPrecommitScheduled)
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrecommit)

		// Receive 2 precommits, combined with our own precommit and schedule timeout
		currentRound.validator(0).precommit(nil)
		currentRound.validator(1).precommit(utils.HeapPtr(starknet.Value(*new(felt.Felt).SetUint64(42)))).expectActions(currentRound.action().scheduleTimeout(types.StepPrecommit))
		assert.True(t, stateMachine.state.timeoutPrecommitScheduled)
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrecommit)

		currentRound.validator(2).precommit(nil).expectActions()
		assert.True(t, stateMachine.state.timeoutPrecommitScheduled)
		assertState(t, stateMachine, types.Height(0), types.Round(0), types.StepPrecommit)
	})
}
