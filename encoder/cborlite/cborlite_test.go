package cborlite_test

import (
	"encoding/binary"
	"math/big"
	"runtime"
	"testing"

	"github.com/NethermindEth/juno/encoder/cborlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHead(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		offset   int
		major    byte
		argument uint64
		next     int
		ok       bool
	}{
		{
			name: "argument inside the header byte",
			data: []byte{0x17}, major: cborlite.UintMajor, argument: 23, next: 1, ok: true,
		},
		{
			name: "one byte argument",
			data: []byte{0x18, 0xff}, major: cborlite.UintMajor, argument: 255, next: 2, ok: true,
		},
		{
			name: "two byte argument",
			data: []byte{0x19, 0x01, 0x00}, major: cborlite.UintMajor, argument: 256, next: 3, ok: true,
		},
		{
			name: "four byte argument",
			data: []byte{0x1a, 0x00, 0x01, 0x00, 0x00}, major: cborlite.UintMajor, argument: 65536, next: 5, ok: true,
		},
		{
			name:  "eight byte argument",
			data:  []byte{0x1b, 0, 0, 0, 1, 0, 0, 0, 0},
			major: cborlite.UintMajor, argument: 1 << 32, next: 9, ok: true,
		},
		{
			name: "reads at an offset",
			data: []byte{0xff, 0xff, 0x63}, offset: 2,
			major: cborlite.TextMajor, argument: 3, next: 3, ok: true,
		},
		{
			name: "major type is decoded independently of the argument",
			data: []byte{0xa1}, major: cborlite.MapMajor, argument: 1, next: 1, ok: true,
		},
		{name: "empty buffer", data: []byte{}},
		{name: "offset past the end", data: []byte{0x01}, offset: 1},
		{name: "negative offset", data: []byte{0x01}, offset: -1},
		{name: "one byte argument truncated", data: []byte{0x18}},
		{name: "two byte argument truncated", data: []byte{0x19, 0x01}},
		{name: "four byte argument truncated", data: []byte{0x1a, 0x00, 0x01}},
		{name: "eight byte argument truncated", data: []byte{0x1b, 0, 0, 0, 1}},
		{name: "reserved additional info 28", data: []byte{0x1c}},
		{name: "reserved additional info 29", data: []byte{0x1d}},
		{name: "reserved additional info 30", data: []byte{0x1e}},
		{name: "indefinite length", data: []byte{0x1f}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			major, argument, next, ok := cborlite.Head(test.data, test.offset)

			require.Equal(t, test.ok, ok)
			if !test.ok {
				return
			}
			assert.Equal(t, test.major, major)
			assert.Equal(t, test.argument, argument)
			assert.Equal(t, test.next, next)
		})
	}
}

func TestByteString(t *testing.T) {
	data := []byte{0x43, 0xaa, 0xbb, 0xcc}

	got, next, ok := cborlite.ByteString(data, 0)
	require.True(t, ok)
	assert.Equal(t, []byte{0xaa, 0xbb, 0xcc}, got)
	assert.Equal(t, 4, next)

	t.Run("aliases the buffer", func(t *testing.T) {
		// Callers rely on this to read a class without copying it.
		got, _, ok := cborlite.ByteString(data, 0)
		require.True(t, ok)

		data[1] = 0x11
		assert.Equal(t, byte(0x11), got[0])
		data[1] = 0xaa
	})

	t.Run("rejects a length past the buffer", func(t *testing.T) {
		_, _, ok := cborlite.ByteString([]byte{0x43, 0xaa}, 0)
		assert.False(t, ok)
	})

	t.Run("rejects another major type", func(t *testing.T) {
		_, _, ok := cborlite.ByteString([]byte{0x63, 'a', 'b', 'c'}, 0)
		assert.False(t, ok)
	})

	t.Run("does not read null", func(t *testing.T) {
		_, _, ok := cborlite.ByteString([]byte{cborlite.Null}, 0)
		assert.False(t, ok)
	})
}

func TestByteStringCopy(t *testing.T) {
	data := []byte{0x43, 0xaa, 0xbb, 0xcc}

	got, next, ok := cborlite.ByteStringCopy(data, 0)
	require.True(t, ok)
	assert.Equal(t, []byte{0xaa, 0xbb, 0xcc}, got)
	assert.Equal(t, 4, next)

	t.Run("does not alias the buffer", func(t *testing.T) {
		data[1] = 0x11
		assert.Equal(t, byte(0xaa), got[0], "the copy must survive the buffer changing")
		data[1] = 0xaa
	})

	t.Run("reads null as nil", func(t *testing.T) {
		got, next, ok := cborlite.ByteStringCopy([]byte{cborlite.Null}, 0)
		require.True(t, ok)
		assert.Nil(t, got)
		assert.Equal(t, 1, next)
	})

	t.Run("empty byte string is not nil", func(t *testing.T) {
		got, _, ok := cborlite.ByteStringCopy([]byte{0x40}, 0)
		require.True(t, ok)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})
}

func TestTextStringAndString(t *testing.T) {
	data := []byte{0xff, 0x63, 'a', 'b', 'c'}

	raw, next, ok := cborlite.TextString(data, 1)
	require.True(t, ok)
	assert.Equal(t, []byte("abc"), raw)
	assert.Equal(t, 5, next)

	str, next, ok := cborlite.String(data, 1)
	require.True(t, ok)
	assert.Equal(t, "abc", str)
	assert.Equal(t, 5, next)

	t.Run("String copies out of the buffer", func(t *testing.T) {
		buffer := []byte{0x63, 'a', 'b', 'c'}
		str, _, ok := cborlite.String(buffer, 0)
		require.True(t, ok)

		buffer[1] = 'z'
		assert.Equal(t, "abc", str)
	})

	t.Run("reject a byte string", func(t *testing.T) {
		_, _, ok := cborlite.TextString([]byte{0x43, 0xaa, 0xbb, 0xcc}, 0)
		assert.False(t, ok)

		_, _, ok = cborlite.String([]byte{0x43, 0xaa, 0xbb, 0xcc}, 0)
		assert.False(t, ok)
	})

	t.Run("reject a truncated length", func(t *testing.T) {
		_, _, ok := cborlite.TextString([]byte{0x63, 'a'}, 0)
		assert.False(t, ok)
	})
}

func TestUint64(t *testing.T) {
	got, next, ok := cborlite.Uint64([]byte{0x18, 0x2a}, 0)
	require.True(t, ok)
	assert.Equal(t, uint64(42), got)
	assert.Equal(t, 2, next)

	t.Run("rejects a negative integer instead of wrapping it", func(t *testing.T) {
		_, _, ok := cborlite.Uint64([]byte{0x20}, 0)
		assert.False(t, ok)
	})

	t.Run("rejects another major type", func(t *testing.T) {
		_, _, ok := cborlite.Uint64([]byte{0x63, 'a', 'b', 'c'}, 0)
		assert.False(t, ok)
	})
}

func TestReadNull(t *testing.T) {
	next, isNull := cborlite.ReadNull([]byte{cborlite.Null}, 0)
	assert.True(t, isNull)
	assert.Equal(t, 1, next)

	t.Run("leaves the offset alone when the item is not null", func(t *testing.T) {
		next, isNull := cborlite.ReadNull([]byte{0x01}, 0)
		assert.False(t, isNull)
		assert.Equal(t, 0, next)
	})

	t.Run("out of range offsets are not null", func(t *testing.T) {
		next, isNull := cborlite.ReadNull([]byte{}, 0)
		assert.False(t, isNull)
		assert.Equal(t, 0, next)

		next, isNull = cborlite.ReadNull([]byte{cborlite.Null}, -1)
		assert.False(t, isNull)
		assert.Equal(t, -1, next)
	})
}

func TestBigInt(t *testing.T) {
	twoToThe64, ok := new(big.Int).SetString("18446744073709551616", 10)
	require.True(t, ok)

	tests := []struct {
		name string
		data []byte
		want *big.Int
		next int
		ok   bool
	}{
		{name: "small unsigned", data: []byte{0x07}, want: big.NewInt(7), next: 1, ok: true},
		{name: "wide unsigned", data: []byte{0x19, 0x01, 0x00}, want: big.NewInt(256), next: 3, ok: true},
		{name: "negative one", data: []byte{0x20}, want: big.NewInt(-1), next: 1, ok: true},
		{name: "negative hundred", data: []byte{0x38, 0x63}, want: big.NewInt(-100), next: 2, ok: true},
		{name: "null", data: []byte{cborlite.Null}, want: nil, next: 1, ok: true},
		{
			name: "positive bignum past uint64",
			data: []byte{0xc2, 0x49, 0x01, 0, 0, 0, 0, 0, 0, 0, 0},
			want: twoToThe64, next: 11, ok: true,
		},
		{
			name: "negative bignum",
			data: []byte{0xc3, 0x41, 0x01},
			want: big.NewInt(-2), next: 3, ok: true,
		},
		{name: "unrelated tag", data: []byte{0xc4, 0x01}},
		{name: "bignum tag without a byte string", data: []byte{0xc2, 0x01}},
		{name: "bignum tag with a truncated byte string", data: []byte{0xc2, 0x43, 0x01}},
		{name: "text string", data: []byte{0x63, 'a', 'b', 'c'}},
		{name: "empty", data: []byte{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, next, ok := cborlite.BigInt(test.data, 0)

			require.Equal(t, test.ok, ok)
			if !test.ok {
				return
			}
			if test.want == nil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Zero(t, test.want.Cmp(got), "want %s, got %s", test.want, got)
			}
			assert.Equal(t, test.next, next)
		})
	}
}

func TestArrayHeader(t *testing.T) {
	length, next, ok := cborlite.ArrayHeader([]byte{0x82, 0x01, 0x02}, 0)
	require.True(t, ok)
	assert.Equal(t, 2, length)
	assert.Equal(t, 1, next)

	t.Run("null reports a negative length", func(t *testing.T) {
		length, next, ok := cborlite.ArrayHeader([]byte{cborlite.Null}, 0)
		require.True(t, ok)
		assert.Negative(t, length)
		assert.Equal(t, 1, next)
	})

	t.Run("empty array is zero, not negative", func(t *testing.T) {
		length, _, ok := cborlite.ArrayHeader([]byte{0x80}, 0)
		require.True(t, ok)
		assert.Zero(t, length)
	})

	t.Run("rejects a count past the remaining bytes", func(t *testing.T) {
		// Guards the allocation: every element needs at least one byte.
		_, _, ok := cborlite.ArrayHeader([]byte{0x85, 0x01, 0x02}, 0)
		assert.False(t, ok)
	})

	t.Run("rejects a huge count", func(t *testing.T) {
		_, _, ok := cborlite.ArrayHeader([]byte{0x1b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, 0)
		assert.False(t, ok)
	})

	t.Run("rejects another major type", func(t *testing.T) {
		_, _, ok := cborlite.ArrayHeader([]byte{0xa1, 0x01, 0x02}, 0)
		assert.False(t, ok)
	})
}

func TestMapHeader(t *testing.T) {
	pairs, next, ok := cborlite.MapHeader([]byte{0xa1, 0x01, 0x02}, 0)
	require.True(t, ok)
	assert.Equal(t, 1, pairs)
	assert.Equal(t, 1, next)

	t.Run("rejects a count past the remaining bytes", func(t *testing.T) {
		_, _, ok := cborlite.MapHeader([]byte{0xa5, 0x01}, 0)
		assert.False(t, ok)
	})

	t.Run("rejects an array", func(t *testing.T) {
		_, _, ok := cborlite.MapHeader([]byte{0x82, 0x01, 0x02}, 0)
		assert.False(t, ok)
	})

	t.Run("does not read null", func(t *testing.T) {
		_, _, ok := cborlite.MapHeader([]byte{cborlite.Null}, 0)
		assert.False(t, ok)
	})
}

func TestStringSlice(t *testing.T) {
	got, next, ok := cborlite.StringSlice([]byte{0x82, 0x61, 'a', 0x61, 'b'}, 0)
	require.True(t, ok)
	assert.Equal(t, []string{"a", "b"}, got)
	assert.Equal(t, 5, next)

	t.Run("null reads as nil", func(t *testing.T) {
		got, next, ok := cborlite.StringSlice([]byte{cborlite.Null}, 0)
		require.True(t, ok)
		assert.Nil(t, got)
		assert.Equal(t, 1, next)
	})

	t.Run("rejects a non-string element", func(t *testing.T) {
		_, _, ok := cborlite.StringSlice([]byte{0x82, 0x61, 'a', 0x01}, 0)
		assert.False(t, ok)
	})
}

// pair is a two-field struct, so StructSlice is exercised with elements that span
// more than one CBOR item.
type pair struct {
	A, B uint64
}

func decodePair(p *pair, data []byte, offset int) (int, bool) {
	var ok bool
	if p.A, offset, ok = cborlite.Uint64(data, offset); !ok {
		return 0, false
	}
	if p.B, offset, ok = cborlite.Uint64(data, offset); !ok {
		return 0, false
	}
	return offset, true
}

func TestStructSlice(t *testing.T) {
	got, next, ok := cborlite.StructSlice([]byte{0x82, 0x01, 0x02, 0x03, 0x04}, 0, decodePair)
	require.True(t, ok)
	assert.Equal(t, []pair{{A: 1, B: 2}, {A: 3, B: 4}}, got)
	assert.Equal(t, 5, next)

	t.Run("null reads as nil", func(t *testing.T) {
		got, next, ok := cborlite.StructSlice([]byte{cborlite.Null}, 0, decodePair)
		require.True(t, ok)
		assert.Nil(t, got)
		assert.Equal(t, 1, next)
	})

	t.Run("empty array reads as empty, not nil", func(t *testing.T) {
		got, _, ok := cborlite.StructSlice([]byte{0x80}, 0, decodePair)
		require.True(t, ok)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("an element the decoder rejects fails the whole slice", func(t *testing.T) {
		_, _, ok := cborlite.StructSlice([]byte{0x82, 0x01, 0x02, 0x03, 0x63}, 0, decodePair)
		assert.False(t, ok)
	})
}

// TestSliceReadersDoNotTrustTheCountForSizing pins the cap on pre-sizing. An
// element can be one byte on the wire, so a header may claim as many elements as
// there are bytes left; sizing the slice from that count alone let a 1 MiB buffer
// allocate 54 MiB before a single element had been checked.
func TestSliceReadersDoNotTrustTheCountForSizing(t *testing.T) {
	const claimed = 1 << 20

	// An array claiming a million elements, long enough for the count to pass the
	// remaining-bytes check, but with an element the reader rejects near the front.
	data := make([]byte, 0, claimed+8)
	data = append(data, 0x80|26)
	data = binary.BigEndian.AppendUint32(data, claimed)
	// Three readable elements, then one every reader rejects.
	data = append(data, 0xa0, 0xa0, 0xa0, 0x1f)
	for len(data) < claimed+5 {
		data = append(data, 0xa0)
	}

	// Wide enough to make the amplification obvious: the class structs are 24 to 56
	// bytes, so a count trusted for sizing turns each input byte into a struct.
	type wide struct{ _ [56]byte }
	decodeElement := func(_ *wide, data []byte, offset int) (int, bool) {
		return cborlite.Skip(data, offset)
	}

	measure := func(read func()) uint64 {
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		read()
		runtime.ReadMemStats(&after)
		return after.TotalAlloc - before.TotalAlloc
	}

	t.Run("StructSlice", func(t *testing.T) {
		allocated := measure(func() {
			_, _, ok := cborlite.StructSlice(data, 0, decodeElement)
			require.False(t, ok, "the rejected element must fail the whole slice")
		})

		// Sizing from the count would be claimed * 56 bytes, about 56 MiB.
		assert.Lessf(t, allocated, uint64(4<<20),
			"allocated %d KiB from a %d KiB buffer, the count is being trusted for sizing",
			allocated/1024, len(data)/1024)
	})

	t.Run("StringSlice", func(t *testing.T) {
		allocated := measure(func() {
			_, _, ok := cborlite.StringSlice(data, 0)
			require.False(t, ok)
		})

		assert.Lessf(t, allocated, uint64(4<<20),
			"allocated %d KiB from a %d KiB buffer, the count is being trusted for sizing",
			allocated/1024, len(data)/1024)
	})
}

func TestSkip(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		next int
		ok   bool
	}{
		{name: "unsigned in the header", data: []byte{0x05}, next: 1, ok: true},
		{name: "unsigned with an argument", data: []byte{0x19, 0x01, 0x00}, next: 3, ok: true},
		{name: "negative", data: []byte{0x38, 0x63}, next: 2, ok: true},
		{name: "null", data: []byte{cborlite.Null}, next: 1, ok: true},
		{name: "boolean", data: []byte{0xf5}, next: 1, ok: true},
		{name: "byte string", data: []byte{0x43, 0xaa, 0xbb, 0xcc}, next: 4, ok: true},
		{name: "text string", data: []byte{0x63, 'a', 'b', 'c'}, next: 4, ok: true},
		{name: "empty array", data: []byte{0x80}, next: 1, ok: true},
		{name: "array of scalars", data: []byte{0x82, 0x01, 0x02}, next: 3, ok: true},
		{name: "map of one pair", data: []byte{0xa1, 0x01, 0x02}, next: 3, ok: true},
		{name: "tag wrapping a byte string", data: []byte{0xc2, 0x41, 0x01}, next: 3, ok: true},
		{
			name: "array holding an array and a map",
			data: []byte{0x82, 0x82, 0x01, 0x02, 0xa1, 0x03, 0x04}, next: 7, ok: true,
		},
		{
			name: "stops at the end of the item, leaving the rest",
			data: []byte{0x82, 0x01, 0x02, 0xff, 0xff}, next: 3, ok: true,
		},
		{name: "empty", data: []byte{}},
		{name: "truncated byte string", data: []byte{0x43, 0xaa}},
		{name: "array with fewer elements than promised", data: []byte{0x82, 0x01}},
		{name: "map with fewer pairs than promised", data: []byte{0xa2, 0x01, 0x02}},
		// Doubling a pair count to compare it against the remaining bytes overflows
		// here, and the wrapped value used to pass, reading this as an empty map.
		{name: "map claiming 2^63 pairs", data: []byte{0xbb, 0x80, 0, 0, 0, 0, 0, 0, 0}},
		{name: "map claiming a count near the uint64 top", data: []byte{0xbb, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
		{name: "array claiming a count near the uint64 top", data: []byte{0x9b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
		{name: "tag with nothing after it", data: []byte{0xc2}},
		{name: "indefinite length", data: []byte{0x9f, 0x01, 0xff}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			next, ok := cborlite.Skip(test.data, 0)

			require.Equal(t, test.ok, ok)
			if test.ok {
				assert.Equal(t, test.next, next)
			}
		})
	}
}
