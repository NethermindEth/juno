package blake2s

import "github.com/NethermindEth/juno/core/felt"

//nolint:mnd // number represents 2**63
var smallFeltThreshold felt.Felt = felt.FromUint64[felt.Felt](0x8000000000000000)

// encodeFeltsToBytes encode each `Felt` into bytes. Returns a slice of bytes, where each
// 4 byte word is in Little Endian Notation. Additionally:
// - Small values (< 2**63) use 8 bytes
// - Large values (>= 2**63) use the full 32 bytes.
func encodeFeltsToBytes(felts ...*felt.Felt) []byte {
	encoding := encodeFeltsToUint32s(felts...)
	return encodeUint32sToBytes(encoding)
}

// encodeFeltsToUint32s encode each `Felt` into 32 bit words. Returns a slice of
// uint32 contaning all the unpacked words, in the same order
//   - Small values `< 2^63` get 2 words: `[high_32_bits, low_32_bits]`
//     from the last 8 bytes of 256-bit BE representation.
//   - Large values `>= 2^63` get 8 words: the full 32-byte big-endian
//     split, **with** the MSB of the first word set as a marker (+2^255)
func encodeFeltsToUint32s(felts ...*felt.Felt) []uint32 {
	// Multiplying the amount of felts per the amount of bytes we expect them to have
	// when small are 2 words and 8 words when bigger. There more numbers that fit into
	// 8 words than into 2, but because there is an intention of using smaller felts
	// 5 seems a sensible default
	const expectedCapMult = 5

	// MSB mark for the first u32 in the 8-limb case
	const largeFeltMarker uint32 = 1 << 31

	encoding := make([]uint32, 0, len(felts)*expectedCapMult)
	for _, f := range felts {
		fb := f.Bits()

		// #nosec G602 // False positive. fb is always [4]uint64
		if f.Cmp(&smallFeltThreshold) < 0 {
			val := fb[0]
			encoding = append(encoding, uint32(val>>32), uint32(val))
		} else {
			start := len(encoding)

			encoding = append(
				encoding,
				uint32(fb[3]>>32), uint32(fb[3]),
				uint32(fb[2]>>32), uint32(fb[2]),
				uint32(fb[1]>>32), uint32(fb[1]),
				uint32(fb[0]>>32), uint32(fb[0]),
			)

			encoding[start] |= largeFeltMarker
		}
	}

	return encoding
}

// encodeUint32sToBytes encodes a stream of uint32 into it's little endian
// byte representation
func encodeUint32sToBytes(values []uint32) []byte {
	bytes := make([]byte, len(values)*4)
	for i, val := range values {
		bytes[i*4] = byte(val)
		bytes[i*4+1] = byte(val >> 8)
		bytes[i*4+2] = byte(val >> 16)
		bytes[i*4+3] = byte(val >> 24)
	}
	return bytes
}
