// These fuzz tests pin the Cursor to the behavior of fxamacker. Arbitrary bytes are read with both,
// and wherever the Cursor succeeds fxamacker must succeed with the same value and the same
// remainder. The Cursor may be stricter, but never more lenient.

package cbor_test

import (
	"bytes"
	"errors"
	"math/big"
	"slices"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	"github.com/NethermindEth/juno/utils/cbor/cursor"
	"github.com/stretchr/testify/require"
)

// fxSkip is what fxamacker does with an item the destination has no field for. The item sits
// under an unknown key, so it is checked for well-formedness and skipped, never decoded.
// It returns the bytes after the item. The key's map adds one nesting level, which wrapped
// compensates for.
func fxSkip(item []byte) ([]byte, error) {
	var none struct{}
	return fxamacker.UnmarshalFirst(wrapped(item), &none)
}

// wrapped puts item under the one-character key "z" of a map.
func wrapped(item []byte) []byte {
	return append([]byte{0xa1, 0x61, 'z'}, item...)
}

func nested(levels int, leaf byte) []byte {
	return append(bytes.Repeat([]byte{0x81}, levels), leaf)
}

// felts is an array of n felts in the uint64 form, as a Sierra program is stored.
func felts(n int) []byte {
	felt := []byte{
		0x84, 0x1b, 1, 2, 3, 4, 5, 6, 7, 8, 0x1b, 1, 2, 3, 4, 5, 6, 7, 8,
		0x1b, 1, 2, 3, 4, 5, 6, 7, 8, 0x1b, 1, 2, 3, 4, 5, 6, 7, 8,
	}
	return slices.Concat([]byte{0x80 | 0x18, byte(n)}, slices.Repeat(felt, n))
}

// seeds are well-formed and malformed items covering every major type.
func seeds() [][]byte {
	return [][]byte{
		{0x00},
		{0x17},
		{0x18, 0x18},
		{0x19, 0x01, 0x00},
		{0x1a, 0, 1, 0, 0},
		{0x1b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		{0x20},
		{0x38, 0xff},
		{0x3b, 0x80, 0, 0, 0, 0, 0, 0, 0},
		{0x40},
		{0x43, 1, 2, 3},
		{0x60},
		{0x62, 'h', 'i'},
		{0x62, 0xc3, 0x28},
		{0x80},
		{0x82, 0x01, 0x62, 'h', 'i'},
		{0xa0},
		{0xa1, 0x61, 'a', 0x01},
		{0xc2, 0x41, 0x01},
		{0xc3, 0x41, 0x01},
		{0xc2, 0x01},
		{0xc1, 0xc1, 0xc1, 0x00},
		{0xf4},
		{0xf5},
		{0xf6},
		{0xf7},
		{0xf8, 0x20},
		{0xf8, 0x10},
		{0xf9, 0x3c, 0},
		{0x5f, 0x41, 1, 0xff},
		{0x9f, 0xff},
		{0xff},
		{0x1c},
		{0x9b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		{0x5b, 0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		nested(31, 0x01), nested(32, 0x01), nested(33, 0x01),
	}
}

// readerSeeds are felts and slices of felts in the uint64 form, with limbs of shorter widths, and
// with the wrong number of limbs, then keys long enough for a one-byte length and with non-ASCII
// text.
func readerSeeds() [][]byte {
	return [][]byte{
		felts(1)[2:],
		felts(3),
		{0x82, 0x84, 0x00, 0x18, 0x1e, 0x19, 0x01, 0x00, 0x1a, 0, 0, 1, 0, 0x83, 0x01, 0x02, 0x03},
		{0x81, 0x85, 0x01, 0x02, 0x03, 0x04, 0x05},
		{0x84, 0x01, 0x20, 0x02, 0x03},
		append([]byte{0x78, 0x18}, "abcdefghijklmnopqrstuvwx"...),
		{0x6a, 'k', 'e', 'y', 0xc3, 0xa9, 'a', 'b', 'c', 'd', 'e'},
	}
}

// FuzzSkipMatchesFxamacker checks Skip against the decoder it replaces. Both must agree on
// whether the first item is well-formed and on where it ends. The one allowed difference is
// ErrUnsupported, an indefinite-length item that the Cursor does not read.
func FuzzSkipMatchesFxamacker(f *testing.F) {
	for _, s := range seeds() {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		// Both sides see the item under a map key, as a skipped field is.
		rest, fxErr := fxSkip(data)
		data = wrapped(data)

		c := cursor.NewCursor(data)
		err := c.Skip()

		if errors.Is(err, cursor.ErrUnsupported) {
			return
		}
		require.Equalf(t, fxErr == nil, err == nil, "fx=%v fast=%v", fxErr, err)
		if err != nil {
			require.Zero(t, c.Pos())
			return
		}
		require.Equal(t, len(rest), c.Len())
		require.True(t, bytes.Equal(rest, c.Remaining()))
	})
}

// FuzzReadersMatchFxamacker checks that every typed reader that succeeds yields what the
// fxamacker decoder yields, with the same remainder. A reader may be stricter than
// fxamacker (it does not coerce between types), never more lenient.
func FuzzReadersMatchFxamacker(f *testing.F) {
	for _, s := range append(seeds(), readerSeeds()...) {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		t.Run("uint64", func(t *testing.T) {
			readerMatches(t, data, (*cursor.Cursor).Uint64, same[uint64])
		})
		t.Run("int64", func(t *testing.T) { readerMatches(t, data, (*cursor.Cursor).Int64, same[int64]) })
		t.Run("bool", func(t *testing.T) { readerMatches(t, data, (*cursor.Cursor).Bool, same[bool]) })
		t.Run("string", func(t *testing.T) {
			readerMatches(t, data, (*cursor.Cursor).String, same[string])
		})
		t.Run("bytes", func(t *testing.T) { readerMatches(t, data, (*cursor.Cursor).Bytes, bytes.Equal) })
		t.Run("bigint", func(t *testing.T) {
			readerMatches(t, data, func(c *cursor.Cursor) (big.Int, error) {
				got, err := c.BigInt()
				if err != nil {
					return big.Int{}, err
				}
				return *got, nil
			}, func(want, got big.Int) bool { return want.Cmp(&got) == 0 })
		})
		t.Run("key", func(t *testing.T) {
			c := cursor.NewCursor(data)
			got, isText, err := c.Key()
			if err != nil {
				require.Zero(t, c.Pos(), "a failed read must not consume")
				return
			}
			if isText {
				var want string
				rest, fxErr := fxamacker.UnmarshalFirst(data, &want)
				require.NoError(t, fxErr)
				require.Equal(t, want, string(got))
				require.Equal(t, rest, c.Remaining())
				return
			}
			var want int64
			rest, fxErr := fxamacker.UnmarshalFirst(data, &want)
			require.NoError(t, fxErr)
			require.Equal(t, rest, c.Remaining())
		})
	})
}

func same[T comparable](want, got T) bool { return want == got }

// readerMatches checks one typed reader against fxamacker decoding the same data into a T. If
// the reader succeeds, fxamacker succeeds with an equal value and the same remainder.
func readerMatches[T any](
	t *testing.T,
	data []byte,
	read func(*cursor.Cursor) (T, error),
	equal func(want, got T) bool,
) {
	t.Helper()
	var want T
	rest, fxErr := fxamacker.UnmarshalFirst(data, &want)
	c := cursor.NewCursor(data)
	got, err := read(&c)
	if err != nil {
		return
	}
	require.NoError(t, fxErr)
	require.True(t, equal(want, got), "fx=%v fast=%v", want, got)
	require.Equal(t, rest, c.Remaining())
}

// FuzzNeverPanics drives every reader over arbitrary bytes, as a decoder does when it walks
// a record, and checks the Cursor stays inside the input.
func FuzzNeverPanics(f *testing.F) {
	for _, s := range seeds() {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		c := cursor.NewCursor(data)
		for range 64 {
			before := c.Pos()
			switch before % 7 {
			case 0:
				_, _ = c.ArrayLen()
			case 1:
				_, _ = c.MapLen()
			case 2:
				_, _ = c.BigInt()
			case 3:
				_, _ = c.String()
			case 4:
				_, _ = c.Bytes()
			case 5:
				_, _ = c.Tag()
			default:
				_ = c.Skip()
			}
			require.LessOrEqual(t, c.Pos(), len(data))
			if c.Len() == 0 {
				break
			}
			if c.Pos() == before {
				if c.Skip() != nil {
					break
				}
			}
		}
	})
}

// tagChainFelt is a felt of 21 items, so one that skips what follows the four limbs. These are
// sixteen negative integers and a last item that is a chain of 33 tags around one more. fxamacker
// counts the chain as 33 nesting levels and rejects it, and so must the readers. Eighteen bytes
// follow.
func tagChainFelt() []byte {
	data := []byte{0x95, 1, 0, 0, 0}
	data = append(data, bytes.Repeat([]byte{0x30}, 16)...)
	data = append(data, bytes.Repeat([]byte{0xd1}, 33)...)
	return append(data, bytes.Repeat([]byte{0x30}, 19)...)
}

func feltSeeds() [][]byte {
	full := []byte{
		0x84, 0x1b, 1, 2, 3, 4, 5, 6, 7, 8, 0x1b, 0xff, 0, 0, 0, 0, 0, 0, 1,
		0x1b, 0, 0, 0, 0, 0, 0, 0, 9, 0x1b, 0x07, 0xff, 0xff, 0xff, 0xff, 0xff, 0xf1, 0x30,
	}
	return [][]byte{
		full,
		{0x84, 0x00, 0x18, 0x18, 0x19, 0x01, 0x00, 0x1a, 0, 1, 0, 0},
		{0x80},
		{0x82, 0x01, 0x02},
		{0x85, 1, 2, 3, 4, 5},
		{0x84, 1, 2, 3, 0x20},
		{0x81, 0x84, 1, 2, 3, 4},
		{0xf6},
		{0x9f, 0xff},
		append([]byte{0x82}, append(append([]byte(nil), full...), full...)...),
		tagChainFelt(),
	}
}

// FuzzFeltsMatchFxamacker checks the felt readers against fxamacker decoding into the same
// shapes, a [4]uint64 and a slice of them. A reader that succeeds must agree with fxamacker on
// the value and the remainder. Where fxamacker succeeds, a reader may only fail by being
// stricter about types (ErrType) or by not reading a form (ErrUnsupported).
func FuzzFeltsMatchFxamacker(f *testing.F) {
	for _, s := range append(feltSeeds(), seeds()...) {
		f.Add(s)
	}
	allowed := func(err error) bool {
		return errors.Is(err, cursor.ErrType) || errors.Is(err, cursor.ErrUnsupported)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		t.Run("limbs", func(t *testing.T) {
			var want, got [cursor.Limbs]uint64
			rest, fxErr := fxamacker.UnmarshalFirst(data, &want)
			c := cursor.NewCursor(data)
			err := c.Limbs(&got)
			if err != nil {
				require.Zero(t, c.Pos(), "a failed read moved the Cursor")
				require.True(t, fxErr != nil || allowed(err), "fxamacker accepts what Limbs rejects: %v", err)
				return
			}
			require.NoError(t, fxErr)
			require.Equal(t, want, got)
			require.Equal(t, rest, c.Remaining())
		})
		t.Run("slice", func(t *testing.T) {
			var want [][cursor.Limbs]uint64
			rest, fxErr := fxamacker.UnmarshalFirst(data, &want)
			c := cursor.NewCursor(data)
			got, err := cursor.LimbSlice[[cursor.Limbs]uint64](&c)
			if err != nil {
				require.Zero(t, c.Pos(), "a failed read moved the Cursor")
				require.True(t, fxErr != nil || allowed(err),
					"fxamacker accepts what LimbSlice rejects: %v", err)
				return
			}
			require.NoError(t, fxErr)
			require.Equal(t, want, got)
			require.Equal(t, rest, c.Remaining())
		})
	})
}
