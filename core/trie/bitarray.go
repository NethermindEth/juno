package trie

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/bits"

	"github.com/NethermindEth/juno/core/felt"
)

const (
	maxUint64 = uint64(math.MaxUint64) // 0xFFFFFFFFFFFFFFFF
	maxUint8  = uint8(math.MaxUint8)
)

var emptyBitArray = new(BitArray)

// Represents a bit array with length representing the number of used bits.
// It uses a little endian representation to do bitwise operations of the words efficiently.
// For example, if len is 10, it means that the 2^9, 2^8, ..., 2^0 bits are used.
// The max length is 255 bits (uint8), because our use case only need up to 251 bits for a given trie key.
// Although words can be used to represent 256 bits, we don't want to add an additional byte for the length.
type BitArray struct {
	len   uint8     // number of used bits
	words [4]uint64 // little endian (i.e. words[0] is the least significant)
}

func NewBitArray(length uint8, val uint64) BitArray {
	var b BitArray
	b.SetUint64(length, val)
	return b
}

// Returns the felt representation of the bit array.
func (b *BitArray) Felt() felt.Felt {
	var f felt.Felt
	bt := b.Bytes()
	f.SetBytes(bt[:])
	return f
}

func (b *BitArray) Len() uint8 {
	return b.len
}

// Returns the bytes representation of the bit array in big endian format
func (b *BitArray) Bytes() [32]byte {
	var res [32]byte

	binary.BigEndian.PutUint64(res[0:8], b.words[3])
	binary.BigEndian.PutUint64(res[8:16], b.words[2])
	binary.BigEndian.PutUint64(res[16:24], b.words[1])
	binary.BigEndian.PutUint64(res[24:32], b.words[0])

	return res
}

// Sets the bit array to the least significant 'n' bits of x.
// n is counted from the least significant bit, starting at 0.
// If length >= x.len, the bit array is an exact copy of x.
// For example:
//
//	x = 11001011 (len=8)
//	LSBsFromLSB(x, 4) = 1011 (len=4)
//	LSBsFromLSB(x, 10) = 11001011 (len=8, original x)
//	LSBsFromLSB(x, 0) = 0 (len=0)
//
//nolint:mnd
func (b *BitArray) LSBsFromLSB(x *BitArray, n uint8) *BitArray {
	if n >= x.len {
		return b.Set(x)
	}

	b.len = n

	switch {
	case n == 0:
		b.words = [4]uint64{0, 0, 0, 0}
	case n <= 64:
		b.words[0] = x.words[0] & (maxUint64 >> (64 - n))
		b.words[1], b.words[2], b.words[3] = 0, 0, 0
	case n <= 128:
		b.words[0] = x.words[0]
		b.words[1] = x.words[1] & (maxUint64 >> (128 - n))
		b.words[2], b.words[3] = 0, 0
	case n <= 192:
		b.words[0] = x.words[0]
		b.words[1] = x.words[1]
		b.words[2] = x.words[2] & (maxUint64 >> (192 - n))
		b.words[3] = 0
	default:
		b.words[0] = x.words[0]
		b.words[1] = x.words[1]
		b.words[2] = x.words[2]
		b.words[3] = x.words[3] & (maxUint64 >> (256 - uint16(n)))
	}

	return b
}

// Returns the least significant bits of `x` with `n` counted from the most significant bit, starting at 0.
// For example:
//
//	x = 11001011 (len=8)
//	LSBs(x, 1) = 1001011 (len=7)
//	LSBs(x, 10) = 0 (len=0)
//	LSBs(x, 0) = 11001011 (len=8, original x)
func (b *BitArray) LSBs(x *BitArray, n uint8) *BitArray {
	if n == 0 {
		return b.Set(x)
	}

	if n > x.Len() {
		return b.clear()
	}

	return b.LSBsFromLSB(x, x.Len()-n)
}

// Checks if the current bit array share the same most significant bits with another, where the length of
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

	// Compare only the first min(b.len, x.len) bits
	minLen := b.len
	if x.len < minLen {
		minLen = x.len
	}

	return new(BitArray).MSBs(b, minLen).Equal(new(BitArray).MSBs(x, minLen))
}

// Sets the bit array to the most significant 'n' bits of x, that is position 0 to n (exclusive).
// If n >= x.len, the bit array is an exact copy of x.
// For example:
//
//	x = 11001011 (len=8)
//	MSBs(x, 4) = 1100 (len=4)
//	MSBs(x, 10) = 11001011 (len=8, original x)
//	MSBs(x, 0) = 0 (len=0)
func (b *BitArray) MSBs(x *BitArray, n uint8) *BitArray {
	if n >= x.len {
		return b.Set(x)
	}

	return b.Rsh(x, x.len-n)
}

// Sets the bit array to the longest sequence of matching most significant bits between two bit arrays.
// For example:
//
//	x = 1101 0111 (len=8)
//	y = 1101 0000 (len=8)
//	CommonMSBs(x,y) = 1101 (len=4)
func (b *BitArray) CommonMSBs(x, y *BitArray) *BitArray {
	if x.len == 0 || y.len == 0 {
		return b.clear()
	}

	long, short := x, y
	if x.len < y.len {
		long, short = y, x
	}

	// Align arrays by right-shifting longer array and then XOR to find differences
	// Example:
	//   short = 1100 (len=4)
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

// Sets the bit array to x >> n and returns the bit array.
//
//nolint:mnd
func (b *BitArray) Rsh(x *BitArray, n uint8) *BitArray {
	if x.len == 0 {
		return b.Set(x)
	}

	if n >= x.len {
		return b.clear()
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

	b.truncateToLength()
	return b
}

// Lsh sets the bit array to x << n and returns the bit array.
//
//nolint:mnd
func (b *BitArray) Lsh(x *BitArray, n uint8) *BitArray {
	if x.len == 0 || n == 0 {
		return b.Set(x)
	}

	// If the result will overflow, we set the length to the max length
	// but we still shift `n` bits
	if n > maxUint8-x.len {
		b.len = maxUint8
	} else {
		b.len = x.len + n
	}

	switch {
	case n >= 192:
		b.lsh192(x)
		n -= 192
		b.words[3] <<= n
	case n >= 128:
		b.lsh128(x)
		n -= 128
		b.words[3] = (x.words[3] << n) | (x.words[2] >> (64 - n))
		b.words[2] <<= n
	case n >= 64:
		b.lsh64(x)
		n -= 64
		b.words[3] = (x.words[3] << n) | (x.words[2] >> (64 - n))
		b.words[2] = (x.words[2] << n) | (x.words[1] >> (64 - n))
		b.words[1] <<= n
	default:
		b.words[3] = (x.words[3] << n) | (x.words[2] >> (64 - n))
		b.words[2] = (x.words[2] << n) | (x.words[1] >> (64 - n))
		b.words[1] = (x.words[1] << n) | (x.words[0] >> (64 - n))
		b.words[0] = x.words[0] << n
	}

	b.truncateToLength()
	return b
}

// Sets the bit array to the concatenation of x and y and returns the bit array.
// For example:
//
//	x = 000 (len=3)
//	y = 111 (len=3)
//	Append(x,y) = 000111 (len=6)
func (b *BitArray) Append(x, y *BitArray) *BitArray {
	if x.len == 0 || y.len == maxUint8 {
		return b.Set(y)
	}
	if y.len == 0 {
		return b.Set(x)
	}

	// Then shift left by y's length and OR with y
	return b.Lsh(x, y.len).Or(b, y)
}

// Sets the bit array to x | y and returns the bit array.
func (b *BitArray) Or(x, y *BitArray) *BitArray {
	b.words[0] = x.words[0] | y.words[0]
	b.words[1] = x.words[1] | y.words[1]
	b.words[2] = x.words[2] | y.words[2]
	b.words[3] = x.words[3] | y.words[3]
	b.len = x.len
	return b
}

// Sets the bit array to x ^ y and returns the bit array.
func (b *BitArray) Xor(x, y *BitArray) *BitArray {
	b.words[0] = x.words[0] ^ y.words[0]
	b.words[1] = x.words[1] ^ y.words[1]
	b.words[2] = x.words[2] ^ y.words[2]
	b.words[3] = x.words[3] ^ y.words[3]
	return b
}

// Checks if two bit arrays are equal
func (b *BitArray) Equal(x *BitArray) bool {
	// TODO(weiihann): this is really not a good thing to do...
	if b == nil && x == nil {
		return true
	} else if b == nil || x == nil {
		return false
	}

	return b.len == x.len &&
		b.words[0] == x.words[0] &&
		b.words[1] == x.words[1] &&
		b.words[2] == x.words[2] &&
		b.words[3] == x.words[3]
}

// Returns true if bit n-th is set, where n = 0 is LSB.
func (b *BitArray) IsBitSetFromLSB(n uint8) bool {
	return b.BitFromLSB(n) == 1
}

// Returns the bit value at position n, where n = 0 is LSB.
// If n is out of bounds, returns 0.
func (b *BitArray) BitFromLSB(n uint8) uint8 {
	if n >= b.len {
		return 0
	}

	if (b.words[n/64] & (1 << (n % 64))) != 0 {
		return 1
	}

	return 0
}

func (b *BitArray) IsBitSet(n uint8) bool {
	return b.Bit(n) == 1
}

// Returns the bit value at position n, where n = 0 is MSB.
// If n is out of bounds, returns 0.
func (b *BitArray) Bit(n uint8) uint8 {
	if n >= b.Len() {
		return 0
	}

	return b.BitFromLSB(b.Len() - n - 1)
}

// Returns the bit value at the most significant bit
func (b *BitArray) MSB() uint8 {
	return b.Bit(0)
}

func (b *BitArray) LSB() uint8 {
	return b.BitFromLSB(0)
}

func (b *BitArray) IsEmpty() bool {
	return b.len == 0
}

// Serialises the BitArray into a bytes buffer in the following format:
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

// Deserialises the BitArray from a bytes buffer in the following format:
// - First byte: length of the bit array (0-255)
// - Remaining bytes: the necessary bytes included in big endian order
// Example:
//
//	[0x0A, 0x03, 0xFF] -> BitArray{len: 10, words: [4]uint64{0x03FF}}
func (b *BitArray) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return errors.New("empty data")
	}

	length := data[0]
	byteCount := (uint(length) + 7) / 8 // Round up to nearest byte

	if len(data) < int(byteCount)+1 {
		return fmt.Errorf("invalid data length: got %d bytes, expected %d", len(data), byteCount+1)
	}

	b.len = length

	var bs [32]byte
	copy(bs[32-byteCount:], data[1:])
	b.setBytes32(bs[:])

	return nil
}

// Sets the bit array to the same value as x.
func (b *BitArray) Set(x *BitArray) *BitArray {
	b.len = x.len
	b.words[0] = x.words[0]
	b.words[1] = x.words[1]
	b.words[2] = x.words[2]
	b.words[3] = x.words[3]
	return b
}

// Sets the bit array to the bytes representation of a felt.
func (b *BitArray) SetFelt(length uint8, f *felt.Felt) *BitArray {
	b.len = length
	b.setFelt(f)
	b.truncateToLength()
	return b
}

// Sets the bit array to the bytes representation of a felt with length 251.
func (b *BitArray) SetFelt251(f *felt.Felt) *BitArray {
	b.len = 251
	b.setFelt(f)
	b.truncateToLength()
	return b
}

// Interprets the data as the big-endian bytes, sets the bit array to that value and returns it.
// If the data is larger than 32 bytes, only the first 32 bytes are used.
func (b *BitArray) SetBytes(length uint8, data []byte) *BitArray {
	b.setBytes32(data)
	b.len = length
	b.truncateToLength()
	return b
}

// Sets the bit array to the uint64 representation of a bit array.
func (b *BitArray) SetUint64(length uint8, data uint64) *BitArray {
	b.words[0] = data
	b.len = length
	b.truncateToLength()
	return b
}

// Sets the bit array to a single bit.
func (b *BitArray) SetBit(bit uint8) *BitArray {
	b.len = 1
	b.words[0] = uint64(bit & 1)
	b.words[1], b.words[2], b.words[3] = 0, 0, 0
	return b
}

// Returns the length of the encoded bit array in bytes.
func (b *BitArray) EncodedLen() uint {
	return b.byteCount() + 1
}

// Returns a deep copy of the bit array.
func (b *BitArray) Copy() BitArray {
	var res BitArray
	res.Set(b)
	return res
}

// Returns the encoded string representation of the bit array.
func (b *BitArray) EncodedString() string {
	var res []byte
	bt := b.Bytes()
	res = append(res, b.len)
	res = append(res, bt[:]...)
	return string(res)
}

// Returns a string representation of the bit array.
// This is typically used for logging or debugging.
func (b *BitArray) String() string {
	bt := b.Bytes()
	return fmt.Sprintf("(%d) %s", b.len, hex.EncodeToString(bt[:]))
}

func (b *BitArray) setFelt(f *felt.Felt) {
	res := f.Bytes()
	b.words[3] = binary.BigEndian.Uint64(res[0:8])
	b.words[2] = binary.BigEndian.Uint64(res[8:16])
	b.words[1] = binary.BigEndian.Uint64(res[16:24])
	b.words[0] = binary.BigEndian.Uint64(res[24:32])
}

func (b *BitArray) setBytes32(data []byte) {
	_ = data[31] // bound check hint, see https://golang.org/issue/14808
	b.words[3] = binary.BigEndian.Uint64(data[0:8])
	b.words[2] = binary.BigEndian.Uint64(data[8:16])
	b.words[1] = binary.BigEndian.Uint64(data[16:24])
	b.words[0] = binary.BigEndian.Uint64(data[24:32])
}

// Returns the minimum number of bytes needed to represent the bit array.
// It rounds up to the nearest byte.
func (b *BitArray) byteCount() uint {
	const bits8 = 8
	return (uint(b.len) + (bits8 - 1)) / uint(bits8)
}

// Returns a slice containing only the bytes that are actually used by the bit array,
// as specified by the length. The returned slice is in big-endian order.
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

func (b *BitArray) lsh64(x *BitArray) {
	b.words[3], b.words[2], b.words[1], b.words[0] = x.words[2], x.words[1], x.words[0], 0
}

func (b *BitArray) lsh128(x *BitArray) {
	b.words[3], b.words[2], b.words[1], b.words[0] = x.words[1], x.words[0], 0, 0
}

func (b *BitArray) lsh192(x *BitArray) {
	b.words[3], b.words[2], b.words[1], b.words[0] = x.words[0], 0, 0, 0
}

func (b *BitArray) clear() *BitArray {
	b.len = 0
	b.words[0], b.words[1], b.words[2], b.words[3] = 0, 0, 0, 0
	return b
}

// Truncates the bit array to the specified length, ensuring that any unused bits are all zeros.
//
// Example:
//
//	b := &BitArray{
//	    len: 5,
//	    words: [4]uint64{
//	        0xFFFFFFFFFFFFFFFF,  // Before: all bits are 1
//	        0x0, 0x0, 0x0,
//	    },
//	}
//	b.truncateToLength()
//	// After: only first 5 bits remain
//	// words[0] = 0x000000000000001F
//	// words[1..3] = 0x0
//
//nolint:mnd
func (b *BitArray) truncateToLength() {
	switch {
	case b.len == 0:
		b.words = [4]uint64{0, 0, 0, 0}
	case b.len <= 64:
		b.words[0] &= maxUint64 >> (64 - b.len)
		b.words[1], b.words[2], b.words[3] = 0, 0, 0
	case b.len <= 128:
		b.words[1] &= maxUint64 >> (128 - b.len)
		b.words[2], b.words[3] = 0, 0
	case b.len <= 192:
		b.words[2] &= maxUint64 >> (192 - b.len)
		b.words[3] = 0
	default:
		b.words[3] &= maxUint64 >> (256 - uint16(b.len))
	}
}

// Returns the position of the first '1' bit in the array, scanning from most significant to least significant bit.
// The bit position is counted from the least significant bit, starting at 0.
// For example:
//
//	array = 0000 0000 ... 0100 (len=251)
//	findFirstSetBit() = 3 // third bit from right is set
func findFirstSetBit(b *BitArray) uint8 {
	if b.len == 0 {
		return 0
	}

	// Start from the most significant and move towards the least significant
	for i := 3; i >= 0; i-- {
		if word := b.words[i]; word != 0 {
			return uint8((i+1)*64 - bits.LeadingZeros64(word))
		}
	}

	// All bits are zero, no set bit found
	return 0
}

// Cmp compares two bit arrays lexicographically.
// The comparison is first done by length, then by content if lengths are equal.
// Returns:
//
//	-1 if b < x
//	 0 if b == x
//	 1 if b > x
func (b *BitArray) Cmp(x *BitArray) int {
	// First compare lengths
	if b.len < x.len {
		return -1
	}
	if b.len > x.len {
		return 1
	}

	// Lengths are equal, compare the actual bits
	d0, carry := bits.Sub64(b.words[0], x.words[0], 0)
	d1, carry := bits.Sub64(b.words[1], x.words[1], carry)
	d2, carry := bits.Sub64(b.words[2], x.words[2], carry)
	d3, carry := bits.Sub64(b.words[3], x.words[3], carry)

	if carry == 1 {
		return -1
	}

	if d0|d1|d2|d3 == 0 {
		return 0
	}

	return 1
}
