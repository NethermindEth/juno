package trie

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/NethermindEth/juno/core/felt"
)

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
	if n > k.len {
		return nil, errors.New(fmt.Sprint("cannot subtract key of length %i from key of length %i", n, k.len))
	}

	newKey := &Key{len: n}
	copy(newKey.bitset[:], k.bitset[len(k.bitset)-int((k.len+7)/8):]) //nolint:gomnd

	// Shift right by the number of bits that are not needed
	shift := k.len - n
	for i := len(newKey.bitset) - 1; i >= 0; i-- {
		newKey.bitset[i] >>= shift
		if i > 0 {
			newKey.bitset[i] |= newKey.bitset[i-1] << (8 - shift)
		}
	}

	return newKey, nil
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

func (k *Key) Test(bit uint8) bool {
	const LSB = uint8(0x1)
	byteIdx := bit / 8
	byteAtIdx := k.bitset[len(k.bitset)-int(byteIdx)-1]
	bitIdx := bit % 8
	return ((byteAtIdx >> bitIdx) & LSB) != 0
}

func (k *Key) String() string {
	return fmt.Sprintf("(%d) %s", k.len, hex.EncodeToString(k.bitset[:]))
}

// DeleteLSB right shifts and shortens the key
func (k *Key) DeleteLSB(n uint8) {
	if k.len < n {
		panic("deleting more bits than there are")
	}

	var bigInt big.Int
	bigInt.SetBytes(k.bitset[:])
	bigInt.Rsh(&bigInt, uint(n))
	bigInt.FillBytes(k.bitset[:])
	k.len -= n
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

func (k *Key) RemoveLastBit() {
	if k.len == 0 {
		return
	}

	k.len--

	unusedBytes := k.unusedBytes()
	clear(unusedBytes)

	// clear upper bits on the last used byte
	inUseBytes := k.inUseBytes()
	unusedBitsCount := 8 - (k.len % 8)
	if unusedBitsCount != 8 && len(inUseBytes) > 0 {
		inUseBytes[0] = (inUseBytes[0] << unusedBitsCount) >> unusedBitsCount
	}
}
