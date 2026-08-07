// Package cborlite is a CBOR reader for values Juno wrote itself, built around one
// rule: never walk a byte you are not decoding.
//
// Two things break that rule, and they are the same mistake wearing different
// clothes. Both come from a decoder that will not say how far it read.
//
//   - The generic decoder proves a whole item is well formed before it fills
//     anything, so it walks every byte twice. For values nothing else ever wrote,
//     that validation is a cost with no buyer.
//   - An all-or-nothing UnmarshalCBOR([]byte) forces a caller to find where an
//     item ends before it can hand the bytes over, which means walking the item
//     first just to measure it. Cheaper than validating, but not cheap: walking an
//     array of felts costs more per element than decoding it.
//
// So every reader here takes the whole buffer plus an offset and returns the offset
// just past what it read. That single convention is what makes both walks
// unnecessary, and it is the reason this package exists.
//
// Reading without validating first is safe because of two more rules:
//
//   - Bounds are checked as it goes, so a truncated or corrupt buffer makes a
//     reader return ok == false instead of panicking or reading out of range.
//   - Any shape a reader does not recognise is also ok == false, never a guess.
//     A caller that gets a false is expected to fall back to the generic decoder
//     for the whole structure it was decoding, which does validate.
//
// So this is not a replacement for the generic decoder and it does not try to cover
// CBOR. It covers what the canonical encoder emits and hands anything else back.
//
// Readers that build a value write it only after the item has been fully read, so a
// rejected item never leaves a half-built value behind.
package cborlite

import (
	"encoding/binary"
	"math/big"
)

const (
	// MajorMask and InfoMask split a header byte into its major type (top 3 bits)
	// and its additional info (low 5 bits).
	MajorMask = 0b1110_0000
	InfoMask  = 0b0001_1111

	// Major types. See https://www.rfc-editor.org/rfc/rfc8949.html#section-3.1
	UintMajor   = 0 << 5
	NegIntMajor = 1 << 5
	BytesMajor  = 2 << 5
	TextMajor   = 3 << 5
	ArrayMajor  = 4 << 5
	MapMajor    = 5 << 5
	TagMajor    = 6 << 5
	SimpleMajor = 7 << 5

	// Null is the encoding of null, which the generic encoder emits for nil
	// pointers, slices and maps.
	Null = 0xf6

	// Additional info values that say how many bytes carry the argument.
	Info1Byte = 24
	Info2Byte = 25
	Info4Byte = 26
	Info8Byte = 27

	// https://www.rfc-editor.org/rfc/rfc8949.html#section-3.4.3
	TagPositiveBignum = 2
	TagNegativeBignum = 3
)

// Head reads the header at data[offset:], returning the major type, the argument
// it carries and the offset right after it.
func Head(data []byte, offset int) (major byte, argument uint64, next int, ok bool) {
	if offset < 0 || offset >= len(data) {
		return 0, 0, 0, false
	}

	header := data[offset]
	major = header & MajorMask
	info := header & InfoMask
	next = offset + 1

	switch {
	case info < Info1Byte:
		return major, uint64(info), next, true
	case info == Info1Byte:
		if next+1 > len(data) {
			return 0, 0, 0, false
		}
		return major, uint64(data[next]), next + 1, true
	case info == Info2Byte:
		if next+2 > len(data) {
			return 0, 0, 0, false
		}
		return major, uint64(binary.BigEndian.Uint16(data[next:])), next + 2, true
	case info == Info4Byte:
		if next+4 > len(data) {
			return 0, 0, 0, false
		}
		return major, uint64(binary.BigEndian.Uint32(data[next:])), next + 4, true
	case info == Info8Byte:
		if next+8 > len(data) {
			return 0, 0, 0, false
		}
		return major, binary.BigEndian.Uint64(data[next:]), next + 8, true
	default:
		// Reserved values and indefinite lengths, which the canonical encoder
		// never emits.
		return 0, 0, 0, false
	}
}

// stringRange returns the bounds of a byte or text string payload.
func stringRange(data []byte, offset int, wantMajor byte) (start, end int, ok bool) {
	major, length, next, ok := Head(data, offset)
	if !ok || major != wantMajor || length > uint64(len(data)-next) {
		return 0, 0, false
	}
	return next, next + int(length), true
}

// ByteString returns the byte string at data[offset:] as a sub-slice of data. The
// result aliases data, so a caller that outlives the buffer has to copy it (see
// [ByteStringCopy]).
func ByteString(data []byte, offset int) ([]byte, int, bool) {
	start, end, ok := stringRange(data, offset, BytesMajor)
	if !ok {
		return nil, 0, false
	}
	return data[start:end], end, true
}

// ByteStringCopy returns a copy of the byte string at data[offset:], for callers
// that keep the result past the life of the buffer. It reads null as a nil slice,
// which is what the generic encoder writes for one.
func ByteStringCopy(data []byte, offset int) ([]byte, int, bool) {
	if next, isNull := ReadNull(data, offset); isNull {
		return nil, next, true
	}

	start, end, ok := stringRange(data, offset, BytesMajor)
	if !ok {
		return nil, 0, false
	}

	out := make([]byte, end-start)
	copy(out, data[start:end])
	return out, end, true
}

// TextString returns the text string at data[offset:] as a sub-slice of data. It
// aliases data, which makes it the right choice for a key a caller only compares
// (see [String] for one it keeps).
func TextString(data []byte, offset int) ([]byte, int, bool) {
	start, end, ok := stringRange(data, offset, TextMajor)
	if !ok {
		return nil, 0, false
	}
	return data[start:end], end, true
}

// String copies the text string at data[offset:] into a Go string, for callers
// that keep the result past the life of the buffer.
func String(data []byte, offset int) (string, int, bool) {
	start, end, ok := stringRange(data, offset, TextMajor)
	if !ok {
		return "", 0, false
	}
	return string(data[start:end]), end, true
}

// Uint64 reads an unsigned integer. A negative integer is a shape mismatch, not a
// value out of range, so it returns false rather than clamping.
func Uint64(data []byte, offset int) (uint64, int, bool) {
	major, value, next, ok := Head(data, offset)
	if !ok || major != UintMajor {
		return 0, 0, false
	}
	return value, next, true
}

// ReadNull reports whether the item at data[offset:] is null, returning the offset
// past it when it is and the offset unchanged when it is not.
func ReadNull(data []byte, offset int) (int, bool) {
	if offset >= 0 && offset < len(data) && data[offset] == Null {
		return offset + 1, true
	}
	return offset, false
}

// BigInt reads an integer of any width, including the bignum tags the encoder uses
// once a value no longer fits a uint64. It reads null as a nil pointer.
func BigInt(data []byte, offset int) (*big.Int, int, bool) {
	if next, isNull := ReadNull(data, offset); isNull {
		return nil, next, true
	}

	major, argument, next, ok := Head(data, offset)
	if !ok {
		return nil, 0, false
	}

	switch major {
	case UintMajor:
		return new(big.Int).SetUint64(argument), next, true
	case NegIntMajor:
		value := new(big.Int).SetUint64(argument)
		return value.Neg(value.Add(value, big.NewInt(1))), next, true
	case TagMajor:
		if argument != TagPositiveBignum && argument != TagNegativeBignum {
			return nil, 0, false
		}

		start, end, ok := stringRange(data, next, BytesMajor)
		if !ok {
			return nil, 0, false
		}

		value := new(big.Int).SetBytes(data[start:end])
		if argument == TagNegativeBignum {
			value.Neg(value.Add(value, big.NewInt(1)))
		}
		return value, end, true
	default:
		return nil, 0, false
	}
}

// ArrayHeader reads an array header, returning the element count. It reports a
// negative count for null, which lets a caller tell an absent array from an empty
// one without a second check.
func ArrayHeader(data []byte, offset int) (length, next int, ok bool) {
	if next, isNull := ReadNull(data, offset); isNull {
		return -1, next, true
	}

	major, count, next, ok := Head(data, offset)
	// Every element takes at least one byte, so a count past the remaining bytes
	// is corrupt. This is what stops a bad length from sizing an allocation.
	if !ok || major != ArrayMajor || count > uint64(len(data)-next) {
		return 0, 0, false
	}
	return int(count), next, true
}

// MapHeader reads a map header, returning the pair count.
func MapHeader(data []byte, offset int) (pairs, next int, ok bool) {
	major, count, next, ok := Head(data, offset)
	// Every pair takes at least two bytes, so a count past the remaining bytes is
	// corrupt.
	if !ok || major != MapMajor || count > uint64(len(data)-next) {
		return 0, 0, false
	}
	return int(count), next, true
}

// StringSlice reads an array of text strings, copying each one. Like [StructSlice]
// it only pre-sizes up to maxPresizedElements, so an unverified count cannot size
// the allocation on its own.
func StringSlice(data []byte, offset int) ([]string, int, bool) {
	length, offset, ok := ArrayHeader(data, offset)
	if !ok || length < 0 {
		return nil, offset, ok
	}

	slice := make([]string, 0, min(length, maxPresizedElements))
	for range length {
		var element string
		if element, offset, ok = String(data, offset); !ok {
			return nil, 0, false
		}
		slice = append(slice, element)
	}
	return slice, offset, true
}

// StructMap reads a struct map, handing each key to field together with the offset
// of its value. field reads the value and returns the offset past it, and should
// hand a key it does not know to [Skip], which is what the generic decoder does
// with a field it has no home for.
func StructMap(
	data []byte,
	offset int,
	field func(key, data []byte, offset int) (int, bool),
) (int, bool) {
	pairs, offset, ok := MapHeader(data, offset)
	if !ok {
		return 0, false
	}

	for range pairs {
		var key []byte
		if key, offset, ok = TextString(data, offset); !ok {
			return 0, false
		}
		if offset, ok = field(key, data, offset); !ok {
			return 0, false
		}
	}
	return offset, true
}

// maxPresizedElements caps how many elements a header is trusted for up front.
//
// An element can be one byte on the wire (an empty map is 0xa0), so a header may
// legitimately claim as many elements as there are bytes left, and sizing a []T
// from that count allocates len(data) * sizeof(T). At 56 bytes per element that
// turns a 1 MiB value into a 54 MiB allocation, off a count nothing has verified
// yet. Past this many elements the slice grows as they are actually read, so the
// cost follows what the buffer really holds. Real values are far below the cap, so
// they still get exactly one allocation.
const maxPresizedElements = 1024

// StructSlice reads an array whose elements decode is able to read, one after the
// other. decode gets the whole buffer and the offset of its element, and returns
// the offset past it, so the elements are read in a single pass.
func StructSlice[T any](
	data []byte,
	offset int,
	decode func(*T, []byte, int) (int, bool),
) ([]T, int, bool) {
	length, offset, ok := ArrayHeader(data, offset)
	if !ok || length < 0 {
		return nil, offset, ok
	}

	slice := make([]T, 0, min(length, maxPresizedElements))
	for range length {
		// Grow first, then decode into place. Decoding into a local and appending
		// that would copy every element twice.
		var zero T
		slice = append(slice, zero)
		if offset, ok = decode(&slice[len(slice)-1], data, offset); !ok {
			return nil, 0, false
		}
	}
	return slice, offset, true
}

// Skip returns the offset right after the item at data[offset:], without building
// a value for it. It is for a field a caller does not want, and it is not a cheap
// way to find where an item ends: walking an array costs more per element than
// decoding it, so a caller that wants the value should decode it directly.
func Skip(data []byte, offset int) (int, bool) {
	major, argument, next, ok := Head(data, offset)
	if !ok {
		return 0, false
	}

	switch major {
	case UintMajor, NegIntMajor, SimpleMajor:
		return next, true
	case BytesMajor, TextMajor:
		if argument > uint64(len(data)-next) {
			return 0, false
		}
		return next + int(argument), true
	case TagMajor:
		return Skip(data, next)
	case ArrayMajor, MapMajor:
		remaining := uint64(len(data) - next)
		if major == MapMajor {
			// A pair is two items, so the bound is halved rather than the count
			// doubled. Doubling first would overflow on a count near the top of a
			// uint64 and wrap to a small number that passes the check, and a map
			// header claiming 2^63 pairs would then be read as an empty one.
			if argument > remaining/2 {
				return 0, false
			}
			argument *= 2
		} else if argument > remaining {
			return 0, false
		}

		for range argument {
			if next, ok = Skip(data, next); !ok {
				return 0, false
			}
		}
		return next, true
	default:
		return 0, false
	}
}
