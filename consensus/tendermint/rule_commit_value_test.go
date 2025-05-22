package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/stretchr/testify/assert"
)

func TestCommitValue(t *testing.T) {
	t.Run("Line 49 (Proposal): commit the value", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)
		nextRound := newTestRound(t, stateMachine, 1, 0)

		committedValue := starknet.Value(10)

		currentRound.start().expectActions(
			currentRound.action().scheduleTimeout(types.StepPropose),
		)
		currentRound.validator(0).precommit(&committedValue)
		currentRound.validator(1).precommit(&committedValue)
		currentRound.validator(2).precommit(&committedValue).expectActions(
			currentRound.action().scheduleTimeout(types.StepPrecommit),
		)
		assert.True(t, stateMachine.state.timeoutPrecommitScheduled)

		currentRound.validator(0).proposal(committedValue, -1).expectActions(
			currentRound.action().broadcastPrevote(&committedValue),
			nextRound.action().scheduleTimeout(types.StepPropose),
		)
		assert.False(t, stateMachine.state.timeoutPrecommitScheduled)

		assertState(t, stateMachine, types.Height(1), types.Round(0), types.StepPropose)

		// TODO: This is a workaround to get the chain. Find a better way to do this.
		chain := stateMachine.blockchain.(*chain)

		assert.Equal(t, chain.decision[0], committedValue)

		assert.Empty(t, stateMachine.messages.Proposals)
		assert.Empty(t, stateMachine.messages.Prevotes)
		assert.Empty(t, stateMachine.messages.Precommits)
	})

	t.Run("Line 49 (Precommit): commit the value", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)
		nextRound := newTestRound(t, stateMachine, 1, 0)
		committedValue := starknet.Value(10)

		currentRound.start().expectActions(
			currentRound.action().scheduleTimeout(types.StepPropose),
		)

		currentRound.validator(0).precommit(&committedValue)
		currentRound.validator(0).proposal(committedValue, -1)
		currentRound.validator(1).precommit(&committedValue)
		currentRound.validator(2).precommit(&committedValue).expectActions(
			currentRound.action().scheduleTimeout(types.StepPrecommit),
			nextRound.action().scheduleTimeout(types.StepPropose),
		)

		assertState(t, stateMachine, types.Height(1), types.Round(0), types.StepPropose)

		// TODO: This is a workaround to get the chain. Find a better way to do this.
		chain := stateMachine.blockchain.(*chain)

		assert.Equal(t, chain.decision[0], committedValue)

		assert.Empty(t, stateMachine.messages.Proposals)
		assert.Empty(t, stateMachine.messages.Prevotes)
		assert.Empty(t, stateMachine.messages.Precommits)
	})
}
