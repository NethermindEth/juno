package tendermint

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCreateNewState(t *testing.T) {
	t.Parallel()
	decider := new(deciderMock)

	t.Run("Initial state with no decider panics", func(t *testing.T) {
		require.Panics(t, func() {
			InitialState(nil)
		})
	})

	t.Run("Initial state with decider does not panic", func(t *testing.T) {
		require.NotPanics(t, func() {
			InitialState(decider)
		})
	})

	t.Run("creates initial state successfully", func(t *testing.T) {
		initialState := InitialState(decider)
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
