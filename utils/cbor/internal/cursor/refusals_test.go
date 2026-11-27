package cursor_test

import (
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/internal/cursor"
	"github.com/stretchr/testify/require"
)

// reader is a Cursor method that reads one item. wrong is an item of a type the reader does not
// read, and short an item it does read that the input ends inside. Either is nil if the reader has
// no such input.
type reader struct {
	name  string
	read  func(*cursor.Cursor) error
	wrong []byte
	short []byte
}

func ignore[T any](_ T, err error) error { return err }

// Items of the wrong type for some reader.
var (
	integer = []byte{0x01}      // the integer 1
	text    = []byte{0x61, 'a'} // the text "a"
	bytes1  = []byte{0x41, 'a'} // a byte string of one byte
)

// Items the input ends inside.
var (
	shortArray  = []byte{0x83, 0x01}             // an array of 3 with one element
	shortMap    = []byte{0xa3, 0x01, 0x02, 0x03} // a map of 3 pairs with three items
	shortStruct = []byte{0xa3, 0x01}             // a map of 3 pairs with one item
	shortBytes  = []byte{0x43, 0x01, 0x02}       // a byte string of three bytes with two
	shortText   = []byte{0x63, 'a', 'b'}         // a text string of three bytes with two
	shortKey    = []byte{0x63, 'A'}              // a text string of three bytes with one
	shortBignum = []byte{0xc2, 0x42, 0x01}       // tag 2, a byte string of two bytes with one
	shortFelt   = []byte{0x84, 0x01, 0x02}       // an array of 4 limbs with two
)

var readers = []reader{
	{"Peek", func(c *cursor.Cursor) error { return ignore(c.Peek()) }, nil, nil},
	{"Tag", func(c *cursor.Cursor) error { return ignore(c.Tag()) }, integer, nil},
	{"ArrayLen", func(c *cursor.Cursor) error { return ignore(c.ArrayLen()) }, integer, shortArray},
	{"MapLen", func(c *cursor.Cursor) error { return ignore(c.MapLen()) }, integer, shortMap},
	{"Uint64", func(c *cursor.Cursor) error { return ignore(c.Uint64()) }, text, nil},
	{"Int64", func(c *cursor.Cursor) error { return ignore(c.Int64()) }, text, nil},
	{"Bool", func(c *cursor.Cursor) error { return ignore(c.Bool()) }, integer, nil},
	{
		"BytesInto",
		func(c *cursor.Cursor) error { return c.BytesInto(make([]byte, 3)) },
		text,
		shortBytes,
	},
	{"Bytes", func(c *cursor.Cursor) error { return ignore(c.Bytes()) }, text, shortBytes},
	{
		"BytesBorrowed",
		func(c *cursor.Cursor) error { return ignore(c.BytesBorrowed()) },
		text,
		shortBytes,
	},
	{"String", func(c *cursor.Cursor) error { return ignore(c.String()) }, bytes1, shortText},
	{"BigInt", func(c *cursor.Cursor) error { return ignore(c.BigInt()) }, text, shortBignum},
	{"Key", func(c *cursor.Cursor) error { _, _, err := c.Key(); return err }, bytes1, shortKey},
	{
		"OpenStruct",
		func(c *cursor.Cursor) error { return ignore(c.OpenStruct(cursor.Fields{})) },
		integer,
		shortStruct,
	},
	{
		"Limbs",
		func(c *cursor.Cursor) error { var f [cursor.Limbs]uint64; return c.Limbs(&f) },
		integer,
		shortFelt,
	},
	{
		"Felt",
		func(c *cursor.Cursor) error { var f [cursor.Limbs]uint64; return cursor.Felt(c, &f) },
		integer,
		shortFelt,
	},
	{
		"LimbSlice",
		func(c *cursor.Cursor) error { return ignore(cursor.LimbSlice[[cursor.Limbs]uint64](c)) },
		integer, nil,
	},
	{"Skip", func(c *cursor.Cursor) error { return c.Skip() }, nil, shortArray},
}

// Every reader that returns an error refuses the same three kinds of input in the same way: the
// error says which, and nothing is consumed.
func TestReadersRefuseWithoutConsuming(t *testing.T) {
	refuses := func(t *testing.T, r reader, data []byte, want error) {
		t.Helper()
		c := cursor.NewCursor(data)

		require.ErrorIs(t, r.read(&c), want)
		require.Zero(t, c.Pos())
	}

	for _, r := range readers {
		t.Run(r.name, func(t *testing.T) {
			t.Run("the end of the input", func(t *testing.T) {
				refuses(t, r, nil, cursor.ErrTruncated)
			})
			if r.wrong != nil {
				t.Run("another type", func(t *testing.T) {
					refuses(t, r, r.wrong, cursor.ErrType)
				})
			}
			if r.short != nil {
				t.Run("an item cut short", func(t *testing.T) {
					refuses(t, r, r.short, cursor.ErrTruncated)
				})
			}
		})
	}
}
