package blake2s_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/crypto/blake2s"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/require"
)

func TestBlakeHash(t *testing.T) {
	t.Run("hash zero", func(t *testing.T) {
		actual := blake2s.Blake2sArray(&felt.Zero)

		expected := felt.UnsafeFromString[felt.Hash](
			"0x5768af071a2f8df7c9df9dc4ca0e7a1c5908d5eff88af963c3264f412dbdf43",
		)

		require.Equal(t, expected, actual)
	})

	t.Run("hash one two", func(t *testing.T) {
		val1 := felt.FromUint64[felt.Felt](1)
		val2 := felt.FromUint64[felt.Felt](2)

		actual := blake2s.Blake2sArray(&val1, &val2)

		expected := felt.UnsafeFromString[felt.Hash](
			"0x5534c03a14b214436366f30e9c77b6e56c8835de7dc5aee36957d4384cce66d",
		)
		require.Equal(t, expected, actual)
	})

	t.Run("empty array", func(t *testing.T) {
		actual := blake2s.Blake2sArray[felt.Felt]()

		expected := felt.UnsafeFromString[felt.Hash](
			"0x1eed01efd0d230c1ea5a12c48b6551f7c4a3542d02111e194809079307a214a",
		)

		require.Equal(t, expected, actual)
	})

	t.Run("small felt", func(t *testing.T) {
		actual := blake2s.Blake2sArray(felt.NewFromUint64[felt.Felt](1<<63 - 1))

		expected := felt.UnsafeFromString[felt.Hash](
			"0x354aef67e2b1a01d5afe9a85707d79349c8be8a4b261629360b2129810d8dd",
		)
		require.Equal(t, expected, actual)
	})

	t.Run("at boundary", func(t *testing.T) {
		actual := blake2s.Blake2sArray(felt.NewFromUint64[felt.Felt](1 << 63))

		expected := felt.UnsafeFromString[felt.Hash](
			"0xb44aeea3e1288e4614b3de3a7a6bee8d592ef9cd30c70304e7e84b52702ef2",
		)

		require.Equal(t, expected, actual)
	})

	t.Run("large felt", func(t *testing.T) {
		actual := blake2s.Blake2sArray(
			felt.NewUnsafeFromString[felt.Felt](
				"0x800000000000011000000000000000000000000000000000000000000000000",
			))

		expected := felt.UnsafeFromString[felt.Hash](
			"0x7c018937c4b4968cc90a67326b1715ca808b7546ad91fb91fbc42a3dc6c52f1",
		)

		require.Equal(t, expected, actual)
	})

	t.Run("mixed small large felts", func(t *testing.T) {
		actual := blake2s.Blake2sArray(
			felt.NewFromUint64[felt.Felt](42),
			felt.NewFromUint64[felt.Felt](1<<63),
			felt.NewFromUint64[felt.Felt](1337),
		)

		expected := felt.UnsafeFromString[felt.Hash](
			"0x27e2140360d26bf4a53778d8bc2f9b103ddfa83abf3451308190b9c340d11a5",
		)

		require.Equal(t, expected, actual)
	})

	t.Run("max u64", func(t *testing.T) {
		actual := blake2s.Blake2sArray(felt.NewFromUint64[felt.Felt](^uint64(0)))

		expected := felt.UnsafeFromString[felt.Hash](
			"0x7c576253e278f7f66f8a9a8776b2d9e1c6f03b1bbdb8e25c9cbcab854199426",
		)

		require.Equal(t, expected, actual)
	})
}
