package records

import (
	"encoding/binary"
	"errors"

	"github.com/NethermindEth/juno/utils/cbor/fast"
	"github.com/bits-and-blooms/bitset"
	"github.com/bits-and-blooms/bloom/v3"
)

// ErrBloomSize means a stored bloom filter declares more bits than its bytes hold.
var ErrBloomSize = errors.New("records: bloom filter is shorter than it declares")

// bloomHeader is the size of m, k and the bit count, which come before the words.
const bloomHeader = 3 * 8

// readBloom reads a bloom filter, stored as its binary form in a byte string.
func readBloom(r *fast.Reader, dst **bloom.BloomFilter) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	data, err := r.BytesNoCopy()
	if err != nil {
		return err
	}
	filter, err := decodeBloom(data)
	if err != nil {
		return err
	}
	*dst = filter
	return nil
}

// decodeBloom builds the filter UnmarshalBinary would from data, without the stream and
// the read buffers UnmarshalBinary goes through. It copies what it keeps.
//
// The form is: m (8 bytes), k (8), the bit count (8), then the words of the bit set, all
// in the bit set package's byte order (big endian unless a program changes it). Bytes after
// the words are ignored, as UnmarshalBinary ignores them.
func decodeBloom(data []byte) (*bloom.BloomFilter, error) {
	if !bloomSizeMatches(data) {
		return nil, ErrBloomSize
	}
	bits := binary.BigEndian.Uint64(data[2*8:])
	// bitset.From gives the set a whole number of words, so a count that is not a multiple
	// of 64, or another byte order, takes the bloom package's own path.
	if bits%64 != 0 || bitset.BinaryOrder() != binary.BigEndian {
		filter := new(bloom.BloomFilter)
		if err := filter.UnmarshalBinary(data); err != nil {
			return nil, err
		}
		return filter, nil
	}

	var words []uint64 // nil when there are none, as UnmarshalBinary leaves it
	if n := bits / 64; n != 0 {
		words = make([]uint64, n)
		for i := range words {
			words[i] = binary.BigEndian.Uint64(data[bloomHeader+8*i:])
		}
	}
	m := binary.BigEndian.Uint64(data)
	k := binary.BigEndian.Uint64(data[8:])
	return bloom.FromWithM(words, uint(m), uint(k)), nil
}

// bloomSizeMatches checks the bit count the binary form declares against the bytes that
// follow it. The bloom package allocates what the count says before it reads anything, so a
// damaged count would otherwise ask for terabytes.
func bloomSizeMatches(data []byte) bool {
	if len(data) < bloomHeader {
		return false
	}
	bits := binary.BigEndian.Uint64(data[2*8:])
	return bits <= uint64(len(data)-bloomHeader)*8
}
