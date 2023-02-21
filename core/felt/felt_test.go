package felt_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/encoder"
	"github.com/stretchr/testify/assert"
)

func TestUnmarshalJson(t *testing.T) {
	var with felt.Felt
	assert.NoError(t, with.UnmarshalJSON([]byte("0x4437ab")))

	var without felt.Felt
	assert.NoError(t, without.UnmarshalJSON([]byte("4437ab")))
	assert.Equal(t, true, without.Equal(&with))

	var fails felt.Felt
	assert.Error(t, fails.UnmarshalJSON([]byte("0x2000000000000000000000000000000000000000000000000000000000000000000")))
	assert.Error(t, fails.UnmarshalJSON([]byte("0x800000000000011000000000000000000000000000000000000000000000001")))
	assert.Error(t, fails.UnmarshalJSON([]byte("0xfb01012100000000000000000000000000000000000000000000000000000000")))
}

func TestFeltCbor(t *testing.T) {
	var val felt.Felt
	_, err := val.SetRandom()
	assert.NoError(t, err)

	encoder.TestSymmetry(t, val)
}
