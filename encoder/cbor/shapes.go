package cbor

import (
	"math"
	"slices"
)

// oneFelt is what a STORED felt looks like in bytes.
// All felts pass through the Montgomery transformation, so they are usually 37 bytes.
var oneFelt = []byte{
	arrayHeader4,
	0x1b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xe1,
	0x1b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0x1b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0x1b, 0x07, 0xff, 0xff, 0xff, 0xff, 0xff, 0xfd, 0xf0,
}

// Shape is a CBOR payload, named for what makes it interesting.
type Shape struct {
	Name string
	Data []byte
}

// FeltAccepted is every payload a felt decoder has to take and write back unchanged.
var FeltAccepted = []Shape{
	{"zero", []byte{arrayHeader4, 0x00, 0x00, 0x00, 0x00}},
	{"all inline", []byte{arrayHeader4, 0x01, 0x02, 0x03, 0x04}},

	// What a stored felt actually looks like.
	{"montgomery shaped", oneFelt},

	// Sparse: a narrow limb, zeros, then a wide one. A random felt almost never looks like this,
	// but the two ends of the field do, in Montgomery form.
	{"sparse, 1-byte then 2-byte", []byte{arrayHeader4, 0x18, 0x20, 0x00, 0x00, 0x19, 0x02, 0x20}},
	{"sparse, inline then 2-byte", []byte{arrayHeader4, 0x10, 0x00, 0x00, 0x19, 0x01, 0x10}},

	{"limb: one", []byte{arrayHeader4, 0x01, 0x00, 0x00, 0x00}},
	{"limb: largest inline", []byte{arrayHeader4, 0x17, 0x00, 0x00, 0x00}},
	{"limb: smallest 1-byte", []byte{arrayHeader4, 0x18, 0x18, 0x00, 0x00, 0x00}},
	{"limb: largest 1-byte", []byte{arrayHeader4, 0x18, 0xff, 0x00, 0x00, 0x00}},
	{"limb: smallest 2-byte", []byte{arrayHeader4, 0x19, 0x01, 0x00, 0x00, 0x00, 0x00}},
	{"limb: largest 2-byte", []byte{arrayHeader4, 0x19, 0xff, 0xff, 0x00, 0x00, 0x00}},
	{"limb: smallest 4-byte", []byte{
		arrayHeader4, 0x1a, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00,
	}},
	{"limb: largest 4-byte", []byte{arrayHeader4, 0x1a, 0xff, 0xff, 0xff, 0xff, 0x00, 0x00, 0x00}},
	{"limb: smallest 8-byte", []byte{
		arrayHeader4, 0x1b, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}},
	{"limb: largest 8-byte", []byte{
		arrayHeader4, 0x1b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x00, 0x00, 0x00,
	}},
}

// FeltRejected is every payload the fast path has to refuse.
var FeltRejected = []Shape{
	{"empty", []byte{}},
	{"nil", nil},
	{"empty array", []byte{arrayMajor}},
	{"array of three", []byte{arrayMajor | 3, 0x00, 0x00, 0x00}},
	{"array of five", []byte{arrayMajor | 5, 0x00, 0x00, 0x00, 0x00, 0x00}},
	{"array of four, one limb missing", []byte{arrayHeader4, 0x00, 0x00, 0x00}},
	{"array of four, one byte too many", []byte{arrayHeader4, 0x00, 0x00, 0x00, 0x00, 0x00}},
	{"indefinite length", []byte{arrayIndefinite, 0x00, 0x00, 0x00, 0x00, breakStop}},
	{"null", []byte{null}},

	{"1-byte limb header, no value", []byte{arrayHeader4, uintMajor | info1Byte}},
	{"2-byte limb header, truncated", []byte{arrayHeader4, uintMajor | info2Byte, 0x00}},
	{"4-byte limb header, truncated", []byte{arrayHeader4, uintMajor | info4Byte, 0x00, 0x00, 0x00}},
	{"8-byte limb header, truncated", []byte{
		arrayHeader4, uintMajor | info8Byte, 0, 0, 0, 0, 0, 0, 0,
	}},
	{"reserved size code as limb", []byte{arrayHeader4, uintMajor | reservedInfo, 0x00, 0x00, 0x00}},

	{"negative first limb", []byte{arrayHeader4, negIntMajor, 0x00, 0x00, 0x00}},
	{"negative every limb", []byte{
		arrayHeader4, negIntMajor, negIntMajor, negIntMajor, negIntMajor,
	}},
	{"float32 first limb", []byte{
		arrayHeader4, simpleMajor | info4Byte, 0x3f, 0xc0, 0x00, 0x00, 0x00, 0x00, 0x00,
	}},
	{"float64 first limb", []byte{
		arrayHeader4, simpleMajor | info8Byte, 0x3f, 0xf8, 0, 0, 0, 0, 0, 0, 0x00, 0x00, 0x00,
	}},
}

// FeltSliceAccepted is every element count a slice decoder has to take and write back unchanged.
var FeltSliceAccepted = []struct {
	Name string
	Size int
}{
	{"empty", 0},
	{"one", 1},
	{"two", 2},
	{"largest inline length", maxInline},
	{"smallest 1-byte length", info1Byte},
	{"largest 1-byte length", math.MaxUint8},
	{"smallest 2-byte length", math.MaxUint8 + 1},
	{"largest 2-byte length", math.MaxUint16},
	{"smallest 4-byte length", math.MaxUint16 + 1},
}

// FeltSliceRejected is every payload the slice fast path has to refuse.
var FeltSliceRejected = []Shape{
	{"empty", []byte{}},
	{"nil", nil},
	{"null", []byte{null}},
	{"not an array", []byte{uintMajor}},
	{"array of one, no element", []byte{arrayMajor | 1}},
	{"array of two, second missing", slices.Concat([]byte{arrayMajor | 2}, oneFelt)},
	{"one felt plus a trailing byte", slices.Concat(
		[]byte{arrayMajor | 1}, oneFelt, []byte{uintMajor},
	)},
	{"element is not felt shaped", []byte{arrayMajor | 1, uintMajor}},
	{"indefinite length", slices.Concat([]byte{arrayIndefinite}, oneFelt, []byte{breakStop})},
	{"1-byte length header, no count", []byte{arrayMajor | info1Byte}},
	{"2-byte length header, truncated count", []byte{arrayMajor | info2Byte, 0x00}},

	// A count no payload could hold. The fast path has to notice before it allocates.
	{"4-byte length header claiming MaxUint32 elements", slices.Concat(
		[]byte{arrayMajor | info4Byte, 0xff, 0xff, 0xff, 0xff}, oneFelt,
	)},
}
