package cursor

import (
	"encoding/binary"
	"errors"
	"math"
	"math/big"
	"math/bits"
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
		return 0, c.mismatch("an integer", c.buf[start])
	}
	if argument > math.MaxInt64 {
		c.pos = start
		return 0, c.fail(ErrType, "integer does not fit int64")
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
		return false, c.mismatch("a boolean", head)
	}
}

// payload reads the head of a definite-length byte or text string and returns its contents, which
// alias the input.
func (c *Cursor) payload(want byte) ([]byte, error) {
	start := c.pos
	length, err := c.expect(want)
	if err != nil {
		return nil, err
	}
	if length > uint64(c.Len()) {
		c.pos = start
		return nil, c.fail(ErrTruncated, "byte or text string longer than the input")
	}
	end := c.pos + int(length)
	out := c.buf[c.pos:end]
	c.pos = end
	return out, nil
}

// BytesInto reads a byte string of exactly len(dst) bytes into dst.
func (c *Cursor) BytesInto(dst []byte) error {
	start := c.pos
	borrowed, err := c.payload(majorBytes)
	if err != nil {
		return err
	}
	if len(borrowed) != len(dst) {
		c.pos = start
		return c.fail(ErrType, "byte string of %d bytes, destination of %d", len(borrowed), len(dst))
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

// BytesBorrowed is Bytes without the copy, so the result aliases the input.
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
		return "", c.fail(ErrSyntax, "text string is not valid UTF-8")
	}
	return string(borrowed), nil
}

// BigInt reads an integer of any width, either a plain integer or a tag 2 or 3 bignum.
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
		if argument != math.MaxUint64 {
			out := new(big.Int).SetUint64(argument + 1)
			return out.Neg(out), nil
		}
		// -1-MaxUint64 has magnitude 2^64, one bit beyond a uint64.
		return bigIntMagnitude([]byte{1, 0, 0, 0, 0, 0, 0, 0, 0}, true, false), nil
	case MajorTag:
		if argument != tagPositiveBignum && argument != tagNegativeBignum {
			c.pos = start
			return nil, c.fail(ErrUnsupported, "tag %d where an integer is expected", argument)
		}
		magnitude, err := c.payload(majorBytes)
		if err != nil {
			c.pos = start
			if errors.Is(err, ErrType) {
				err = c.fail(ErrSyntax, "tag 2 or 3 must hold a byte string")
			}
			return nil, err
		}
		negative := argument == tagNegativeBignum
		return bigIntMagnitude(magnitude, negative, negative), nil
	default:
		c.pos = start
		return nil, c.mismatch("an integer", c.buf[start])
	}
}

// Tags for big numbers. https://www.rfc-editor.org/rfc/rfc8949.html#section-3.4.3
const (
	tagPositiveBignum = 2
	tagNegativeBignum = 3
)

// bigIntMagnitude constructs an integer from owned words. CBOR tag 3 stores
// one less than the negative number's magnitude, so increment adds that one.
// SetBits normalizes leading zero words; Neg is called on the same Int, avoiding
// the assembly arithmetic used by Int.Not. No returned word aliases data.
func bigIntMagnitude(data []byte, negative, increment bool) *big.Int {
	const wordBytes = bits.UintSize / 8
	count := len(data) / wordBytes
	if len(data)%wordBytes != 0 {
		count++
	}
	if increment {
		// Room for a carry beyond every input word.
		count++
	}
	var words []big.Word
	if count != 0 {
		words = make([]big.Word, count)
	}
	i := 0
	for len(data) >= wordBytes {
		start := len(data) - wordBytes
		if wordBytes == 8 {
			words[i] = big.Word(binary.BigEndian.Uint64(data[start:]))
		} else {
			words[i] = big.Word(binary.BigEndian.Uint32(data[start:]))
		}
		data = data[:start]
		i++
	}
	if len(data) != 0 {
		var word big.Word
		for _, b := range data {
			word = word<<8 | big.Word(b)
		}
		words[i] = word
	}
	if increment {
		for i := range words {
			words[i]++
			if words[i] != 0 {
				break
			}
		}
	}
	out := new(big.Int).SetBits(words)
	if negative {
		out.Neg(out)
	}
	return out
}
