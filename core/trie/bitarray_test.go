package trie

import (
	"bytes"
	"encoding/binary"
	"math/bits"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	ones63 = 0x7FFFFFFFFFFFFFFF
)

func TestBytes(t *testing.T) {
	tests := []struct {
		name string
		ba   BitArray
		want [32]byte
	}{
		{
			name: "length == 0",
			ba:   BitArray{len: 0, words: maxBits},
			want: [32]byte{},
		},
		{
			name: "length < 64",
			ba:   BitArray{len: 38, words: maxBits},
			want: func() [32]byte {
				var b [32]byte
				binary.BigEndian.PutUint64(b[24:32], 0x3FFFFFFFFF)
				return b
			}(),
		},
		{
			name: "64 <= length < 128",
			ba:   BitArray{len: 100, words: maxBits},
			want: func() [32]byte {
				var b [32]byte
				binary.BigEndian.PutUint64(b[16:24], 0xFFFFFFFFF)
				binary.BigEndian.PutUint64(b[24:32], maxUint64)
				return b
			}(),
		},
		{
			name: "128 <= length < 192",
			ba:   BitArray{len: 130, words: maxBits},
			want: func() [32]byte {
				var b [32]byte
				binary.BigEndian.PutUint64(b[8:16], 0x3)
				binary.BigEndian.PutUint64(b[16:24], maxUint64)
				binary.BigEndian.PutUint64(b[24:32], maxUint64)
				return b
			}(),
		},
		{
			name: "192 <= length < 255",
			ba:   BitArray{len: 201, words: maxBits},
			want: func() [32]byte {
				var b [32]byte
				binary.BigEndian.PutUint64(b[0:8], 0x1FF)
				binary.BigEndian.PutUint64(b[8:16], maxUint64)
				binary.BigEndian.PutUint64(b[16:24], maxUint64)
				binary.BigEndian.PutUint64(b[24:32], maxUint64)
				return b
			}(),
		},
		{
			name: "length == 254",
			ba:   BitArray{len: 254, words: maxBits},
			want: func() [32]byte {
				var b [32]byte
				binary.BigEndian.PutUint64(b[0:8], 0x3FFFFFFFFFFFFFFF)
				binary.BigEndian.PutUint64(b[8:16], maxUint64)
				binary.BigEndian.PutUint64(b[16:24], maxUint64)
				binary.BigEndian.PutUint64(b[24:32], maxUint64)
				return b
			}(),
		},
		{
			name: "length == 255",
			ba:   BitArray{len: 255, words: maxBits},
			want: func() [32]byte {
				var b [32]byte
				binary.BigEndian.PutUint64(b[0:8], ones63)
				binary.BigEndian.PutUint64(b[8:16], maxUint64)
				binary.BigEndian.PutUint64(b[16:24], maxUint64)
				binary.BigEndian.PutUint64(b[24:32], maxUint64)
				return b
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ba.Bytes()
			if !bytes.Equal(got, tt.want[:]) {
				t.Errorf("BitArray.Bytes() = %v, want %v", got, tt.want)
			}

			// check if the received bytes has the same bit count as the BitArray.len
			count := 0
			for _, b := range got {
				count += bits.OnesCount8(b)
			}
			if count != int(tt.ba.len) {
				t.Errorf("BitArray.Bytes() bit count = %v, want %v", count, tt.ba.len)
			}
		})
	}
}

func TestRsh(t *testing.T) {
	tests := []struct {
		name     string
		initial  *BitArray
		shiftBy  uint8
		expected *BitArray
	}{
		{
			name: "zero length array",
			initial: &BitArray{
				len:   0,
				words: [4]uint64{0, 0, 0, 0},
			},
			shiftBy: 5,
			expected: &BitArray{
				len:   0,
				words: [4]uint64{0, 0, 0, 0},
			},
		},
		{
			name: "shift by 0",
			initial: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			shiftBy: 0,
			expected: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
		},
		{
			name: "shift by more than length",
			initial: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, maxUint64, 0, 0},
			},
			shiftBy: 65,
			expected: &BitArray{
				len:   0,
				words: [4]uint64{0, 0, 0, 0},
			},
		},
		{
			name: "shift by less than 64",
			initial: &BitArray{
				len:   128,
				words: [4]uint64{maxUint64, maxUint64, 0, 0},
			},
			shiftBy: 32,
			expected: &BitArray{
				len:   96,
				words: [4]uint64{maxUint64, 0x00000000FFFFFFFF, 0, 0},
			},
		},
		{
			name: "shift by exactly 64",
			initial: &BitArray{
				len:   128,
				words: [4]uint64{maxUint64, maxUint64, 0, 0},
			},
			shiftBy: 64,
			expected: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
		},
		{
			name: "shift by 127",
			initial: &BitArray{
				len:   255,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, ones63},
			},
			shiftBy: 127,
			expected: &BitArray{
				len:   128,
				words: [4]uint64{maxUint64, maxUint64, 0, 0},
			},
		},
		{
			name: "shift by 128",
			initial: &BitArray{
				len:   251,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, maxUint64},
			},
			shiftBy: 128,
			expected: &BitArray{
				len:   123,
				words: [4]uint64{maxUint64, maxUint64, 0, 0},
			},
		},
		{
			name: "shift by 192",
			initial: &BitArray{
				len:   251,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, maxUint64},
			},
			shiftBy: 192,
			expected: &BitArray{
				len:   59,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := new(BitArray).Rsh(tt.initial, tt.shiftBy)
			if !result.Equal(tt.expected) {
				t.Errorf("Rsh() got = %+v, want %+v", result, tt.expected)
			}
		})
	}
}

func TestPrefixEqual(t *testing.T) {
	tests := []struct {
		name string
		a    *BitArray
		b    *BitArray
		want bool
	}{
		{
			name: "equal lengths, equal values",
			a: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			b: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			want: true,
		},
		{
			name: "equal lengths, different values",
			a: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			b: &BitArray{
				len:   64,
				words: [4]uint64{0xFFFFFFFFFFFFFFF0, 0, 0, 0},
			},
			want: false,
		},
		{
			name: "different lengths, a longer but same prefix",
			a: &BitArray{
				len:   128,
				words: [4]uint64{maxUint64, maxUint64, 0, 0},
			},
			b: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			want: true,
		},
		{
			name: "different lengths, b longer but same prefix",
			a: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			b: &BitArray{
				len:   128,
				words: [4]uint64{maxUint64, maxUint64, 0, 0},
			},
			want: true,
		},
		{
			name: "different lengths, different prefix",
			a: &BitArray{
				len:   128,
				words: [4]uint64{maxUint64, maxUint64, 0, 0},
			},
			b: &BitArray{
				len:   64,
				words: [4]uint64{0xFFFFFFFFFFFFFFF0, 0, 0, 0},
			},
			want: false,
		},
		{
			name: "zero length arrays",
			a: &BitArray{
				len:   0,
				words: [4]uint64{0, 0, 0, 0},
			},
			b: &BitArray{
				len:   0,
				words: [4]uint64{0, 0, 0, 0},
			},
			want: true,
		},
		{
			name: "one zero length array",
			a: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			b: &BitArray{
				len:   0,
				words: [4]uint64{0, 0, 0, 0},
			},
			want: true,
		},
		{
			name: "max length difference",
			a: &BitArray{
				len:   251,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, 0x7FFFFFFFFFFFFFF},
			},
			b: &BitArray{
				len:   1,
				words: [4]uint64{0x1, 0, 0, 0},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.EqualMSBs(tt.b); got != tt.want {
				t.Errorf("PrefixEqual() = %v, want %v", got, tt.want)
			}
			// Test symmetry: a.PrefixEqual(b) should equal b.PrefixEqual(a)
			if got := tt.b.EqualMSBs(tt.a); got != tt.want {
				t.Errorf("PrefixEqual() symmetric test = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLSBs(t *testing.T) {
	tests := []struct {
		name     string
		initial  BitArray
		length   uint8
		expected BitArray
	}{
		{
			name: "truncate to zero",
			initial: BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			length: 0,
			expected: BitArray{
				len:   0,
				words: [4]uint64{0, 0, 0, 0},
			},
		},
		{
			name: "truncate within first word - 32 bits",
			initial: BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			length: 32,
			expected: BitArray{
				len:   32,
				words: [4]uint64{0x00000000FFFFFFFF, 0, 0, 0},
			},
		},
		{
			name: "truncate to single bit",
			initial: BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			length: 1,
			expected: BitArray{
				len:   1,
				words: [4]uint64{0x0000000000000001, 0, 0, 0},
			},
		},
		{
			name: "truncate across words - 100 bits",
			initial: BitArray{
				len:   128,
				words: [4]uint64{maxUint64, maxUint64, 0, 0},
			},
			length: 100,
			expected: BitArray{
				len:   100,
				words: [4]uint64{maxUint64, 0x0000000FFFFFFFFF, 0, 0},
			},
		},
		{
			name: "truncate at word boundary - 64 bits",
			initial: BitArray{
				len:   128,
				words: [4]uint64{maxUint64, maxUint64, 0, 0},
			},
			length: 64,
			expected: BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
		},
		{
			name: "truncate at word boundary - 128 bits",
			initial: BitArray{
				len:   192,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, 0},
			},
			length: 128,
			expected: BitArray{
				len:   128,
				words: [4]uint64{maxUint64, maxUint64, 0, 0},
			},
		},
		{
			name: "truncate in third word - 150 bits",
			initial: BitArray{
				len:   192,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, 0},
			},
			length: 150,
			expected: BitArray{
				len:   150,
				words: [4]uint64{maxUint64, maxUint64, 0x3FFFFF, 0},
			},
		},
		{
			name: "truncate in fourth word - 220 bits",
			initial: BitArray{
				len:   255,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, maxUint64},
			},
			length: 220,
			expected: BitArray{
				len:   220,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, 0xFFFFFFF},
			},
		},
		{
			name: "truncate max length - 251 bits",
			initial: BitArray{
				len:   255,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, maxUint64},
			},
			length: 251,
			expected: BitArray{
				len:   251,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, 0x7FFFFFFFFFFFFFF},
			},
		},
		{
			name: "truncate sparse bits",
			initial: BitArray{
				len:   128,
				words: [4]uint64{0xAAAAAAAAAAAAAAAA, 0x5555555555555555, 0, 0},
			},
			length: 100,
			expected: BitArray{
				len:   100,
				words: [4]uint64{0xAAAAAAAAAAAAAAAA, 0x0000000555555555, 0, 0},
			},
		},
		{
			name: "no change when new length equals current length",
			initial: BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			length: 64,
			expected: BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
		},
		{
			name: "no change when new length greater than current length",
			initial: BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			length: 128,
			expected: BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := new(BitArray).LSBs(&tt.initial, tt.length)
			if !result.Equal(&tt.expected) {
				t.Errorf("Truncate() got = %+v, want %+v", result, tt.expected)
			}
		})
	}
}

func TestMSBs(t *testing.T) {
	tests := []struct {
		name string
		x    *BitArray
		n    uint8
		want *BitArray
	}{
		{
			name: "empty array",
			x:    emptyBitArray,
			n:    0,
			want: emptyBitArray,
		},
		{
			name: "get all bits",
			x: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			n: 64,
			want: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
		},
		{
			name: "get more bits than available",
			x: &BitArray{
				len:   32,
				words: [4]uint64{0xFFFFFFFF, 0, 0, 0},
			},
			n: 64,
			want: &BitArray{
				len:   32,
				words: [4]uint64{0xFFFFFFFF, 0, 0, 0},
			},
		},
		{
			name: "get half of available bits",
			x: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			n: 32,
			want: &BitArray{
				len:   32,
				words: [4]uint64{0xFFFFFFFF00000000 >> 32, 0, 0, 0},
			},
		},
		{
			name: "get MSBs across word boundary",
			x: &BitArray{
				len:   128,
				words: [4]uint64{maxUint64, maxUint64, 0, 0},
			},
			n: 100,
			want: &BitArray{
				len:   100,
				words: [4]uint64{maxUint64, maxUint64 >> 28, 0, 0},
			},
		},
		{
			name: "get MSBs from max length array",
			x: &BitArray{
				len:   255,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, ones63},
			},
			n: 64,
			want: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
		},
		{
			name: "get zero bits",
			x: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			n: 0,
			want: &BitArray{
				len:   0,
				words: [4]uint64{0, 0, 0, 0},
			},
		},
		{
			name: "sparse bits",
			x: &BitArray{
				len:   128,
				words: [4]uint64{0xAAAAAAAAAAAAAAAA, 0x5555555555555555, 0, 0},
			},
			n: 64,
			want: &BitArray{
				len:   64,
				words: [4]uint64{0x5555555555555555, 0, 0, 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := new(BitArray).MSBs(tt.x, tt.n)
			if !got.Equal(tt.want) {
				t.Errorf("MSBs() = %v, want %v", got, tt.want)
			}

			if got.len != tt.want.len {
				t.Errorf("MSBs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWriteAndUnmarshalBinary(t *testing.T) {
	tests := []struct {
		name string
		ba   BitArray
		want []byte // Expected bytes after writing
	}{
		{
			name: "empty bit array",
			ba: BitArray{
				len:   0,
				words: [4]uint64{0, 0, 0, 0},
			},
			want: []byte{0}, // Just the length byte
		},
		{
			name: "8 bits",
			ba: BitArray{
				len:   8,
				words: [4]uint64{0xFF, 0, 0, 0},
			},
			want: []byte{8, 0xFF}, // length byte + 1 data byte
		},
		{
			name: "10 bits requiring 2 bytes",
			ba: BitArray{
				len:   10,
				words: [4]uint64{0x3FF, 0, 0, 0}, // 1111111111 in binary
			},
			want: []byte{10, 0x3, 0xFF}, // length byte + 2 data bytes
		},
		{
			name: "64 bits",
			ba: BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			want: append(
				[]byte{64}, // length byte
				[]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}..., // 8 data bytes
			),
		},
		{
			name: "251 bits",
			ba: BitArray{
				len:   251,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, 0x7FFFFFFFFFFFFFF},
			},
			want: func() []byte {
				b := make([]byte, 33) // 1 length byte + 32 data bytes
				b[0] = 251            // length byte
				// First byte is 0x07 (from the most significant bits)
				b[1] = 0x07
				// Rest of the bytes are 0xFF
				for i := 2; i < 33; i++ {
					b[i] = 0xFF
				}
				return b
			}(),
		},
		{
			name: "sparse bits",
			ba: BitArray{
				len:   16,
				words: [4]uint64{0xAAAA, 0, 0, 0}, // 1010101010101010 in binary
			},
			want: []byte{16, 0xAA, 0xAA}, // length byte + 2 data bytes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			gotN, err := tt.ba.Write(buf)
			assert.NoError(t, err)

			// Check number of bytes written
			if gotN != len(tt.want) {
				t.Errorf("Write() wrote %d bytes, want %d", gotN, len(tt.want))
			}

			// Check written bytes
			got := buf.Bytes()
			if !bytes.Equal(got, tt.want) {
				t.Errorf("Write() = %v, want %v", got, tt.want)
			}

			var gotBitArray BitArray
			gotBitArray.UnmarshalBinary(got)
			if !gotBitArray.Equal(&tt.ba) {
				t.Errorf("UnmarshalBinary() = %v, want %v", gotBitArray, tt.ba)
			}
		})
	}
}

func TestCommonPrefix(t *testing.T) {
	tests := []struct {
		name string
		x    *BitArray
		y    *BitArray
		want *BitArray
	}{
		{
			name: "empty arrays",
			x:    emptyBitArray,
			y:    emptyBitArray,
			want: emptyBitArray,
		},
		{
			name: "one empty array",
			x: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			y:    emptyBitArray,
			want: emptyBitArray,
		},
		{
			name: "identical arrays - single word",
			x: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			y: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			want: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
		},
		{
			name: "identical arrays - multiple words",
			x: &BitArray{
				len:   192,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, 0},
			},
			y: &BitArray{
				len:   192,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, 0},
			},
			want: &BitArray{
				len:   192,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, 0},
			},
		},
		{
			name: "different lengths with common prefix - first word",
			x: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			y: &BitArray{
				len:   32,
				words: [4]uint64{0xFFFFFFFF, 0, 0, 0},
			},
			want: &BitArray{
				len:   32,
				words: [4]uint64{0xFFFFFFFF, 0, 0, 0},
			},
		},
		{
			name: "different lengths with common prefix - multiple words",
			x: &BitArray{
				len:   255,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, ones63},
			},
			y: &BitArray{
				len:   127,
				words: [4]uint64{maxUint64, ones63, 0, 0},
			},
			want: &BitArray{
				len:   127,
				words: [4]uint64{maxUint64, ones63, 0, 0},
			},
		},
		{
			name: "different at first bit",
			x: &BitArray{
				len:   64,
				words: [4]uint64{ones63, 0, 0, 0},
			},
			y: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			want: &BitArray{
				len:   0,
				words: [4]uint64{0, 0, 0, 0},
			},
		},
		{
			name: "different in middle of first word",
			x: &BitArray{
				len:   64,
				words: [4]uint64{0xFFFFFFFF0FFFFFFF, 0, 0, 0},
			},
			y: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			want: &BitArray{
				len:   32,
				words: [4]uint64{0xFFFFFFFF, 0, 0, 0},
			},
		},
		{
			name: "different in second word",
			x: &BitArray{
				len:   128,
				words: [4]uint64{maxUint64, 0xFFFFFFFF0FFFFFFF, 0, 0},
			},
			y: &BitArray{
				len:   128,
				words: [4]uint64{maxUint64, maxUint64, 0, 0},
			},
			want: &BitArray{
				len:   32,
				words: [4]uint64{0xFFFFFFFF, 0, 0, 0},
			},
		},
		{
			name: "different in third word",
			x: &BitArray{
				len:   192,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, 0},
			},
			y: &BitArray{
				len:   192,
				words: [4]uint64{0, 0, 0xFFFFFFFFFFFFFF0F, 0},
			},
			want: &BitArray{
				len:   56,
				words: [4]uint64{0xFFFFFFFFFFFFFF, 0, 0, 0},
			},
		},
		{
			name: "different in last word",
			x: &BitArray{
				len:   251,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, 0x7FFFFFFFFFFFFFF},
			},
			y: &BitArray{
				len:   251,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, 0x7FFFFFF0FFFFFFF},
			},
			want: &BitArray{
				len:   27,
				words: [4]uint64{0x7FFFFFF},
			},
		},
		{
			name: "sparse bits with common prefix",
			x: &BitArray{
				len:   128,
				words: [4]uint64{0xAAAAAAAAAAAAAAAA, 0xAAAAAAAAAAAAAAAA, 0, 0},
			},
			y: &BitArray{
				len:   128,
				words: [4]uint64{0xAAAAAAAAAAAAAAAA, 0xAAAAAAAAAAAAA000, 0, 0},
			},
			want: &BitArray{
				len:   52,
				words: [4]uint64{0xAAAAAAAAAAAAA, 0, 0, 0},
			},
		},
		{
			name: "max length difference",
			x: &BitArray{
				len:   255,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, ones63},
			},
			y: &BitArray{
				len:   1,
				words: [4]uint64{0x1, 0, 0, 0},
			},
			want: &BitArray{
				len:   1,
				words: [4]uint64{0x1, 0, 0, 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := new(BitArray)
			gotSymmetric := new(BitArray)

			got.CommonMSBs(tt.x, tt.y)
			if !got.Equal(tt.want) {
				t.Errorf("CommonMSBs() = %v, want %v", got, tt.want)
			}

			// Test symmetry: x.CommonMSBs(y) should equal y.CommonMSBs(x)
			gotSymmetric.CommonMSBs(tt.y, tt.x)
			if !gotSymmetric.Equal(tt.want) {
				t.Errorf("CommonMSBs() symmetric test = %v, want %v", gotSymmetric, tt.want)
			}
		})
	}
}

func TestIsBitSet(t *testing.T) {
	tests := []struct {
		name string
		ba   BitArray
		pos  uint8
		want bool
	}{
		{
			name: "empty array",
			ba: BitArray{
				len:   0,
				words: [4]uint64{0, 0, 0, 0},
			},
			pos:  0,
			want: false,
		},
		{
			name: "first bit set",
			ba: BitArray{
				len:   64,
				words: [4]uint64{1, 0, 0, 0},
			},
			pos:  0,
			want: true,
		},
		{
			name: "last bit in first word",
			ba: BitArray{
				len:   64,
				words: [4]uint64{1 << 63, 0, 0, 0},
			},
			pos:  63,
			want: true,
		},
		{
			name: "first bit in second word",
			ba: BitArray{
				len:   128,
				words: [4]uint64{0, 1, 0, 0},
			},
			pos:  64,
			want: true,
		},
		{
			name: "bit beyond length",
			ba: BitArray{
				len:   64,
				words: [4]uint64{maxUint64, maxUint64, 0, 0},
			},
			pos:  65,
			want: false,
		},
		{
			name: "alternating bits",
			ba: BitArray{
				len:   8,
				words: [4]uint64{0xAA, 0, 0, 0}, // 10101010 in binary
			},
			pos:  1,
			want: true,
		},
		{
			name: "alternating bits - unset position",
			ba: BitArray{
				len:   8,
				words: [4]uint64{0xAA, 0, 0, 0}, // 10101010 in binary
			},
			pos:  0,
			want: false,
		},
		{
			name: "bit in last word",
			ba: BitArray{
				len:   251,
				words: [4]uint64{0, 0, 0, 1 << 59},
			},
			pos:  251,
			want: false, // position 251 is beyond the highest valid bit (250)
		},
		{
			name: "highest valid bit (255)",
			ba: BitArray{
				len:   255,
				words: [4]uint64{0, 0, 0, 1 << 62}, // bit 255 set
			},
			pos:  254,
			want: true,
		},
		{
			name: "position at length boundary",
			ba: BitArray{
				len:   100,
				words: [4]uint64{maxUint64, maxUint64, 0, 0},
			},
			pos:  100,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ba.IsBitSet(tt.pos)
			if got != tt.want {
				t.Errorf("IsBitSet(%d) = %v, want %v", tt.pos, got, tt.want)
			}
		})
	}
}

func TestFeltConversion(t *testing.T) {
	tests := []struct {
		name   string
		ba     BitArray
		length uint8
		want   string // hex representation of felt
	}{
		{
			name: "empty bit array",
			ba: BitArray{
				len:   0,
				words: [4]uint64{0, 0, 0, 0},
			},
			length: 0,
			want:   "0x0",
		},
		{
			name: "single word",
			ba: BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			length: 64,
			want:   "0xffffffffffffffff",
		},
		{
			name: "two words",
			ba: BitArray{
				len:   128,
				words: [4]uint64{maxUint64, maxUint64, 0, 0},
			},
			length: 128,
			want:   "0xffffffffffffffffffffffffffffffff",
		},
		{
			name: "three words",
			ba: BitArray{
				len:   192,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, 0},
			},
			length: 192,
			want:   "0xffffffffffffffffffffffffffffffffffffffffffffffff",
		},
		{
			name: "251 bits",
			ba: BitArray{
				len:   251,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, 0x7FFFFFFFFFFFFFF},
			},
			length: 251,
			want:   "0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		},
		{
			name: "sparse bits",
			ba: BitArray{
				len:   128,
				words: [4]uint64{0xAAAAAAAAAAAAAAAA, 0x5555555555555555, 0, 0},
			},
			length: 128,
			want:   "0x5555555555555555aaaaaaaaaaaaaaaa",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Felt() conversion
			gotFelt := tt.ba.Felt()
			assert.Equal(t, tt.want, gotFelt.String())

			// Test SetFelt() conversion (round trip)
			var newBA BitArray
			newBA.SetFelt(tt.length, &gotFelt)
			assert.Equal(t, tt.ba.len, newBA.len)
			assert.Equal(t, tt.ba.words, newBA.words)
		})
	}
}

func TestSetFeltValidation(t *testing.T) {
	tests := []struct {
		name        string
		feltStr     string
		length      uint8
		shouldMatch bool
	}{
		{
			name:        "valid felt with matching length",
			feltStr:     "0xf",
			length:      4,
			shouldMatch: true,
		},
		{
			name:        "felt larger than specified length",
			feltStr:     "0xff",
			length:      4,
			shouldMatch: false,
		},
		{
			name:        "zero felt with non-zero length",
			feltStr:     "0x0",
			length:      8,
			shouldMatch: true,
		},
		{
			name:        "max felt with max length",
			feltStr:     "0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
			length:      251,
			shouldMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var f felt.Felt
			_, err := f.SetString(tt.feltStr)
			require.NoError(t, err)

			var ba BitArray
			ba.SetFelt(tt.length, &f)

			// Convert back to felt and compare
			roundTrip := ba.Felt()
			if tt.shouldMatch {
				assert.True(t, roundTrip.Equal(&f),
					"expected %s, got %s", f.String(), roundTrip.String())
			} else {
				assert.False(t, roundTrip.Equal(&f),
					"values should not match: original %s, roundtrip %s",
					f.String(), roundTrip.String())
			}
		})
	}
}
