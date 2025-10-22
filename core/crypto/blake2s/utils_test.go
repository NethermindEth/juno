package blake2s

import (
	"testing"
	"unsafe"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeltToUint32Encoding(t *testing.T) {
	t.Run("small felt", func(t *testing.T) {
		val := felt.FromUint64[felt.Felt](0x1122334455667788)
		actual := encodeFeltsToUint32s(&val)
		expected := []uint32{0x11223344, 0x55667788}
		require.Equal(t, expected, actual)
	})

	t.Run("big felt", func(t *testing.T) {
		val := felt.FromUint64[felt.Felt](0x8000000000000000)
		actual := encodeFeltsToUint32s(&val)
		expected := []uint32{
			0x80000000, 0x0,
			0x0, 0x0,
			0x0, 0x0,
			0x80000000, 0x0,
		}
		require.Equal(t, expected, actual)
	})
}

func TestUint32ToBytes(t *testing.T) {
	values := []uint32{0x11, 0x2233, 0x445566}
	actual := encodeUint32sToBytes(values)
	expected := []byte{
		0x11, 0x00, 0x00, 0x00,
		0x33, 0x22, 0x00, 0x00,
		0x66, 0x55, 0x44, 0x00,
	}
	require.Equal(t, expected, actual)
}

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

func TestUnsafeConversion(t *testing.T) {
	val := []*[4]uint64{
		{10, 12, 14, 16},
		{15, 20, 21, 23},
		{17, 19, 23, 29},
	}

	felts := unsafe.Slice((**felt.Felt)(unsafe.Pointer(&val[0])), len(val))
	for i, f := range felts {
		assert.Equal(t, *val[i], [4]uint64(*f))
	}
}
