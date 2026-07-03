package blake2s

import "github.com/NethermindEth/juno/core/felt"

//nolint:mnd // number represents 2**63
const smallFeltThresholdLow uint64 = 0x8000000000000000

// encodeFeltsToBytes encodes felts as little-endian u32 words.
// Small values (< 2**63) use two words.
// Larger values use eight words with a marker bit on the first word.
func encodeFeltsToBytes(felts ...*felt.Felt) []byte {
	const expectedCapMult = 5
	buf := make([]byte, 0, len(felts)*expectedCapMult*4)
	return appendFeltsToBytes(buf, felts...)
}

func appendFeltsToBytes(buf []byte, felts ...*felt.Felt) []byte {
	for _, f := range felts {
		buf = appendFeltBytes(buf, f)
	}
	return buf
}

func appendFeltBytes(buf []byte, f *felt.Felt) []byte {
	const largeFeltMarker uint32 = 1 << 31

	// Bits() leaves Montgomery form once.
	// Comparing limbs directly avoids Cmp,
	// which would call Bits() again on both operands.
	fb := f.Bits()
	if fb[3] == 0 && fb[2] == 0 && fb[1] == 0 && fb[0] < smallFeltThresholdLow {
		val := fb[0]
		buf = appendUint32LE(buf, uint32(val>>32))
		buf = appendUint32LE(buf, uint32(val))
		return buf
	}

	// Word order is most-significant to least-significant.
	// appendUint32LE handles the byte order within each word.
	buf = appendUint32LE(buf, uint32(fb[3]>>32)|largeFeltMarker)
	buf = appendUint32LE(buf, uint32(fb[3]))
	buf = appendUint32LE(buf, uint32(fb[2]>>32))
	buf = appendUint32LE(buf, uint32(fb[2]))
	buf = appendUint32LE(buf, uint32(fb[1]>>32))
	buf = appendUint32LE(buf, uint32(fb[1]))
	buf = appendUint32LE(buf, uint32(fb[0]>>32))
	buf = appendUint32LE(buf, uint32(fb[0]))
	return buf
}

// LE is shorthand for little endian.
func appendUint32LE(buf []byte, val uint32) []byte {
	return append(buf, byte(val), byte(val>>8), byte(val>>16), byte(val>>24))
}
