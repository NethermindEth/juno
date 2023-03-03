package felt_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/encoder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestShortString(t *testing.T) {
	var f felt.Felt

	t.Run("less than 8 digits", func(t *testing.T) {
		_, err := f.SetString("0x1234567")
		require.NoError(t, err)
		assert.Equal(t, "0x1234567", f.ShortString())
	})

	t.Run("8 digits", func(t *testing.T) {
		_, err := f.SetString("0x12345678")
		require.NoError(t, err)
		assert.Equal(t, "0x12345678", f.ShortString())
	})

	t.Run("more than 8 digits", func(t *testing.T) {
		_, err := f.SetString("0x123456789")
		require.NoError(t, err)
		assert.Equal(t, "0x1234...6789", f.ShortString())
	})
}
