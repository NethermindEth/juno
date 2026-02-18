package trieutils

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
	ones63 = 0x7FFFFFFFFFFFFFFF // 63 bits of 1
)

func TestBytes(t *testing.T) {
	tests := []struct {
		name string
		ba   BitArray
		want [32]byte
	}{
		{
			name: "length == 0",
			ba:   BitArray{len: 0, words: [4]uint64{0, 0, 0, 0}},
			want: [32]byte{},
		},
		{
			name: "length < 64",
			ba:   BitArray{len: 38, words: [4]uint64{0x3FFFFFFFFF, 0, 0, 0}},
			want: func() [32]byte {
				var b [32]byte
				binary.BigEndian.PutUint64(b[24:32], 0x3FFFFFFFFF)
				return b
			}(),
		},
		{
			name: "64 <= length < 128",
			ba:   BitArray{len: 100, words: [4]uint64{maxUint64, 0xFFFFFFFFF, 0, 0}},
			want: func() [32]byte {
				var b [32]byte
				binary.BigEndian.PutUint64(b[16:24], 0xFFFFFFFFF)
				binary.BigEndian.PutUint64(b[24:32], maxUint64)
				return b
			}(),
		},
		{
			name: "128 <= length < 192",
			ba:   BitArray{len: 130, words: [4]uint64{maxUint64, maxUint64, 0x3, 0}},
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
			ba:   BitArray{len: 201, words: [4]uint64{maxUint64, maxUint64, maxUint64, 0x1FF}},
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
			ba:   BitArray{len: 254, words: [4]uint64{maxUint64, maxUint64, maxUint64, 0x3FFFFFFFFFFFFFFF}},
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
			ba:   BitArray{len: 255, words: [4]uint64{maxUint64, maxUint64, maxUint64, ones63}},
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
			if !bytes.Equal(got[:], tt.want[:]) {
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
				words: [4]uint64{maxUint64, 0x7FFFFFFFFFFFFFF, 0, 0},
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
				words: [4]uint64{0x7FFFFFFFFFFFFFF, 0, 0, 0},
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

func TestLsh(t *testing.T) {
	tests := []struct {
		name string
		x    *BitArray
		n    uint8
		want *BitArray
	}{
		{
			name: "empty array",
			x:    emptyBitArray,
			n:    5,
			want: emptyBitArray,
		},
		{
			name: "shift by 0",
			x: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			n: 0,
			want: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
		},
		{
			name: "shift within first word",
			x: &BitArray{
				len:   4,
				words: [4]uint64{0xF, 0, 0, 0}, // 1111
			},
			n: 4,
			want: &BitArray{
				len:   8,
				words: [4]uint64{0xF0, 0, 0, 0}, // 11110000
			},
		},
		{
			name: "shift across word boundary",
			x: &BitArray{
				len:   4,
				words: [4]uint64{0xF, 0, 0, 0}, // 1111
			},
			n: 62,
			want: &BitArray{
				len:   66,
				words: [4]uint64{0xC000000000000000, 0x3, 0, 0},
			},
		},
		{
			name: "shift by 64 (full word)",
			x: &BitArray{
				len:   8,
				words: [4]uint64{0xFF, 0, 0, 0}, // 11111111
			},
			n: 64,
			want: &BitArray{
				len:   72,
				words: [4]uint64{0, 0xFF, 0, 0},
			},
		},
		{
			name: "shift by 128",
			x: &BitArray{
				len:   8,
				words: [4]uint64{0xFF, 0, 0, 0}, // 11111111
			},
			n: 128,
			want: &BitArray{
				len:   136,
				words: [4]uint64{0, 0, 0xFF, 0},
			},
		},
		{
			name: "shift by 192",
			x: &BitArray{
				len:   8,
				words: [4]uint64{0xFF, 0, 0, 0}, // 11111111
			},
			n: 192,
			want: &BitArray{
				len:   200,
				words: [4]uint64{0, 0, 0, 0xFF},
			},
		},
		{
			name: "shift causing length overflow",
			x: &BitArray{
				len:   200,
				words: [4]uint64{0xFF, 0, 0, 0},
			},
			n: 60,
			want: &BitArray{
				len: 255, // capped at maxUint8
				words: [4]uint64{
					0xF000000000000000,
					0xF,
					0,
					0,
				},
			},
		},
		{
			name: "shift sparse bits",
			x: &BitArray{
				len:   8,
				words: [4]uint64{0xAA, 0, 0, 0}, // 10101010
			},
			n: 4,
			want: &BitArray{
				len:   12,
				words: [4]uint64{0xAA0, 0, 0, 0}, // 101010100000
			},
		},
		{
			name: "shift partial word across boundary",
			x: &BitArray{
				len:   100,
				words: [4]uint64{0xFF, 0xFF, 0, 0},
			},
			n: 60,
			want: &BitArray{
				len: 160,
				words: [4]uint64{
					0xF000000000000000,
					0xF00000000000000F,
					0xF,
					0,
				},
			},
		},
		{
			name: "near maximum length shift",
			x: &BitArray{
				len:   251,
				words: [4]uint64{0xFF, 0, 0, 0},
			},
			n: 4,
			want: &BitArray{
				len:   255, // capped at maxUint8
				words: [4]uint64{0xFF0, 0, 0, 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := new(BitArray).Lsh(tt.x, tt.n)
			if !got.Equal(tt.want) {
				t.Errorf("Lsh() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAppend(t *testing.T) {
	tests := []struct {
		name string
		x    *BitArray
		y    *BitArray
		want *BitArray
	}{
		{
			name: "both empty arrays",
			x:    emptyBitArray,
			y:    emptyBitArray,
			want: emptyBitArray,
		},
		{
			name: "first array empty",
			x:    emptyBitArray,
			y: &BitArray{
				len:   4,
				words: [4]uint64{0xF, 0, 0, 0}, // 1111
			},
			want: &BitArray{
				len:   4,
				words: [4]uint64{0xF, 0, 0, 0}, // 1111
			},
		},
		{
			name: "second array empty",
			x: &BitArray{
				len:   4,
				words: [4]uint64{0xF, 0, 0, 0}, // 1111
			},
			y: emptyBitArray,
			want: &BitArray{
				len:   4,
				words: [4]uint64{0xF, 0, 0, 0}, // 1111
			},
		},
		{
			name: "within first word",
			x: &BitArray{
				len:   4,
				words: [4]uint64{0xF, 0, 0, 0}, // 1111
			},
			y: &BitArray{
				len:   4,
				words: [4]uint64{0xF, 0, 0, 0}, // 1111
			},
			want: &BitArray{
				len:   8,
				words: [4]uint64{0xFF, 0, 0, 0}, // 11111111
			},
		},
		{
			name: "different lengths within word",
			x: &BitArray{
				len:   4,
				words: [4]uint64{0xF, 0, 0, 0}, // 1111
			},
			y: &BitArray{
				len:   2,
				words: [4]uint64{0x3, 0, 0, 0}, // 11
			},
			want: &BitArray{
				len:   6,
				words: [4]uint64{0x3F, 0, 0, 0}, // 111111
			},
		},
		{
			name: "across word boundary",
			x: &BitArray{
				len:   62,
				words: [4]uint64{0x3FFFFFFFFFFFFFFF, 0, 0, 0},
			},
			y: &BitArray{
				len:   4,
				words: [4]uint64{0xF, 0, 0, 0}, // 1111
			},
			want: &BitArray{
				len:   66,
				words: [4]uint64{maxUint64, 0x3, 0, 0},
			},
		},
		{
			name: "across multiple words",
			x: &BitArray{
				len:   128,
				words: [4]uint64{maxUint64, maxUint64, 0, 0},
			},
			y: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			want: &BitArray{
				len:   192,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, 0},
			},
		},
		{
			name: "sparse bits",
			x: &BitArray{
				len:   8,
				words: [4]uint64{0xAA, 0, 0, 0}, // 10101010
			},
			y: &BitArray{
				len:   8,
				words: [4]uint64{0x55, 0, 0, 0}, // 01010101
			},
			want: &BitArray{
				len:   16,
				words: [4]uint64{0xAA55, 0, 0, 0}, // 1010101001010101
			},
		},
		{
			name: "result exactly at length limit",
			x: &BitArray{
				len:   251,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, 0x7FFFFFFFFFFFFFFF},
			},
			y: &BitArray{
				len:   4,
				words: [4]uint64{0xF, 0, 0, 0},
			},
			want: &BitArray{
				len:   255,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, 0x7FFFFFFFFFFFFFFF},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := new(BitArray).Append(tt.x, tt.y)
			if !got.Equal(tt.want) {
				t.Errorf("Append() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEqualMSBs(t *testing.T) {
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
		name string
		x    *BitArray
		pos  uint8
		want *BitArray
	}{
		{
			name: "zero position",
			x: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			pos: 0,
			want: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
		},
		{
			name: "position beyond length",
			x: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			pos: 65,
			want: &BitArray{
				len:   0,
				words: [4]uint64{0, 0, 0, 0},
			},
		},
		{
			name: "get last 4 bits",
			x: &BitArray{
				len:   8,
				words: [4]uint64{0xFF, 0, 0, 0}, // 11111111
			},
			pos: 4,
			want: &BitArray{
				len:   4,
				words: [4]uint64{0x0F, 0, 0, 0}, // 1111
			},
		},
		{
			name: "get bits across word boundary",
			x: &BitArray{
				len:   128,
				words: [4]uint64{maxUint64, maxUint64, 0, 0},
			},
			pos: 64,
			want: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
		},
		{
			name: "get bits from max length array",
			x: &BitArray{
				len:   251,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, 0x7FFFFFFFFFFFFFF},
			},
			pos: 200,
			want: &BitArray{
				len:   51,
				words: [4]uint64{0x7FFFFFFFFFFFF, 0, 0, 0},
			},
		},
		{
			name: "empty array",
			x:    emptyBitArray,
			pos:  1,
			want: &BitArray{
				len:   0,
				words: [4]uint64{0, 0, 0, 0},
			},
		},
		{
			name: "sparse bits",
			x: &BitArray{
				len:   16,
				words: [4]uint64{0xAAAA, 0, 0, 0}, // 1010101010101010
			},
			pos: 8,
			want: &BitArray{
				len:   8,
				words: [4]uint64{0xAA, 0, 0, 0}, // 10101010
			},
		},
		{
			name: "position equals length",
			x: &BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			pos: 64,
			want: &BitArray{
				len:   0,
				words: [4]uint64{0, 0, 0, 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := new(BitArray).LSBs(tt.x, tt.pos)
			if !got.Equal(tt.want) {
				t.Errorf("LSBs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLSBsFromLSB(t *testing.T) {
	tests := []struct {
		name     string
		initial  BitArray
		length   uint8
		expected BitArray
	}{
		{
			name: "zero",
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
			name: "get 32 LSBs",
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
			name: "get 1 LSB",
			initial: BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			length: 1,
			expected: BitArray{
				len:   1,
				words: [4]uint64{0x1, 0, 0, 0},
			},
		},
		{
			name: "get 100 LSBs across words",
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
			name: "get 64 LSBs at word boundary",
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
			name: "get 128 LSBs at word boundary",
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
			name: "get 150 LSBs in third word",
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
			name: "get 220 LSBs in fourth word",
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
			name: "get 251 LSBs",
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
			name: "get 100 LSBs from sparse bits",
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
			result := new(BitArray).LSBsFromLSB(&tt.initial, tt.length)
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
			want: []byte{0xFF, 0x08}, // 1 data byte + length byte
		},
		{
			name: "10 bits requiring 2 bytes",
			ba: BitArray{
				len:   10,
				words: [4]uint64{0x3FF, 0, 0, 0}, // 1111111111 in binary
			},
			want: []byte{0x3, 0xFF, 0x0A}, // 2 data bytes + length byte
		},
		{
			name: "64 bits",
			ba: BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
			want: append(
				[]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, // 8 data bytes
				[]byte{64}..., // length byte
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
				// First byte is 0x07 (from the most significant bits)
				b[0] = 0x07
				// Rest of the bytes are 0xFF
				for i := 1; i < 32; i++ {
					b[i] = 0xFF
				}
				b[32] = 251 // length byte
				return b
			}(),
		},
		{
			name: "sparse bits",
			ba: BitArray{
				len:   16,
				words: [4]uint64{0xAAAA, 0, 0, 0}, // 1010101010101010 in binary
			},
			want: []byte{0xAA, 0xAA, 0x10}, // 2 data bytes + length byte
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
			err = gotBitArray.UnmarshalBinary(got)
			require.NoError(t, err)
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

func TestIsBitSetFromLSB(t *testing.T) {
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
			name: "251 bits",
			ba: BitArray{
				len:   251,
				words: [4]uint64{0, 0, 0, 1 << 58},
			},
			pos:  250,
			want: true,
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
			got := tt.ba.IsBitSetFromLSB(tt.pos)
			if got != tt.want {
				t.Errorf("IsBitSetFromLSB(%d) = %v, want %v", tt.pos, got, tt.want)
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
			name: "first bit (MSB) set",
			ba: BitArray{
				len:   8,
				words: [4]uint64{0x80, 0, 0, 0}, // 10000000
			},
			pos:  0,
			want: true,
		},
		{
			name: "last bit (LSB) set",
			ba: BitArray{
				len:   8,
				words: [4]uint64{0x01, 0, 0, 0}, // 00000001
			},
			pos:  7,
			want: true,
		},
		{
			name: "alternating bits",
			ba: BitArray{
				len:   8,
				words: [4]uint64{0xAA, 0, 0, 0}, // 10101010
			},
			pos:  0,
			want: true,
		},
		{
			name: "alternating bits - unset position",
			ba: BitArray{
				len:   8,
				words: [4]uint64{0xAA, 0, 0, 0}, // 10101010
			},
			pos:  1,
			want: false,
		},
		{
			name: "position beyond length",
			ba: BitArray{
				len:   8,
				words: [4]uint64{0xFF, 0, 0, 0},
			},
			pos:  8,
			want: false,
		},
		{
			name: "bit in second word",
			ba: BitArray{
				len:   128,
				words: [4]uint64{0, 1, 0, 0},
			},
			pos:  63,
			want: true,
		},
		{
			name: "251 bits",
			ba: BitArray{
				len:   251,
				words: [4]uint64{0, 0, 0, 1 << 58},
			},
			pos:  0,
			want: true,
		},
		{
			name: "position at length boundary",
			ba: BitArray{
				len:   100,
				words: [4]uint64{maxUint64, maxUint64, 0, 0},
			},
			pos:  99,
			want: true,
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

func TestSetBit(t *testing.T) {
	tests := []struct {
		name string
		bit  uint8
		want BitArray
	}{
		{
			name: "set bit 0",
			bit:  0,
			want: BitArray{
				len:   1,
				words: [4]uint64{0, 0, 0, 0},
			},
		},
		{
			name: "set bit 1",
			bit:  1,
			want: BitArray{
				len:   1,
				words: [4]uint64{1, 0, 0, 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := new(BitArray).SetBit(tt.bit)
			if !got.Equal(&tt.want) {
				t.Errorf("SetBit(%v) = %v, want %v", tt.bit, got, tt.want)
			}
		})
	}
}

func TestCmp(t *testing.T) {
	tests := []struct {
		name string
		x    BitArray
		y    BitArray
		want int
	}{
		{
			name: "equal empty arrays",
			x:    BitArray{len: 0, words: [4]uint64{0, 0, 0, 0}},
			y:    BitArray{len: 0, words: [4]uint64{0, 0, 0, 0}},
			want: 0,
		},
		{
			name: "equal non-empty arrays",
			x:    BitArray{len: 64, words: [4]uint64{maxUint64, 0, 0, 0}},
			y:    BitArray{len: 64, words: [4]uint64{maxUint64, 0, 0, 0}},
			want: 0,
		},
		{
			name: "different lengths - x shorter",
			x:    BitArray{len: 32, words: [4]uint64{maxUint64, 0, 0, 0}},
			y:    BitArray{len: 64, words: [4]uint64{maxUint64, 0, 0, 0}},
			want: -1,
		},
		{
			name: "different lengths - x longer",
			x:    BitArray{len: 64, words: [4]uint64{maxUint64, 0, 0, 0}},
			y:    BitArray{len: 32, words: [4]uint64{maxUint64, 0, 0, 0}},
			want: 1,
		},
		{
			name: "same length, x < y in first word",
			x:    BitArray{len: 64, words: [4]uint64{0xFFFFFFFFFFFFFFFE, 0, 0, 0}},
			y:    BitArray{len: 64, words: [4]uint64{maxUint64, 0, 0, 0}},
			want: -1,
		},
		{
			name: "same length, x > y in first word",
			x:    BitArray{len: 64, words: [4]uint64{maxUint64, 0, 0, 0}},
			y:    BitArray{len: 64, words: [4]uint64{0xFFFFFFFFFFFFFFFE, 0, 0, 0}},
			want: 1,
		},
		{
			name: "same length, difference in last word",
			x:    BitArray{len: 251, words: [4]uint64{0, 0, 0, 0x7FFFFFFFFFFFFFF}},
			y:    BitArray{len: 251, words: [4]uint64{0, 0, 0, 0x7FFFFFFFFFFFFF0}},
			want: 1,
		},
		{
			name: "same length, sparse bits",
			x:    BitArray{len: 128, words: [4]uint64{0xAAAAAAAAAAAAAAAA, 0xAAAAAAAAAAAAAAAA, 0, 0}},
			y:    BitArray{len: 128, words: [4]uint64{0xAAAAAAAAAAAAAAAA, 0xAAAAAAAAAAAAA000, 0, 0}},
			want: 1,
		},
		{
			name: "max length difference",
			x:    BitArray{len: 255, words: [4]uint64{maxUint64, maxUint64, maxUint64, ones63}},
			y:    BitArray{len: 1, words: [4]uint64{0x1, 0, 0, 0}},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.x.Cmp(&tt.y)
			if got != tt.want {
				t.Errorf("Cmp() = %v, want %v", got, tt.want)
			}

			// Test anti-symmetry: if x.Cmp(y) = z then y.Cmp(x) = -z
			gotReverse := tt.y.Cmp(&tt.x)
			if gotReverse != -tt.want {
				t.Errorf("Reverse Cmp() = %v, want %v", gotReverse, -tt.want)
			}

			// Test transitivity with self: x.Cmp(x) should always be 0
			if tt.x.Cmp(&tt.x) != 0 {
				t.Error("Self Cmp() != 0")
			}
		})
	}
}

func TestSetBytes(t *testing.T) {
	tests := []struct {
		name   string
		length uint8
		data   []byte
		want   BitArray
	}{
		{
			name:   "empty data",
			length: 0,
			data:   []byte{},
			want: BitArray{
				len:   0,
				words: [4]uint64{0, 0, 0, 0},
			},
		},
		{
			name:   "single byte",
			length: 8,
			data:   []byte{0xFF},
			want: BitArray{
				len:   8,
				words: [4]uint64{0xFF, 0, 0, 0},
			},
		},
		{
			name:   "two bytes",
			length: 16,
			data:   []byte{0xAA, 0xFF},
			want: BitArray{
				len:   16,
				words: [4]uint64{0xAAFF, 0, 0, 0},
			},
		},
		{
			name:   "three bytes",
			length: 24,
			data:   []byte{0xAA, 0xBB, 0xCC},
			want: BitArray{
				len:   24,
				words: [4]uint64{0xAABBCC, 0, 0, 0},
			},
		},
		{
			name:   "four bytes",
			length: 32,
			data:   []byte{0xAA, 0xBB, 0xCC, 0xDD},
			want: BitArray{
				len:   32,
				words: [4]uint64{0xAABBCCDD, 0, 0, 0},
			},
		},
		{
			name:   "eight bytes (full word)",
			length: 64,
			data:   []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			want: BitArray{
				len:   64,
				words: [4]uint64{maxUint64, 0, 0, 0},
			},
		},
		{
			name:   "sixteen bytes (two words)",
			length: 128,
			data: []byte{
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
				0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA,
			},
			want: BitArray{
				len: 128,
				words: [4]uint64{
					0xAAAAAAAAAAAAAAAA,
					0xFFFFFFFFFFFFFFFF,
					0, 0,
				},
			},
		},
		{
			name:   "thirty-two bytes (full array)",
			length: 251,
			data: []byte{
				0x7F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
			},
			want: BitArray{
				len: 251,
				words: [4]uint64{
					maxUint64,
					maxUint64,
					maxUint64,
					0x7FFFFFFFFFFFFFF,
				},
			},
		},
		{
			name:   "truncate to length",
			length: 4,
			data:   []byte{0xFF},
			want: BitArray{
				len:   4,
				words: [4]uint64{0xF, 0, 0, 0},
			},
		},
		{
			name:   "data larger than 32 bytes",
			length: 251,
			data: []byte{
				0x7F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
				0xFF, 0xFF, 0xFF, 0xFF, // extra bytes should be ignored
			},
			want: BitArray{
				len: 251,
				words: [4]uint64{
					maxUint64,
					maxUint64,
					maxUint64,
					0x7FFFFFFFFFFFFFF,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := new(BitArray).SetBytes(tt.length, tt.data)
			if !got.Equal(&tt.want) {
				t.Errorf("SetBytes(%d, %v) = %v, want %v", tt.length, tt.data, got, tt.want)
			}
		})
	}
}

func TestSubset(t *testing.T) {
	tests := []struct {
		name     string
		x        *BitArray
		startPos uint8
		endPos   uint8
		want     *BitArray
	}{
		{
			name:     "empty array",
			x:        emptyBitArray,
			startPos: 0,
			endPos:   0,
			want:     emptyBitArray,
		},
		{
			name: "invalid range - start >= end",
			x: &BitArray{
				len:   8,
				words: [4]uint64{0xFF, 0, 0, 0}, // 11111111
			},
			startPos: 4,
			endPos:   2,
			want:     emptyBitArray,
		},
		{
			name: "invalid range - start >= length",
			x: &BitArray{
				len:   8,
				words: [4]uint64{0xFF, 0, 0, 0}, // 11111111
			},
			startPos: 8,
			endPos:   10,
			want:     emptyBitArray,
		},
		{
			name: "full range",
			x: &BitArray{
				len:   8,
				words: [4]uint64{0xFF, 0, 0, 0}, // 11111111
			},
			startPos: 0,
			endPos:   8,
			want: &BitArray{
				len:   8,
				words: [4]uint64{0xFF, 0, 0, 0}, // 11111111
			},
		},
		{
			name: "middle subset",
			x: &BitArray{
				len:   8,
				words: [4]uint64{0xFF, 0, 0, 0}, // 11111111
			},
			startPos: 2,
			endPos:   5,
			want: &BitArray{
				len:   3,
				words: [4]uint64{0x7, 0, 0, 0}, // 111
			},
		},
		{
			name: "end subset",
			x: &BitArray{
				len:   8,
				words: [4]uint64{0xFF, 0, 0, 0}, // 11111111
			},
			startPos: 5,
			endPos:   8,
			want: &BitArray{
				len:   3,
				words: [4]uint64{0x7, 0, 0, 0}, // 111
			},
		},
		{
			name: "start subset",
			x: &BitArray{
				len:   8,
				words: [4]uint64{0xFF, 0, 0, 0}, // 11111111
			},
			startPos: 0,
			endPos:   3,
			want: &BitArray{
				len:   3,
				words: [4]uint64{0x7, 0, 0, 0}, // 111
			},
		},
		{
			name: "endPos beyond length",
			x: &BitArray{
				len:   8,
				words: [4]uint64{0xFF, 0, 0, 0}, // 11111111
			},
			startPos: 4,
			endPos:   10,
			want: &BitArray{
				len:   4,
				words: [4]uint64{0xF, 0, 0, 0}, // 1111
			},
		},
		{
			name: "sparse bits",
			x: &BitArray{
				len:   8,
				words: [4]uint64{0xAA, 0, 0, 0}, // 10101010
			},
			startPos: 2,
			endPos:   6,
			want: &BitArray{
				len:   4,
				words: [4]uint64{0xA, 0, 0, 0}, // 1010
			},
		},
		{
			name: "across word boundary",
			x: &BitArray{
				len:   128,
				words: [4]uint64{0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF, 0, 0},
			},
			startPos: 60,
			endPos:   68,
			want: &BitArray{
				len:   8,
				words: [4]uint64{0xFF, 0, 0, 0},
			},
		},
		{
			name: "max length subset",
			x: &BitArray{
				len:   251,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, 0x7FFFFFFFFFFFFFF},
			},
			startPos: 1,
			endPos:   251,
			want: &BitArray{
				len:   250,
				words: [4]uint64{maxUint64, maxUint64, maxUint64, 0x3FFFFFFFFFFFFFF},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := new(BitArray).Subset(tt.x, tt.startPos, tt.endPos)
			if !got.Equal(tt.want) {
				t.Errorf("Subset(%v, %d, %d) = %v, want %v", tt.x, tt.startPos, tt.endPos, got, tt.want)
			}
		})
	}
}

// serializationCases are shared by TestWrite, TestEncodedBytes, and TestEncodedString.
// They must produce the same byte sequence: the minimal big-endian
// encoding of the bit array's significant bytes, followed by its bit length.
//
// Expected format: [active_bytes...] [b.len]
// where active_bytes = ceil(b.len / 8) bytes from the big-endian Bytes() representation.
func serializationCases() []struct {
	name      string
	ba        BitArray
	wantBytes []byte
} {
	return []struct {
		name      string
		ba        BitArray
		wantBytes []byte
	}{
		{
			// No active bytes, just the length byte.
			name:      "empty",
			ba:        BitArray{len: 0, words: [4]uint64{0, 0, 0, 0}},
			wantBytes: []byte{0x00},
		},
		{
			// 1 active byte; bit value is 1.
			name:      "1 bit / set",
			ba:        BitArray{len: 1, words: [4]uint64{1, 0, 0, 0}},
			wantBytes: []byte{0x01, 0x01},
		},
		{
			// 1 active byte; bit value is 0.
			name:      "1 bit / unset",
			ba:        BitArray{len: 1, words: [4]uint64{0, 0, 0, 0}},
			wantBytes: []byte{0x00, 0x01},
		},
		{
			// Still fits in 1 byte (len 7 → ceil(7/8)=1 active byte).
			name:      "7 bits",
			ba:        BitArray{len: 7, words: [4]uint64{0x7F, 0, 0, 0}},
			wantBytes: []byte{0x7F, 0x07},
		},
		{
			// Exact byte boundary: 1 active byte, all bits set.
			name:      "8 bits / all set",
			ba:        BitArray{len: 8, words: [4]uint64{0xFF, 0, 0, 0}},
			wantBytes: []byte{0xFF, 0x08},
		},
		{
			// Exact byte boundary: 1 active byte, all bits cleared.
			name:      "8 bits / all zeros",
			ba:        BitArray{len: 8, words: [4]uint64{0, 0, 0, 0}},
			wantBytes: []byte{0x00, 0x08},
		},
		{
			// Just past the first byte boundary (2 active bytes).
			name:      "9 bits",
			ba:        BitArray{len: 9, words: [4]uint64{0x1FF, 0, 0, 0}},
			wantBytes: []byte{0x01, 0xFF, 0x09},
		},
		{
			// 2 active bytes; taken from the existing TestWriteAndUnmarshalBinary suite.
			name:      "10 bits",
			ba:        BitArray{len: 10, words: [4]uint64{0x3FF, 0, 0, 0}},
			wantBytes: []byte{0x03, 0xFF, 0x0A},
		},
		{
			// 2 active bytes, alternating-bit pattern (0xAAAA).
			name:      "16 bits / sparse",
			ba:        BitArray{len: 16, words: [4]uint64{0xAAAA, 0, 0, 0}},
			wantBytes: []byte{0xAA, 0xAA, 0x10},
		},
		{
			// Just under the 64-bit boundary (8 active bytes).
			name: "63 bits",
			ba:   BitArray{len: 63, words: [4]uint64{ones63, 0, 0, 0}},
			wantBytes: []byte{
				0x7F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // big-endian ones63
				0x3F, // 63
			},
		},
		{
			// Exact 64-bit (word) boundary: 8 active bytes.
			name: "64 bits / word boundary",
			ba:   BitArray{len: 64, words: [4]uint64{maxUint64, 0, 0, 0}},
			wantBytes: []byte{
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // big-endian maxUint64
				0x40, // 64
			},
		},
		{
			// Just past the 64-bit boundary (9 active bytes).
			// words[1]=1 contributes the 65th bit; Bytes()[23] == 0x01.
			name: "65 bits",
			ba:   BitArray{len: 65, words: [4]uint64{maxUint64, 1, 0, 0}},
			wantBytes: []byte{
				0x01,
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // maxUint64
				0x41, // 65
			},
		},
		{
			// Just under the 128-bit boundary (16 active bytes).
			name: "127 bits",
			ba:   BitArray{len: 127, words: [4]uint64{maxUint64, ones63, 0, 0}},
			wantBytes: []byte{
				// big-endian of ones63 (words[1])
				0x7F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
				// big-endian of maxUint64 (words[0])
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
				0x7F, // 127
			},
		},
		{
			// Exact 128-bit (two-word) boundary: 16 active bytes.
			// len=128 is also the threshold where the length byte itself >= 0x80,
			// which exposes the UTF-8 encoding difference in EncodedString.
			name: "128 bits / word boundary",
			ba:   BitArray{len: 128, words: [4]uint64{maxUint64, maxUint64, 0, 0}},
			wantBytes: []byte{
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
				0x80, // 128
			},
		},
		{
			// Exact 192-bit (three-word) boundary: 24 active bytes.
			name: "192 bits / word boundary",
			ba:   BitArray{len: 192, words: [4]uint64{maxUint64, maxUint64, maxUint64, 0}},
			wantBytes: []byte{
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
				0xC0, // 192
			},
		},
		{
			// Full 251-bit StarkNet trie key: 32 active bytes (all four words used).
			// words[3]=0x07FFFFFFFFFFFFFF gives leading byte 0x07.
			name: "251 bits / full StarkNet key",
			ba:   BitArray{len: 251, words: [4]uint64{maxUint64, maxUint64, maxUint64, 0x07FFFFFFFFFFFFFF}},
			wantBytes: []byte{
				0x07, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
				0xFB, // 251
			},
		},
		{
			// 251-bit key with a sparse lower word to verify per-word byte ordering.
			// words[0]=0xAAAAAAAAAAAAAAAA → big-endian last 8 bytes are all 0xAA.
			name: "251 bits / sparse lower word",
			ba:   BitArray{len: 251, words: [4]uint64{0xAAAAAAAAAAAAAAAA, maxUint64, maxUint64, 0x07FFFFFFFFFFFFFF}},
			wantBytes: []byte{
				0x07, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // words[3]
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // words[2]
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // words[1]
				0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA, // words[0]
				0xFB, // 251
			},
		},
	}
}

// TestWrite verifies that BitArray.Write produces the expected byte sequence
// (active bytes + length byte).
func TestWrite(t *testing.T) {
	for _, tt := range serializationCases() {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			n, err := tt.ba.Write(buf)
			require.NoError(t, err)
			assert.Equal(t, len(tt.wantBytes), n, "BitArray.Write: wrong byte count")
			assert.Equal(t, tt.wantBytes, buf.Bytes(), "BitArray.Write: wrong bytes")

			// --- BitArrayOld (old implementation) ---
			oldBuf := new(bytes.Buffer)
			old := BitArrayOld(tt.ba)
			oldN, oldErr := old.Write(oldBuf)
			require.NoError(t, oldErr)
			assert.Equal(t, len(tt.wantBytes), oldN, "BitArrayOld.Write: wrong byte count")
			assert.Equal(t, tt.wantBytes, oldBuf.Bytes(), "BitArrayOld.Write: wrong bytes")

			// Both implementations must produce identical output.
			assert.Equal(t, buf.Bytes(), oldBuf.Bytes(), "Write: BitArray and BitArrayOld outputs differ")
			assert.Equal(t, n, oldN, "Write: BitArray and BitArrayOld byte counts differ")
		})
	}
}

// TestEncodedBytes verifies that BitArray.EncodedBytes returns the expected byte
// slice (active bytes + length byte) and that its output is consistent with Write.
func TestEncodedBytes(t *testing.T) {
	for _, tt := range serializationCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ba.EncodedBytes()
			assert.Equal(t, tt.wantBytes, got, "BitArray.EncodedBytes: wrong bytes")

			// EncodedBytes must be consistent with Write.
			buf := new(bytes.Buffer)
			_, _ = tt.ba.Write(buf)
			assert.Equal(t, buf.Bytes(), got, "EncodedBytes: output inconsistent with Write")

			// Verify that modifying the returned slice does not affect internal state.
			if len(got) > 0 {
				got[0] ^= 0xFF
				assert.Equal(t, tt.wantBytes, tt.ba.EncodedBytes(), "EncodedBytes: returned slice shares memory with BitArray")
			}

			// --- BitArrayOld (old implementation) ---
			old := BitArrayOld(tt.ba)
			oldGot := old.EncodedBytes()
			assert.Equal(t, tt.wantBytes, oldGot, "BitArrayOld.EncodedBytes: wrong bytes")

			// Both implementations must produce identical output.
			assert.Equal(t, got, oldGot, "EncodedBytes: BitArray and BitArrayOld outputs differ")
		})
	}
}

// TestEncodedString verifies that EncodedString returns a string whose bytes are
// identical to those produced by EncodedBytes: active bytes in big-endian order
// followed by the bit length as a single byte.
func TestEncodedString(t *testing.T) {
	for _, tt := range serializationCases() {
		t.Run(tt.name, func(t *testing.T) {
			want := string(tt.wantBytes)
			assert.Equal(t, want, tt.ba.EncodedString(), "BitArray.EncodedString: wrong result")

			// --- BitArrayOld (old implementation) ---
			old := BitArrayOld(tt.ba)
			assert.Equal(t, want, old.EncodedString(), "BitArrayOld.EncodedString: wrong result")
		})
	}
}
