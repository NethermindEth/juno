package cborlite_test

import (
	"encoding/binary"
	"encoding/json"
	"reflect"
	"runtime"
	"testing"

	"github.com/NethermindEth/juno/encoder/cborlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// One test per Go kind the engine handles, on the smallest type that exercises it.

func TestUnmarshalScalars(t *testing.T) {
	type scalars struct {
		Number uint64
		Small  uint8
		Text   string
		Flag   bool
	}

	data := cborMap(
		cborText("Number"), []byte{0x19, 0x01, 0x00}, // 256
		cborText("Small"), []byte{0x07},
		cborText("Text"), cborText("hi"),
		cborText("Flag"), []byte{cborlite.SimpleTrue},
	)

	var got scalars
	require.NoError(t, cborlite.Unmarshal(data, &got))
	assert.Equal(t, scalars{Number: 256, Small: 7, Text: "hi", Flag: true}, got)

	t.Run("a uint that does not fit the field is rejected", func(t *testing.T) {
		tooBig := cborMap(cborText("Small"), []byte{0x19, 0x01, 0x00}) // 256 into a uint8

		var out scalars
		assert.ErrorContains(t, cborlite.Unmarshal(tooBig, &out), "Small")
	})
}

func TestUnmarshalPointer(t *testing.T) {
	type inner struct{ Value uint64 }
	type holder struct{ Ptr *inner }

	t.Run("allocates and fills", func(t *testing.T) {
		data := cborMap(cborText("Ptr"), cborMap(cborText("Value"), []byte{0x07}))

		var got holder
		require.NoError(t, cborlite.Unmarshal(data, &got))
		require.NotNil(t, got.Ptr)
		assert.Equal(t, uint64(7), got.Ptr.Value)
	})

	t.Run("null reads as nil, and clears what was there", func(t *testing.T) {
		data := cborMap(cborText("Ptr"), []byte{cborlite.Null})

		got := holder{Ptr: &inner{Value: 9}}
		require.NoError(t, cborlite.Unmarshal(data, &got))
		assert.Nil(t, got.Ptr)
	})
}

func TestUnmarshalSlice(t *testing.T) {
	type element struct{ Value uint64 }
	type holder struct {
		Elements []element
		Numbers  []uint64
	}

	data := cborMap(
		cborText("Elements"), cborArray(
			cborMap(cborText("Value"), []byte{0x01}),
			cborMap(cborText("Value"), []byte{0x02}),
		),
		cborText("Numbers"), cborArray([]byte{0x03}, []byte{0x04}),
	)

	var got holder
	require.NoError(t, cborlite.Unmarshal(data, &got))
	assert.Equal(t, []element{{Value: 1}, {Value: 2}}, got.Elements)
	assert.Equal(t, []uint64{3, 4}, got.Numbers)

	t.Run("null reads as nil, empty as empty", func(t *testing.T) {
		data := cborMap(
			cborText("Elements"), []byte{cborlite.Null},
			cborText("Numbers"), cborArray(),
		)

		var got holder
		require.NoError(t, cborlite.Unmarshal(data, &got))
		assert.Nil(t, got.Elements)
		assert.NotNil(t, got.Numbers)
		assert.Empty(t, got.Numbers)
	})

	t.Run("an element that does not decode names its index", func(t *testing.T) {
		data := cborMap(cborText("Numbers"), cborArray([]byte{0x03}, cborText("no")))

		var got holder
		err := cborlite.Unmarshal(data, &got)
		assert.ErrorContains(t, err, "Numbers")
		assert.ErrorContains(t, err, "[1]")
	})
}

func TestUnmarshalMap(t *testing.T) {
	type holder struct{ Bounds map[uint32]uint64 }

	data := cborMap(cborText("Bounds"), cborMap([]byte{0x01}, []byte{0x0a}, []byte{0x02}, []byte{0x14}))

	var got holder
	require.NoError(t, cborlite.Unmarshal(data, &got))
	assert.Equal(t, map[uint32]uint64{1: 10, 2: 20}, got.Bounds)

	t.Run("null reads as nil", func(t *testing.T) {
		data := cborMap(cborText("Bounds"), []byte{cborlite.Null})

		var got holder
		require.NoError(t, cborlite.Unmarshal(data, &got))
		assert.Nil(t, got.Bounds)
	})

	t.Run("a key that does not decode is named", func(t *testing.T) {
		data := cborMap(cborText("Bounds"), cborMap(cborText("no"), []byte{0x0a}))

		var got holder
		err := cborlite.Unmarshal(data, &got)
		assert.ErrorContains(t, err, "key")
	})

	// The header counts pairs, so a key twice leaves the map shorter than it claims.
	t.Run("declines a key given twice", func(t *testing.T) {
		data := cborMap(cborText("Bounds"),
			cborMap([]byte{0x01}, []byte{0x0a}, []byte{0x01}, []byte{0x14}))

		var got holder
		require.Error(t, cborlite.Unmarshal(data, &got))
	})
}

// TestUnmarshalMapValueDoesNotInheritTheOneBefore covers the pair reader being reused
// across entries: without a reset, a value leaving a field out keeps the last one's.
func TestUnmarshalMapValueDoesNotInheritTheOneBefore(t *testing.T) {
	type bounds struct {
		Low  uint64
		High uint64
	}
	type holder struct{ Bounds map[uint32]bounds }

	data := cborMap(cborText("Bounds"),
		cborMap(
			[]byte{0x01}, cborMap(cborText("Low"), []byte{0x0a}, cborText("High"), []byte{0x14}),
			// The second entry has no High, so it must come out zero.
			[]byte{0x02}, cborMap(cborText("Low"), []byte{0x0b}),
		))

	var got holder
	require.NoError(t, cborlite.Unmarshal(data, &got))
	assert.Equal(t, map[uint32]bounds{
		1: {Low: 10, High: 20},
		2: {Low: 11, High: 0},
	}, got.Bounds)
}

func TestUnmarshalArray(t *testing.T) {
	type holder struct {
		Address [3]byte
		Limbs   [2]uint64
	}

	data := cborMap(
		cborText("Address"), cborBytes(0xaa, 0xbb, 0xcc),
		cborText("Limbs"), cborArray([]byte{0x01}, []byte{0x02}),
	)

	var got holder
	require.NoError(t, cborlite.Unmarshal(data, &got))
	assert.Equal(t, [3]byte{0xaa, 0xbb, 0xcc}, got.Address)
	assert.Equal(t, [2]uint64{1, 2}, got.Limbs)

	t.Run("a byte array of the wrong length is rejected", func(t *testing.T) {
		data := cborMap(cborText("Address"), cborBytes(0xaa, 0xbb))

		var got holder
		assert.ErrorContains(t, cborlite.Unmarshal(data, &got), "Address")
	})

	// Shorter would leave elements at their zero value, longer has nowhere to go.
	//
	// Decoded on their own rather than as a struct field, because the count has to be
	// what refuses these. Any container around them counts its own items, so a reader
	// that read past a short array would be caught by that count coming up one short,
	// and the check under test could be gone without the test noticing. At the top
	// level only the total length is checked, and taking one item too many meets it.
	t.Run("a fixed array whose count is not the Go length is rejected", func(t *testing.T) {
		for name, data := range map[string][]byte{
			"too few":  append(cborArray([]byte{0x01}), 0x02),
			"too many": cborArray([]byte{0x01}, []byte{0x02}, []byte{0x03}),
		} {
			t.Run(name, func(t *testing.T) {
				var got [2]uint64
				assert.Error(t, cborlite.Unmarshal(data, &got))
				assert.Zero(t, got, "a refused decode must not write the destination")
			})
		}
	})
}

func TestUnmarshalByteStrings(t *testing.T) {
	type holder struct {
		Raw  json.RawMessage
		Blob []byte
	}

	// A []byte and its named forms are a byte string on the wire, not an array of small
	// ints, which is what their Go kind would suggest.
	data := cborMap(
		cborText("Raw"), cborBytes('{', '}'),
		cborText("Blob"), cborBytes(0x01, 0x02),
	)

	var got holder
	require.NoError(t, cborlite.Unmarshal(data, &got))
	assert.Equal(t, json.RawMessage("{}"), got.Raw)
	assert.Equal(t, []byte{0x01, 0x02}, got.Blob)

	t.Run("an array where a byte string belongs is rejected", func(t *testing.T) {
		data := cborMap(cborText("Blob"), cborArray([]byte{0x01}))

		var got holder
		assert.ErrorContains(t, cborlite.Unmarshal(data, &got), "Blob")
	})

	// The generic encoder writes null for a nil slice, and no reader in the package reads
	// null on its own, so this is the one place that can catch the prologue going missing.
	t.Run("null reads as nil", func(t *testing.T) {
		data := cborMap(cborText("Blob"), []byte{null})

		got := holder{Blob: []byte{0xff}}
		require.NoError(t, cborlite.Unmarshal(data, &got))
		assert.Nil(t, got.Blob)
	})
}

/****************************************************
		Interfaces, through a tag
*****************************************************/

type shape interface{ area() int }

type square struct{ Side int }

func (s *square) area() int { return s.Side * s.Side }

type circle struct{ Radius int }

func (c *circle) area() int { return 3 * c.Radius * c.Radius }

const (
	tagSquare = 70001
	tagCircle = 70002
)

//nolint:gochecknoinits // the mapping has to exist before the first read
func init() {
	cborlite.RegisterTag(tagSquare, reflect.TypeFor[square]())
	cborlite.RegisterTag(tagCircle, reflect.TypeFor[circle]())
}

func TestUnmarshalInterface(t *testing.T) {
	type holder struct{ Shape shape }

	t.Run("the tag picks the concrete type", func(t *testing.T) {
		data := cborMap(cborText("Shape"),
			cborTagged(tagCircle, cborMap(cborText("Radius"), head(uintMajor, 2))))

		var got holder
		require.NoError(t, cborlite.Unmarshal(data, &got))
		require.IsType(t, &circle{}, got.Shape)
		assert.Equal(t, 12, got.Shape.area())
	})

	t.Run("null reads as nil", func(t *testing.T) {
		data := cborMap(cborText("Shape"), []byte{cborlite.Null})

		var got holder
		require.NoError(t, cborlite.Unmarshal(data, &got))
		assert.Nil(t, got.Shape)
	})

	t.Run("an unregistered tag is rejected", func(t *testing.T) {
		data := cborMap(cborText("Shape"), cborTagged(79999, cborMap()))

		var got holder
		assert.ErrorContains(t, cborlite.Unmarshal(data, &got), "Shape")
	})

	t.Run("a tag whose type does not implement the interface is rejected", func(t *testing.T) {
		type otherHolder struct{ Value interface{ String() string } }

		data := cborMap(cborText("Value"),
			cborTagged(tagSquare, cborMap(cborText("Side"), head(uintMajor, 2))))

		var got otherHolder
		assert.Error(t, cborlite.Unmarshal(data, &got))
	})

	t.Run("something that is not a tag is rejected", func(t *testing.T) {
		data := cborMap(cborText("Shape"), cborMap())

		var got holder
		assert.ErrorContains(t, cborlite.Unmarshal(data, &got), "Shape")
	})
}

func TestUnmarshalSignedIntegers(t *testing.T) {
	type holder struct {
		Small int8
		Wide  int64
	}

	tests := []struct {
		name  string
		data  []byte
		small int8
		wide  int64
		ok    bool
	}{
		{name: "zero", data: cborMap(cborText("Wide"), []byte{0x00}), ok: true},
		{name: "positive", data: cborMap(cborText("Wide"), []byte{0x18, 0x2a}), wide: 42, ok: true},
		// A negative integer's argument is the magnitude minus one, so 0x20 is -1.
		{name: "minus one", data: cborMap(cborText("Wide"), []byte{0x20}), wide: -1, ok: true},
		{
			name: "minus one hundred",
			data: cborMap(cborText("Wide"), []byte{0x38, 0x63}), wide: -100, ok: true,
		},
		{
			name: "the most negative int64",
			data: cborMap(cborText("Wide"), []byte{0x3b, 0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}),
			wide: -1 << 63, ok: true,
		},
		{
			name: "past int64 on the negative side",
			data: cborMap(cborText("Wide"), []byte{0x3b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}),
		},
		{
			name: "past int64 on the positive side",
			data: cborMap(cborText("Wide"), []byte{0x1b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}),
		},
		{name: "past int8", data: cborMap(cborText("Small"), []byte{0x18, 0xff})},
		{name: "past int8 negative", data: cborMap(cborText("Small"), []byte{0x38, 0xff})},
		{name: "not an integer", data: cborMap(cborText("Wide"), cborText("no"))},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got holder
			err := cborlite.Unmarshal(test.data, &got)

			if !test.ok {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.small, got.Small)
			assert.Equal(t, test.wide, got.Wide)
		})
	}
}

// namedByte is a byte under another name, which the encoder still writes as a byte
// string. reflect.Copy refuses to mix it with byte, so the array reader has to fill one
// element at a time or it panics.
type namedByte byte

func TestUnmarshalNamedByteSliceAndArray(t *testing.T) {
	type holder struct {
		Slice []namedByte
		Array [2]namedByte
	}

	data := cborMap(
		cborText("Slice"), cborBytes(0xaa, 0xbb),
		cborText("Array"), cborBytes(0xcc, 0xdd),
	)

	var got holder
	require.NoError(t, cborlite.Unmarshal(data, &got))
	assert.Equal(t, []namedByte{0xaa, 0xbb}, got.Slice)
	assert.Equal(t, [2]namedByte{0xcc, 0xdd}, got.Array)
}

// TestSliceDecodingDoesNotTrustTheCountForSizing pins the cap on pre-sizing. An
// element can be one byte on the wire, so a header may claim as many elements as
// there are bytes left; sizing the slice from that count alone let a 1 MiB buffer
// allocate 54 MiB before a single element had been checked.
func TestSliceDecodingDoesNotTrustTheCountForSizing(t *testing.T) {
	const claimed = 1 << 20

	// Wide enough to make the amplification obvious: the class structs are 24 to 56
	// bytes, so a count trusted for sizing turns each input byte into a struct.
	type wide struct{ A, B, C, D, E, F, G uint64 }
	type holder struct{ Items []wide }

	// {"Items": [<a million promised, one rejected near the front>]}
	items := make([]byte, 0, claimed+8)
	items = append(items, 0x80|26)
	items = binary.BigEndian.AppendUint32(items, claimed)
	// Three readable elements, then one every reader rejects.
	items = append(items, 0xa0, 0xa0, 0xa0, 0x1f)
	for len(items) < claimed+5 {
		items = append(items, 0xa0)
	}

	data := append([]byte{
		byte(cborlite.MapMajor) | 1,
		byte(cborlite.StringMajor) | 5, 'I', 't', 'e', 'm', 's',
	}, items...)

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	var out holder
	require.Error(t, cborlite.Unmarshal(data, &out), "the rejected element must fail the decode")

	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc

	// Sizing from the count would be claimed * 56 bytes, about 56 MiB.
	assert.Lessf(t, allocated, uint64(4<<20),
		"allocated %d KiB from a %d KiB buffer, the count is being trusted for sizing",
		allocated/1024, len(data)/1024)
}
