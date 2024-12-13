package trie

import (
	"encoding/binary"

	"github.com/NethermindEth/juno/core/felt"
)

const (
	mask64 = uint64(1 << 63)
)

var maxBitArray = [4]uint64{0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF}

// bitArray is a structure that represents a bit array with a max length of 255 bits.
// The reason why 255 bits is the max length is because we only need up to 252 bits for the felt.
// Though words can be used to represent 256 bits, we don't want to add an additional byte for the length.
// It uses a little endian representation to do bitwise operations of the words efficiently.
// Unlike normal bit arrays, it has a `len` field that represents the number of used bits.
// For example, if len is 10, it means that the 2^9, 2^8, ..., 2^0 bits are used.
type bitArray struct {
	len   uint8     // number of used bits
	words [4]uint64 // little endian (i.e. words[0] is the least significant)
}

// Bytes returns the bytes representation of the bit array in big endian format.
func (b *bitArray) Bytes() [32]byte {
	var res [32]byte

	switch {
	case b.len == 0:
		return res
	case b.len >= 192:
		// len is 0-based, so 255 (not 256) represents all bits used
		// subtracting from 255 ensures correct mask when len=255
		// For example, when len is 255, it means all bits from index 0
		// to 254 are used (total of 255 bits).
		// So when we create the mask, we shift 255 - 255 = 0 bits to the right.
		// This creates a mask that covers all bits from index 0 to 254.
		mask := ^mask64 >> (255 - b.len)
		binary.BigEndian.PutUint64(res[0:8], b.words[3]&mask)
		binary.BigEndian.PutUint64(res[8:16], b.words[2])
		binary.BigEndian.PutUint64(res[16:24], b.words[1])
		binary.BigEndian.PutUint64(res[24:32], b.words[0])
	case b.len >= 128:
		// Similar pattern for 191 boundary (3 words × 64 bits - 1)
		mask := ^mask64 >> (191 - b.len)
		binary.BigEndian.PutUint64(res[8:16], b.words[2]&mask)
		binary.BigEndian.PutUint64(res[16:24], b.words[1])
		binary.BigEndian.PutUint64(res[24:32], b.words[0])
	case b.len >= 64:
		mask := ^mask64 >> (127 - b.len)
		binary.BigEndian.PutUint64(res[16:24], b.words[1]&mask)
		binary.BigEndian.PutUint64(res[24:32], b.words[0])
	default:
		mask := ^mask64 >> (63 - b.len)
		binary.BigEndian.PutUint64(res[24:32], b.words[0]&mask)
	}

	return res
}

func (b *bitArray) SetFelt(f *felt.Felt) *bitArray {
	res := f.Bytes()
	b.words[3] = binary.BigEndian.Uint64(res[0:8])
	b.words[2] = binary.BigEndian.Uint64(res[8:16])
	b.words[1] = binary.BigEndian.Uint64(res[16:24])
	b.words[0] = binary.BigEndian.Uint64(res[24:32])
	b.len = felt.Bits - 1
	return b
}

// Rsh shifts the bit array to the right by n bits.
func (b *bitArray) Rsh(x *bitArray, n uint8) *bitArray {
	if b.len == 0 {
		return b
	}

	if n >= b.len {
		return b.clear()
	}

	switch {
	case n == 0:
		return b.set(x)
	case n >= 192:
		b.rsh192(x)
		n -= 192
		b.words[0] >>= n
		b.len -= n
	case n >= 128:
		b.rsh128(x)
		n -= 128
		b.words[0] = (b.words[0] >> n) | (b.words[1] << (64 - n))
		b.words[1] >>= n
		b.len -= n
	case n >= 64:
		b.rsh64(x)
		n -= 64
		b.words[0] = (b.words[0] >> n) | (b.words[1] << (64 - n))
		b.words[1] = (b.words[1] >> n) | (b.words[2] << (64 - n))
		b.words[2] = (b.words[2] >> n) | (b.words[3] << (64 - n))
		b.words[3] >>= n
		b.len -= n
	default:
		b.set(x)
		b.words[3] = (b.words[3] >> n) | (b.words[2] << (64 - n))
		b.words[2] = (b.words[2] >> n) | (b.words[1] << (64 - n))
		b.words[1] = (b.words[1] >> n) | (b.words[0] << (64 - n))
		b.words[0] >>= n
		b.len -= n
	}

	return b
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
