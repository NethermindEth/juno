package trie

import (
	"encoding/hex"
	"fmt"
	"io"
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

// MostSignificantBits returns a new Key containing the first n most significant bits of the original key
func (k *Key) MostSignificantBits(n uint8) (*Key, error) {
	if n > k.len {
		return nil, fmt.Errorf("cannot take %d bits from key of length %d", n, k.len)
	}

	if n == k.len {
		return k.Copy(), nil
	}

	newKey := &Key{len: n}

	// Calculate how many bytes we need to copy
	bytesToCopy := (n + 7) / 8
	if bytesToCopy > 0 {
		// Copy the required bytes from the original key
		startPos := len(k.bitset) - int((k.len+7)/8) //nolint:mnd
		copy(newKey.bitset[len(newKey.bitset)-int(bytesToCopy):], k.bitset[startPos:])
	}

	// Clear any extra bits in the last byte if necessary
	if n%8 != 0 && bytesToCopy > 0 {
		lastBytePos := len(newKey.bitset) - int(bytesToCopy)
		mask := byte(0xFF >> (8 - (n % 8))) //nolint:mnd
		newKey.bitset[lastBytePos] &= mask
	}

	// Clear any remaining bytes
	for i := 0; i < len(newKey.bitset)-int(bytesToCopy); i++ {
		newKey.bitset[i] = 0
	}

	return newKey, nil
}

func (k *Key) SubKey(n uint8) (*Key, error) {
	if n > k.len {
		return nil, fmt.Errorf("cannot subtract key of length %d from key of length %d", n, k.len)
	}
	if n == k.len {
		return &Key{}, nil
	}

	newKey := &Key{len: n}
	copy(newKey.bitset[:], k.bitset[len(k.bitset)-int((k.len+7)/8):]) //nolint:mnd

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

func (k *Key) WriteTo(buf io.Writer) (int64, error) {
	if _, err := buf.Write([]byte{k.len}); err != nil {
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

func (k *Key) RemoveMostSignificantBit() {
	if k.len == 0 {
		return
	}

	k.Truncate(k.len - 1)
}

func (k *Key) Copy() *Key {
	return &Key{len: k.len, bitset: k.bitset}
}
