package trie_test

import (
	"bytes"
	"fmt"
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
		assert.Equal(t, i == 1 || i == 12, key.Test(uint8(i)), i)
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

func Test_cmp(t *testing.T) {
	tests := []struct {
		n1       int
		n2       int
		isHigher bool
	}{
		{
			n1:       10,
			n2:       0,
			isHigher: true,
		},
		{
			n1:       5,
			n2:       0,
			isHigher: true,
		},
		{
			n1:       5,
			n2:       4,
			isHigher: true,
		},
		{
			n1:       5,
			n2:       5,
			isHigher: false,
		},
		{
			n1:       4,
			n2:       5,
			isHigher: false,
		},
		{
			n1:       0,
			n2:       5,
			isHigher: false,
		},
		{
			n1:       300,
			n2:       1,
			isHigher: true,
		},
		{
			n1:       1,
			n2:       300,
			isHigher: false,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%d %d %v", test.n1, test.n2, test.isHigher), func(t *testing.T) {
			k1 := numToKey(test.n1)
			k2 := numToKey(test.n2)

			assert.Equal(t,
				k1.CmpAligned(&k2) > 0,
				test.isHigher)
		})
	}
}

func numToKey(num int) trie.Key {
	return trie.NewKey(8, []byte{byte(num)})
}

func TestKeyAppend(t *testing.T) {
	tests := map[string]struct {
		Key1        trie.Key
		Key2        trie.Key
		ExpectedKey trie.Key
	}{
		"no append": {
			Key1:        trie.NewKey(1, []byte{0x01}),
			Key2:        trie.NewKey(0, []byte{0x00}),
			ExpectedKey: trie.NewKey(1, []byte{0x01}),
		},
		"from zero append": {
			Key1:        trie.NewKey(0, []byte{0x00}),
			Key2:        trie.NewKey(1, []byte{0x01}),
			ExpectedKey: trie.NewKey(1, []byte{0x01}),
		},
		"append shift": {
			Key1:        trie.NewKey(1, []byte{0x01}),
			Key2:        trie.NewKey(7, []byte{0x00}),
			ExpectedKey: trie.NewKey(8, []byte{0x80}),
		},
		"append to a new byte": {
			Key1:        trie.NewKey(8, []byte{0xff}),
			Key2:        trie.NewKey(1, []byte{0x01}),
			ExpectedKey: trie.NewKey(9, []byte{0x01, 0xff}),
		},
		"append multi byte": {
			Key1:        trie.NewKey(11, []byte{0x00, 0xff}),       //  000 1111 1111
			Key2:        trie.NewKey(12, []byte{0x00, 0xff}),       // 0000 1111 1111
			ExpectedKey: trie.NewKey(23, []byte{0x0f, 0xf0, 0xff}), //  000 1111 1111 0000 1111 1111
		},
	}

	for desc, test := range tests {
		t.Run(desc, func(t *testing.T) {
			appended := test.Key1.Append(&test.Key2)
			assert.Equal(t, test.ExpectedKey, appended)
		})
	}
}

func TestKeyAppendBit(t *testing.T) {
	k1 := trie.NewKey(1, []byte{0x01})
	k2 := k1.AppendBit(true)
	expected := trie.NewKey(2, []byte{0x03})
	assert.Equal(t, k2, expected)

	k1 = trie.NewKey(1, []byte{0x00})
	k2 = k1.AppendBit(true)
	expected = trie.NewKey(2, []byte{0x01})
	assert.Equal(t, k2, expected)
}
