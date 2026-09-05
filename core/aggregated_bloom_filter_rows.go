package core

import (
	"encoding/binary"
	"io"

	"github.com/bits-and-blooms/bitset"
)

// wordsPerFilterRow is the uint64 word count bitset writes for one
// NumBlocksPerFilter-bit row: (bits + 63) / 64.
const wordsPerFilterRow = int((NumBlocksPerFilter + 63) / 64)

const (
	filterBytesUint32 = 4
	filterBytesUint64 = 8
	// Header layout (see MarshalBinary): fromBlock + toBlock + count.
	filterHeaderSize    = 2*filterBytesUint64 + filterBytesUint32
	filterRowLenSize    = filterBytesUint32
	filterBitsetLenSize = filterBytesUint64
	// A canonical row is a bitset length prefix plus wordsPerFilterRow words.
	filterRowBlobLen = filterBitsetLenSize + wordsPerFilterRow*filterBytesUint64
	filterRowSize    = filterRowLenSize + filterRowBlobLen
)

type bloomRowsMethods interface {
	// indices are raw bloom locations, not yet reduced modulo EventsBloomLength.
	intersectRows(rawIndices []uint64, innerMatches *bitset.BitSet) error
	// unmarshalRows assumes parseHeader already validated data.
	unmarshalRows(data []byte) error
}

type bloomRows[R any] interface {
	*R
	bloomRowsMethods
}

// memRows is a decoded bit matrix: one NumBlocksPerFilter-bit row per bloom bit.
type memRows []bitset.BitSet

var _ bloomRowsMethods = (*memRows)(nil)

func (r *memRows) intersectRows(rawIndices []uint64, innerMatches *bitset.BitSet) error {
	rows := *r
	for _, index := range rawIndices {
		row := rows[index%EventsBloomLength]
		innerMatches.InPlaceIntersection(&row)
	}
	return nil
}

func (r *memRows) unmarshalRows(data []byte) error {
	backing := make([]uint64, EventsBloomLength*wordsPerFilterRow)
	rows := make(memRows, EventsBloomLength)

	// The count precheck in parseHeader guarantees room for count rows of
	// filterRowSize each, so every row's window is in bounds; no per-row
	// length check is needed.
	offset := filterHeaderSize
	for i := range EventsBloomLength {
		blobLen := int(binary.BigEndian.Uint32(data[offset:]))
		offset += filterRowLenSize
		// bitsetLen and blobLen are independent fields, so both are checked.
		// Pinning blobLen to filterRowBlobLen keeps the word reads below in bounds.
		if blobLen != filterRowBlobLen {
			return ErrBloomFilterSizeMismatch
		}
		if bitsetLen := binary.BigEndian.Uint64(data[offset:]); bitsetLen != NumBlocksPerFilter {
			return ErrBloomFilterSizeMismatch
		}

		rowStart := i * wordsPerFilterRow
		row := backing[rowStart : rowStart+wordsPerFilterRow : rowStart+wordsPerFilterRow]
		wordsAt := offset + filterBitsetLenSize
		for w := range wordsPerFilterRow {
			row[w] = binary.BigEndian.Uint64(data[wordsAt+w*filterBytesUint64:])
		}
		rows[i] = *bitset.FromWithLength(uint(NumBlocksPerFilter), row)

		offset += blobLen
	}

	// Trailing bytes mean framing corruption; a canonical blob is consumed exactly.
	if offset != len(data) {
		return io.ErrUnexpectedEOF
	}

	*r = rows
	return nil
}
