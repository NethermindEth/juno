package cborlite_test

import (
	"encoding/binary"
	"math"
	"math/big"
	"slices"

	"github.com/NethermindEth/juno/encoder/cborlite"
)

// Short names for the constants
const (
	uintMajor   = cborlite.UintMajor
	negIntMajor = cborlite.NegIntMajor
	bytesMajor  = cborlite.BytesMajor
	stringMajor = cborlite.StringMajor
	arrayMajor  = cborlite.ArrayMajor
	mapMajor    = cborlite.MapMajor
	tagMajor    = cborlite.TagMajor
	simpleMajor = cborlite.SimpleMajor

	info1Byte = cborlite.Info1Byte
	info2Byte = cborlite.Info2Byte
	info4Byte = cborlite.Info4Byte
	info8Byte = cborlite.Info8Byte

	simpleFalse = cborlite.SimpleFalse
	simpleTrue  = cborlite.SimpleTrue
	null        = cborlite.Null

	tagPositiveBignum = cborlite.TagPositiveBignum
	tagNegativeBignum = cborlite.TagNegativeBignum
)

// The smallest whole number bigger than UInt64
var beyondUint64 = new(big.Int).Lsh(big.NewInt(1), 64)

// initialByte packs a major type with its additional info.
func initialByte(major, info byte) byte {
	return major | info
}

// head writes the initial byte plus the argument, in the narrowest width that holds it.
func head(major byte, argument uint64) []byte {
	switch {
	case argument < info1Byte:
		return []byte{initialByte(major, byte(argument))}
	case argument <= math.MaxUint8:
		return []byte{initialByte(major, info1Byte), byte(argument)}
	case argument <= math.MaxUint16:
		return binary.BigEndian.AppendUint16([]byte{initialByte(major, info2Byte)}, uint16(argument))
	case argument <= math.MaxUint32:
		return binary.BigEndian.AppendUint32([]byte{initialByte(major, info4Byte)}, uint32(argument))
	default:
		return binary.BigEndian.AppendUint64([]byte{initialByte(major, info8Byte)}, argument)
	}
}

func trailByJunk(item []byte) []byte {
	return slices.Concat(item, []byte{0xff, 0xff})
}

// headerFollowedBy writes a header byte and argumentSize zero bytes after it.
func headerFollowedBy(header byte, argumentSize int) []byte {
	return append([]byte{header}, make([]byte, argumentSize)...)
}

/*
	Well-formed items.
*/

func cborBytes(payload ...byte) []byte {
	return append(head(bytesMajor, uint64(len(payload))), payload...)
}

func cborText(text string) []byte {
	return append(head(stringMajor, uint64(len(text))), text...)
}

func cborArray(items ...[]byte) []byte {
	out := head(arrayMajor, uint64(len(items)))
	for _, element := range items {
		out = append(out, element...)
	}
	return out
}

// cborMap takes its pairs flattened, a key followed by its value.
func cborMap(pairs ...[]byte) []byte {
	out := head(mapMajor, uint64(len(pairs)/2))
	for _, half := range pairs {
		out = append(out, half...)
	}
	return out
}

// cborTagged wraps an item in a tag.
func cborTagged(number uint64, tagged []byte) []byte {
	return append(head(tagMajor, number), tagged...)
}
