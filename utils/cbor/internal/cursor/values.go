package cursor

import (
	"errors"
	"math"
	"math/big"
	"unicode/utf8"
)

// Uint64 reads an unsigned integer. A negative one is a type error.
func (c *Cursor) Uint64() (uint64, error) {
	return c.expect(MajorUint)
}

// Int64 reads a signed integer that fits int64.
func (c *Cursor) Int64() (int64, error) {
	start := c.pos
	major, argument, err := c.head()
	if err != nil {
		return 0, err
	}
	if major != MajorUint && major != majorNegInt {
		c.pos = start
		return 0, mismatch(major)
	}
	if argument > math.MaxInt64 {
		c.pos = start
		return 0, ErrType
	}
	if major == majorNegInt {
		return ^int64(argument), nil
	}
	return int64(argument), nil
}

// Bool reads a boolean.
func (c *Cursor) Bool() (bool, error) {
	head, err := c.Peek()
	if err != nil {
		return false, err
	}
	switch head {
	case simpleFalse:
		c.pos++
		return false, nil
	case simpleTrue:
		c.pos++
		return true, nil
	default:
		return false, mismatch(head & majorMask)
	}
}

// payload reads a definite-length string head and returns the payload. It aliases the input.
func (c *Cursor) payload(want byte) ([]byte, error) {
	start := c.pos
	length, err := c.expect(want)
	if err != nil {
		return nil, err
	}
	if length > uint64(c.Len()) {
		c.pos = start
		return nil, ErrTruncated
	}
	end := c.pos + int(length)
	out := c.buf[c.pos:end]
	c.pos = end
	return out, nil
}

// BytesInto reads a byte string of exactly len(dst) bytes into dst. A string of another length
// is ErrType and consumes nothing.
func (c *Cursor) BytesInto(dst []byte) error {
	start := c.pos
	borrowed, err := c.payload(majorBytes)
	if err != nil {
		return err
	}
	if len(borrowed) != len(dst) {
		c.pos = start
		return ErrType
	}
	copy(dst, borrowed)
	return nil
}

// Bytes reads a byte string into a new slice. An empty string gives an empty, non-nil slice.
func (c *Cursor) Bytes() ([]byte, error) {
	borrowed, err := c.payload(majorBytes)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(borrowed))
	copy(out, borrowed)
	return out, nil
}

// BytesBorrowed reads a byte string and returns its payload without copying it. The payload aliases
// the input, so it is only valid while the input is.
func (c *Cursor) BytesBorrowed() ([]byte, error) {
	return c.payload(majorBytes)
}

// String reads a text string into a new string and checks it is valid UTF-8.
func (c *Cursor) String() (string, error) {
	start := c.pos
	borrowed, err := c.payload(majorString)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(borrowed) {
		c.pos = start
		return "", ErrSyntax
	}
	return string(borrowed), nil
}

// BigInt reads an integer of any width: a plain integer or a tag 2/3 bignum.
func (c *Cursor) BigInt() (*big.Int, error) {
	start := c.pos
	major, argument, err := c.head()
	if err != nil {
		return nil, err
	}

	switch major {
	case MajorUint:
		return new(big.Int).SetUint64(argument), nil
	case majorNegInt:
		return new(big.Int).Not(new(big.Int).SetUint64(argument)), nil
	case MajorTag:
		if argument != tagPositiveBignum && argument != tagNegativeBignum {
			c.pos = start
			return nil, ErrUnsupported
		}
		magnitude, err := c.payload(majorBytes)
		if err != nil {
			c.pos = start
			if errors.Is(err, ErrType) {
				err = ErrSyntax // tag 2 and 3 must hold a byte string
			}
			return nil, err
		}
		out := new(big.Int).SetBytes(magnitude)
		if argument == tagNegativeBignum {
			out.Not(out)
		}
		return out, nil
	default:
		c.pos = start
		return nil, ErrType
	}
}

// Tags for big numbers. https://www.rfc-editor.org/rfc/rfc8949.html#section-3.4.3
const (
	tagPositiveBignum = 2
	tagNegativeBignum = 3
)
