package trie

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/bits"

	"github.com/NethermindEth/juno/core/felt"
)

const (
	maxUint64 = uint64(math.MaxUint64)
	byteBits  = 8
)

var (
	maxBitArray   = [4]uint64{math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64}
	emptyBitArray = &BitArray{len: 0, words: [4]uint64{0, 0, 0, 0}}
)

// BitArray is a structure that represents a bit array with a max length of 255 bits.
// It uses a little endian representation to do bitwise operations of the words efficiently.
// Unlike normal bit arrays, it has a `len` field that represents the number of used bits.
// For example, if len is 10, it means that the 2^9, 2^8, ..., 2^0 bits are used.
// The reason why 255 bits is the max length is because we only need up to 251 bits for a given trie key.
// Though words can be used to represent 256 bits, we don't want to add an additional byte for the length.
type BitArray struct {
	len   uint8     // number of used bits
	words [4]uint64 // little endian (i.e. words[0] is the least significant)
}

func (b *BitArray) Felt() *felt.Felt {
	bs := b.Bytes()
	return new(felt.Felt).SetBytes(bs[:])
}

func (b *BitArray) Len() uint8 {
	return b.len
}

// Bytes returns the bytes representation of the bit array in big endian format
func (b *BitArray) Bytes() [32]byte {
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

// EqualMSBs checks if two bit arrays share the same most significant bits, where the length of
// the check is determined by the shorter array. Returns true if either array has
// length 0, or if the first min(b.len, x.len) MSBs are identical.
//
// For example:
//
//	a = 1101 (len=4)
//	b = 11010111 (len=8)
//	a.EqualMSBs(b) = true  // First 4 MSBs match
//
//	a = 1100 (len=4)
//	b = 1101 (len=4)
//	a.EqualMSBs(b) = false // All bits compared, not equal
//
//	a = 1100 (len=4)
//	b = [] (len=0)
//	a.EqualMSBs(b) = true  // Zero length is always a prefix match
func (b *BitArray) EqualMSBs(x *BitArray) bool {
	if b.len == x.len {
		return b.Equal(x)
	}

	if b.len == 0 || x.len == 0 {
		return true
	}

	var long, short *BitArray

	long, short = b, x
	if b.len < x.len {
		long, short = x, b
	}

	return long.Rsh(long, long.len-short.len).Equal(short)
}

// Truncate sets b to the first 'length' bits of x (starting from the least significant bit).
// If length >= x.len, b is an exact copy of x.
// Any bits beyond the specified length are cleared to zero.
// For example:
//
//	x = 11001011 (len=8)
//	Truncate(x, 4) = 1011 (len=4)
//	Truncate(x, 10) = 11001011 (len=8, original x)
//	Truncate(x, 0) = 0 (len=0)
func (b *BitArray) Truncate(x *BitArray, length uint8) *BitArray {
	if length >= x.len {
		return b.Set(x)
	}

	b.Set(x)
	b.len = length

	// Clear all words beyond what's needed
	switch {
	case length == 0:
		b.words = [4]uint64{0, 0, 0, 0}
	case length <= 64:
		mask := maxUint64 >> (64 - length)
		b.words[0] &= mask
		b.words[1] = 0
		b.words[2] = 0
		b.words[3] = 0
	case length <= 128:
		mask := maxUint64 >> (128 - length)
		b.words[1] &= mask
		b.words[2] = 0
		b.words[3] = 0
	case length <= 192:
		mask := maxUint64 >> (192 - length)
		b.words[2] &= mask
		b.words[3] = 0
	default:
		mask := maxUint64 >> (256 - uint16(length))
		b.words[3] &= mask
	}

	return b
}

// CommonMSBs sets b to the longest sequence of matching most significant bits between two bit arrays.
// For example:
//
//	x = 1101 0111 (len=8)
//	y = 1101 0000 (len=8)
//	CommonMSBs(x,y) = 1101 (len=4)
func (b *BitArray) CommonMSBs(x, y *BitArray) *BitArray {
	if x.len == 0 || y.len == 0 {
		return emptyBitArray
	}

	long, short := x, y
	if x.len < y.len {
		long, short = y, x
	}

	// Align arrays by right-shifting longer array and then XOR to find differences
	// Example:
	//   short = 1101 (len=4)
	//   long  = 1101 0111 (len=8)
	//
	// Step 1: Right shift longer array by 4
	//   short = 1100
	//   long  = 1101
	//
	// Step 2: XOR shows difference at last bit
	//   1100 (short)
	//   1101 (aligned long)
	//   ---- XOR
	//   0001 (difference at last position)
	// We can then use the position of the first set bit and right-shift to get the common MSBs
	diff := long.len - short.len
	b.Rsh(long, diff).Xor(b, short)
	divergentBit := findFirstSetBit(b)

	return b.Rsh(short, divergentBit)
}

// findFirstSetBit returns the position of the first '1' bit in the array,
// scanning from most significant to least significant bit.
//
// The bit position is counted from the least significant bit, starting at 0.
// For example:
//
//	array = 0000 0000 ... 0100 (len=251)
//	findFirstSetBit() = 2 // third bit from right is set
func findFirstSetBit(b *BitArray) uint8 {
	if b.len == 0 {
		return 0
	}

	for i := 3; i >= 0; i-- {
		if word := b.words[i]; word != 0 {
			return uint8((i+1)*64 - bits.LeadingZeros64(word))
		}
	}

	return 0
}

// Rsh sets b = x >> n and returns b.
func (b *BitArray) Rsh(x *BitArray, n uint8) *BitArray {
	if x.len == 0 {
		return b.Set(x)
	}

	if n >= x.len {
		x.clear()
		return b.Set(x)
	}

	switch {
	case n == 0:
		return b.Set(x)
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
		b.Set(x)
		b.len -= n
		b.words[0] = (b.words[0] >> n) | (b.words[1] << (64 - n))
		b.words[1] = (b.words[1] >> n) | (b.words[2] << (64 - n))
		b.words[2] = (b.words[2] >> n) | (b.words[3] << (64 - n))
		b.words[3] >>= n
	}

	return b
}

// Xor sets b = x ^ y and returns b.
func (b *BitArray) Xor(x, y *BitArray) *BitArray {
	b.words[0] = x.words[0] ^ y.words[0]
	b.words[1] = x.words[1] ^ y.words[1]
	b.words[2] = x.words[2] ^ y.words[2]
	b.words[3] = x.words[3] ^ y.words[3]
	return b
}

// Eq checks if two bit arrays are equal
func (b *BitArray) Equal(x *BitArray) bool {
	return b.len == x.len &&
		b.words[0] == x.words[0] &&
		b.words[1] == x.words[1] &&
		b.words[2] == x.words[2] &&
		b.words[3] == x.words[3]
}

// IsBitSit returns true if bit n-th is set, where n = 0 is LSB.
// The n must be <= 255.
func (b *BitArray) IsBitSet(n uint8) bool {
	if n >= b.len {
		return false
	}

	return (b.words[n/64] & (1 << (n % 64))) != 0
}

// Write serialises the BitArray into a bytes buffer in the following format:
// - First byte: length of the bit array (0-255)
// - Remaining bytes: the necessary bytes included in big endian order
// Example:
//
//	BitArray{len: 10, words: [4]uint64{0x03FF}} -> [0x0A, 0x03, 0xFF]
func (b *BitArray) Write(buf *bytes.Buffer) (int, error) {
	if err := buf.WriteByte(b.len); err != nil {
		return 0, err
	}

	n, err := buf.Write(b.activeBytes())
	return n + 1, err
}

// UnmarshalBinary deserialises the BitArray from a bytes buffer in the following format:
// - First byte: length of the bit array (0-255)
// - Remaining bytes: the necessary bytes included in big endian order
// Example:
//
//	[0x0A, 0x03, 0xFF] -> BitArray{len: 10, words: [4]uint64{0x03FF}}
func (b *BitArray) UnmarshalBinary(data []byte) error {
	b.len = data[0]

	var bs [32]byte
	copy(bs[32-b.byteCount():], data[1:])
	b.SetBytes32(bs)
	return nil
}

func (b *BitArray) SetFelt(length uint8, f *felt.Felt) *BitArray {
	b.setFelt(f)
	b.len = length
	return b
}

func (b *BitArray) SetFelt251(f *felt.Felt) *BitArray {
	b.setFelt(f)
	b.len = 251
	return b
}

func (b *BitArray) SetBytes32(data [32]byte) *BitArray {
	b.words[3] = binary.BigEndian.Uint64(data[0:8])
	b.words[2] = binary.BigEndian.Uint64(data[8:16])
	b.words[1] = binary.BigEndian.Uint64(data[16:24])
	b.words[0] = binary.BigEndian.Uint64(data[24:32])
	return b
}

func (b *BitArray) setFelt(f *felt.Felt) {
	res := f.Bytes()
	b.words[3] = binary.BigEndian.Uint64(res[0:8])
	b.words[2] = binary.BigEndian.Uint64(res[8:16])
	b.words[1] = binary.BigEndian.Uint64(res[16:24])
	b.words[0] = binary.BigEndian.Uint64(res[24:32])
}

func (b *BitArray) Set(x *BitArray) *BitArray {
	b.len = x.len
	b.words[0] = x.words[0]
	b.words[1] = x.words[1]
	b.words[2] = x.words[2]
	b.words[3] = x.words[3]
	return b
}

// byteCount returns the minimum number of bytes needed to represent the bit array.
// It rounds up to the nearest byte.
func (b *BitArray) byteCount() uint8 {
	// Cast to uint16 to avoid overflow
	return uint8((uint16(b.len) + uint16(byteBits-1)) / uint16(byteBits))
}

// activeBytes returns a slice containing only the bytes that are actually used
// by the bit array, excluding leading zero bytes. The returned slice is in
// big-endian order.
//
// Example:
//
//	len = 10, words = [0x3FF, 0, 0, 0] -> [0x03, 0xFF]
func (b *BitArray) activeBytes() []byte {
	wordsBytes := b.Bytes()
	return wordsBytes[32-b.byteCount():]
}

func (b *BitArray) rsh64(x *BitArray) {
	b.words[3], b.words[2], b.words[1], b.words[0] = 0, x.words[3], x.words[2], x.words[1]
}

func (b *BitArray) rsh128(x *BitArray) {
	b.words[3], b.words[2], b.words[1], b.words[0] = 0, 0, x.words[3], x.words[2]
}

func (b *BitArray) rsh192(x *BitArray) {
	b.words[3], b.words[2], b.words[1], b.words[0] = 0, 0, 0, x.words[3]
}

func (b *BitArray) clear() *BitArray {
	b.len = 0
	b.words[0], b.words[1], b.words[2], b.words[3] = 0, 0, 0, 0
	return b
}
