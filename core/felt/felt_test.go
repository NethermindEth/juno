package felt_test

import (
	"strings"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/encoder"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
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
		"0\"",
		"\"",
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

func FuzzUnmarshalJson(f *testing.F) {
	var ft felt.Felt
	f.Fuzz(func(t *testing.T, bytes []byte) {
		isErr := false
		expected := ""
		if b := bytes[:]; len(b) > fp.Bits*3 {
			isErr = true
		} else {
			if len(b) > 0 && b[0] == '"' && b[len(b)-1] == '"' {
				startsWithQuot := b[0] == '"'
				endsWithQuot := b[len(b)-1] == '"'
				if startsWithQuot != endsWithQuot || startsWithQuot && len(b) == 1 { // checks for `"*`, `*"` and `"` cases
					isErr = true
				} else {
					b = b[1 : len(b)-1]
				}
			}
			_, err := ft.SetString(string(b))
			expected = ft.ShortString()
			if err != nil {
				isErr = true
			} else {
				if !strings.HasPrefix(expected, "0x") {
					expected = "0x" + expected
				}
			}
		}
		err := ft.UnmarshalJSON(bytes)
		if isErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, expected, ft.ShortString())
		}
	})
}
