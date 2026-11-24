package cursor_test

import (
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/internal/cursor"
	"github.com/stretchr/testify/require"
)

// The accepted forms of every value reader are in rfc8949_test.go, and what they refuse in
// refusals_test.go. These tests are about the rest: what a byte string is copied or borrowed from,
// and the refusals only one reader has.

func TestBool(t *testing.T) {
	for want, head := range map[bool]byte{false: 0xf4, true: 0xf5} {
		c := cursor.NewCursor([]byte{head, 0x01})

		got, err := c.Bool()
		require.NoError(t, err)
		require.Equal(t, want, got)
		require.Equal(t, 1, c.Pos())
	}

	t.Run("null is not a boolean", func(t *testing.T) {
		c := cursor.NewCursor([]byte{0xf6})

		_, err := c.Bool()
		require.ErrorIs(t, err, cursor.ErrType)
		require.Zero(t, c.Pos())
	})
}

func TestBytesInto(t *testing.T) {
	t.Run("copies a byte string of the length of dst", func(t *testing.T) {
		c := cursor.NewCursor([]byte{0x43, 0x01, 0x02, 0x03, 0xf6})
		dst := make([]byte, 3)

		require.NoError(t, c.BytesInto(dst))
		require.Equal(t, []byte{0x01, 0x02, 0x03}, dst)
		require.Equal(t, []byte{0xf6}, c.Remaining())
	})

	t.Run("an empty string fills an empty dst", func(t *testing.T) {
		c := cursor.NewCursor([]byte{0x40})

		require.NoError(t, c.BytesInto(nil))
		require.Zero(t, c.Len())
	})

	t.Run("a string of another length is refused and leaves dst alone", func(t *testing.T) {
		for name, data := range map[string][]byte{
			"shorter": {0x42, 0x01, 0x02},
			"longer":  {0x44, 0x01, 0x02, 0x03, 0x04},
		} {
			t.Run(name, func(t *testing.T) {
				c := cursor.NewCursor(data)
				dst := []byte{9, 9, 9}

				require.ErrorIs(t, c.BytesInto(dst), cursor.ErrType)
				require.Equal(t, []byte{9, 9, 9}, dst)
				require.Zero(t, c.Pos())
			})
		}
	})
}

func TestBytes(t *testing.T) {
	t.Run("returns a copy", func(t *testing.T) {
		data := []byte{0x42, 0x01, 0x02}
		c := cursor.NewCursor(data)

		got, err := c.Bytes()
		require.NoError(t, err)
		require.Equal(t, []byte{0x01, 0x02}, got)

		data[1] = 0xff
		require.Equal(t, []byte{0x01, 0x02}, got)
	})

	t.Run("an empty string is an empty slice that is not nil", func(t *testing.T) {
		c := cursor.NewCursor([]byte{0x40})

		got, err := c.Bytes()
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Empty(t, got)
	})
}

func TestBytesBorrowed(t *testing.T) {
	data := []byte{0x42, 0x01, 0x02, 0xf6}
	c := cursor.NewCursor(data)

	got, err := c.BytesBorrowed()
	require.NoError(t, err)
	require.Equal(t, []byte{0x01, 0x02}, got)
	require.Same(t, &data[1], &got[0], "it aliases the input")
	require.Equal(t, []byte{0xf6}, c.Remaining())
}

func TestString(t *testing.T) {
	t.Run("returns a copy", func(t *testing.T) {
		data := []byte{0x62, 'h', 'i'}
		c := cursor.NewCursor(data)

		got, err := c.String()
		require.NoError(t, err)
		require.Equal(t, "hi", got)

		data[1] = 'x'
		require.Equal(t, "hi", got)
	})

	t.Run("text that is not UTF-8 is refused and consumes nothing", func(t *testing.T) {
		c := cursor.NewCursor([]byte{0x62, 0xff, 0xfe})

		_, err := c.String()
		require.ErrorIs(t, err, cursor.ErrSyntax)
		require.Zero(t, c.Pos())
	})
}

func TestBigInt(t *testing.T) {
	t.Run("a bignum of any width", func(t *testing.T) {
		wide := []byte{0x49, 0x01, 0, 0, 0, 0, 0, 0, 0, 0} // 2^64
		for name, tt := range map[string]struct {
			data []byte
			want string
		}{
			"tag 2 is positive":      {append([]byte{0xc2}, wide...), "18446744073709551616"},
			"tag 3 is negative":      {append([]byte{0xc3}, wide...), "-18446744073709551617"},
			"an empty tag 2 is zero": {[]byte{0xc2, 0x40}, "0"},
		} {
			t.Run(name, func(t *testing.T) {
				c := cursor.NewCursor(append(tt.data, 0xf6))

				got, err := c.BigInt()
				require.NoError(t, err)
				require.Equal(t, tt.want, got.String())
				require.Equal(t, []byte{0xf6}, c.Remaining())
			})
		}
	})

	t.Run("a plain integer is a big integer", func(t *testing.T) {
		c := cursor.NewCursor([]byte{0x1b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff})

		got, err := c.BigInt()
		require.NoError(t, err)
		require.Zero(t, got.Cmp(new(big.Int).SetUint64(1<<64-1)))
	})

	t.Run("a refused tag consumes nothing", func(t *testing.T) {
		tests := []struct {
			name string
			data []byte
			err  error
		}{
			{"another tag", []byte{0xc4, 0x42, 0x01, 0x02}, cursor.ErrUnsupported},
			{"a bignum that does not hold a byte string", []byte{0xc2, 0x01}, cursor.ErrSyntax},
			{"a tag with nothing after it", []byte{0xc2}, cursor.ErrTruncated},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				c := cursor.NewCursor(tt.data)

				_, err := c.BigInt()
				require.ErrorIs(t, err, tt.err)
				require.Zero(t, c.Pos())
			})
		}
	})
}
