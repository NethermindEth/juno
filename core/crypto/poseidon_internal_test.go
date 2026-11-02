package crypto

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
	"github.com/stretchr/testify/assert"
)

// Test vectors from https://github.com/starkware-industries/poseidon
func TestHadesPermutation(t *testing.T) {
	state := [3]fp.Element{}
	hadesPermutation(&state)

	want := [3]string{
		"3446325744004048536138401612021367625846492093718951375866996507163446763827",
		"1590252087433376791875644726012779423683501236913937337746052470473806035332",
		"867921192302518434283879514999422690776342565400001269945778456016268852423",
	}
	for i, w := range want {
		assert.Equal(t, w, state[i].Text(felt.Base10))
	}
}
