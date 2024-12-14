package trie

import (
	"encoding/binary"
	"math"

	"github.com/NethermindEth/juno/core/felt"
)

const (
	maxUint64 = uint64(math.MaxUint64)
)

var maxBitArray = [4]uint64{math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64}

// bitArray is a structure that represents a bit array with a max length of 255 bits.
// It uses a little endian representation to do bitwise operations of the words efficiently.
// Unlike normal bit arrays, it has a `len` field that represents the number of used bits.
// For example, if len is 10, it means that the 2^9, 2^8, ..., 2^0 bits are used.
// The reason why 255 bits is the max length is because we only need up to 251 bits for a given trie key.
// Though words can be used to represent 256 bits, we don't want to add an additional byte for the length.
type bitArray struct {
	len   uint8     // number of used bits
	words [4]uint64 // little endian (i.e. words[0] is the least significant)
}

// Bytes returns the bytes representation of the bit array in big endian format
func (b *bitArray) Bytes() [32]byte {
	var res [32]byte

	switch {
	case b.len == 0:
		// all zeros
		return res
	case b.len >= 192:
		// Create mask for top word: keeps only valid bits above 192
		// e.g., if len=200, keeps lowest 8 bits (200-192)
		mask := maxUint64 >> (256 - uint16(b.len))
		binary.BigEndian.PutUint64(res[0:8], b.words[3]&mask)
		binary.BigEndian.PutUint64(res[8:16], b.words[2])
		binary.BigEndian.PutUint64(res[16:24], b.words[1])
		binary.BigEndian.PutUint64(res[24:32], b.words[0])
	case b.len >= 128:
		// Mask for bits 128-191: keeps only valid bits above 128
		// e.g., if len=150, keeps lowest 22 bits (150-128)
		mask := maxUint64 >> (192 - b.len)
		binary.BigEndian.PutUint64(res[8:16], b.words[2]&mask)
		binary.BigEndian.PutUint64(res[16:24], b.words[1])
		binary.BigEndian.PutUint64(res[24:32], b.words[0])
	case b.len >= 64:
		// You get the idea
		mask := maxUint64 >> (128 - b.len)
		binary.BigEndian.PutUint64(res[16:24], b.words[1]&mask)
		binary.BigEndian.PutUint64(res[24:32], b.words[0])
	default:
		mask := maxUint64 >> (64 - b.len)
		binary.BigEndian.PutUint64(res[24:32], b.words[0]&mask)
	}

	return res
}

func (b *bitArray) SetFelt(length uint8, f *felt.Felt) *bitArray {
	b.setFelt(f)
	b.len = length
	return b
}

func (b *bitArray) SetFelt251(f *felt.Felt) *bitArray {
	b.setFelt(f)
	b.len = 251
	return b
}

func (b *bitArray) setFelt(f *felt.Felt) {
	res := f.Bytes()
	b.words[3] = binary.BigEndian.Uint64(res[0:8])
	b.words[2] = binary.BigEndian.Uint64(res[8:16])
	b.words[1] = binary.BigEndian.Uint64(res[16:24])
	b.words[0] = binary.BigEndian.Uint64(res[24:32])
}

func (b *bitArray) PrefixEqual(x *bitArray) bool {
	if b.len == x.len {
		return b.Equal(x)
	}

	var long, short *bitArray
	long, short = b, x

	if b.len < x.len {
		long, short = x, b
	}

	return long.Rsh(long, long.len-short.len).Equal(short)
}

// Rsh sets b = x >> n and returns b.
func (b *bitArray) Rsh(x *bitArray, n uint8) *bitArray {
	if x.len == 0 {
		return b.set(x)
	}

	if n >= x.len {
		x.clear()
		return b.set(x)
	}

	switch {
	case n == 0:
		return b.set(x)
	case n >= 192:
		b.rsh192(x)
		b.len = x.len - n
		n -= 192
		b.words[0] >>= n
	case n >= 128:
		b.rsh128(x)
		b.len = x.len - n
		n -= 128
		b.words[0] = (b.words[0] >> n) | (b.words[1] << (64 - n))
		b.words[1] >>= n
	case n >= 64:
		b.rsh64(x)
		b.len = x.len - n
		n -= 64
		b.words[0] = (b.words[0] >> n) | (b.words[1] << (64 - n))
		b.words[1] = (b.words[1] >> n) | (b.words[2] << (64 - n))
		b.words[2] >>= n
	default:
		b.set(x)
		b.len -= n
		b.words[0] = (b.words[0] >> n) | (b.words[1] << (64 - n))
		b.words[1] = (b.words[1] >> n) | (b.words[2] << (64 - n))
		b.words[2] = (b.words[2] >> n) | (b.words[3] << (64 - n))
		b.words[3] >>= n
	}

	return b
}

// Eq checks if two bit arrays are equal
func (b *bitArray) Equal(x *bitArray) bool {
	return b.len == x.len &&
		b.words[0] == x.words[0] &&
		b.words[1] == x.words[1] &&
		b.words[2] == x.words[2] &&
		b.words[3] == x.words[3]
}

func (b *bitArray) set(x *bitArray) *bitArray {
	b.len = x.len
	b.words[0] = x.words[0]
	b.words[1] = x.words[1]
	b.words[2] = x.words[2]
	b.words[3] = x.words[3]
	return b
}

func (b *bitArray) rsh64(x *bitArray) {
	b.words[3], b.words[2], b.words[1], b.words[0] = 0, x.words[3], x.words[2], x.words[1]
}

func (b *bitArray) rsh128(x *bitArray) {
	b.words[3], b.words[2], b.words[1], b.words[0] = 0, 0, x.words[3], x.words[2]
}

func (b *bitArray) rsh192(x *bitArray) {
	b.words[3], b.words[2], b.words[1], b.words[0] = 0, 0, 0, x.words[3]
}

func (b *bitArray) clear() *bitArray {
	b.len = 0
	b.words[0], b.words[1], b.words[2], b.words[3] = 0, 0, 0, 0
	return b
}
