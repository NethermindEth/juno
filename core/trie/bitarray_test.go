package trie

import (
	"bytes"
	"encoding/binary"
	"math/bits"
	"testing"
)

func TestBytes(t *testing.T) {
	tests := []struct {
		name     string
		bitArray bitArray
		want     [32]byte
	}{
		{
			name:     "length == 0",
			bitArray: bitArray{len: 0, words: maxBitArray},
			want:     [32]byte{},
		},
		{
			name:     "length < 64",
			bitArray: bitArray{len: 38, words: maxBitArray},
			want: func() [32]byte {
				var b [32]byte
				binary.BigEndian.PutUint64(b[24:32], 0x3FFFFFFFFF)
				return b
			}(),
		},
		{
			name:     "64 <= length < 128",
			bitArray: bitArray{len: 100, words: maxBitArray},
			want: func() [32]byte {
				var b [32]byte
				binary.BigEndian.PutUint64(b[16:24], 0xFFFFFFFFF)
				binary.BigEndian.PutUint64(b[24:32], 0xFFFFFFFFFFFFFFFF)
				return b
			}(),
		},
		{
			name:     "128 <= length < 192",
			bitArray: bitArray{len: 130, words: maxBitArray},
			want: func() [32]byte {
				var b [32]byte
				binary.BigEndian.PutUint64(b[8:16], 0x3)
				binary.BigEndian.PutUint64(b[16:24], 0xFFFFFFFFFFFFFFFF)
				binary.BigEndian.PutUint64(b[24:32], 0xFFFFFFFFFFFFFFFF)
				return b
			}(),
		},
		{
			name:     "192 <= length < 255",
			bitArray: bitArray{len: 201, words: maxBitArray},
			want: func() [32]byte {
				var b [32]byte
				binary.BigEndian.PutUint64(b[0:8], 0x1FF)
				binary.BigEndian.PutUint64(b[8:16], 0xFFFFFFFFFFFFFFFF)
				binary.BigEndian.PutUint64(b[16:24], 0xFFFFFFFFFFFFFFFF)
				binary.BigEndian.PutUint64(b[24:32], 0xFFFFFFFFFFFFFFFF)
				return b
			}(),
		},
		{
			name:     "length == 254",
			bitArray: bitArray{len: 254, words: maxBitArray},
			want: func() [32]byte {
				var b [32]byte
				binary.BigEndian.PutUint64(b[0:8], 0x3FFFFFFFFFFFFFFF)
				binary.BigEndian.PutUint64(b[8:16], 0xFFFFFFFFFFFFFFFF)
				binary.BigEndian.PutUint64(b[16:24], 0xFFFFFFFFFFFFFFFF)
				binary.BigEndian.PutUint64(b[24:32], 0xFFFFFFFFFFFFFFFF)
				return b
			}(),
		},
		{
			name:     "length == 255",
			bitArray: bitArray{len: 255, words: maxBitArray},
			want: func() [32]byte {
				var b [32]byte
				binary.BigEndian.PutUint64(b[0:8], 0x7FFFFFFFFFFFFFFF)
				binary.BigEndian.PutUint64(b[8:16], 0xFFFFFFFFFFFFFFFF)
				binary.BigEndian.PutUint64(b[16:24], 0xFFFFFFFFFFFFFFFF)
				binary.BigEndian.PutUint64(b[24:32], 0xFFFFFFFFFFFFFFFF)
				return b
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.bitArray.Bytes()
			if !bytes.Equal(got[:], tt.want[:]) {
				t.Errorf("bitArray.Bytes() = %v, want %v", got, tt.want)
			}

			// check if the received bytes has the same bit count as the bitArray.len
			count := 0
			for _, b := range got {
				count += bits.OnesCount8(b)
			}
			if count != int(tt.bitArray.len) {
				t.Errorf("bitArray.Bytes() bit count = %v, want %v", count, tt.bitArray.len)
			}
		})
	}
}

func TestRsh(t *testing.T) {
	tests := []struct {
		name     string
		initial  *bitArray
		shiftBy  uint8
		expected *bitArray
	}{
		{
			name: "zero length array",
			initial: &bitArray{
				len:   0,
				words: [4]uint64{0, 0, 0, 0},
			},
			shiftBy: 5,
			expected: &bitArray{
				len:   0,
				words: [4]uint64{0, 0, 0, 0},
			},
		},
		{
			name: "shift by 0",
			initial: &bitArray{
				len:   64,
				words: [4]uint64{0xFFFFFFFFFFFFFFFF, 0, 0, 0},
			},
			shiftBy: 0,
			expected: &bitArray{
				len:   64,
				words: [4]uint64{0xFFFFFFFFFFFFFFFF, 0, 0, 0},
			},
		},
		{
			name: "shift by more than length",
			initial: &bitArray{
				len:   64,
				words: [4]uint64{0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF, 0, 0},
			},
			shiftBy: 65,
			expected: &bitArray{
				len:   0,
				words: [4]uint64{0, 0, 0, 0},
			},
		},
		{
			name: "shift by less than 64",
			initial: &bitArray{
				len:   128,
				words: [4]uint64{0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF, 0, 0},
			},
			shiftBy: 32,
			expected: &bitArray{
				len:   96,
				words: [4]uint64{0xFFFFFFFFFFFFFFFF, 0x00000000FFFFFFFF, 0, 0},
			},
		},
		{
			name: "shift by exactly 64",
			initial: &bitArray{
				len:   128,
				words: [4]uint64{0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF, 0, 0},
			},
			shiftBy: 64,
			expected: &bitArray{
				len:   64,
				words: [4]uint64{0xFFFFFFFFFFFFFFFF, 0, 0, 0},
			},
		},
		{
			name: "shift by 128",
			initial: &bitArray{
				len:   251,
				words: [4]uint64{0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF},
			},
			shiftBy: 128,
			expected: &bitArray{
				len:   123,
				words: [4]uint64{0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF, 0, 0},
			},
		},
		{
			name: "shift by 192",
			initial: &bitArray{
				len:   251,
				words: [4]uint64{0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF},
			},
			shiftBy: 192,
			expected: &bitArray{
				len:   59,
				words: [4]uint64{0xFFFFFFFFFFFFFFFF, 0, 0, 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := new(bitArray).Rsh(tt.initial, tt.shiftBy)
			if !result.Equal(tt.expected) {
				t.Errorf("Rsh() got = %+v, want %+v", result, tt.expected)
			}
		})
	}
}
