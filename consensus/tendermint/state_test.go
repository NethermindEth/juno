package tendermint

import (
	consensus "github.com/NethermindEth/juno/consensus/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCreateNewState(t *testing.T) {
	t.Parallel()

	t.Run("Initial state with no decider panics", func(t *testing.T) {

		require.Panics(t, func() {
			InitialState(nil)
		})
	})

	t.Run("Initial state with decider does not panics", func(t *testing.T) {
		var decider consensus.Decider
		require.NotPanics(t, func() {
			InitialState(&decider)
		})
	})

	t.Run("creates initial state successfully", func(t *testing.T) {
		var decider consensus.Decider
		initialState := InitialState(&decider)
		assert.Equal(t, STEP_PROPOSE, initialState.step)
		assert.Equal(t, RoundType(0), initialState.round)
		assert.Equal(t, ROUND_NONE, initialState.validRound)
		assert.Equal(t, ROUND_NONE, initialState.lockedRound)
		assert.Equal(t, VOTE_NONE, initialState.validValue)
		assert.Equal(t, VOTE_NONE, initialState.lockedValue)
		assert.NotNil(t, initialState.decider)
		assert.True(t, initialState.isFirstPreCommit)
		assert.True(t, initialState.isFirstPreVote)
	})
}
