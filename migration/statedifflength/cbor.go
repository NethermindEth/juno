package statedifflength

import (
	"errors"
	"fmt"
	"io"
)

// A minimal CBOR walker, enough to count collection entries without decoding their
// contents. Head and argument encoding follow RFC 8949 section 3.
const (
	majorUnsignedInt = 0
	majorNegativeInt = 1
	majorByteString  = 2
	majorTextString  = 3
	majorArray       = 4
	majorMap         = 5
	majorTag         = 6
	// majorSimple covers simple values, among them null, and floats.
	majorSimple = 7
)

const (
	// Additional info below 24 is the argument itself, and 24..27 mean 1, 2, 4 or 8
	// argument bytes follow. From 28 up is unsupported: 28..30 are reserved, and 31
	// marks an indefinite length, which the encoder never writes because
	// cbor.CanonicalEncOptions always emits a length.
	additionalInfoOneByte     = 24
	additionalInfoUnsupported = 28

	// maxNesting bounds recursion so malformed input fails instead of overflowing the stack.
	maxNesting = 64

	// cborNull is the whole encoding of null; simpleValueNull is its head argument.
	cborNull        = 0xf6
	simpleValueNull = 22
)

var (
	errUnsupportedAdditionalInfo = errors.New("cbor: reserved or indefinite length")
	errNestingTooDeep            = errors.New("cbor: nesting too deep")
	errNotACollection            = errors.New("cbor: item is not a map or an array")
	errNotATextString            = errors.New("cbor: item is not a text string")
)

// cursor walks CBOR items in data, left to right.
type cursor struct {
	data []byte
	pos  int
}

// item is the head of a single CBOR data item.
type item struct {
	major byte
	// argument is the head's argument: a value, a length or an entry count, depending
	// on the major type.
	argument uint64
}

// isNull reports whether the item is CBOR null, which the encoder writes for a nil
// map, slice or pointer.
func (i item) isNull() bool {
	return i.major == majorSimple && i.argument == simpleValueNull
}

// next reads the next item's head, leaving the cursor on its content.
func (c *cursor) next() (item, error) {
	if c.pos >= len(c.data) {
		return item{}, io.ErrUnexpectedEOF
	}
	initial := c.data[c.pos]
	c.pos++

	major := initial >> 5
	additionalInfo := initial & 0x1f
	switch {
	case additionalInfo < additionalInfoOneByte:
		return item{major: major, argument: uint64(additionalInfo)}, nil
	case additionalInfo >= additionalInfoUnsupported:
		return item{}, errUnsupportedAdditionalInfo
	}

	argumentLen := 1 << (additionalInfo - additionalInfoOneByte) // 1, 2, 4 or 8
	if c.pos+argumentLen > len(c.data) {
		return item{}, io.ErrUnexpectedEOF
	}
	var argument uint64
	for _, b := range c.data[c.pos : c.pos+argumentLen] {
		argument = argument<<8 | uint64(b)
	}
	c.pos += argumentLen
	return item{major: major, argument: argument}, nil
}

// advance consumes n content bytes.
func (c *cursor) advance(n uint64) error {
	if n > uint64(len(c.data)-c.pos) {
		return io.ErrUnexpectedEOF
	}
	c.pos += int(n)
	return nil
}

// skip consumes the next item in full, including any nested items.
func (c *cursor) skip() error {
	return c.skipDepth(0)
}

func (c *cursor) skipDepth(depth int) error {
	if depth > maxNesting {
		return errNestingTooDeep
	}
	head, err := c.next()
	if err != nil {
		return err
	}

	switch head.major {
	case majorUnsignedInt, majorNegativeInt, majorSimple:
		return nil // the head carries the whole value
	case majorByteString, majorTextString:
		return c.advance(head.argument)
	case majorArray, majorMap:
		_, err := c.collectionLength(head, depth)
		return err
	case majorTag:
		return c.skipDepth(depth + 1) // the tagged item follows the tag
	default:
		// Unreachable, and so uncovered: major is three bits and every one of its eight
		// values is handled above. It is here to satisfy the compiler.
		return fmt.Errorf("cbor: unknown major type %d", head.major)
	}
}

// collectionLength consumes the array or map whose head has been read and returns its
// element count: array elements, or map entries of a key and a value each. Because
// every counted element is skipped over, the count is always backed by real bytes and
// so cannot exceed the length of data.
func (c *cursor) collectionLength(head item, depth int) (uint64, error) {
	itemsPerElement := uint64(1)
	if head.major == majorMap {
		itemsPerElement = 2
	}
	for range head.argument {
		for range itemsPerElement {
			if err := c.skipDepth(depth + 1); err != nil {
				return 0, err
			}
		}
	}
	return head.argument, nil
}

// entries consumes the next item and returns its element count. A null item — what
// the encoder writes for a nil map or slice — counts as empty.
func (c *cursor) entries() (uint64, error) {
	head, err := c.next()
	if err != nil {
		return 0, err
	}
	if head.isNull() {
		return 0, nil
	}
	if head.major != majorArray && head.major != majorMap {
		return 0, errNotACollection
	}
	return c.collectionLength(head, 0)
}

// nextIsNull reports whether the next item is null, without consuming it.
func (c *cursor) nextIsNull() bool {
	return c.pos < len(c.data) && c.data[c.pos] == cborNull
}

// eachMapEntry calls visit once per entry of the next item, which must be a map or
// null, with the cursor positioned on the entry's key. visit must consume the key and
// the value exactly.
func (c *cursor) eachMapEntry(visit func() error) error {
	head, err := c.next()
	if err != nil {
		return err
	}
	if head.isNull() {
		return nil
	}
	if head.major != majorMap {
		return errNotACollection
	}

	for range head.argument {
		if err := visit(); err != nil {
			return err
		}
	}
	return nil
}

// textString consumes the next item and returns its bytes, which alias data.
func (c *cursor) textString() ([]byte, error) {
	head, err := c.next()
	if err != nil {
		return nil, err
	}
	if head.major != majorTextString {
		return nil, errNotATextString
	}
	start := c.pos
	if err := c.advance(head.argument); err != nil {
		return nil, err
	}
	return c.data[start:c.pos], nil
}
