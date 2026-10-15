package fast_test

import (
	"bytes"
	"errors"
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/fast"
	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	"github.com/stretchr/testify/require"
)

// fxSkip is what fxamacker does with an item the destination has no field for: the item sits
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

// readerSeeds are felts and felt arrays in the 37 byte form, with short limbs, and with the
// wrong number of limbs, then keys long enough for a one-byte length and with non-ASCII text.
func readerSeeds() [][]byte {
	return [][]byte{
		feltArray(1)[5:],
		feltArray(3),
		{0x82, 0x84, 0x00, 0x18, 0x1e, 0x19, 0x01, 0x00, 0x1a, 0, 0, 1, 0, 0x83, 0x01, 0x02, 0x03},
		{0x81, 0x85, 0x01, 0x02, 0x03, 0x04, 0x05},
		{0x84, 0x01, 0x20, 0x02, 0x03},
		{0x78, 0x18, 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x'},
		{0x6a, 'k', 'e', 'y', 0xc3, 0xa9, 'a', 'b', 'c', 'd', 'e'},
	}
}

// FuzzSkipMatchesFxamacker checks Skip against the decoder it replaces: both must agree on
// whether the first item is well-formed and on where it ends. The one allowed difference is
// ErrUnsupported, an indefinite-length item that this package leaves to the caller's fallback.
func FuzzSkipMatchesFxamacker(f *testing.F) {
	for _, s := range seeds() {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		// Both sides see the item under a map key, as a skipped field is.
		rest, fxErr := fxSkip(data)
		data = wrapped(data)

		r := fast.NewReader(data)
		err := r.Skip()

		if errors.Is(err, fast.ErrUnsupported) {
			return
		}
		require.Equalf(t, fxErr == nil, err == nil, "fx=%v fast=%v", fxErr, err)
		if err != nil {
			require.Zero(t, r.Pos())
			return
		}
		require.Equal(t, len(rest), r.Len())
		require.True(t, bytes.Equal(rest, r.Remaining()))
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
			var want uint64
			rest, fxErr := fxamacker.UnmarshalFirst(data, &want)
			r := fast.NewReader(data)
			got, err := r.Uint64()
			if err != nil {
				return
			}
			require.NoError(t, fxErr)
			require.Equal(t, want, got)
			require.Equal(t, rest, r.Remaining())
		})
		t.Run("int64", func(t *testing.T) {
			var want int64
			rest, fxErr := fxamacker.UnmarshalFirst(data, &want)
			r := fast.NewReader(data)
			got, err := r.Int64()
			if err != nil {
				return
			}
			require.NoError(t, fxErr)
			require.Equal(t, want, got)
			require.Equal(t, rest, r.Remaining())
		})
		t.Run("bool", func(t *testing.T) {
			var want bool
			rest, fxErr := fxamacker.UnmarshalFirst(data, &want)
			r := fast.NewReader(data)
			got, err := r.Bool()
			if err != nil {
				return
			}
			require.NoError(t, fxErr)
			require.Equal(t, want, got)
			require.Equal(t, rest, r.Remaining())
		})
		t.Run("string", func(t *testing.T) {
			var want string
			rest, fxErr := fxamacker.UnmarshalFirst(data, &want)
			r := fast.NewReader(data)
			got, err := r.String()
			if err != nil {
				return
			}
			require.NoError(t, fxErr)
			require.Equal(t, want, got)
			require.Equal(t, rest, r.Remaining())
		})
		t.Run("key", func(t *testing.T) {
			r := fast.NewReader(data)
			got, isText, err := r.Key()
			if err != nil {
				require.Zero(t, r.Pos(), "a failed read must not consume")
				return
			}
			if isText {
				var want string
				rest, fxErr := fxamacker.UnmarshalFirst(data, &want)
				require.NoError(t, fxErr)
				require.Equal(t, want, string(got))
				require.Equal(t, rest, r.Remaining())
				return
			}
			var want int64
			rest, fxErr := fxamacker.UnmarshalFirst(data, &want)
			require.NoError(t, fxErr)
			require.Equal(t, rest, r.Remaining())
		})
		t.Run("bytes", func(t *testing.T) {
			var want []byte
			rest, fxErr := fxamacker.UnmarshalFirst(data, &want)
			r := fast.NewReader(data)
			got, err := r.Bytes()
			if err != nil {
				return
			}
			require.NoError(t, fxErr)
			require.Equal(t, want, got)
			require.Equal(t, rest, r.Remaining())
		})
		t.Run("bigint", func(t *testing.T) {
			var want big.Int
			rest, fxErr := fxamacker.UnmarshalFirst(data, &want)
			r := fast.NewReader(data)
			got, err := r.BigInt()
			if err != nil {
				return
			}
			require.NoError(t, fxErr)
			require.Zero(t, want.Cmp(got))
			require.Equal(t, rest, r.Remaining())
		})
	})
}

// FuzzNeverPanics drives every reader over arbitrary bytes, as a decoder does when it walks
// a record, and checks the cursor stays inside the input.
func FuzzNeverPanics(f *testing.F) {
	for _, s := range seeds() {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		r := fast.NewReader(data)
		for range 64 {
			before := r.Pos()
			switch before % 7 {
			case 0:
				_, _ = r.ArrayLen()
			case 1:
				_, _ = r.MapLen()
			case 2:
				_, _ = r.BigInt()
			case 3:
				_, _ = r.StringNoCopy()
			case 4:
				_, _ = r.BytesNoCopy()
			case 5:
				_, _ = r.Tag()
			default:
				_ = r.Skip()
			}
			require.LessOrEqual(t, r.Pos(), len(data))
			if r.Len() == 0 {
				break
			}
			if r.Pos() == before {
				if r.Skip() != nil {
					break
				}
			}
		}
	})
}
