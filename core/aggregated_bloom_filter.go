package core

import (
	"bytes"
	"encoding/binary"
	"errors"

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
	filterView[memRows, *memRows]
}

const (
	NumBlocksPerFilter uint64 = 8192
	// MaxBlockOffsetPerFilter is the last block a filter covers, relative to
	// its fromBlock: toBlock == fromBlock + MaxBlockOffsetPerFilter.
	MaxBlockOffsetPerFilter = NumBlocksPerFilter - 1
)

var (
	ErrAggregatedBloomFilterBlockOutOfRange error = errors.New("block number is not within range")
	ErrBloomFilterSizeMismatch              error = errors.New("bloom filter len mismatch")
	ErrMatchesBufferNil                     error = errors.New("matches buffer must not be nil")
	ErrMatchesBufferSizeMismatch            error = errors.New("matches buffer size mismatch")
)

// NewAggregatedFilter creates a new AggregatedBloomFilter starting from the specified block number.
// It initialises the bitmap array with empty bitsets of size NumBlocksPerFilter.
func NewAggregatedFilter(fromBlock uint64) AggregatedBloomFilter {
	bitmap := make(memRows, EventsBloomLength)
	for i := range bitmap {
		bitmap[i] = makeBitset()
	}

	return AggregatedBloomFilter{
		filterView: filterView[memRows, *memRows]{
			bitmap:    bitmap,
			fromBlock: fromBlock,
			toBlock:   fromBlock + MaxBlockOffsetPerFilter,
		},
	}
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
	blockMatches := bitset.New(uint(NumBlocksPerFilter))
	// memRows never fails and the buffer is correctly sized, so no error can occur.
	if err := f.BlocksForKeysInto(keys, blockMatches); err != nil {
		panic(err)
	}
	return blockMatches
}

// Copy creates a deep copy of the AggregatedBloomFilter.
func (f *AggregatedBloomFilter) Clone() AggregatedBloomFilter {
	bitmapCopy := make(memRows, len(f.bitmap))
	for i, bitset := range f.bitmap {
		bitset.CopyFull(&bitmapCopy[i])
	}

	return AggregatedBloomFilter{
		filterView: filterView[memRows, *memRows]{
			bitmap:    bitmapCopy,
			fromBlock: f.fromBlock,
			toBlock:   f.toBlock,
		},
	}
}

// MarshalBinary encodes f as a big-endian header followed by one bitset blob
// per row. Row blobs use bitset's package-global byte order (big-endian by
// default), while UnmarshalBinary hardcodes big-endian; a bitset.LittleEndian()
// call anywhere would desync the two and corrupt the DB. Left as an invariant
// since no caller flips it and a round-trip would fail immediately if one did.
// Additional context: https://github.com/NethermindEth/juno/pull/3796/changes/a04903f9e6a6fc57842ad483a6d6af0abda25451#r3592406017
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

func makeBitset() bitset.BitSet {
	b := bitset.BitSet{}
	b.Set(uint(MaxBlockOffsetPerFilter))
	b.Clear(uint(MaxBlockOffsetPerFilter))
	return b
}
