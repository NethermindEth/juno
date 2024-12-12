package trie_test

import (
	"bytes"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeyEncoding(t *testing.T) {
	tests := map[string]struct {
		Len   uint8
		Bytes []byte
	}{
		"multiple of 8": {
			Len:   4 * 8,
			Bytes: []byte{0xDE, 0xAD, 0xBE, 0xEF},
		},
		"0 len": {
			Len:   0,
			Bytes: []byte{},
		},
		"odd len": {
			Len:   3,
			Bytes: []byte{0x03},
		},
	}

	for desc, test := range tests {
		t.Run(desc, func(t *testing.T) {
			key := trie.NewKey(test.Len, test.Bytes)

			var keyBuffer bytes.Buffer
			n, err := key.WriteTo(&keyBuffer)
			require.NoError(t, err)
			assert.Equal(t, len(test.Bytes)+1, int(n))

			keyBytes := keyBuffer.Bytes()
			require.Len(t, keyBytes, int(n))
			assert.Equal(t, test.Len, keyBytes[0])
			assert.Equal(t, test.Bytes, keyBytes[1:])

			var decodedKey trie.Key
			require.NoError(t, decodedKey.UnmarshalBinary(keyBytes))
			assert.Equal(t, key, decodedKey)
		})
	}
}

func BenchmarkKeyEncoding(b *testing.B) {
	val, err := new(felt.Felt).SetRandom()
	require.NoError(b, err)
	valBytes := val.Bytes()

	key := trie.NewKey(felt.Bits, valBytes[:])
	buffer := bytes.Buffer{}
	buffer.Grow(felt.Bytes + 1)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := key.WriteTo(&buffer)
		require.NoError(b, err)
		require.NoError(b, key.UnmarshalBinary(buffer.Bytes()))
		buffer.Reset()
	}
}

func TestTruncate(t *testing.T) {
	tests := map[string]struct {
		key         trie.Key
		newLen      uint8
		expectedKey trie.Key
	}{
		"truncate to 12 bits": {
			key:         trie.NewKey(16, []byte{0xF3, 0x14}),
			newLen:      12,
			expectedKey: trie.NewKey(12, []byte{0x03, 0x14}),
		},
		"truncate to 9 bits": {
			key:         trie.NewKey(16, []byte{0xF3, 0x14}),
			newLen:      9,
			expectedKey: trie.NewKey(9, []byte{0x01, 0x14}),
		},
		"truncate to 3 bits": {
			key:         trie.NewKey(16, []byte{0xF3, 0x14}),
			newLen:      3,
			expectedKey: trie.NewKey(3, []byte{0x04}),
		},
		"truncate to multiple of 8": {
			key: trie.NewKey(251, []uint8{
				0x7, 0x40, 0x33, 0x8c, 0xbc, 0x9, 0xeb, 0xf, 0xb7, 0xab,
				0xc5, 0x20, 0x35, 0xc6, 0x4d, 0x4e, 0xa5, 0x78, 0x18, 0x9e, 0xd6, 0x37, 0x47, 0x91, 0xd0,
				0x6e, 0x44, 0x1e, 0xf7, 0x7f, 0xf, 0x5f,
			}),
			newLen: 248,
			expectedKey: trie.NewKey(248, []uint8{
				0x0, 0x40, 0x33, 0x8c, 0xbc, 0x9, 0xeb, 0xf, 0xb7, 0xab,
				0xc5, 0x20, 0x35, 0xc6, 0x4d, 0x4e, 0xa5, 0x78, 0x18, 0x9e, 0xd6, 0x37, 0x47, 0x91, 0xd0,
				0x6e, 0x44, 0x1e, 0xf7, 0x7f, 0xf, 0x5f,
			}),
		},
	}

	for desc, test := range tests {
		t.Run(desc, func(t *testing.T) {
			copyKey := test.key
			copyKey.Truncate(test.newLen)
			assert.Equal(t, test.expectedKey, copyKey)
		})
	}
}

func TestKeyTest(t *testing.T) {
	key := trie.NewKey(44, []byte{0x10, 0x02})
	for i := 0; i < int(key.Len()); i++ {
		assert.Equal(t, i == 1 || i == 12, key.IsBitSet(uint8(i)), i)
	}
}

func TestIsBitSet(t *testing.T) {
	tests := map[string]struct {
		key      trie.Key
		position uint8
		expected bool
	}{
		"single byte, LSB set": {
			key:      trie.NewKey(8, []byte{0x01}),
			position: 0,
			expected: true,
		},
		"single byte, MSB set": {
			key:      trie.NewKey(8, []byte{0x80}),
			position: 7,
			expected: true,
		},
		"single byte, middle bit set": {
			key:      trie.NewKey(8, []byte{0x10}),
			position: 4,
			expected: true,
		},
		"single byte, bit not set": {
			key:      trie.NewKey(8, []byte{0xFE}),
			position: 0,
			expected: false,
		},
		"multiple bytes, LSB set": {
			key:      trie.NewKey(16, []byte{0x00, 0x02}),
			position: 1,
			expected: true,
		},
		"multiple bytes, MSB set": {
			key:      trie.NewKey(16, []byte{0x01, 0x00}),
			position: 8,
			expected: true,
		},
		"multiple bytes, no bits set": {
			key:      trie.NewKey(16, []byte{0x00, 0x00}),
			position: 7,
			expected: false,
		},
		"check all bits in pattern": {
			key:      trie.NewKey(8, []byte{0xA5}), // 10100101
			position: 0,
			expected: true,
		},
	}

	// Additional test for 0xA5 pattern
	key := trie.NewKey(8, []byte{0xA5}) // 10100101
	expectedBits := []bool{true, false, true, false, false, true, false, true}
	for i, expected := range expectedBits {
		assert.Equal(t, expected, key.IsBitSet(uint8(i)), "bit %d in 0xA5", i)
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := tc.key.IsBitSet(tc.position)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestMostSignificantBits(t *testing.T) {
	tests := []struct {
		name      string
		key       trie.Key
		n         uint8
		want      trie.Key
		expectErr bool
	}{
		{
			name:      "Valid case",
			key:       trie.NewKey(8, []byte{0b11110000}),
			n:         4,
			want:      trie.NewKey(4, []byte{0b00001111}),
			expectErr: false,
		},
		{
			name:      "Request more bits than available",
			key:       trie.NewKey(8, []byte{0b11110000}),
			n:         10,
			want:      trie.Key{},
			expectErr: true,
		},
		{
			name:      "Zero bits requested",
			key:       trie.NewKey(8, []byte{0b11110000}),
			n:         0,
			want:      trie.NewKey(0, []byte{}),
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.key.MostSignificantBits(tt.n)
			if (err != nil) != tt.expectErr {
				t.Errorf("MostSignificantBits() error = %v, expectErr %v", err, tt.expectErr)
				return
			}
			if !tt.expectErr && !got.Equal(&tt.want) {
				t.Errorf("MostSignificantBits() = %v, want %v", got, tt.want)
			}
		})
	}
}
