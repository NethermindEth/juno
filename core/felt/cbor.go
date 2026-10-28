package felt

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
)

// A felt has two CBOR forms. The legacy form is an array of the 4 Montgomery limbs, each a
// minimally sized uint. The compact form is a uint below 24, a byte string of up to 30 trimmed
// big-endian bytes, or a 32-byte string of Montgomery limbs for values whose canonical form needs
// 31 or 32 bytes, so the length alone tells the forms apart. Every build decodes both; the
// compactfelt build tag picks which form MarshalCBOR and appendCBOR write, and whether StoredBytes
// trims standalone values (cbor_compact.go or cbor_legacy.go). SetMinimalBytes reads both.

var (
	_ cbor.SelfEncoder = (*Felt)(nil)
	_ cbor.SelfDecoder = (*Felt)(nil)
)

// UnmarshalCBOR decodes a compact or legacy felt that spans all of data, writing z only on success.
func (z *Typed[K]) UnmarshalCBOR(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("decoding felt: %w", errFeltEmpty)
	}
	if !isCompactCBOR(data[0]) {
		return unmarshalLegacyCBOR(data, (*fp.Element)(z))
	}

	var decoded fp.Element
	consumed, err := decodeCompact(data, &decoded)
	if err != nil {
		return fmt.Errorf("decoding felt: %w", err)
	}
	if consumed != len(data) {
		return fmt.Errorf("decoding felt: %w", errFeltTrailingBytes)
	}
	*(*fp.Element)(z) = decoded
	return nil
}

const (
	// These derive from the CBOR spec, see https://www.rfc-editor.org/rfc/rfc8949.html#section-3
	cborUint8AdditionalInfo  = 24 // 1 byte follows
	cborUint16AdditionalInfo = 25 // 2 bytes follow
	cborUint32AdditionalInfo = 26 // 4 bytes follow
	cborUint64AdditionalInfo = 27 // 8 bytes follow

	// The top 3 bits of a CBOR header byte are the major type, the low 5 the additional info.
	cborMajorTypeMask      = 0b1110_0000
	cborAdditionalInfoMask = 0b0001_1111

	cborUintMajor       = 0 << 5
	cborByteStringMajor = 2 << 5
	cborArrayMajor      = 4 << 5

	// cborLegacyArrayHeader is the first byte of a legacy felt: an array of Limbs items.
	cborLegacyArrayHeader = cborArrayMajor | Limbs

	// Header + 8 bytes following
	maxCBORUintLen = 1 + 8
	// Header + Limbs * maxCBORUintLen
	maxCBORLegacyFeltLen = 1 + Limbs*maxCBORUintLen

	// maxCanonicalByteLen is the longest canonical byte string; longer values use Montgomery form.
	maxCanonicalByteLen = feltByteLen - 2
	// A compact value below 24 is a single header byte
	minCBORFeltLen = 1
)

var (
	errFeltTrailingBytes  = errors.New("trailing bytes")
	errFeltTruncated      = errors.New("truncated byte string")
	errFeltUintTooLarge   = errors.New("uint of 24 or more must be a byte string")
	errFeltSmallBytes     = errors.New("byte string holds a value below 24")
	errFeltLeadingZero    = errors.New("byte string has a leading zero byte")
	errFeltLengthHeader   = errors.New("non-minimal byte string length")
	errFeltLengthEncoding = errors.New("unsupported byte string length encoding")
	errFeltReservedLength = errors.New("31-byte byte string is reserved")
	errFeltUnexpectedType = errors.New("unexpected CBOR major type")
)

// smallMontgomery holds the Montgomery form of every value a one-byte CBOR uint can carry, so
// decoding one is a copy instead of a field multiplication.
var smallMontgomery = func() [cborUint8AdditionalInfo]fp.Element {
	var table [cborUint8AdditionalInfo]fp.Element
	for index := range table {
		table[index].SetUint64(uint64(index))
	}
	return table
}()

// modulusLimbs holds the field modulus as little-endian limbs.
var modulusLimbs = func() [Limbs]uint64 {
	var buffer [feltByteLen]byte
	fp.Modulus().FillBytes(buffer[:])
	return [Limbs]uint64{
		binary.BigEndian.Uint64(buffer[24:32]),
		binary.BigEndian.Uint64(buffer[16:24]),
		binary.BigEndian.Uint64(buffer[8:16]),
		binary.BigEndian.Uint64(buffer[0:8]),
	}
}()

// unmarshalLegacyCBOR decodes a Montgomery-limb felt, falling back to the generic decoder.
func unmarshalLegacyCBOR(data []byte, value *fp.Element) error {
	var decoded fp.Element
	if consumed, ok := decodeLegacyLimbs(data, &decoded); ok && consumed == len(data) {
		*value = decoded
		return nil
	}
	if err := cbor.Unmarshal(data, value); err != nil {
		return fmt.Errorf("decoding legacy felt: %w", err)
	}
	return nil
}

// isCompactCBOR reports whether header starts a compact felt rather than a legacy one.
func isCompactCBOR(header byte) bool {
	majorType := header & cborMajorTypeMask
	return majorType == cborUintMajor || majorType == cborByteStringMajor
}

// decodeCompact decodes one compact felt at data[0:], returning the number of bytes consumed.
// It accepts only the single compact encoding of each value.
func decodeCompact(data []byte, value *fp.Element) (int, error) {
	if len(data) == 0 {
		return 0, errFeltEmpty
	}

	header := data[0]
	additionalInfo := header & cborAdditionalInfoMask
	switch header & cborMajorTypeMask {
	case cborUintMajor:
		// Values below 24 are the header byte itself; wider uints are never written, so a
		// multi-byte uint header is rejected rather than decoded.
		if additionalInfo >= cborUint8AdditionalInfo {
			return 0, errFeltUintTooLarge
		}
		*value = smallMontgomery[additionalInfo]
		return 1, nil
	case cborByteStringMajor:
		length, offset, err := decodeByteStringHeader(data)
		if err != nil {
			return 0, err
		}
		if length > feltByteLen {
			return 0, errFeltTooLong
		}
		if len(data)-offset < length {
			return 0, errFeltTruncated
		}
		if err := decodeByteStringBody(data[offset:offset+length], value); err != nil {
			return 0, err
		}
		return offset + length, nil
	default:
		return 0, errFeltUnexpectedType
	}
}

// decodeByteStringHeader reads the minimal length header of the byte string at data[0:],
// returning the body length and the header size.
func decodeByteStringHeader(data []byte) (length, offset int, err error) {
	additionalInfo := data[0] & cborAdditionalInfoMask
	switch {
	case additionalInfo < cborUint8AdditionalInfo:
		return int(additionalInfo), 1, nil
	case additionalInfo == cborUint8AdditionalInfo && len(data) >= 2:
		if data[1] < cborUint8AdditionalInfo {
			return 0, 0, errFeltLengthHeader
		}
		return int(data[1]), 2, nil
	case additionalInfo == cborUint8AdditionalInfo:
		return 0, 0, errFeltTruncated
	default:
		// Wider length headers and indefinite lengths never hold a felt
		return 0, 0, errFeltLengthEncoding
	}
}

// decodeByteStringBody decodes a felt byte string body, telling Montgomery and canonical values
// apart by length.
func decodeByteStringBody(body []byte, value *fp.Element) error {
	length := len(body)
	switch {
	case length == feltByteLen:
		montgomery := [Limbs]uint64{
			binary.BigEndian.Uint64(body[24:32]),
			binary.BigEndian.Uint64(body[16:24]),
			binary.BigEndian.Uint64(body[8:16]),
			binary.BigEndian.Uint64(body[0:8]),
		}
		if !belowModulus(&montgomery) {
			return errFeltOutOfRange
		}
		*value = fp.Element(montgomery)
		return nil
	case length > maxCanonicalByteLen:
		return errFeltReservedLength
	case length == 0 || (length == 1 && body[0] < cborUint8AdditionalInfo):
		return errFeltSmallBytes
	case body[0] == 0:
		return errFeltLeadingZero
	}

	var buffer [feltByteLen]byte
	copy(buffer[feltByteLen-length:], body)
	element, err := fp.BigEndian.Element(&buffer)
	if err != nil {
		return errFeltOutOfRange
	}
	*value = element
	return nil
}

// belowModulus reports whether little-endian limbs hold a value below the field modulus.
func belowModulus(limbs *[Limbs]uint64) bool {
	for index := Limbs - 1; index >= 0; index-- {
		if limbs[index] != modulusLimbs[index] {
			return limbs[index] < modulusLimbs[index]
		}
	}
	return false
}

// decodeCBOR decodes one compact or legacy felt at data[0:], returning the bytes consumed.
// It reports false on anything the fast path rejects, so the caller can fall back.
func decodeCBOR(data []byte, value *fp.Element) (int, bool) {
	if len(data) > 0 && isCompactCBOR(data[0]) {
		consumed, err := decodeCompact(data, value)
		return consumed, err == nil
	}
	return decodeLegacyLimbs(data, value)
}

// decodeLegacyLimbs decodes one limb-encoded felt at data[0:],
// returning the number of bytes consumed and a flag to signal success.
// It writes value only on success, so a rejected input can't partially
// corrupt it before falling back to the generic decoder.
func decodeLegacyLimbs(data []byte, value *fp.Element) (int, bool) {
	// The data format is: [cborLegacyArrayHeader] [limb 0] [limb 1] [limb 2] [limb 3]
	if len(data) == 0 || data[0] != cborLegacyArrayHeader {
		return 0, false
	}

	// Header-byte offset of each limb in the all-uint64 shape; the 8-byte payload follows.
	const (
		limb0Header = 1
		limb1Header = limb0Header + maxCBORUintLen
		limb2Header = limb1Header + maxCBORUintLen
		limb3Header = limb2Header + maxCBORUintLen
	)

	// Legacy felts are in Montgomery form, so in practice every limb exceeds MaxUint32 and
	// encodes as a full uint64, giving one fixed 37-byte shape. Decode it without the
	// per-limb header switch; anything else falls through to the general loop.
	if len(data) >= maxCBORLegacyFeltLen &&
		data[limb0Header] == cborUint64AdditionalInfo &&
		data[limb1Header] == cborUint64AdditionalInfo &&
		data[limb2Header] == cborUint64AdditionalInfo &&
		data[limb3Header] == cborUint64AdditionalInfo {
		(*value)[0] = binary.BigEndian.Uint64(data[limb0Header+1 : limb1Header])
		(*value)[1] = binary.BigEndian.Uint64(data[limb1Header+1 : limb2Header])
		(*value)[2] = binary.BigEndian.Uint64(data[limb2Header+1 : limb3Header])
		(*value)[3] = binary.BigEndian.Uint64(data[limb3Header+1 : maxCBORLegacyFeltLen])
		return maxCBORLegacyFeltLen, true
	}

	return decodeVariableLegacyLimbs(data, value)
}

// decodeVariableLegacyLimbs decodes the limbs at data[1:] one header byte at a time, covering the
// shapes the fixed-shape fast path rejects; data[0] must already be a validated array header.
func decodeVariableLegacyLimbs(data []byte, value *fp.Element) (int, bool) {
	var limbs fp.Element
	offset := 1

	for limbIndex := range Limbs {
		if offset >= len(data) {
			return 0, false
		}

		headerByte := data[offset]
		offset++

		var limb uint64
		switch {
		case headerByte > cborUint64AdditionalInfo: // invalid header byte for uint64
			return 0, false

		case headerByte == cborUint64AdditionalInfo:
			if offset+8 > len(data) {
				return 0, false
			}
			limb = binary.BigEndian.Uint64(data[offset:])
			offset += 8

		case headerByte == cborUint32AdditionalInfo:
			if offset+4 > len(data) {
				return 0, false
			}
			limb = uint64(binary.BigEndian.Uint32(data[offset:]))
			offset += 4

		case headerByte == cborUint16AdditionalInfo:
			if offset+2 > len(data) {
				return 0, false
			}
			limb = uint64(binary.BigEndian.Uint16(data[offset:]))
			offset += 2

		case headerByte == cborUint8AdditionalInfo:
			if offset+1 > len(data) {
				return 0, false
			}
			limb = uint64(data[offset])
			offset++

		default: // headerByte < cborUint8AdditionalInfo
			limb = uint64(headerByte)
		}

		limbs[limbIndex] = limb
	}

	*value = limbs
	return offset, true
}
