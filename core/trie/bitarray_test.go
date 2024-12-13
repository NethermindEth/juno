package trie

import (
	"bytes"
	"encoding/binary"
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
			bitArray: bitArray{pos: 0, words: maxBitArray},
			want:     [32]byte{},
		},
		{
			name:     "length < 64",
			bitArray: bitArray{pos: 38, words: maxBitArray},
			want: func() [32]byte {
				var b [32]byte
				binary.BigEndian.PutUint64(b[24:32], 0x7FFFFFFFFF)
				return b
			}(),
		},
		{
			name:     "64 <= length < 128",
			bitArray: bitArray{pos: 100, words: maxBitArray},
			want: func() [32]byte {
				var b [32]byte
				binary.BigEndian.PutUint64(b[16:24], 0x1FFFFFFFFF)
				binary.BigEndian.PutUint64(b[24:32], 0xFFFFFFFFFFFFFFFF)
				return b
			}(),
		},
		{
			name:     "128 <= length < 192",
			bitArray: bitArray{pos: 130, words: maxBitArray},
			want: func() [32]byte {
				var b [32]byte
				binary.BigEndian.PutUint64(b[8:16], 0x7)
				binary.BigEndian.PutUint64(b[16:24], 0xFFFFFFFFFFFFFFFF)
				binary.BigEndian.PutUint64(b[24:32], 0xFFFFFFFFFFFFFFFF)
				return b
			}(),
		},
		{
			name:     "192 <= length < 255",
			bitArray: bitArray{pos: 201, words: maxBitArray},
			want: func() [32]byte {
				var b [32]byte
				binary.BigEndian.PutUint64(b[0:8], 0x3FF)
				binary.BigEndian.PutUint64(b[8:16], 0xFFFFFFFFFFFFFFFF)
				binary.BigEndian.PutUint64(b[16:24], 0xFFFFFFFFFFFFFFFF)
				binary.BigEndian.PutUint64(b[24:32], 0xFFFFFFFFFFFFFFFF)
				return b
			}(),
		},
		{
			name:     "length == 254",
			bitArray: bitArray{pos: 254, words: maxBitArray},
			want: func() [32]byte {
				var b [32]byte
				binary.BigEndian.PutUint64(b[0:8], 0x7FFFFFFFFFFFFFFF)
				binary.BigEndian.PutUint64(b[8:16], 0xFFFFFFFFFFFFFFFF)
				binary.BigEndian.PutUint64(b[16:24], 0xFFFFFFFFFFFFFFFF)
				binary.BigEndian.PutUint64(b[24:32], 0xFFFFFFFFFFFFFFFF)
				return b
			}(),
		},
		{
			name:     "length == 255",
			bitArray: bitArray{pos: 255, words: maxBitArray},
			want: func() [32]byte {
				var b [32]byte
				binary.BigEndian.PutUint64(b[0:8], 0xFFFFFFFFFFFFFFFF)
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
		})
	}
}
