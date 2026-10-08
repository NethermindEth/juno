package fast

import "encoding/binary"

const (
	// majorMask and infoMask split a head byte into major type (top 3 bits) and
	// additional info (low 5 bits).
	majorMask = 0b1110_0000
	infoMask  = 0b0001_1111
)

// Major types. See https://www.rfc-editor.org/rfc/rfc8949.html#section-3.1
const (
	MajorUint   = 0 << 5
	MajorNegInt = 1 << 5
	MajorBytes  = 2 << 5
	MajorString = 3 << 5
	MajorArray  = 4 << 5
	MajorMap    = 5 << 5
	MajorTag    = 6 << 5
	MajorSimple = 7 << 5
)

// Head bytes of the simple values the stored records use.
const (
	SimpleFalse     = MajorSimple | 20
	SimpleTrue      = MajorSimple | 21
	SimpleNull      = MajorSimple | 22
	SimpleUndefined = MajorSimple | 23
)

// Additional info values. Below info1Byte the argument is the info itself.
const (
	info1Byte      = 24
	info2Byte      = 25
	info4Byte      = 26
	info8Byte      = 27
	infoIndefinite = 31
)

// MaxNestedLevels is how deep arrays, maps and tags may nest. It matches the default of
// the fxamacker decoder the wrapper uses today, so both accept the same records.
const MaxNestedLevels = 32

// Reader is a cursor over a CBOR buffer. The zero value reads an empty buffer.
// A Reader is not safe for concurrent use.
type Reader struct {
	buf []byte
	pos int
	// depth is how many arrays and maps are open around the next item. ArrayLen and MapLen
	// open one and Leave closes it, so Skip can hold what it passes over to MaxNestedLevels
	// counted from the start of the record.
	depth int
}

// NewReader returns a Reader positioned at the start of data.
func NewReader(data []byte) Reader {
	return Reader{buf: data, pos: 0, depth: 0}
}

// Remaining returns the unread suffix of the input. It aliases the input buffer.
func (r *Reader) Remaining() []byte { return r.buf[r.pos:] }

// Len returns the number of unread bytes.
func (r *Reader) Len() int { return len(r.buf) - r.pos }

// Pos returns the number of bytes consumed so far.
func (r *Reader) Pos() int { return r.pos }

// Peek returns the next head byte without consuming it.
func (r *Reader) Peek() (byte, error) {
	if r.pos >= len(r.buf) {
		return 0, ErrTruncated
	}
	return r.buf[r.pos], nil
}

// Head reads the head of the next item: its major type and its argument. The argument is
// the value of an integer or simple item, the length of a string, the element count of an
// array or map, or the number of a tag. It consumes nothing on error.
func (r *Reader) Head() (major byte, argument uint64, err error) {
	// The argument is small enough to live in the info bits.
	if r.pos < len(r.buf) {
		if head := r.buf[r.pos]; head&infoMask < info1Byte {
			r.pos++
			return head & majorMask, uint64(head & infoMask), nil
		}
	}
	return r.headSlow()
}

// headSlow is Head for a missing head or an argument that follows the head byte.
func (r *Reader) headSlow() (major byte, argument uint64, err error) {
	if r.pos >= len(r.buf) {
		return 0, 0, ErrTruncated
	}
	head := r.buf[r.pos]
	major = head & majorMask
	info := head & infoMask

	if info < info1Byte {
		r.pos++
		return major, uint64(info), nil
	}

	if info > info8Byte {
		// 28-30 are reserved. 31 is a break or an indefinite length, well-formed only for
		// strings, arrays and maps; those are valid CBOR this package does not read.
		if info == infoIndefinite && major >= MajorBytes && major <= MajorMap {
			return 0, 0, ErrUnsupported
		}
		return 0, 0, ErrSyntax
	}

	size := 1 << (info - info1Byte)
	start := r.pos + 1
	if len(r.buf)-start < size {
		return 0, 0, ErrTruncated
	}
	switch size {
	case 1:
		argument = uint64(r.buf[start])
	case 2:
		argument = uint64(binary.BigEndian.Uint16(r.buf[start:]))
	case 4:
		argument = uint64(binary.BigEndian.Uint32(r.buf[start:]))
	default:
		argument = binary.BigEndian.Uint64(r.buf[start:])
	}
	r.pos = start + size
	return major, argument, nil
}

// expect reads a head and requires the given major type.
// It consumes nothing unless the major type matches.
func (r *Reader) expect(want byte) (uint64, error) {
	if r.pos < len(r.buf) {
		if info := r.buf[r.pos] - want; info < info1Byte {
			r.pos++
			return uint64(info), nil
		}
	}
	return r.expectSlow(want)
}

func (r *Reader) expectSlow(want byte) (uint64, error) {
	start := r.pos
	major, argument, err := r.headSlow()
	if err != nil {
		return 0, err
	}
	if major != want {
		r.pos = start
		return 0, mismatch(major)
	}
	return argument, nil
}

// mismatch is the error for finding an item of the given major type where another was
// expected. A tag is not a mismatch of data: fxamacker looks through tags it has no use for,
// which this package leaves to it.
func mismatch(found byte) error {
	if found == MajorTag {
		return ErrUnsupported
	}
	return ErrType
}

// ArrayLen reads an array head and returns its element count. The count is capped by the
// bytes left, since every element takes at least one, so it is safe to size an allocation
// from it for one-byte elements; scale by the element size before presizing anything bigger.
func (r *Reader) ArrayLen() (int, error) {
	if r.pos < len(r.buf) && r.depth < MaxNestedLevels {
		// count < bytes left including the head is count <= bytes left after it.
		if count := r.buf[r.pos] - MajorArray; count < info1Byte && int(count) < len(r.buf)-r.pos {
			r.pos++
			r.depth++
			return int(count), nil
		}
	}
	return r.containerLen(MajorArray, 1)
}

// MapLen reads a map head and returns its pair count, capped by the bytes left
// since every pair takes at least two.
func (r *Reader) MapLen() (int, error) {
	if r.pos < len(r.buf) && r.depth < MaxNestedLevels {
		if count := r.buf[r.pos] - MajorMap; count < info1Byte && 2*int(count) < len(r.buf)-r.pos {
			r.pos++
			r.depth++
			return int(count), nil
		}
	}
	return r.containerLen(MajorMap, 2)
}

// containerLen reads an array or map head whose items take at least perItem bytes each.
func (r *Reader) containerLen(want byte, perItem uint64) (int, error) {
	start := r.pos
	count, err := r.expectSlow(want)
	if err != nil {
		return 0, err
	}
	if count > uint64(r.Len())/perItem {
		r.pos = start
		return 0, ErrTruncated
	}
	if err := r.enter(); err != nil {
		r.pos = start
		return 0, err
	}
	return int(count), nil
}

func (r *Reader) enter() error {
	if r.depth >= MaxNestedLevels {
		return ErrDepth
	}
	r.depth++
	return nil
}

// Leave closes the array or map that ArrayLen or MapLen opened, once all its items are read.
func (r *Reader) Leave() { r.depth-- }

// Tag reads a tag number. The content item follows.
func (r *Reader) Tag() (uint64, error) {
	return r.expect(MajorTag)
}

// Null consumes the next item if it is null or undefined, reporting whether it was.
// fxamacker decodes both the same way.
func (r *Reader) Null() bool {
	if r.pos < len(r.buf) && (r.buf[r.pos] == SimpleNull || r.buf[r.pos] == SimpleUndefined) {
		r.pos++
		return true
	}
	return false
}
