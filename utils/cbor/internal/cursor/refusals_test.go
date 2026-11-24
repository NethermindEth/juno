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

var (
	integer = []byte{0x01}
	text    = []byte{0x61, 'a'}
	bytes1  = []byte{0x41, 'a'}
)

var readers = []reader{
	{"Peek", func(c *cursor.Cursor) error { return ignore(c.Peek()) }, nil, nil},
	{"Tag", func(c *cursor.Cursor) error { return ignore(c.Tag()) }, integer, nil},
	{"ArrayLen", func(c *cursor.Cursor) error { return ignore(c.ArrayLen()) }, integer, []byte{0x83, 0x01}},
	{"MapLen", func(c *cursor.Cursor) error { return ignore(c.MapLen()) }, integer, []byte{0xa3, 0x01, 0x02, 0x03}},
	{"Uint64", func(c *cursor.Cursor) error { return ignore(c.Uint64()) }, text, nil},
	{"Int64", func(c *cursor.Cursor) error { return ignore(c.Int64()) }, text, nil},
	{"Bool", func(c *cursor.Cursor) error { return ignore(c.Bool()) }, integer, nil},
	{"BytesInto", func(c *cursor.Cursor) error { return c.BytesInto(make([]byte, 3)) }, text, []byte{0x43, 0x01, 0x02}},
	{"Bytes", func(c *cursor.Cursor) error { return ignore(c.Bytes()) }, text, []byte{0x43, 0x01, 0x02}},
	{"BytesBorrowed", func(c *cursor.Cursor) error { return ignore(c.BytesBorrowed()) }, text, []byte{0x43, 0x01, 0x02}},
	{"String", func(c *cursor.Cursor) error { return ignore(c.String()) }, bytes1, []byte{0x63, 'a', 'b'}},
	{"BigInt", func(c *cursor.Cursor) error { return ignore(c.BigInt()) }, text, []byte{0xc2, 0x42, 0x01}},
	{"Key", func(c *cursor.Cursor) error { _, _, err := c.Key(); return err }, bytes1, []byte{0x63, 'A'}},
	{
		"OpenStruct",
		func(c *cursor.Cursor) error { return ignore(c.OpenStruct(cursor.Fields{})) },
		integer,
		[]byte{0xa3, 0x01},
	},
	{
		"Limbs",
		func(c *cursor.Cursor) error { var f [cursor.Limbs]uint64; return c.Limbs(&f) },
		integer,
		[]byte{0x84, 0x01, 0x02},
	},
	{
		"Felt",
		func(c *cursor.Cursor) error { var f [cursor.Limbs]uint64; return cursor.Felt(c, &f) },
		integer,
		[]byte{0x84, 0x01, 0x02},
	},
	{
		"LimbSlice",
		func(c *cursor.Cursor) error { return ignore(cursor.LimbSlice[[cursor.Limbs]uint64](c)) },
		integer, nil,
	},
	{"Skip", func(c *cursor.Cursor) error { return c.Skip() }, nil, []byte{0x83, 0x01, 0x02}},
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
