package blake2s_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/crypto/blake2s"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/require"
)

func TestBlakeHash(t *testing.T) {
	t.Run("hash zero", func(t *testing.T) {
		actual, err := blake2s.Blake2sArray(&felt.Zero)
		require.NoError(t, err)

		expected := felt.UnsafeFromString[felt.Hash](
			"0x5768af071a2f8df7c9df9dc4ca0e7a1c5908d5eff88af963c3264f412dbdf43",
		)

		require.Equal(t, expected, actual)
	})

	t.Run("hash one two", func(t *testing.T) {
		val1 := felt.FromUint64[felt.Felt](1)
		val2 := felt.FromUint64[felt.Felt](2)

		actual, err := blake2s.Blake2sArray(&val1, &val2)
		require.NoError(t, err)

		expected := felt.UnsafeFromString[felt.Hash](
			"0x5534c03a14b214436366f30e9c77b6e56c8835de7dc5aee36957d4384cce66d",
		)
		require.Equal(t, expected, actual)
	})
}
