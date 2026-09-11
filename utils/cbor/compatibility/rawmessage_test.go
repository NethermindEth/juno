package cbor_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// the array [1, 2, 3], already encoded
var cborArray123 = []byte{0x83, 0x01, 0x02, 0x03}

// two cborArray123 inside an array of two
var cborTwoArrays = []byte{0x82, 0x83, 0x01, 0x02, 0x03, 0x83, 0x01, 0x02, 0x03}

// rawMessageContract is the behaviour a RawMessage has to have, whichever encoder is underneath.
// Each version declares its own, so the type comes in as a parameter.
func rawMessageContract[R ~[]byte](
	t *testing.T,
	marshal func(any) ([]byte, error),
	unmarshal func([]byte, any) error,
) {
	t.Helper()

	t.Run("MarshalUnchanged", func(t *testing.T) {
		out, err := marshal([]R{R(cborArray123), R(cborArray123)})
		require.NoError(t, err)
		require.Equal(t, cborTwoArrays, out)
	})

	t.Run("MarshalEmptyAsNull", func(t *testing.T) {
		out, err := marshal(R(nil))
		require.NoError(t, err)
		require.Equal(t, []byte{0xf6}, out)
	})

	t.Run("UnmarshalUnchanged", func(t *testing.T) {
		var decoded []R
		require.NoError(t, unmarshal(cborTwoArrays, &decoded))
		require.Equal(t, []R{R(cborArray123), R(cborArray123)}, decoded)
	})

	t.Run("UnmarshalNullAsNull", func(t *testing.T) {
		var decoded R
		require.NoError(t, unmarshal([]byte{0xf6}, &decoded))
		require.Equal(t, R{0xf6}, decoded)
	})

	t.Run("UnmarshalReplacesTarget", func(t *testing.T) {
		decoded := R{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
		require.NoError(t, unmarshal(cborArray123, &decoded))
		require.Equal(t, R(cborArray123), decoded)
	})

	t.Run("UnmarshalCopies", func(t *testing.T) {
		data := append([]byte{}, cborArray123...)

		var decoded R
		require.NoError(t, unmarshal(data, &decoded))

		data[1] = 0xff
		require.Equal(t, R(cborArray123), decoded)
	})
}
