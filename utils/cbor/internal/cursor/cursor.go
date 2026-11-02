// Package cursor holds the primitives of a single-pass CBOR reader: a Cursor with bounds-checked
// readers, felts as four limbs, struct keys matched to field numbers, and a Skip that checks the
// syntax of what it passes over.
//
// Every length, count, nesting depth and UTF-8 check happens inline, so malformed input returns an
// error and never panics, hangs or allocates more than the input can back. Readers return copies,
// except Key, BytesBorrowed and Remaining, which alias the input. A form a reader does not take is
// ErrUnsupported.
//
// The package imports no Juno package. Its tests here use RFC 8949 Appendix A and literal bytes;
// the parity and fuzz tests against fxamacker are in utils/cbor/compatibility.
package cursor

import (
	"encoding/binary"
	"unsafe"
)

const (
	// majorMask and infoMask split a head byte into major type and additional info.
	majorMask = 0b1110_0000
	infoMask  = 0b0001_1111
)

// Major types (RFC 8949, section 3.1). Only the two that code outside the package reads are
// exported.
const (
	MajorUint   = 0 << 5
	majorNegInt = 1 << 5
	majorBytes  = 2 << 5
	majorString = 3 << 5
	majorArray  = 4 << 5
	majorMap    = 5 << 5
	MajorTag    = 6 << 5
	majorSimple = 7 << 5
)

// Head bytes of the simple values the stored records use.
const (
	simpleFalse     = majorSimple | 20
	simpleTrue      = majorSimple | 21
	simpleNull      = majorSimple | 22
	simpleUndefined = majorSimple | 23
)

// Additional info values. Below info1Byte the argument is the info itself.
const (
	info1Byte      = 24
	info2Byte      = 25
	info4Byte      = 26
	info8Byte      = 27
	infoIndefinite = 31
)

// maxNestedLevels is how deep arrays, maps and tags may nest. Deeper input is ErrDepth.
const maxNestedLevels = 32

// A Cursor walks a CBOR buffer, one item at a time. The zero value reads an empty buffer.
// It is not safe for concurrent use.
type Cursor struct {
	buf []byte
	pos int
	// depth is how many arrays and maps are open around the next item. ArrayLen and MapLen
	// open one and Leave closes it, so Skip can hold what it passes over to maxNestedLevels
	// counted from the start of the record.
	depth int
	// scratch is room for a value that is read and then copied, such as the key of a map. A Cursor
	// is pooled, so it is on the heap and a pointer to this does not move.
	scratch [4]uint64
}

// Scratch returns room for four words, for a value that is read and then copied. Its contents are
// lost when the Cursor is reused.
func (c *Cursor) Scratch() unsafe.Pointer { return unsafe.Pointer(&c.scratch) }

// NewCursor returns a Cursor positioned at the start of data.
func NewCursor(data []byte) Cursor {
	return Cursor{buf: data}
}

// Remaining returns the unread suffix of the input. It aliases the input buffer.
func (c *Cursor) Remaining() []byte { return c.buf[c.pos:] }

// Len returns the number of unread bytes.
func (c *Cursor) Len() int { return len(c.buf) - c.pos }

// Pos returns the number of bytes consumed so far.
func (c *Cursor) Pos() int { return c.pos }

// Peek returns the next head byte without consuming it.
func (c *Cursor) Peek() (byte, error) {
	if c.pos >= len(c.buf) {
		return 0, ErrTruncated
	}
	return c.buf[c.pos], nil
}

// head reads the head of the next item: its major type and its argument. The argument is
// the value of an integer or simple item, the length of a string, the element count of an
// array or map, or the number of a tag. It consumes nothing on error.
func (c *Cursor) head() (major byte, argument uint64, err error) {
	// The argument is small enough to live in the info bits.
	if c.pos < len(c.buf) {
		if head := c.buf[c.pos]; head&infoMask < info1Byte {
			c.pos++
			return head & majorMask, uint64(head & infoMask), nil
		}
	}
	return c.headSlow()
}

// headSlow is head for a missing head or an argument that follows the head byte.
func (c *Cursor) headSlow() (major byte, argument uint64, err error) {
	if c.pos >= len(c.buf) {
		return 0, 0, ErrTruncated
	}
	head := c.buf[c.pos]
	major = head & majorMask
	info := head & infoMask

	if info < info1Byte {
		c.pos++
		return major, uint64(info), nil
	}

	if info > info8Byte {
		// 28-30 are reserved. 31 is a break or an indefinite length, well-formed only for
		// strings, arrays and maps; those are valid CBOR this package does not read.
		if info == infoIndefinite && major >= majorBytes && major <= majorMap {
			return 0, 0, ErrUnsupported
		}
		return 0, 0, ErrSyntax
	}

	size := 1 << (info - info1Byte)
	start := c.pos + 1
	if len(c.buf)-start < size {
		return 0, 0, ErrTruncated
	}
	switch size {
	case 1:
		argument = uint64(c.buf[start])
	case 2:
		argument = uint64(binary.BigEndian.Uint16(c.buf[start:]))
	case 4:
		argument = uint64(binary.BigEndian.Uint32(c.buf[start:]))
	default:
		argument = binary.BigEndian.Uint64(c.buf[start:])
	}
	c.pos = start + size
	return major, argument, nil
}

// expect reads a head and requires the given major type.
// It consumes nothing unless the major type matches.
func (c *Cursor) expect(want byte) (uint64, error) {
	if c.pos < len(c.buf) {
		if info := c.buf[c.pos] - want; info < info1Byte {
			c.pos++
			return uint64(info), nil
		}
	}
	return c.expectSlow(want)
}

func (c *Cursor) expectSlow(want byte) (uint64, error) {
	start := c.pos
	major, argument, err := c.headSlow()
	if err != nil {
		return 0, err
	}
	if major != want {
		c.pos = start
		return 0, mismatch(major)
	}
	return argument, nil
}

// mismatch is the error for finding an item of the given major type where another was expected.
// A tag is ErrUnsupported, since this package does not look through tags. Anything else is ErrType.
func mismatch(found byte) error {
	if found == MajorTag {
		return ErrUnsupported
	}
	return ErrType
}

// ArrayLen reads an array head and returns its element count, capped by the bytes left (an
// element takes at least one), so it is safe to presize a slice of one-byte elements from it.
func (c *Cursor) ArrayLen() (int, error) {
	if c.pos < len(c.buf) && c.depth < maxNestedLevels {
		// count < bytes left including the head is count <= bytes left after it.
		if count := c.buf[c.pos] - majorArray; count < info1Byte && int(count) < len(c.buf)-c.pos {
			c.pos++
			c.depth++
			return int(count), nil
		}
	}
	return c.containerLen(majorArray, 1)
}

// MapLen reads a map head and returns its pair count, capped by the bytes left (a pair takes at
// least two).
func (c *Cursor) MapLen() (int, error) {
	if c.pos < len(c.buf) && c.depth < maxNestedLevels {
		if count := c.buf[c.pos] - majorMap; count < info1Byte && 2*int(count) < len(c.buf)-c.pos {
			c.pos++
			c.depth++
			return int(count), nil
		}
	}
	return c.containerLen(majorMap, 2)
}

// containerLen reads an array or map head whose items take at least perItem bytes each.
func (c *Cursor) containerLen(want byte, perItem uint64) (int, error) {
	start := c.pos
	count, err := c.expectSlow(want)
	if err != nil {
		return 0, err
	}
	if count > uint64(c.Len())/perItem {
		c.pos = start
		return 0, ErrTruncated
	}
	if err := c.enter(); err != nil {
		c.pos = start
		return 0, err
	}
	return int(count), nil
}

func (c *Cursor) enter() error {
	if c.depth >= maxNestedLevels {
		return ErrDepth
	}
	c.depth++
	return nil
}

// Leave closes the array or map that ArrayLen or MapLen opened, once all its items are read.
func (c *Cursor) Leave() { c.depth-- }

// Tag reads a tag number. The content item follows.
func (c *Cursor) Tag() (uint64, error) {
	return c.expect(MajorTag)
}

// Null consumes the next item if it is null or undefined, reporting whether it was.
// The two are the same to a reader.
func (c *Cursor) Null() bool {
	if c.pos < len(c.buf) && (c.buf[c.pos] == simpleNull || c.buf[c.pos] == simpleUndefined) {
		c.pos++
		return true
	}
	return false
}
