package trie

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/NethermindEth/juno/core/felt"
)

var NilKey = &Key{len: 0, bitset: [32]byte{}}

type Key struct {
	len    uint8
	bitset [32]byte
}

func NewKey(length uint8, keyBytes []byte) Key {
	k := Key{len: length}
	if len(keyBytes) > len(k.bitset) {
		panic("bytes does not fit in bitset")
	}
	copy(k.bitset[len(k.bitset)-len(keyBytes):], keyBytes)
	return k
}

func (k *Key) SubKey(n uint8) (*Key, error) {
	panic("TODO(weiihann): not used")
}

func (k *Key) bytesNeeded() uint {
	const byteBits = 8
	return (uint(k.len) + (byteBits - 1)) / byteBits
}

func (k *Key) inUseBytes() []byte {
	return k.bitset[len(k.bitset)-int(k.bytesNeeded()):]
}

func (k *Key) unusedBytes() []byte {
	return k.bitset[:len(k.bitset)-int(k.bytesNeeded())]
}

func (k *Key) WriteTo(buf *bytes.Buffer) (int64, error) {
	if err := buf.WriteByte(k.len); err != nil {
		return 0, err
	}

	n, err := buf.Write(k.inUseBytes())
	return int64(1 + n), err
}

func (k *Key) UnmarshalBinary(data []byte) error {
	k.len = data[0]
	k.bitset = [32]byte{}
	copy(k.inUseBytes(), data[1:1+k.bytesNeeded()])
	return nil
}

func (k *Key) EncodedLen() uint {
	return k.bytesNeeded() + 1
}

func (k *Key) Len() uint8 {
	return k.len
}

func (k *Key) Felt() felt.Felt {
	var f felt.Felt
	f.SetBytes(k.bitset[:])
	return f
}

func (k *Key) Equal(other *Key) bool {
	if k == nil && other == nil {
		return true
	} else if k == nil || other == nil {
		return false
	}
	return k.len == other.len && k.bitset == other.bitset
}

// IsBitSet returns whether the bit at the given position is 1.
// Position 0 represents the least significant (rightmost) bit.
func (k *Key) IsBitSet(position uint8) bool {
	const LSB = uint8(0x1)
	byteIdx := position / 8
	byteAtIdx := k.bitset[len(k.bitset)-int(byteIdx)-1]
	bitIdx := position % 8
	return ((byteAtIdx >> bitIdx) & LSB) != 0
}

// ShiftRight removes n least significant bits from the key by performing a right shift
// operation and reducing the key length. For example, if the key contains bits
// "1111 0000" (length=8) and n=4, the result will be "1111" (length=4).
//
// The operation is destructive - it modifies the key in place.
func (k *Key) ShiftRight(n uint8) {
	if k.len < n {
		panic("deleting more bits than there are")
	}

	if n == 0 {
		return
	}

	var bigInt big.Int
	bigInt.SetBytes(k.bitset[:])
	bigInt.Rsh(&bigInt, uint(n))
	bigInt.FillBytes(k.bitset[:])
	k.len -= n
}

func (k *Key) MostSignificantBits(n uint8) (*Key, error) {
	if n > k.len {
		return nil, fmt.Errorf("cannot get more bits than the key length")
	}

	keyCopy := k.Copy()
	keyCopy.ShiftRight(k.len - n)
	return &keyCopy, nil
}

// Truncate truncates key to `length` bits by clearing the remaining upper bits
func (k *Key) Truncate(length uint8) {
	k.len = length

	unusedBytes := k.unusedBytes()
	clear(unusedBytes)

	// clear upper bits on the last used byte
	inUseBytes := k.inUseBytes()
	unusedBitsCount := 8 - (k.len % 8)
	if unusedBitsCount != 8 && len(inUseBytes) > 0 {
		inUseBytes[0] = (inUseBytes[0] << unusedBitsCount) >> unusedBitsCount
	}
}

func (k *Key) String() string {
	return fmt.Sprintf("(%d) %s", k.len, hex.EncodeToString(k.bitset[:]))
}

// Copy returns a deep copy of the key
func (k *Key) Copy() Key {
	newKey := Key{len: k.len}
	copy(newKey.bitset[:], k.bitset[:])
	return newKey
}

// findCommonKey finds the set of common MSB bits in two key bitsets.
func findCommonKey(longerKey, shorterKey *Key) (Key, bool) {
	divergentBit := findDivergentBit(longerKey, shorterKey)
	commonKey := *shorterKey
	commonKey.ShiftRight(shorterKey.Len() - divergentBit + 1)
	return commonKey, divergentBit == shorterKey.Len()+1
}

func findDivergentBit(longerKey, shorterKey *Key) uint8 {
	divergentBit := uint8(0)
	for divergentBit <= shorterKey.Len() &&
		longerKey.IsBitSet(longerKey.Len()-divergentBit) == shorterKey.IsBitSet(shorterKey.Len()-divergentBit) {
		divergentBit++
	}
	return divergentBit
}

func isSubset(longerKey, shorterKey *Key) bool {
	divergentBit := findDivergentBit(longerKey, shorterKey)
	return divergentBit == shorterKey.Len()+1
}
