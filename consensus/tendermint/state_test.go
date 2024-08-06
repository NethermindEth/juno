package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"

	"github.com/stretchr/testify/assert"
)

type testFelt felt.Felt

func (t testFelt) Hash() felt.Felt {
	r, _ := new(felt.Felt).SetRandom()
	return *r
}

func TestState(t *testing.T) {
	// Does nothing, for now, just here to easily check for compilation issues.
	s := state[Message[testFelt, felt.Felt], testFelt, felt.Felt, felt.Felt]{}
	assert.Nil(t, s.lockedRound)
}
