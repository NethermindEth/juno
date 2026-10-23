package cborlite

import "encoding/binary"

const (
	// MajorMask and InfoMask split a header byte into
	// major type (top 3 bits) and additional info (low 5 bits).
	MajorMask = 0b1110_0000
	InfoMask  = 0b0001_1111

	// Major types. They define the object type.
	// See https://www.rfc-editor.org/rfc/rfc8949.html#section-3.1
	UintMajor   = 0 << 5
	NegIntMajor = 1 << 5
	BytesMajor  = 2 << 5
	StringMajor = 3 << 5
	ArrayMajor  = 4 << 5
	MapMajor    = 5 << 5
	TagMajor    = 6 << 5
	SimpleMajor = 7 << 5

	// bool values and nil pointers.
	SimpleFalse = SimpleMajor | 20
	SimpleTrue  = SimpleMajor | 21
	Null        = SimpleMajor | 22

	// Tag types to define Big Numbers
	// https://www.rfc-editor.org/rfc/rfc8949.html#section-3.4.3
	TagPositiveBignum = 2
	TagNegativeBignum = 3

	// Additional info values that say how many bytes follow the argument.
	Info1Byte = 24
	Info2Byte = 25
	Info4Byte = 26
	Info8Byte = 27

	// Always one byte.
	headerSize = 1
)

// Head reads the header at the start of data.
// argument is a value for a scalar, the length for a string, or an item count for an array/map.
func Head(data []byte) (major byte, argument uint64, consumed int, ok bool) {
	if len(data) == 0 {
		return 0, 0, 0, false
	}

	header := data[0]
	major = header & MajorMask
	info := header & InfoMask

	// The argument is so small it fits inside info.
	if info < Info1Byte {
		return major, uint64(info), headerSize, true
	}

	// info says the number of bytes that follow, max 8.
	if info > Info8Byte {
		return 0, 0, 0, false
	}
	infoByteSize := 1 << (info - Info1Byte)
	if len(data) < headerSize+infoByteSize {
		return 0, 0, 0, false
	}

	switch infoByteSize {
	case 1:
		argument = uint64(data[headerSize])
	case 2:
		argument = uint64(binary.BigEndian.Uint16(data[headerSize:]))
	case 4:
		argument = uint64(binary.BigEndian.Uint32(data[headerSize:]))
	default:
		argument = binary.BigEndian.Uint64(data[headerSize:])
	}
	return major, argument, headerSize + infoByteSize, true
}

// ArrayHeader reads an array header and its element count.
func ArrayHeader(data []byte) (length, consumed int, ok bool) {
	major, count, consumed, ok := Head(data)
	// An element takes at least one byte, data must have space for it.
	if !ok || major != ArrayMajor || count > uint64(len(data)-consumed) {
		return 0, 0, false
	}
	return int(count), consumed, true
}

// MapHeader reads a map header and its pair count.
func MapHeader(data []byte) (pairsCount, consumed int, ok bool) {
	major, count, consumed, ok := Head(data)
	// A pair takes at least two bytes, data must have space for it.
	if !ok || major != MapMajor || count > uint64(len(data)-consumed)/2 {
		return 0, 0, false
	}
	return int(count), consumed, true
}
