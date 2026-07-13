package blake2s

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/require"
)

func TestFeltToBytesEncoding(t *testing.T) {
	t.Run("small felt", func(t *testing.T) {
		val := felt.FromUint64[felt.Felt](0x1122334455667788)
		actual := encodeFeltsToBytes(&val)
		expected := []byte{
			0x44, 0x33, 0x22, 0x11, 0x88, 0x77, 0x66, 0x55,
		}

		require.Equal(t, expected, actual)
	})

	t.Run("big felt", func(t *testing.T) {
		val := felt.FromUint64[felt.Felt](0x8000000000000000)
		actual := encodeFeltsToBytes(&val)

		// The expected array of bytes divided in 4 rows of 8 bytes.
		// We expect the mark at the most significant word (first four bytes)
		// but because it is encoded in LE, it is at the last byte.
		expected := []byte{
			0x00, 0x00, 0x00, 0x80, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x80, 0x00, 0x00, 0x00, 0x00,
		}

		require.Equal(t, expected, actual)
	})
}
