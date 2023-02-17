package crypto

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
)

// Test vector from https://github.com/starkware-industries/poseidon
func TestPermutate(t *testing.T) {
	state := []felt.Felt{{}, {}, {}}
	hadesPermutation(state)
	assert.Equal(t, "3446325744004048536138401612021367625846492093718951375866996507163446763827", state[0].Text(10))
	assert.Equal(t, "1590252087433376791875644726012779423683501236913937337746052470473806035332", state[1].Text(10))
	assert.Equal(t, "867921192302518434283879514999422690776342565400001269945778456016268852423", state[2].Text(10))
}
