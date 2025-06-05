package core

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"

	"github.com/bits-and-blooms/bitset"
	"github.com/bits-and-blooms/bloom/v3"
)

// AggregatedBloomFilter provides a space-efficient, probabilistic data structure for
// testing set membership of keys (such as event topics or contract addresses) across
// large block ranges in a blockchain.
//
// When querying which blocks in a large range might contain a certain key, it is
// inefficient to load and individually check every block’s Bloom filter. To optimise
// this, AggregatedBloomFilter aggregates multiple Bloom filters (spanning a range of
// blocks) into a single structure. This aggregation makes it possible to check, in
// a single operation, which blocks in the range might include a given key.
//
// Internally, AggregatedBloomFilter is represented as a bit matrix: each row corresponds
// to a Bloom filter index, and each column corresponds to a block in the range.
// When adding a key for a particular block, the indices mapped by Bloom hash functions are determined,
// and the bits at those row-column intersections are set for that block.
//
// Visually, this can be thought of as "rotating" the per-block Bloom filters into columns of a matrix.
//
// -----| Block 0 | Block 1 | Block 2 | ... | Block 9 |
// Idx0 |   0     |    0    |    0    | ... |   0     |
// Idx1 |   1     |    0    |    1    | ... |   0     |
// Idx2 |   0     |    1    |    0    | ... |   0     |
// Idx3 |   1     |    0    |    0    | ... |   0     |
// Idx4 |   1     |    0    |    1    | ... |   1     |
// Idx5 |   0     |    0    |    0    | ... |   0     |
// Idx6 |   0     |    0    |    1    | ... |   0     |
// Idx7 |   0     |    1    |    0    | ... |   0     |
//
// To query for a key, the AggregatedBloomFilter:
//
//  1. Determines the relevant indices for the key using the same hash functions.
//  2. Performs a bitwise AND over the selected rows, producing a bit vector.
//  3. The set bits in this result indicate block numbers within the filter's range
//     where the key may be present (with the usual caveat of possible false positives).
//     Note: The set bit positions are *relative to the filter's range start* (i.e., the
//     range's first block number), not absolute global block numbers.
//
// Query example for a key mapping to indices Idx1 and Idx4:
//
// Select rows 1 and 4 (Idx1 & Idx4):
// Idx1:    1    0    1   ...   0
// Idx4:    1    0    1   ...   1
//
// -------------------------------
// AND:     1    0    1   ...   0
//
// After AND: Resulting vector is 1 0 1 ... 0
//
// This means Block 0 and Block 2 are possible matches for this key.
//
// This approach allows for efficient, bulk event queries on blockchain data
// without needing to individually examine every single block’s Bloom filter.
//
// Using this method, you can quickly identify candidate blocks for a key, improving
// the performance of large-range event queries.
type AggregatedBloomFilter struct {
	bitmap    []bitset.BitSet
	fromBlock uint64
	toBlock   uint64
}

const (
	AggregateBloomBlockRangeLen uint64 = 8192
)

var (
	ErrAggregatedBloomFilterBlockOutOfRange error = errors.New("block number is not within range")
	ErrBloomFilterSizeMismatch              error = errors.New("bloom filter len mismatch")
	ErrMatchesBufferNil                     error = errors.New("matches buffer must not be nil")
	ErrMatchesBufferSizeMismatch            error = errors.New("matches buffer size mismatch")
)

// NewAggregatedFilter creates a new AggregatedBloomFilter starting from the specified block number.
// It initialises the bitmap array with empty bitsets of size AggregateBloomBlockRangeLen.
func NewAggregatedFilter(fromBlock uint64) *AggregatedBloomFilter {
	bitmap := make([]bitset.BitSet, EventsBloomLength)
	for i := range bitmap {
		bitmap[i] = *bitset.New(uint(AggregateBloomBlockRangeLen))
	}

	return &AggregatedBloomFilter{
		bitmap:    bitmap,
		fromBlock: fromBlock,
		toBlock:   fromBlock + AggregateBloomBlockRangeLen - 1,
	}
}

// FromBlock returns the starting block number of the filter's range.
func (f *AggregatedBloomFilter) FromBlock() uint64 {
	return f.fromBlock
}

// ToBlock returns the ending block number of the filter's range.
func (f *AggregatedBloomFilter) ToBlock() uint64 {
	return f.toBlock
}

// Insert adds a bloom filter's data for a specific block number into the aggregated filter.
// If filter is nil, no-op.
// Returns an error if the block number is out of range or if the bloom filter size doesn't match.
func (f *AggregatedBloomFilter) Insert(filter *bloom.BloomFilter, blockNumber uint64) error {
	if f.fromBlock > blockNumber || f.toBlock < blockNumber {
		return ErrAggregatedBloomFilterBlockOutOfRange
	}

	if filter == nil {
		return nil
	}

	bitmap := filter.BitSet()
	if bitmap.Len() != EventsBloomLength {
		return ErrBloomFilterSizeMismatch
	}

	setBitIndices := make([]uint, bitmap.Count())
	bitmap.NextSetMany(0, setBitIndices)
	relativeBlockNumber := blockNumber - f.fromBlock

	for _, index := range setBitIndices {
		f.bitmap[index].Set(uint(relativeBlockNumber))
	}

	return nil
}

// Clears the bloom filter of given block.
// Returns an error if the block number is out of range
func (f *AggregatedBloomFilter) clear(blockNumber uint64) error {
	if f.fromBlock > blockNumber || f.toBlock < blockNumber {
		return ErrAggregatedBloomFilterBlockOutOfRange
	}

	relativeBlockNumber := blockNumber - f.fromBlock

	for index := range EventsBloomLength {
		f.bitmap[index].Clear(uint(relativeBlockNumber))
	}

	return nil
}

// BlocksForKeys returns a bitset indicating which blocks within the range might contain
// the given keys. If no keys are provided, returns a bitset with all bits set.
func (f *AggregatedBloomFilter) BlocksForKeys(keys [][]byte) *bitset.BitSet {
	blockMatches := bitset.New(uint(AggregateBloomBlockRangeLen))
	if len(keys) == 0 {
		return blockMatches.SetAll()
	}

	innerMatches := bitset.New(uint(AggregateBloomBlockRangeLen))
	for _, key := range keys {
		innerMatches.SetAll()
		rawIndices := bloom.Locations(key, EventsBloomHashFuncs)

		for _, index := range rawIndices {
			row := f.bitmap[index%EventsBloomLength]
			innerMatches.InPlaceIntersection(&row)
		}

		blockMatches.InPlaceUnion(innerMatches)
	}
	return blockMatches
}

// BlocksForKeysInto reuses a preallocated bitset (should be AggregateBloomBlockRangeLen bits).
func (f *AggregatedBloomFilter) BlocksForKeysInto(keys [][]byte, out *bitset.BitSet) error {
	if out == nil {
		return ErrMatchesBufferNil
	}

	if out.Len() != uint(AggregateBloomBlockRangeLen) {
		return ErrMatchesBufferSizeMismatch
	}

	if len(keys) == 0 {
		out.SetAll()
		return nil
	}

	out.ClearAll()
	innerMatches := bitset.New(uint(AggregateBloomBlockRangeLen))
	for _, key := range keys {
		innerMatches.SetAll()
		rawIndices := bloom.Locations(key, EventsBloomHashFuncs)
		for _, index := range rawIndices {
			row := f.bitmap[index%EventsBloomLength]
			innerMatches.InPlaceIntersection(&row)
		}
		out.InPlaceUnion(innerMatches)
	}

	return nil
}

// Copy creates a deep copy of the AggregatedBloomFilter.
func (f *AggregatedBloomFilter) Copy() *AggregatedBloomFilter {
	bitmapCopy := make([]bitset.BitSet, len(f.bitmap))
	for i, bitset := range f.bitmap {
		bitset.CopyFull(&bitmapCopy[i])
	}

	return &AggregatedBloomFilter{
		bitmap:    bitmapCopy,
		fromBlock: f.fromBlock,
		toBlock:   f.toBlock,
	}
}

func (f *AggregatedBloomFilter) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer

	if err := binary.Write(&buf, binary.BigEndian, f.fromBlock); err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.BigEndian, f.toBlock); err != nil {
		return nil, err
	}

	count := uint32(len(f.bitmap))
	if err := binary.Write(&buf, binary.BigEndian, count); err != nil {
		return nil, err
	}

	for _, bs := range f.bitmap {
		b, err := bs.MarshalBinary()
		if err != nil {
			return nil, err
		}

		length := uint32(len(b))
		if err := binary.Write(&buf, binary.BigEndian, length); err != nil {
			return nil, err
		}

		if _, err := buf.Write(b); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func (f *AggregatedBloomFilter) UnmarshalBinary(data []byte) error {
	r := bytes.NewReader(data)

	if err := binary.Read(r, binary.BigEndian, &f.fromBlock); err != nil {
		return err
	}
	if err := binary.Read(r, binary.BigEndian, &f.toBlock); err != nil {
		return err
	}

	var count uint32
	if err := binary.Read(r, binary.BigEndian, &count); err != nil {
		return err
	}

	f.bitmap = make([]bitset.BitSet, count)
	for i := range count {
		var length uint32
		if err := binary.Read(r, binary.BigEndian, &length); err != nil {
			return err
		}

		b := make([]byte, length)
		if _, err := io.ReadFull(r, b); err != nil {
			return err
		}

		if err := f.bitmap[i].UnmarshalBinary(b); err != nil {
			return err
		}
	}
	return nil
}
