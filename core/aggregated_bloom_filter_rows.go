package core

import (
	"bytes"
	"encoding/binary"

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
	// indices are already reduced modulo EventsBloomLength.
	intersectRows(indices, acc []uint64) error
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

func (r *memRows) intersectRows(indices, acc []uint64) error {
	rows := *r
	// Constant-length reslices let the compiler drop the per-word bounds checks.
	acc = acc[:wordsPerFilterRow]
	for _, index := range indices {
		row := rows[index].Words()[:wordsPerFilterRow]
		for w := range wordsPerFilterRow {
			acc[w] &= row[w]
		}
	}
	return nil
}

func (r *memRows) unmarshalRows(data []byte) error {
	blob := blobRows(data)
	backing := make([]uint64, EventsBloomLength*wordsPerFilterRow)
	rows := make(memRows, EventsBloomLength)

	for i := range EventsBloomLength {
		words, err := blob.rowWords(uint64(i))
		if err != nil {
			return err
		}

		rowStart := i * wordsPerFilterRow
		row := backing[rowStart : rowStart+wordsPerFilterRow : rowStart+wordsPerFilterRow]
		for w := range wordsPerFilterRow {
			row[w] = binary.BigEndian.Uint64(words[w*filterBytesUint64:])
		}
		rows[i] = *bitset.FromWithLength(uint(NumBlocksPerFilter), row)
	}

	*r = rows
	return nil
}

// blobRows reads rows in place from an AggregatedBloomFilter MarshalBinary blob.
type blobRows []byte

var _ bloomRowsMethods = (*blobRows)(nil)

func (r *blobRows) intersectRows(indices, acc []uint64) error {
	blob := *r
	// Constant-length reslices let the compiler drop the per-word bounds checks.
	acc = acc[:wordsPerFilterRow]
	for _, index := range indices {
		words, err := blob.rowWords(index)
		if err != nil {
			return err
		}
		// The error-path phi hides words' fixed length, so reslice again for BCE.
		words = words[:wordsPerFilterRow*filterBytesUint64]
		for w := range wordsPerFilterRow {
			acc[w] &= binary.BigEndian.Uint64(words[w*filterBytesUint64:])
		}
	}
	return nil
}

func (r *blobRows) unmarshalRows(data []byte) error {
	// The BinaryUnmarshaler contract forbids retaining data, so keep a copy.
	*r = bytes.Clone(data)
	return nil
}

// rowWords validates row index's length prefixes and returns its raw word bytes.
func (r blobRows) rowWords(index uint64) ([]byte, error) {
	offset := filterHeaderSize + int(index)*filterRowSize
	// bitsetLen and blobLen are independent fields, so both are checked.
	if blobLen := int(binary.BigEndian.Uint32(r[offset:])); blobLen != filterRowBlobLen {
		return nil, ErrBloomFilterSizeMismatch
	}
	offset += filterRowLenSize
	if bitsetLen := binary.BigEndian.Uint64(r[offset:]); bitsetLen != NumBlocksPerFilter {
		return nil, ErrBloomFilterSizeMismatch
	}
	offset += filterBitsetLenSize
	return r[offset : offset+wordsPerFilterRow*filterBytesUint64], nil
}
