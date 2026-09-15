package cborlite_test

import (
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/encoder/cborlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBytesNoCopy(t *testing.T) {
	data := cborBytes(0xaa, 0xbb, 0xcc)

	got, next, ok := cborlite.BytesNoCopy(data)
	require.True(t, ok)
	assert.Equal(t, []byte{0xaa, 0xbb, 0xcc}, got)
	assert.Equal(t, 4, next)

	t.Run("aliases the buffer", func(t *testing.T) {
		got, _, ok := cborlite.BytesNoCopy(data)
		require.True(t, ok)

		data[1] = 0x11
		assert.Equal(t, byte(0x11), got[0])
		data[1] = 0xaa
	})

	t.Run("rejects a length past the buffer", func(t *testing.T) {
		_, _, ok := cborlite.BytesNoCopy([]byte{initialByte(bytesMajor, 3), 0xaa})
		assert.False(t, ok)
	})

	t.Run("rejects another major type", func(t *testing.T) {
		_, _, ok := cborlite.BytesNoCopy(cborText("abc"))
		assert.False(t, ok)
	})

	t.Run("does not read null", func(t *testing.T) {
		_, _, ok := cborlite.BytesNoCopy([]byte{null})
		assert.False(t, ok)
	})
}

func TestBytes(t *testing.T) {
	data := cborBytes(0xaa, 0xbb, 0xcc)

	got, next, ok := cborlite.Bytes(data)
	require.True(t, ok)
	assert.Equal(t, []byte{0xaa, 0xbb, 0xcc}, got)
	assert.Equal(t, 4, next)

	t.Run("does not alias the buffer", func(t *testing.T) {
		data[1] = 0x11
		assert.Equal(t, byte(0xaa), got[0], "the copy must survive the buffer changing")
		data[1] = 0xaa
	})

	// Null is another shape, and no reader here takes it.
	t.Run("does not read null", func(t *testing.T) {
		_, _, ok := cborlite.Bytes([]byte{null})
		assert.False(t, ok)
	})

	t.Run("empty byte string is not nil", func(t *testing.T) {
		got, _, ok := cborlite.Bytes(cborBytes())
		require.True(t, ok)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})
}

func TestStringNoCopyAndString(t *testing.T) {
	data := trailByJunk(cborText("abc"))

	raw, consumed, ok := cborlite.StringNoCopy(data)
	require.True(t, ok)
	assert.Equal(t, []byte("abc"), raw)
	assert.Equal(t, 4, consumed)

	str, consumed, ok := cborlite.String(data)
	require.True(t, ok)
	assert.Equal(t, "abc", str)
	assert.Equal(t, 4, consumed)

	t.Run("String copies out of the buffer", func(t *testing.T) {
		buffer := cborText("abc")
		str, _, ok := cborlite.String(buffer)
		require.True(t, ok)

		buffer[1] = 'z'
		assert.Equal(t, "abc", str)
	})

	t.Run("reject a byte string", func(t *testing.T) {
		_, _, ok := cborlite.StringNoCopy(cborBytes(0xaa, 0xbb, 0xcc))
		assert.False(t, ok)

		_, _, ok = cborlite.String(cborBytes(0xaa, 0xbb, 0xcc))
		assert.False(t, ok)
	})

	t.Run("reject a truncated length", func(t *testing.T) {
		_, _, ok := cborlite.StringNoCopy([]byte{initialByte(stringMajor, 3), 'a'})
		assert.False(t, ok)
	})
}

func TestUint64(t *testing.T) {
	got, next, ok := cborlite.Uint64(head(uintMajor, 42))
	require.True(t, ok)
	assert.Equal(t, uint64(42), got)
	assert.Equal(t, 2, next)

	t.Run("rejects a negative integer instead of wrapping it", func(t *testing.T) {
		_, _, ok := cborlite.Uint64(head(negIntMajor, 0))
		assert.False(t, ok)
	})

	t.Run("rejects another major type", func(t *testing.T) {
		_, _, ok := cborlite.Uint64(cborText("abc"))
		assert.False(t, ok)
	})
}

func TestBool(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		value    bool
		consumed int
		ok       bool
	}{
		{name: "false", data: []byte{simpleFalse}, value: false, consumed: 1, ok: true},
		{name: "true", data: []byte{simpleTrue}, value: true, consumed: 1, ok: true},
		{
			name:  "reads one byte and leaves the rest",
			data:  trailByJunk([]byte{simpleTrue}),
			value: true, consumed: 1, ok: true,
		},
		// All the following return ok as false
		{name: "null is not false", data: []byte{null}},
		{name: "undefined is not false", data: head(simpleMajor, 23)},
		{name: "a simple value with a following byte", data: head(simpleMajor, 32)},
		{name: "zero is not false", data: head(uintMajor, 0)},
		{name: "empty text string is not false", data: cborText("")},
		{name: "empty", data: []byte{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, consumed, ok := cborlite.Bool(test.data)

			require.Equal(t, test.ok, ok)
			if !test.ok {
				return
			}
			assert.Equal(t, test.value, value)
			assert.Equal(t, test.consumed, consumed)
		})
	}
}

func TestTag(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		value    uint64
		consumed int
		ok       bool
	}{
		{
			name:  "small enough for the header",
			data:  head(tagMajor, tagPositiveBignum),
			value: 2, consumed: 1, ok: true,
		},
		{
			name:  "the first tag the encoder registers",
			data:  head(tagMajor, 65536),
			value: 65536, consumed: 5, ok: true,
		},
		{name: "an unsigned integer", data: head(uintMajor, 1)},
		{name: "a truncated argument", data: []byte{initialByte(tagMajor, info4Byte), 0x00}},
		{name: "empty", data: []byte{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, consumed, ok := cborlite.Tag(test.data)

			require.Equal(t, test.ok, ok)
			if !test.ok {
				return
			}
			assert.Equal(t, test.value, value)
			assert.Equal(t, test.consumed, consumed)
		})
	}
}

func TestReadNull(t *testing.T) {
	consumed, isNull := cborlite.ReadNull([]byte{null})
	assert.True(t, isNull)
	assert.Equal(t, 1, consumed)

	t.Run("consumes nothing when the item is not null", func(t *testing.T) {
		// A caller that goes on to read the item for real has to start where it was.
		consumed, isNull := cborlite.ReadNull(head(uintMajor, 1))
		assert.False(t, isNull)
		assert.Zero(t, consumed)
	})

	t.Run("an empty buffer is not null", func(t *testing.T) {
		consumed, isNull := cborlite.ReadNull([]byte{})
		assert.False(t, isNull)
		assert.Zero(t, consumed)
	})
}

func TestSkip(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		consumed int
		ok       bool
	}{
		{name: "unsigned in the header", data: head(uintMajor, 5), consumed: 1, ok: true},
		{name: "unsigned with an argument", data: head(uintMajor, 256), consumed: 3, ok: true},
		{name: "negative", data: head(negIntMajor, 99), consumed: 2, ok: true},
		{name: "null", data: []byte{null}, consumed: 1, ok: true},
		{name: "boolean", data: []byte{simpleTrue}, consumed: 1, ok: true},
		{name: "byte string", data: cborBytes(0xaa, 0xbb, 0xcc), consumed: 4, ok: true},
		{name: "text string", data: cborText("abc"), consumed: 4, ok: true},
		{name: "empty array", data: cborArray(), consumed: 1, ok: true},
		{
			name: "array of scalars", data: cborArray(head(uintMajor, 1), head(uintMajor, 2)),
			consumed: 3, ok: true,
		},
		{
			name: "map of one pair", data: cborMap(head(uintMajor, 1), head(uintMajor, 2)),
			consumed: 3, ok: true,
		},
		{
			name:     "tag wrapping a byte string",
			data:     cborTagged(tagPositiveBignum, cborBytes(0x01)),
			consumed: 3, ok: true,
		},
		{
			name: "array holding an array and a map",
			data: cborArray(
				cborArray(head(uintMajor, 1), head(uintMajor, 2)),
				cborMap(head(uintMajor, 3), head(uintMajor, 4)),
			),
			consumed: 7, ok: true,
		},
		{
			name: "stops at the end of the item, leaving the rest",
			data: trailByJunk(cborArray(head(uintMajor, 1), head(uintMajor, 2))), consumed: 3, ok: true,
		},
		{name: "empty", data: []byte{}},
		{name: "truncated byte string", data: []byte{initialByte(bytesMajor, 3), 0xaa}},
		{name: "array with fewer elements than promised", data: []byte{initialByte(arrayMajor, 2), 0x01}},
		{name: "map with fewer pairs than promised", data: []byte{initialByte(mapMajor, 2), 0x01, 0x02}},
		// Doubling a pair count to compare it against the remaining bytes overflows
		// here, and the wrapped value used to pass, reading this as an empty map.
		{name: "map claiming 2^63 pairs", data: head(mapMajor, 1<<63)},
		{
			name: "map claiming a count near the uint64 top",
			data: head(mapMajor, math.MaxUint64),
		},
		{
			name: "array claiming a count near the uint64 top",
			data: head(arrayMajor, math.MaxUint64),
		},
		{name: "tag with nothing after it", data: []byte{initialByte(tagMajor, tagPositiveBignum)}},
		{name: "indefinite length", data: []byte{initialByte(arrayMajor, 31), 0x01, 0xff}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			consumed, ok := cborlite.Skip(test.data)

			require.Equal(t, test.ok, ok)
			if test.ok {
				assert.Equal(t, test.consumed, consumed)
			}
		})
	}
}

func TestSkipStopsBeforeTheStackOverflows(t *testing.T) {
	nested := func(depth int) []byte {
		data := make([]byte, 0, depth+1)
		for range depth {
			data = append(data, initialByte(arrayMajor, 1))
		}
		return append(data, initialByte(uintMajor, 0))
	}

	t.Run("nesting a real value reaches is still read", func(t *testing.T) {
		consumed, ok := cborlite.Skip(nested(8))

		require.True(t, ok)
		require.Equal(t, 9, consumed)
	})

	for _, depth := range []int{10_000, 5_000_000} {
		t.Run(fmt.Sprintf("declines %d levels instead of crashing", depth), func(t *testing.T) {
			consumed, ok := cborlite.Skip(nested(depth))

			require.False(t, ok)
			require.Zero(t, consumed)
		})
	}
}

func TestBigInt(t *testing.T) {
	bignumPayload := cborBytes(beyondUint64.Bytes()...)

	tests := []struct {
		name     string
		data     []byte
		value    *big.Int
		consumed int
		ok       bool
	}{
		{name: "does not read null", data: []byte{null}},
		{
			name: "positive in the header", data: head(uintMajor, 5),
			value: big.NewInt(5), consumed: 1, ok: true,
		},
		{
			name: "positive with an argument", data: head(uintMajor, 42),
			value: big.NewInt(42), consumed: 2, ok: true,
		},
		// A negative integer's argument is the magnitude minus one, so 0x20 is -1.
		{name: "minus one", data: head(negIntMajor, 0), value: big.NewInt(-1), consumed: 1, ok: true},
		{
			name: "minus one hundred", data: head(negIntMajor, 99),
			value: big.NewInt(-100), consumed: 2, ok: true,
		},
		{
			name:  "positive bignum",
			data:  cborTagged(tagPositiveBignum, bignumPayload),
			value: beyondUint64, consumed: 11, ok: true,
		},
		{
			name:  "negative bignum",
			data:  cborTagged(tagNegativeBignum, bignumPayload),
			value: new(big.Int).Not(beyondUint64), consumed: 11, ok: true,
		},
		{
			name: "a tag that is not a bignum",
			data: cborTagged(4, bignumPayload),
		},
		{
			name: "a bignum wrapping something other than bytes",
			data: cborTagged(tagPositiveBignum, head(uintMajor, 1)),
		},
		{
			name: "a bignum with a truncated payload",
			data: append(head(tagMajor, tagPositiveBignum), initialByte(bytesMajor, 9), 0x01),
		},
		{name: "a text string", data: cborText("abc")},
		{name: "empty", data: []byte{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, consumed, ok := cborlite.BigInt(test.data)

			require.Equal(t, test.ok, ok)
			if !test.ok {
				return
			}
			assert.Equal(t, test.consumed, consumed)
			// Comparing with Cmp, since two equal big.Ints can hold different internals.
			assert.Zerof(t, test.value.Cmp(value), "want %s, got %s", test.value, value)
		})
	}
}

func TestInt64(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		value    int64
		consumed int
		ok       bool
	}{
		{name: "zero", data: head(uintMajor, 0), consumed: 1, ok: true},
		{
			name: "positive with an argument", data: head(uintMajor, 42),
			value: 42, consumed: 2, ok: true,
		},
		{name: "minus one", data: head(negIntMajor, 0), value: -1, consumed: 1, ok: true},
		{
			name: "minus one hundred", data: head(negIntMajor, 99),
			value: -100, consumed: 2, ok: true,
		},
		{
			name:  "the largest int64",
			data:  head(uintMajor, math.MaxInt64),
			value: math.MaxInt64, consumed: 9, ok: true,
		},
		{
			name:  "the most negative int64",
			data:  head(negIntMajor, math.MaxInt64),
			value: math.MinInt64, consumed: 9, ok: true,
		},
		{
			name: "past int64 going up",
			data: head(uintMajor, math.MaxUint64),
		},
		{
			name: "past int64 going down",
			data: head(negIntMajor, math.MaxUint64),
		},
		{name: "a text string", data: cborText("abc")},
		{name: "a bignum", data: cborTagged(tagPositiveBignum, cborBytes(0x01))},
		{name: "empty", data: []byte{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, consumed, ok := cborlite.Int64(test.data)

			require.Equal(t, test.ok, ok)
			if !test.ok {
				return
			}
			assert.Equal(t, test.value, value)
			assert.Equal(t, test.consumed, consumed)
		})
	}
}
