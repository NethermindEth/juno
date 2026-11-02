// Package cursor reads CBOR from a byte slice in a single pass. A Cursor knows CBOR but no Go
// types, so a decoder built on top decides what to read and the Cursor reads it.
//
// A general-purpose CBOR decoder checks that the whole input is well-formed and then decodes it,
// accepting every form the standard allows. A Cursor checks as it reads, and reads only the forms
// Juno stores. These are integers, strings, felts as four limbs and struct fields by number, with
// Skip for what is not wanted. Anything else, and anything malformed, is an error.
//
// Its primitives are pinned to the examples of RFC 8949 Appendix A, and its felts to literal
// bytes. The examples are at https://www.rfc-editor.org/rfc/rfc8949.html#appendix-A
package cursor

import (
	"encoding/binary"
	"fmt"
	"unsafe"
)

const (
	// majorMask and infoMask split a head byte into major type and additional info.
	majorMask = 0b1110_0000
	infoMask  = 0b0001_1111
)

// Major types (RFC 8949, section 3.1); only two are exported.
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

// maxNestedLevels is how deep arrays, maps and chained tags may nest.
const maxNestedLevels = 32

// A Cursor walks a CBOR buffer one item at a time. It is not safe for concurrent use. depth is how
// many arrays and maps are open around the next item, and scratch is room for a value that is read
// and then copied, such as a map key.
type Cursor struct {
	buf     []byte
	pos     int
	depth   int
	scratch [4]uint64
}

// Scratch returns room for four words. Its contents are lost when the Cursor is reused.
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
		return 0, c.fail(ErrTruncated, "input ends before the item")
	}
	return c.buf[c.pos], nil
}

// head reads the head of the next item, which gives its major type and argument. It consumes
// nothing on error.
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
		return 0, 0, c.fail(ErrTruncated, "input ends before the item")
	}
	head := c.buf[c.pos]
	major = head & majorMask
	info := head & infoMask

	if info < info1Byte {
		c.pos++
		return major, uint64(info), nil
	}

	if info > info8Byte {
		// 28-30 are reserved. 31 is an indefinite length, which this package does not read, or a break.
		if info == infoIndefinite && major >= majorBytes && major <= majorMap {
			return 0, 0, c.fail(ErrUnsupported, "indefinite-length item")
		}
		if info == infoIndefinite && major == majorSimple {
			return 0, 0, c.fail(ErrSyntax, "break code outside an indefinite-length item")
		}
		return 0, 0, c.fail(ErrSyntax, "invalid additional information %d for %s", info, describe(head))
	}

	size := 1 << (info - info1Byte)
	start := c.pos + 1
	if len(c.buf)-start < size {
		return 0, 0, c.fail(ErrTruncated, "input ends inside the head")
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

// expect reads a head of the given major type, and consumes nothing if it is another.
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
		return 0, c.mismatch(describe(want), c.buf[start])
	}
	return argument, nil
}

// mismatch is the error for finding a tag or another type where want was expected.
func (c *Cursor) mismatch(want string, head byte) error {
	if head&majorMask == MajorTag {
		return c.fail(ErrUnsupported, "tag where %s is expected", want)
	}
	return c.fail(ErrType, "expected %s, found %s", want, describe(head))
}

func describe(head byte) string {
	switch head & majorMask {
	case MajorUint:
		return "an unsigned integer"
	case majorNegInt:
		return "a negative integer"
	case majorBytes:
		return "a byte string"
	case majorString:
		return "a text string"
	case majorArray:
		return "an array"
	case majorMap:
		return "a map"
	case MajorTag:
		return "a tag"
	default:
		return fmt.Sprintf("a simple value or float (head 0x%02x)", head)
	}
}

// ArrayLen reads an array head and returns its element count, capped by the bytes left.
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

// MapLen reads a map head and returns its pair count, capped by the bytes left.
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
		return 0, c.fail(ErrTruncated, "count exceeds the input")
	}
	if err := c.enter(start); err != nil {
		c.pos = start
		return 0, err
	}
	return int(count), nil
}

func (c *Cursor) enter(start int) error {
	if c.depth >= maxNestedLevels {
		return errDepth(start)
	}
	c.depth++
	return nil
}

// Leave closes the array or map that ArrayLen or MapLen opened.
func (c *Cursor) Leave() { c.depth-- }

// Tag reads a tag number. The content item follows.
func (c *Cursor) Tag() (uint64, error) {
	return c.expect(MajorTag)
}

// Null consumes the next item if it is null or undefined, and reports whether it did.
func (c *Cursor) Null() bool {
	if c.pos < len(c.buf) && (c.buf[c.pos] == simpleNull || c.buf[c.pos] == simpleUndefined) {
		c.pos++
		return true
	}
	return false
}
