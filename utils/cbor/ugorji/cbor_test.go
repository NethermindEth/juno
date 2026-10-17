package ugorji_test

import (
	"bytes"
	"encoding/binary"
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/ugorji"
	"github.com/stretchr/testify/require"
)

func TestUnmarshal(t *testing.T) {
	for _, data := range [][]byte{nil, {0xa1}, {1, 2}} {
		var value any
		require.Error(t, ugorji.Unmarshal(data, &value))
		// A failed decode must not affect the next use of the decoder pool.
		var number uint64
		require.NoError(t, ugorji.Unmarshal([]byte{7}, &number))
		require.Equal(t, uint64(7), number)
	}
}

// Collections are sized once from their declared length, also beyond Ugorji's
// default of 1 MB, as long compiled class bytecode needs.
func TestUnmarshalSizesLongCollections(t *testing.T) {
	const n = 100_000
	data := binary.BigEndian.AppendUint32([]byte{0x9a}, n)
	for range n {
		data = append(data, 0x84, 0, 0, 0, 0)
	}
	var values [][4]uint64
	require.NoError(t, ugorji.Unmarshal(data, &values))
	require.Len(t, values, n)
	require.Equal(t, n, cap(values))
}

func TestUnmarshalBigInt(t *testing.T) {
	type record struct{ N *big.Int }
	key := []byte{0xa1, 0x61, 'N'} // {"N": ...}
	for _, tc := range []struct {
		name     string
		value    []byte
		expected *big.Int
		wantErr  bool
	}{
		{name: "unsigned bignum", value: []byte{0xc2, 0x42, 0x01, 0x00}, expected: big.NewInt(256)},
		{
			name:     "one-byte length",
			value:    append([]byte{0xc2, 0x58, 0x20, 0x08}, make([]byte, 31)...),
			expected: new(big.Int).Lsh(big.NewInt(1), 251),
		},
		{name: "empty magnitude", value: []byte{0xc2, 0x40}, expected: new(big.Int)},
		{name: "null", value: []byte{0xf6}},
		{name: "negative bignum", value: []byte{0xc3, 0x41, 0x01}, wantErr: true},
		{name: "integer", value: []byte{0x01}, wantErr: true},
		{name: "text content", value: []byte{0xc2, 0x61, 'a'}, wantErr: true},
		{name: "untagged bytes", value: []byte{0x41, 0x01}, wantErr: true},
		{
			name:     "indefinite-length content",
			value:    []byte{0xc2, 0x5f, 0x41, 0x01, 0xff},
			expected: big.NewInt(1),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out *record
			err := ugorji.Unmarshal(append(bytes.Clone(key), tc.value...), &out)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, &record{N: tc.expected}, out)
		})
	}
}
