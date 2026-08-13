package cborlite_test

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/NethermindEth/juno/encoder/cborlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalScalars(t *testing.T) {
	type scalars struct {
		Number uint64
		Small  uint8
		Text   string
		Flag   bool
	}

	data := cborMap(
		cborText("Number"), head(uintMajor, 256),
		cborText("Small"), head(uintMajor, 7),
		cborText("Text"), cborText("hi"),
		cborText("Flag"), []byte{simpleTrue},
	)

	var got scalars
	require.NoError(t, cborlite.Unmarshal(data, &got))
	assert.Equal(t, scalars{Number: 256, Small: 7, Text: "hi", Flag: true}, got)

	t.Run("a uint that does not fit the field is rejected", func(t *testing.T) {
		tooBig := cborMap(cborText("Small"), head(uintMajor, 256)) // 256 into a uint8

		var out scalars
		assert.ErrorContains(t, cborlite.Unmarshal(tooBig, &out), "Small")
	})
}

func TestUnmarshalPointer(t *testing.T) {
	type inner struct{ Value uint64 }
	type holder struct{ Ptr *inner }

	t.Run("allocates and fills", func(t *testing.T) {
		data := cborMap(cborText("Ptr"), cborMap(cborText("Value"), head(uintMajor, 7)))

		var got holder
		require.NoError(t, cborlite.Unmarshal(data, &got))
		require.NotNil(t, got.Ptr)
		assert.Equal(t, uint64(7), got.Ptr.Value)
	})

	t.Run("null reads as nil, and clears what was there", func(t *testing.T) {
		data := cborMap(cborText("Ptr"), []byte{null})

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
			cborMap(cborText("Value"), head(uintMajor, 1)),
			cborMap(cborText("Value"), head(uintMajor, 2)),
		),
		cborText("Numbers"), cborArray(head(uintMajor, 3), head(uintMajor, 4)),
	)

	var got holder
	require.NoError(t, cborlite.Unmarshal(data, &got))
	assert.Equal(t, []element{{Value: 1}, {Value: 2}}, got.Elements)
	assert.Equal(t, []uint64{3, 4}, got.Numbers)

	t.Run("null reads as nil, empty as empty", func(t *testing.T) {
		data := cborMap(
			cborText("Elements"), []byte{null},
			cborText("Numbers"), cborArray(),
		)

		var got holder
		require.NoError(t, cborlite.Unmarshal(data, &got))
		assert.Nil(t, got.Elements)
		assert.NotNil(t, got.Numbers)
		assert.Empty(t, got.Numbers)
	})

	t.Run("an element that does not decode names its index", func(t *testing.T) {
		data := cborMap(cborText("Numbers"), cborArray(head(uintMajor, 3), cborText("no")))

		var got holder
		err := cborlite.Unmarshal(data, &got)
		assert.ErrorContains(t, err, "Numbers")
		assert.ErrorContains(t, err, "[1]")
	})
}

func TestUnmarshalMap(t *testing.T) {
	type holder struct{ Bounds map[uint32]uint64 }

	data := cborMap(cborText("Bounds"), cborMap(
		head(uintMajor, 1), head(uintMajor, 10),
		head(uintMajor, 2), head(uintMajor, 20),
	))

	var got holder
	require.NoError(t, cborlite.Unmarshal(data, &got))
	assert.Equal(t, map[uint32]uint64{1: 10, 2: 20}, got.Bounds)

	t.Run("null reads as nil", func(t *testing.T) {
		data := cborMap(cborText("Bounds"), []byte{null})

		var got holder
		require.NoError(t, cborlite.Unmarshal(data, &got))
		assert.Nil(t, got.Bounds)
	})

	t.Run("a key that does not decode is named", func(t *testing.T) {
		data := cborMap(cborText("Bounds"), cborMap(cborText("no"), head(uintMajor, 10)))

		var got holder
		err := cborlite.Unmarshal(data, &got)
		assert.ErrorContains(t, err, "key")
	})

	// The header counts pairs, so a key twice leaves the map shorter than it claims.
	t.Run("declines a key given twice", func(t *testing.T) {
		data := cborMap(cborText("Bounds"),
			cborMap(head(uintMajor, 1), head(uintMajor, 10), head(uintMajor, 1), head(uintMajor, 20)))

		var got holder
		require.Error(t, cborlite.Unmarshal(data, &got))
	})
}

func TestUnmarshalMapValueDoesNotInheritTheOneBefore(t *testing.T) {
	type bounds struct {
		Low  uint64
		High uint64
	}
	type holder struct{ Bounds map[uint32]bounds }

	data := cborMap(cborText("Bounds"),
		cborMap(
			head(uintMajor, 1), cborMap(
				cborText("Low"), head(uintMajor, 10),
				cborText("High"), head(uintMajor, 20),
			),
			// The second entry has no High, so it must come out zero.
			head(uintMajor, 2), cborMap(cborText("Low"), head(uintMajor, 11)),
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
		cborText("Limbs"), cborArray(head(uintMajor, 1), head(uintMajor, 2)),
	)

	var got holder
	require.NoError(t, cborlite.Unmarshal(data, &got))
	assert.Equal(t, [3]byte{0xaa, 0xbb, 0xcc}, got.Address)
	assert.Equal(t, [2]uint64{1, 2}, got.Limbs)

	t.Run("null reads as the zero array", func(t *testing.T) {
		data := cborMap(cborText("Address"), []byte{null}, cborText("Limbs"), []byte{null})

		got := holder{Address: [3]byte{9, 9, 9}, Limbs: [2]uint64{9, 9}}
		require.NoError(t, cborlite.Unmarshal(data, &got))
		assert.Zero(t, got.Address)
		assert.Zero(t, got.Limbs)
	})

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
			"too few":  append(cborArray(head(uintMajor, 1)), head(uintMajor, 2)...),
			"too many": cborArray(head(uintMajor, 1), head(uintMajor, 2), head(uintMajor, 3)),
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

	data := cborMap(
		cborText("Raw"), cborBytes('{', '}'),
		cborText("Blob"), cborBytes(0x01, 0x02),
	)

	var got holder
	require.NoError(t, cborlite.Unmarshal(data, &got))
	assert.Equal(t, json.RawMessage("{}"), got.Raw)
	assert.Equal(t, []byte{0x01, 0x02}, got.Blob)

	t.Run("an array where a byte string belongs is rejected", func(t *testing.T) {
		data := cborMap(cborText("Blob"), cborArray(head(uintMajor, 1)))

		var got holder
		assert.ErrorContains(t, cborlite.Unmarshal(data, &got), "Blob")
	})

	// The generic encoder writes null for a nil slice, so the shape is on disk. Bytes does
	// not read null, which is why readByteSlice opens with a prologue for it.
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
		data := cborMap(cborText("Shape"), []byte{null})

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
		{name: "zero", data: cborMap(cborText("Wide"), head(uintMajor, 0)), ok: true},
		{name: "positive", data: cborMap(cborText("Wide"), head(uintMajor, 42)), wide: 42, ok: true},
		// A negative integer's argument is the magnitude minus one, so 0x20 is -1.
		{name: "minus one", data: cborMap(cborText("Wide"), head(negIntMajor, 0)), wide: -1, ok: true},
		{
			name: "minus one hundred",
			data: cborMap(cborText("Wide"), head(negIntMajor, 99)), wide: -100, ok: true,
		},
		{
			name: "the most negative int64",
			data: cborMap(cborText("Wide"), head(negIntMajor, math.MaxInt64)),
			wide: -1 << 63, ok: true,
		},
		{
			name: "past int64 on the negative side",
			data: cborMap(cborText("Wide"), head(negIntMajor, math.MaxUint64)),
		},
		{
			name: "past int64 on the positive side",
			data: cborMap(cborText("Wide"), head(uintMajor, math.MaxUint64)),
		},
		{name: "past int8", data: cborMap(cborText("Small"), head(uintMajor, 255))},
		{name: "past int8 negative", data: cborMap(cborText("Small"), head(negIntMajor, 255))},
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

func TestUnmarshalNamedByteSliceAndArray(t *testing.T) {
	type namedByte byte

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

func TestSliceDecodingDoesNotTrustTheCountForSizing(t *testing.T) {
	type holder struct{ Items []uint64 }

	// More elements than the budget holds, every one of them valid and one byte wide.
	const claimedSize = 200_000
	items := slices.Concat(
		head(arrayMajor, claimedSize),
		bytes.Repeat(head(uintMajor, 0), claimedSize),
	)

	var got holder
	require.NoError(t, cborlite.Unmarshal(cborMap(cborText("Items"), items), &got))
	require.Len(t, got.Items, claimedSize)

	// Decoder allocated from an internal budget, not from the claimedSize.
	assert.Greater(t, cap(got.Items), len(got.Items),
		"the slice was sized from the claimed count")
}
