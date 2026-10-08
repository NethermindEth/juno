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
		readWords(words, data[bloomHeader:])
	}
	m := binary.BigEndian.Uint64(data)
	k := binary.BigEndian.Uint64(data[8:])
	return bloom.FromWithM(words, uint(m), uint(k)), nil
}

// readWords fills dst with the big endian words at the start of src, which holds at least
// as many. Eight words a round let the compiler drop the bounds checks, which makes it about
// four times as fast as a word at a time.
func readWords(dst []uint64, src []byte) {
	src = src[:8*len(dst)]
	for len(dst) >= 8 && len(src) >= 64 {
		dst[0] = binary.BigEndian.Uint64(src[0:8])
		dst[1] = binary.BigEndian.Uint64(src[8:16])
		dst[2] = binary.BigEndian.Uint64(src[16:24])
		dst[3] = binary.BigEndian.Uint64(src[24:32])
		dst[4] = binary.BigEndian.Uint64(src[32:40])
		dst[5] = binary.BigEndian.Uint64(src[40:48])
		dst[6] = binary.BigEndian.Uint64(src[48:56])
		dst[7] = binary.BigEndian.Uint64(src[56:64])
		dst, src = dst[8:], src[64:]
	}
	for i := range dst {
		dst[i] = binary.BigEndian.Uint64(src[8*i:])
	}
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
