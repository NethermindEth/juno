package trie

import (
	"encoding/binary"

	"github.com/NethermindEth/juno/core/felt"
)

const (
	mask64 = uint64(1 << 63)
)

type bitArray struct {
	pos   uint8     // position of the most significant bit
	words [4]uint64 // little endian (i.e. words[0] is the least significant)
}

func (b *bitArray) Bytes() [32]byte {
	var res [32]byte

	switch {
	case b.pos == 0:
		return res
	case b.pos == 255:
		binary.BigEndian.PutUint64(res[0:8], b.words[3])
		binary.BigEndian.PutUint64(res[8:16], b.words[2])
		binary.BigEndian.PutUint64(res[16:24], b.words[1])
		binary.BigEndian.PutUint64(res[24:32], b.words[0])
	case b.pos >= 192:
		rem := 255 - uint(b.pos)
		mask := ^mask64 >> (rem - 1)
		binary.BigEndian.PutUint64(res[0:8], b.words[3]&mask)
		binary.BigEndian.PutUint64(res[8:16], b.words[2])
		binary.BigEndian.PutUint64(res[16:24], b.words[1])
		binary.BigEndian.PutUint64(res[24:32], b.words[0])
	case b.pos >= 128:
		rem := 191 - b.pos
		mask := ^mask64 >> (rem - 1)
		binary.BigEndian.PutUint64(res[8:16], b.words[2]&mask)
		binary.BigEndian.PutUint64(res[16:24], b.words[1])
		binary.BigEndian.PutUint64(res[24:32], b.words[0])
	case b.pos >= 64:
		rem := 127 - b.pos
		mask := ^mask64 >> (rem - 1)
		binary.BigEndian.PutUint64(res[16:24], b.words[1]&mask)
		binary.BigEndian.PutUint64(res[24:32], b.words[0])
	default:
		rem := 63 - b.pos
		mask := ^mask64 >> (rem - 1)
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
	b.pos = felt.Bits - 1
	return b
}

// Rsh shifts the bit array to the right by n bits.
func (b *bitArray) Rsh(x *bitArray, n uint8) *bitArray {
	if b.pos == 0 {
		return b
	}

	if n >= b.pos {
		return b.clear()
	}

	switch {
	case n == 0:
		return b.set(x)
	case n >= 192:
		b.rsh192(x)
		n -= 192
		b.words[0] >>= n
		b.pos -= n
	case n >= 128:
		b.rsh128(x)
		n -= 128
		b.words[0] = (b.words[0] >> n) | (b.words[1] << (64 - n))
		b.words[1] >>= n
		b.pos -= n
	case n >= 64:
		b.rsh64(x)
		n -= 64
		b.words[0] = (b.words[0] >> n) | (b.words[1] << (64 - n))
		b.words[1] = (b.words[1] >> n) | (b.words[2] << (64 - n))
		b.words[2] = (b.words[2] >> n) | (b.words[3] << (64 - n))
		b.words[3] >>= n
		b.pos -= n
	default:
		b.set(x)
		b.words[3] = (b.words[3] >> n) | (b.words[2] << (64 - n))
		b.words[2] = (b.words[2] >> n) | (b.words[1] << (64 - n))
		b.words[1] = (b.words[1] >> n) | (b.words[0] << (64 - n))
		b.words[0] >>= n
		b.pos -= n
	}

	return b
}

func (b *bitArray) set(x *bitArray) *bitArray {
	b.pos = x.pos
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
	b.pos = 0
	b.words[0], b.words[1], b.words[2], b.words[3] = 0, 0, 0, 0
	return b
}
