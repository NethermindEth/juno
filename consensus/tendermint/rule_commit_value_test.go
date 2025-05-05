package tendermint

import (
	"slices"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
)

func TestCommitValue(t *testing.T) {
	t.Run("Line 49 (Proposal): commit the value", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)
		nextRound := newTestRound(t, stateMachine, 1, 0)

		committedValue := value(10)

		currentRound.start().expectActions(
			currentRound.action().scheduleTimeout(propose),
		)
		val0Precommit := currentRound.validator(0).precommit(&committedValue).inputMessage
		val1Precommit := currentRound.validator(1).precommit(&committedValue).inputMessage
		val2Precommit := currentRound.validator(2).precommit(&committedValue).expectActions(
			currentRound.action().scheduleTimeout(precommit),
		).inputMessage
		assert.True(t, stateMachine.state.timeoutPrecommitScheduled)

		currentRound.validator(0).proposal(committedValue, -1).expectActions(
			currentRound.action().broadcastPrevote(&committedValue),
			nextRound.action().scheduleTimeout(propose),
		)
		assert.False(t, stateMachine.state.timeoutPrecommitScheduled)

		assertState(t, stateMachine, height(1), round(0), propose)

		// TODO: This is a workaround to get the chain. Find a better way to do this.
		chain := stateMachine.blockchain.(*chain)

		precommits := []Precommit[felt.Felt, felt.Felt]{val0Precommit, val1Precommit, val2Precommit}
		assert.Equal(t, chain.decision[0], committedValue)
		for _, p := range chain.decisionCertificates[0] {
			assert.True(t, slices.Contains(precommits, p))
		}

		assert.Empty(t, stateMachine.messages.proposals)
		assert.Empty(t, stateMachine.messages.prevotes)
		assert.Empty(t, stateMachine.messages.precommits)
	})

	t.Run("Line 49 (Precommit): commit the value", func(t *testing.T) {
		stateMachine := setupStateMachine(t, 4, 3)
		currentRound := newTestRound(t, stateMachine, 0, 0)
		nextRound := newTestRound(t, stateMachine, 1, 0)
		committedValue := value(10)

		currentRound.start().expectActions(
			currentRound.action().scheduleTimeout(propose),
		)

		val0Precommit := currentRound.validator(0).precommit(&committedValue).inputMessage
		currentRound.validator(0).proposal(committedValue, -1)
		val1Precommit := currentRound.validator(1).precommit(&committedValue).inputMessage
		val2Precommit := currentRound.validator(2).precommit(&committedValue).expectActions(
			currentRound.action().scheduleTimeout(precommit),
			nextRound.action().scheduleTimeout(propose),
		).inputMessage

		assertState(t, stateMachine, height(1), round(0), propose)

		// TODO: This is a workaround to get the chain. Find a better way to do this.
		chain := stateMachine.blockchain.(*chain)

		precommits := []Precommit[felt.Felt, felt.Felt]{val0Precommit, val1Precommit, val2Precommit}
		assert.Equal(t, chain.decision[0], committedValue)
		for _, p := range chain.decisionCertificates[0] {
			assert.True(t, slices.Contains(precommits, p))
		}

		assert.Empty(t, stateMachine.messages.proposals)
		assert.Empty(t, stateMachine.messages.prevotes)
		assert.Empty(t, stateMachine.messages.precommits)
	})
}
