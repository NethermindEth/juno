package fast_test

import (
	"math"
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/fast"
	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	"github.com/stretchr/testify/require"
)

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := fxamacker.Marshal(v)
	require.NoError(t, err)
	return b
}

func TestHeadArguments(t *testing.T) {
	for _, want := range []uint64{
		0, 1, 23, 24, 255, 256, 65535, 65536, math.MaxUint32, math.MaxUint32 + 1, math.MaxUint64,
	} {
		r := fast.NewReader(mustMarshal(t, want))
		got, err := r.Uint64()
		require.NoError(t, err)
		require.Equal(t, want, got)
		require.Zero(t, r.Len())
	}
}

func TestHeadErrors(t *testing.T) {
	tests := map[string]struct {
		data []byte
		err  error
	}{
		"empty":                {nil, fast.ErrTruncated},
		"1-byte arg missing":   {[]byte{0x18}, fast.ErrTruncated},
		"8-byte arg truncated": {[]byte{0x1b, 1, 2, 3}, fast.ErrTruncated},
		"reserved info 28":     {[]byte{0x1c}, fast.ErrSyntax},
		"reserved info 30":     {[]byte{0x1e}, fast.ErrSyntax},
		"indefinite uint":      {[]byte{0x1f}, fast.ErrSyntax},
		"break":                {[]byte{0xff}, fast.ErrSyntax},
		"indefinite bytes":     {[]byte{0x5f, 0xff}, fast.ErrUnsupported},
		"indefinite array":     {[]byte{0x9f, 0xff}, fast.ErrUnsupported},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r := fast.NewReader(tt.data)
			_, _, err := r.Head()
			require.ErrorIs(t, err, tt.err)
			require.Zero(t, r.Pos(), "a failed head must not consume")
		})
	}
}

func TestTypeMismatchDoesNotConsume(t *testing.T) {
	r := fast.NewReader(mustMarshal(t, "abc"))
	_, err := r.Uint64()
	require.ErrorIs(t, err, fast.ErrType)
	_, err = r.Bytes()
	require.ErrorIs(t, err, fast.ErrType)
	_, err = r.ArrayLen()
	require.ErrorIs(t, err, fast.ErrType)
	_, err = r.Bool()
	require.ErrorIs(t, err, fast.ErrType)
	require.Zero(t, r.Pos())

	got, err := r.String()
	require.NoError(t, err)
	require.Equal(t, "abc", got)
}

func TestInt64(t *testing.T) {
	for _, want := range []int64{0, 1, -1, 23, -24, -25, 255, -256, math.MaxInt64, math.MinInt64} {
		r := fast.NewReader(mustMarshal(t, want))
		got, err := r.Int64()
		require.NoError(t, err)
		require.Equal(t, want, got)
	}

	// 2^63 and -2^63-1 are well-formed but do not fit int64.
	for _, data := range [][]byte{
		{0x1b, 0x80, 0, 0, 0, 0, 0, 0, 0},
		{0x3b, 0x80, 0, 0, 0, 0, 0, 0, 0},
	} {
		r := fast.NewReader(data)
		_, err := r.Int64()
		require.ErrorIs(t, err, fast.ErrType)
		require.Zero(t, r.Pos())
	}
}

func TestBool(t *testing.T) {
	for _, want := range []bool{true, false} {
		r := fast.NewReader(mustMarshal(t, want))
		got, err := r.Bool()
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
	r := fast.NewReader(nil)
	_, err := r.Bool()
	require.ErrorIs(t, err, fast.ErrTruncated)
}

func TestNull(t *testing.T) {
	r := fast.NewReader([]byte{0xf6, 0x01})
	require.True(t, r.Null())
	require.False(t, r.Null())
	got, err := r.Uint64()
	require.NoError(t, err)
	require.EqualValues(t, 1, got)
	require.False(t, r.Null(), "no input is not null")
}

func TestBytesAndStrings(t *testing.T) {
	t.Run("copy owns its memory", func(t *testing.T) {
		data := mustMarshal(t, []byte{1, 2, 3})
		r := fast.NewReader(data)
		got, err := r.Bytes()
		require.NoError(t, err)
		clear(data)
		require.Equal(t, []byte{1, 2, 3}, got)
	})

	t.Run("string copy owns its memory", func(t *testing.T) {
		data := mustMarshal(t, "hello")
		r := fast.NewReader(data)
		got, err := r.String()
		require.NoError(t, err)
		clear(data)
		require.Equal(t, "hello", got)
	})

	t.Run("no copy aliases the input", func(t *testing.T) {
		data := mustMarshal(t, []byte{1, 2, 3})
		r := fast.NewReader(data)
		got, err := r.BytesNoCopy()
		require.NoError(t, err)
		data[len(data)-1] = 9
		require.Equal(t, []byte{1, 2, 9}, got)
	})

	t.Run("empty", func(t *testing.T) {
		r := fast.NewReader(mustMarshal(t, []byte{}))
		got, err := r.Bytes()
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Empty(t, got)
	})

	t.Run("length beyond input", func(t *testing.T) {
		// Declares 2^32 bytes, carries none.
		r := fast.NewReader([]byte{0x5a, 0xff, 0xff, 0xff, 0xff})
		_, err := r.Bytes()
		require.ErrorIs(t, err, fast.ErrTruncated)
		require.Zero(t, r.Pos())

		r = fast.NewReader([]byte{0x7b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
		_, err = r.String()
		require.ErrorIs(t, err, fast.ErrTruncated)
	})

	t.Run("invalid utf-8", func(t *testing.T) {
		r := fast.NewReader([]byte{0x62, 0xc3, 0x28})
		_, err := r.String()
		require.ErrorIs(t, err, fast.ErrSyntax)
		require.Zero(t, r.Pos())
	})
}

func TestContainerHeaders(t *testing.T) {
	t.Run("array", func(t *testing.T) {
		r := fast.NewReader(mustMarshal(t, []uint64{1, 2, 3}))
		n, err := r.ArrayLen()
		require.NoError(t, err)
		require.Equal(t, 3, n)
	})

	t.Run("map", func(t *testing.T) {
		r := fast.NewReader(mustMarshal(t, map[string]uint64{"a": 1, "b": 2}))
		n, err := r.MapLen()
		require.NoError(t, err)
		require.Equal(t, 2, n)
	})

	t.Run("counts the input cannot back", func(t *testing.T) {
		// 2^32 elements in a 5-byte record must not reach an allocation.
		r := fast.NewReader([]byte{0x9a, 0xff, 0xff, 0xff, 0xff})
		_, err := r.ArrayLen()
		require.ErrorIs(t, err, fast.ErrTruncated)
		require.Zero(t, r.Pos())

		// Three pairs need six bytes, only five follow the head.
		r = fast.NewReader([]byte{0xa3, 1, 2, 3, 4, 5})
		_, err = r.MapLen()
		require.ErrorIs(t, err, fast.ErrTruncated)
	})
}

func TestTag(t *testing.T) {
	r := fast.NewReader([]byte{0xda, 0x00, 0x01, 0x00, 0x00, 0x01})
	tag, err := r.Tag()
	require.NoError(t, err)
	require.EqualValues(t, 65536, tag)

	r = fast.NewReader([]byte{0x01})
	_, err = r.Tag()
	require.ErrorIs(t, err, fast.ErrType)
}

func TestBigInt(t *testing.T) {
	huge := new(big.Int).Lsh(big.NewInt(1), 200)
	for _, want := range []*big.Int{
		big.NewInt(0), big.NewInt(-1), big.NewInt(42), big.NewInt(-42),
		new(big.Int).SetUint64(math.MaxUint64),
		new(big.Int).Neg(new(big.Int).SetUint64(math.MaxUint64)),
		new(big.Int).Sub(new(big.Int).Neg(new(big.Int).SetUint64(math.MaxUint64)), big.NewInt(1)),
		huge, new(big.Int).Neg(huge),
	} {
		r := fast.NewReader(mustMarshal(t, want))
		got, err := r.BigInt()
		require.NoError(t, err)
		require.Zerof(t, want.Cmp(got), "want %s, got %s", want, got)
		require.Zero(t, r.Len())
	}

	t.Run("tag 2 without a byte string", func(t *testing.T) {
		r := fast.NewReader([]byte{0xc2, 0x01})
		_, err := r.BigInt()
		require.ErrorIs(t, err, fast.ErrSyntax)
		require.Zero(t, r.Pos())
	})

	t.Run("other tag", func(t *testing.T) {
		// Some other tag: fxamacker looks through it, so the reader leaves it to fxamacker.
		r := fast.NewReader([]byte{0xc4, 0x41, 0x01})
		_, err := r.BigInt()
		require.ErrorIs(t, err, fast.ErrUnsupported)
		require.Zero(t, r.Pos())
	})
}

func TestTaggedValuesAreLeftToFxamacker(t *testing.T) {
	tagged := []byte{0xd8, 0x64, 0x01} // tag 100 over 1
	r := fast.NewReader(tagged)
	_, err := r.Uint64()
	require.ErrorIs(t, err, fast.ErrUnsupported)
	_, err = r.Int64()
	require.ErrorIs(t, err, fast.ErrUnsupported)
	_, err = r.String()
	require.ErrorIs(t, err, fast.ErrUnsupported)
	_, err = r.ArrayLen()
	require.ErrorIs(t, err, fast.ErrUnsupported)
	require.Zero(t, r.Pos())

	// A tag is still readable as one.
	tag, err := r.Tag()
	require.NoError(t, err)
	require.EqualValues(t, 100, tag)
}

func TestUndefinedIsNull(t *testing.T) {
	r := fast.NewReader([]byte{0xf7})
	require.True(t, r.Null())
}
