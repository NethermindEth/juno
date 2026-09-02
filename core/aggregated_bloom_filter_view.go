package core

import (
	"encoding/binary"
	"io"

	"github.com/bits-and-blooms/bitset"
	"github.com/bits-and-blooms/bloom/v3"
)

// AggregatedBloomFilterView reads an AggregatedBloomFilter's MarshalBinary
// blob in place, decoding only the rows a query's keys hash to.
type AggregatedBloomFilterView = filterView[blobRows, *blobRows]

// filterView is the shared core: a block range plus key queries over rows R.
type filterView[R any, PR bloomRows[R]] struct {
	bitmap    R
	fromBlock uint64
	toBlock   uint64
}

// FromBlock returns the starting block number of the filter's range.
func (v *filterView[R, PR]) FromBlock() uint64 {
	return v.fromBlock
}

// ToBlock returns the ending block number of the filter's range.
func (v *filterView[R, PR]) ToBlock() uint64 {
	return v.toBlock
}

// BlocksForKeysInto reuses a preallocated bitset (should be NumBlocksPerFilter bits).
func (v *filterView[R, PR]) BlocksForKeysInto(keys [][]byte, out *bitset.BitSet) error {
	if out == nil {
		return ErrMatchesBufferNil
	}

	if out.Len() != uint(NumBlocksPerFilter) {
		return ErrMatchesBufferSizeMismatch
	}

	if len(keys) == 0 {
		out.SetAll()
		return nil
	}

	out.ClearAll()
	innerWords := make([]uint64, wordsPerFilterRow)
	// innerMatches shares innerWords as backing.
	innerMatches := bitset.FromWithLength(uint(NumBlocksPerFilter), innerWords)
	rows := PR(&v.bitmap)
	for _, key := range keys {
		innerMatches.SetAll()
		indices := bloom.Locations(key, EventsBloomHashFuncs)
		for i := range indices {
			indices[i] %= EventsBloomLength
		}
		if err := rows.intersectRows(indices, innerWords); err != nil {
			return err
		}
		out.InPlaceUnion(innerMatches)
	}

	return nil
}

// UnmarshalBinary decodes MarshalBinary's output in place. Returns an error
// in case of DB corruption. On error v may be partially written and must be
// discarded.
func (v *filterView[R, PR]) UnmarshalBinary(data []byte) error {
	if err := v.parseHeader(data); err != nil {
		return err
	}
	return PR(&v.bitmap).unmarshalRows(data)
}

// parseHeader validates the blob framing and sets v's block range.
func (v *filterView[R, PR]) parseHeader(data []byte) error {
	if len(data) < filterHeaderSize {
		return io.ErrUnexpectedEOF
	}

	count := int(binary.BigEndian.Uint32(data[2*filterBytesUint64 : filterHeaderSize]))
	// Consumers index the matrix with these fixed constants, so a header that
	// disagrees would panic or silently corrupt results on later use.
	if count != EventsBloomLength {
		return ErrBloomFilterSizeMismatch
	}
	// Rows are fixed-size, so a canonical blob has exactly this length; anything
	// else is framing corruption. Row length prefixes are still checked per row.
	if len(data) != filterHeaderSize+count*filterRowSize {
		return io.ErrUnexpectedEOF
	}

	v.fromBlock = binary.BigEndian.Uint64(data[0:filterBytesUint64])
	v.toBlock = binary.BigEndian.Uint64(data[filterBytesUint64 : 2*filterBytesUint64])
	if v.toBlock != v.fromBlock+MaxBlockOffsetPerFilter {
		return ErrBloomFilterSizeMismatch
	}

	return nil
}
