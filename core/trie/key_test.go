package trie_test

import (
	"bytes"
	"errors"
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

func TestKeyTest(t *testing.T) {
	key := trie.NewKey(44, []byte{0x10, 0x02})
	for i := 0; i < int(key.Len()); i++ {
		assert.Equal(t, i == 1 || i == 12, key.IsBitSet(uint8(i)), i)
	}
}

func TestDeleteLSB(t *testing.T) {
	key := trie.NewKey(16, []byte{0xF3, 0x04})

	tests := map[string]struct {
		shiftAmount uint8
		expectedKey trie.Key
	}{
		"delete 0 bits": {
			shiftAmount: 0,
			expectedKey: key,
		},
		"delete 4 bits": {
			shiftAmount: 4,
			expectedKey: trie.NewKey(12, []byte{0x0F, 0x30}),
		},
		"delete 8 bits": {
			shiftAmount: 8,
			expectedKey: trie.NewKey(8, []byte{0xF3}),
		},
		"delete 9 bits": {
			shiftAmount: 9,
			expectedKey: trie.NewKey(7, []byte{0x79}),
		},
	}

	for desc, test := range tests {
		t.Run(desc, func(t *testing.T) {
			copyKey := key
			copyKey.DeleteLSB(test.shiftAmount)
			assert.Equal(t, test.expectedKey, copyKey)
		})
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

func TestKeyErrorHandling(t *testing.T) {
	t.Run("passed too long key bytes panics", func(t *testing.T) {
		defer func() {
			r := recover()
			require.NotNil(t, r)
			require.Contains(t, r.(string), "bytes does not fit in bitset")
		}()
		tooLongKeyB := make([]byte, 33)
		trie.NewKey(8, tooLongKeyB)
	})
	t.Run("MostSignificantBits n greater than key length", func(t *testing.T) {
		key := trie.NewKey(8, []byte{0x01})
		_, err := key.MostSignificantBits(9)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot take 9 bits from key of length 8")
	})
	t.Run("MostSignificantBits equals key length return copy of key", func(t *testing.T) {
		key := trie.NewKey(8, []byte{0x01})
		kCopy, err := key.MostSignificantBits(8)
		require.NoError(t, err)
		require.Equal(t, key, *kCopy)
	})
	t.Run("SubKey n greater than key length", func(t *testing.T) {
		key := trie.NewKey(8, []byte{0x01})
		_, err := key.SubKey(9)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot subtract key of length 9 from key of length 8")
	})
	t.Run("SubKey n equals k length returns empty key", func(t *testing.T) {
		key := trie.NewKey(8, []byte{0x01})
		kCopy, err := key.SubKey(8)
		require.NoError(t, err)
		require.Equal(t, trie.Key{}, *kCopy)
	})
	t.Run("delete more bits than key length panics", func(t *testing.T) {
		defer func() {
			r := recover()
			require.NotNil(t, r)
			require.Contains(t, r.(string), "deleting more bits than there are")
		}()
		key := trie.NewKey(8, []byte{0x01})
		key.DeleteLSB(9)
	})
	t.Run("WriteTo returns error", func(t *testing.T) {
		key := trie.NewKey(8, []byte{0x01})
		wrote, err := key.WriteTo(&errorBuffer{})
		require.Error(t, err)
		require.Equal(t, int64(0), wrote)
	})
}

type errorBuffer struct{}

func (*errorBuffer) Write([]byte) (int, error) {
	return 0, errors.New("expected to fail")
}
