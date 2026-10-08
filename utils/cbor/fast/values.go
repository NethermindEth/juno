package fast

import (
	"errors"
	"math"
	"math/big"
	"unicode/utf8"
)

// Uint64 reads an unsigned integer. A negative one is a type error.
func (r *Reader) Uint64() (uint64, error) {
	return r.expect(MajorUint)
}

// Int64 reads a signed integer that fits int64.
func (r *Reader) Int64() (int64, error) {
	start := r.pos
	major, argument, err := r.Head()
	if err != nil {
		return 0, err
	}
	if (major != MajorUint && major != MajorNegInt) || argument > math.MaxInt64 {
		r.pos = start
		return 0, ErrType
	}
	if major == MajorNegInt {
		return ^int64(argument), nil
	}
	return int64(argument), nil
}

// Bool reads a boolean.
func (r *Reader) Bool() (bool, error) {
	head, err := r.Peek()
	if err != nil {
		return false, err
	}
	switch head {
	case SimpleFalse:
		r.pos++
		return false, nil
	case SimpleTrue:
		r.pos++
		return true, nil
	default:
		return false, ErrType
	}
}

// payload reads a definite-length string head and returns the payload. It aliases the input.
func (r *Reader) payload(want byte) ([]byte, error) {
	start := r.pos
	length, err := r.expect(want)
	if err != nil {
		return nil, err
	}
	if length > uint64(r.Len()) {
		r.pos = start
		return nil, ErrTruncated
	}
	end := r.pos + int(length)
	out := r.buf[r.pos:end]
	r.pos = end
	return out, nil
}

// BytesNoCopy reads a byte string and returns a sub-slice of the input,
// valid only while the input is.
func (r *Reader) BytesNoCopy() ([]byte, error) {
	return r.payload(MajorBytes)
}

// Bytes reads a byte string into a new slice. An empty string gives an empty, non-nil slice.
func (r *Reader) Bytes() ([]byte, error) {
	borrowed, err := r.payload(MajorBytes)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(borrowed))
	copy(out, borrowed)
	return out, nil
}

// StringNoCopy reads a text string, checks it is valid UTF-8 and returns a sub-slice of the input,
// valid only while the input is.
func (r *Reader) StringNoCopy() ([]byte, error) {
	start := r.pos
	borrowed, err := r.payload(MajorString)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(borrowed) {
		r.pos = start
		return nil, ErrSyntax
	}
	return borrowed, nil
}

// String reads a text string into a new string and checks it is valid UTF-8.
func (r *Reader) String() (string, error) {
	borrowed, err := r.StringNoCopy()
	if err != nil {
		return "", err
	}
	return string(borrowed), nil
}

// BigInt reads an integer of any width: a plain integer or a tag 2/3 bignum.
func (r *Reader) BigInt() (*big.Int, error) {
	start := r.pos
	major, argument, err := r.Head()
	if err != nil {
		return nil, err
	}

	switch major {
	case MajorUint:
		return new(big.Int).SetUint64(argument), nil
	case MajorNegInt:
		return new(big.Int).Not(new(big.Int).SetUint64(argument)), nil
	case MajorTag:
		if argument != tagPositiveBignum && argument != tagNegativeBignum {
			r.pos = start
			return nil, ErrType
		}
		magnitude, err := r.payload(MajorBytes)
		if err != nil {
			r.pos = start
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
		r.pos = start
		return nil, ErrType
	}
}

// Tags for big numbers. https://www.rfc-editor.org/rfc/rfc8949.html#section-3.4.3
const (
	tagPositiveBignum = 2
	tagNegativeBignum = 3
)

// KeyNoCopy reads a map key that is a text string and returns its bytes, valid only while
// the input is. It does not check UTF-8: a key is only compared against known field names,
// so invalid text just fails to match, as in fxamacker.
func (r *Reader) KeyNoCopy() ([]byte, error) {
	return r.payload(MajorString)
}
