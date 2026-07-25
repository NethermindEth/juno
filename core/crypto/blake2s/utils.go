package blake2s

import (
	"encoding/binary"

	"github.com/NethermindEth/juno/core/felt"
)

const (
	//nolint:mnd // number represents 2**63
	smallFeltThresholdLow uint64 = 0x8000000000000000

	// maxWordsPerFelt is the worst-case number of u32 words a single felt encodes
	// to (large felts use eight words; small ones use two).
	maxWordsPerFelt int = 8
)

// encodeFelts encodes felts as little-endian u32 words.
// Small values (< 2**63) use two words.
// Larger values use eight words with a marker bit on the first word.
func encodeFelts(felts ...*felt.Felt) []byte {
	buf := make([]byte, len(felts)*maxWordsPerFelt*4)
	return buf[:encodeFeltsInto(buf, felts...)]
}

// encodeFeltsInto encodes felts into data and returns the number of bytes written.
func encodeFeltsInto(data []byte, felts ...*felt.Felt) int {
	offset := 0
	for _, f := range felts {
		offset += encodeFeltInto(data[offset:], f)
	}
	return offset
}

// encodeFeltInto encodes a single felt into data and returns the number of bytes written.
func encodeFeltInto(data []byte, f *felt.Felt) int {
	const largeFeltMarker uint32 = 1 << 31

	// Bits() leaves Montgomery form once.
	// Comparing limbs directly avoids Cmp,
	// which would call Bits() again on both operands.
	fb := f.Bits()
	if fb[3] == 0 && fb[2] == 0 && fb[1] == 0 && fb[0] < smallFeltThresholdLow {
		val := fb[0]
		binary.LittleEndian.PutUint32(data, uint32(val>>32))
		binary.LittleEndian.PutUint32(data[4:], uint32(val))
		return 8
	}

	// Each limb is written as two little-endian u32 words, high word first.
	// Word order across the whole felt is most-significant to least-significant.
	binary.LittleEndian.PutUint32(data, uint32(fb[3]>>32)|largeFeltMarker)
	binary.LittleEndian.PutUint32(data[4:], uint32(fb[3]))
	binary.LittleEndian.PutUint32(data[8:], uint32(fb[2]>>32))
	binary.LittleEndian.PutUint32(data[12:], uint32(fb[2]))
	binary.LittleEndian.PutUint32(data[16:], uint32(fb[1]>>32))
	binary.LittleEndian.PutUint32(data[20:], uint32(fb[1]))
	binary.LittleEndian.PutUint32(data[24:], uint32(fb[0]>>32))
	binary.LittleEndian.PutUint32(data[28:], uint32(fb[0]))
	return 32
}
