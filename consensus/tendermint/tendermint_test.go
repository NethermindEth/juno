package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"

	"github.com/stretchr/testify/assert"
)

type testValue uint64

func (t testValue) Hash() felt.Felt {
	return *new(felt.Felt).SetUint64(uint64(t))
}

func TestState(t *testing.T) {
	//t.Run("initial tendermint state", func(t *testing.T) {
	//	s := New()
	//})
	// Does nothing, for now, just here to easily check for compilation issues.
	s := state[testValue, felt.Felt]{}
	assert.Nil(t, s.lockedRound)
}
