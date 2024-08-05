package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"

	"github.com/stretchr/testify/assert"
)

type testFelt felt.Felt

func (t testFelt) Hash() felt.Felt {
	return *new(felt.Felt).SetBytes([]byte("Some random felt's hash"))
}

func TestState(t *testing.T) {
	// Does nothing, for now, just here to easily check for compilation issues.
	s := state[testFelt, felt.Felt]{}
	assert.Nil(t, s.lockedRound)
}
