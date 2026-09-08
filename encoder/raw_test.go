package encoder_test

import (
	"testing"

	"github.com/NethermindEth/juno/encoder"
	"github.com/stretchr/testify/require"
)

// the array [1, 2, 3], already encoded
var cborArray123 = []byte{0x83, 0x01, 0x02, 0x03}

// two cborArray123 inside an array of two
var cborTwoArrays = []byte{0x82, 0x83, 0x01, 0x02, 0x03, 0x83, 0x01, 0x02, 0x03}

func TestRawMessageMarshalUnchanged(t *testing.T) {
	out, err := encoder.Marshal([]encoder.RawMessage{cborArray123, cborArray123})
	require.NoError(t, err)
	require.Equal(t, cborTwoArrays, out)
}

func TestRawMessageMarshalEmptyAsNull(t *testing.T) {
	out, err := encoder.Marshal(encoder.RawMessage(nil))
	require.NoError(t, err)
	require.Equal(t, []byte{0xf6}, out)
}

func TestRawMessageUnmarshalUnchanged(t *testing.T) {
	var decoded []encoder.RawMessage
	require.NoError(t, encoder.Unmarshal(cborTwoArrays, &decoded))
	require.Equal(t, []encoder.RawMessage{cborArray123, cborArray123}, decoded)
}

func TestRawMessageUnmarshalNullAsNull(t *testing.T) {
	var decoded encoder.RawMessage
	require.NoError(t, encoder.Unmarshal([]byte{0xf6}, &decoded))
	require.Equal(t, encoder.RawMessage{0xf6}, decoded)
}

func TestRawMessageUnmarshalReplacesTarget(t *testing.T) {
	decoded := encoder.RawMessage{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	require.NoError(t, encoder.Unmarshal(cborArray123, &decoded))
	require.Equal(t, encoder.RawMessage(cborArray123), decoded)
}

func TestRawMessageUnmarshalCopies(t *testing.T) {
	data := append([]byte{}, cborArray123...)

	var decoded encoder.RawMessage
	require.NoError(t, encoder.Unmarshal(data, &decoded))

	data[1] = 0xff
	require.Equal(t, encoder.RawMessage(cborArray123), decoded)
}
