package cborlite

import (
	"math"
	"math/big"
)

// bytesPayload returns the payload of bytes after the header.
func bytesPayload(data []byte, expectedMajor byte) (value []byte, consumed int, ok bool) {
	major, length, header, ok := Head(data)
	if !ok || major != expectedMajor || length > uint64(len(data)-header) {
		return nil, 0, false
	}
	end := header + int(length)
	return data[header:end], end, true
}

// BytesNoCopy returns a sub-slice of data, valid only while data is. [Bytes] copies.
func BytesNoCopy(data []byte) (value []byte, consumed int, ok bool) {
	return bytesPayload(data, BytesMajor)
}

// Bytes copies and returns the payload of bytes after the header.
func Bytes(data []byte) (value []byte, consumed int, ok bool) {
	borrowed, consumed, ok := bytesPayload(data, BytesMajor)
	if !ok {
		return nil, 0, false
	}

	out := make([]byte, len(borrowed))
	copy(out, borrowed)

	return out, consumed, true
}

// StringNoCopy returns a sub-slice of data, valid only while data is. [String] copies.
func StringNoCopy(data []byte) (value []byte, consumed int, ok bool) {
	return bytesPayload(data, StringMajor)
}

// String copies the string out of data.
func String(data []byte) (value string, consumed int, ok bool) {
	borrowed, consumed, ok := bytesPayload(data, StringMajor)
	if !ok {
		return "", 0, false
	}
	return string(borrowed), consumed, true
}

// Uint64 reads an unsigned integer. A negative one is rejected.
func Uint64(data []byte) (value uint64, consumed int, ok bool) {
	major, value, consumed, ok := Head(data)
	if !ok || major != UintMajor {
		return 0, 0, false
	}
	return value, consumed, true
}

// Bool reads a boolean.
func Bool(data []byte) (value bool, consumed int, ok bool) {
	if len(data) == 0 {
		return false, 0, false
	}

	switch data[0] {
	case SimpleFalse:
		return false, 1, true
	case SimpleTrue:
		return true, 1, true
	default:
		return false, 0, false
	}
}

// Tag reads a tag number. The encoder writes one to record which concrete type went
// into an interface field.
func Tag(data []byte) (value uint64, consumed int, ok bool) {
	major, tag, consumed, ok := Head(data)
	if !ok || major != TagMajor {
		return 0, 0, false
	}
	return tag, consumed, true
}

// ReadNull reports whether the item is null.
// It does not consume the bytes when it is not null.
// No other reader checks null, so call this first wherever null is a possibility.
func ReadNull(data []byte) (consumed int, isNull bool) {
	if len(data) > 0 && data[0] == Null {
		return 1, true
	}
	return 0, false
}

// Avoid deeply malformed structures
const maxSkipDepth = 1024

// Skip reports how many bytes the next items take.
func Skip(data []byte) (consumed int, ok bool) {
	return skip(data, maxSkipDepth)
}

func skip(data []byte, depth int) (consumed int, ok bool) {
	if depth <= 0 {
		return 0, false
	}

	major, argument, consumed, ok := Head(data)
	if !ok {
		return 0, false
	}

	switch major {
	case UintMajor, NegIntMajor, SimpleMajor:
		return consumed, true

	case BytesMajor, StringMajor:
		if argument > uint64(len(data)-consumed) {
			return 0, false
		}
		return consumed + int(argument), true

	case TagMajor:
		tagged, ok := skip(data[consumed:], depth-1)
		if !ok {
			return 0, false
		}
		return consumed + tagged, true

	case ArrayMajor, MapMajor:
		// TODO(granza): the first byte already tells the size of most items.
		// Add that size and move on, instead of calling Skip for every element.
		remainingBytes := uint64(len(data) - consumed)
		itemsCount := argument

		if major == MapMajor {
			// avoid overflows
			if itemsCount > remainingBytes/2 {
				return 0, false
			}
			itemsCount *= 2
		} else if itemsCount > remainingBytes {
			return 0, false
		}

		for range itemsCount {
			element, ok := skip(data[consumed:], depth-1)
			if !ok {
				return 0, false
			}
			consumed += element
		}
		return consumed, true

	default:
		// Unreachable. A major is three bits and the cases above name all eight.
		return 0, false
	}
}

// BigInt reads an integer of any width.
func BigInt(data []byte) (value *big.Int, consumed int, ok bool) {
	major, argument, consumed, ok := Head(data)
	if !ok {
		return nil, 0, false
	}

	switch major {
	case UintMajor, NegIntMajor:
		out := new(big.Int).SetUint64(argument)
		if major == NegIntMajor {
			out.Not(out)
		}
		return out, consumed, true

	case TagMajor:
		if argument != TagPositiveBignum && argument != TagNegativeBignum {
			return nil, 0, false
		}

		bignum, used, ok := bytesPayload(data[consumed:], BytesMajor)
		if !ok {
			return nil, 0, false
		}
		consumed += used

		out := new(big.Int).SetBytes(bignum)
		if argument == TagNegativeBignum {
			out.Not(out)
		}
		return out, consumed, true

	default:
		return nil, 0, false
	}
}

// Int64 reads a signed integer.
func Int64(data []byte) (value int64, consumed int, ok bool) {
	major, argument, consumed, ok := Head(data)
	if !ok || argument > math.MaxInt64 {
		return 0, 0, false
	}

	switch major {
	case UintMajor, NegIntMajor:
		out := int64(argument)
		if major == NegIntMajor {
			out = ^out
		}
		return out, consumed, true

	default:
		return 0, 0, false
	}
}
