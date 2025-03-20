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
	t.Run("with prefix 0x", func(t *testing.T) {
		assert.NoError(t, with.UnmarshalJSON([]byte("0x4437ab")))
	})

	t.Run("without prefix 0x", func(t *testing.T) {
		var without felt.Felt
		assert.NoError(t, without.UnmarshalJSON([]byte("4437ab")))
		assert.Equal(t, true, without.Equal(&with))
	})

	var failF felt.Felt

	fails := []string{
		"0x2000000000000000000000000000000000000000000000000000000000000000000",
		"0x800000000000011000000000000000000000000000000000000000000000001",
		"0xfb01012100000000000000000000000000000000000000000000000000000000",
	}

	for _, hex := range fails {
		t.Run(hex+" fails", func(t *testing.T) {
			assert.Error(t, failF.UnmarshalJSON([]byte(hex)))
		})
	}
}

func TestFeltCbor(t *testing.T) {
	var val felt.Felt
	_, err := val.SetRandom()
	require.NoError(t, err)

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

func TestFeltMarshalAndUnmarshal(t *testing.T) {
	f := new(felt.Felt).SetBytes([]byte("somebytes"))

	fBytes := f.Marshal()

	f2 := new(felt.Felt)
	f2.Unmarshal(fBytes)

	assert.True(t, f2.Equal(f))
}

func TestFeltFromBytes(t *testing.T) {
	bytes := [32]byte{1, 3, 5, 7, 11}
	f := felt.FromBytes(bytes[:])

	require.Equal(t, bytes[:], f.Marshal())
}

func TestFeltFromUint64(t *testing.T) {
	var num uint64 = 1993
	f := felt.FromUint64(num)

	require.Equal(t, num, f.Uint64())
}
