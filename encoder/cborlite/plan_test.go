package cborlite_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/encoder/cborlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalHonoursCBORTags(t *testing.T) {
	t.Run("a tag name is the wire key, not the Go field name", func(t *testing.T) {
		type tagged struct {
			Price *big.Int `cbor:"gasprice"`
		}

		// Keyed on the tag name, which is what the encoder writes.
		data := cborMap(cborText("gasprice"), head(uintMajor, 7))

		var got tagged
		require.NoError(t, cborlite.Unmarshal(data, &got))
		require.NotNil(t, got.Price)
		assert.Equal(t, int64(7), got.Price.Int64())
	})

	t.Run("the Go name is not accepted when a tag renames the field", func(t *testing.T) {
		type tagged struct {
			Price *big.Int `cbor:"gasprice"`
		}

		// Keyed on the Go name: unknown to the plan, so skipped, and the field stays nil.
		data := cborMap(cborText("Price"), head(uintMajor, 7))

		var got tagged
		require.NoError(t, cborlite.Unmarshal(data, &got))
		assert.Nil(t, got.Price, "an unknown key is skipped, which is how a typo goes quiet")
	})

	t.Run("an empty tag name keeps the Go field name", func(t *testing.T) {
		type tagged struct {
			Value uint64 `cbor:",omitempty"`
		}

		var got tagged
		require.NoError(t, cborlite.Unmarshal(cborMap(cborText("Value"), head(uintMajor, 7)), &got))
		assert.Equal(t, uint64(7), got.Value)
	})

	t.Run("a dash means the field is not on the wire, so the build fails", func(t *testing.T) {
		type tagged struct {
			Value uint64 `cbor:"-"`
		}

		var got tagged
		assert.ErrorContains(t, cborlite.Unmarshal(cborMap(), &got), "unsupported cbor tag")
	})

	t.Run("keyasint is not implemented, and says so", func(t *testing.T) {
		type tagged struct {
			Value uint64 `cbor:"1,keyasint"`
		}

		var got tagged
		assert.ErrorContains(t, cborlite.Unmarshal(cborMap(), &got), "unsupported cbor tag")
	})
}

func TestUnmarshalEmbeddedFieldIsRefused(t *testing.T) {
	type inner struct{ Low uint64 }
	type outer struct {
		inner
		High uint64
	}

	var got outer
	err := cborlite.Unmarshal(cborMap(cborText("Low"), head(uintMajor, 1)), &got)
	assert.ErrorContains(t, err, "embedded")
}

// cycleOuter cannot be built: float64 has no reader. cycleInner takes part in the cycle.
type (
	cycleOuter struct {
		Inner cycleInner
		Good  uint64
		Bad   float64
	}
	cycleInner struct{ Outer *cycleOuter }
)

// Local types cannot do cross reference
type (
	outerCycle struct{ Inner []innerCycle }
	innerCycle struct {
		Outer []outerCycle
		Value uint64
	}
)

func TestUnmarshalSelfReferentialStruct(t *testing.T) {
	t.Run("through a slice of itself", func(t *testing.T) {
		type node struct {
			Children []node
			Length   uint64
		}

		data := cborMap(
			cborText("Length"), head(uintMajor, 1),
			cborText("Children"), cborArray(
				cborMap(cborText("Length"), head(uintMajor, 2)),
				cborMap(
					cborText("Length"), head(uintMajor, 3),
					cborText("Children"), cborArray(cborMap(cborText("Length"), head(uintMajor, 4))),
				),
			),
		)

		var got node
		require.NoError(t, cborlite.Unmarshal(data, &got))
		assert.Equal(t, node{
			Length: 1,
			Children: []node{
				{Length: 2},
				{Length: 3, Children: []node{{Length: 4}}},
			},
		}, got)
	})

	t.Run("through a pointer to itself", func(t *testing.T) {
		type node struct {
			Next  *node
			Value uint64
		}

		data := cborMap(
			cborText("Value"), head(uintMajor, 1),
			cborText("Next"), cborMap(
				cborText("Value"), head(uintMajor, 2),
				cborText("Next"), []byte{null},
			),
		)

		var got node
		require.NoError(t, cborlite.Unmarshal(data, &got))
		assert.Equal(t, node{Value: 1, Next: &node{Value: 2}}, got)
	})

	t.Run("through another type that points back", func(t *testing.T) {
		data := cborMap(cborText("Inner"), cborArray(
			cborMap(
				cborText("Value"), head(uintMajor, 7),
				cborText("Outer"), cborArray(cborMap(cborText("Inner"), cborArray())),
			),
		))

		var got outerCycle
		require.NoError(t, cborlite.Unmarshal(data, &got))
		assert.Equal(t, outerCycle{Inner: []innerCycle{{
			Value: 7,
			Outer: []outerCycle{{Inner: []innerCycle{}}},
		}}}, got)
	})
}

// A struct by value has no nil to hold, so null zeroes it.
// Compatibility with the generic decoder
func TestNullWhereAStructIsExpected(t *testing.T) {
	type inner struct{ Value uint64 }
	type outer struct {
		Nested inner
		After  uint64
	}

	data := cborMap(
		cborText("Nested"), []byte{null},
		cborText("After"), head(uintMajor, 7),
	)

	var got outer
	require.NoError(t, cborlite.Unmarshal(data, &got))
	assert.Zero(t, got.Nested)
	assert.Equal(t, uint64(7), got.After)
}

func TestReadingStopsBeforeTheStackOverflows(t *testing.T) {
	type node struct{ Children []node }

	build := func(depth int) []byte {
		out := make([]byte, 0, depth*12+1)
		for range depth {
			out = append(out, initialByte(mapMajor, 1))
			out = append(out, cborText("Children")...)
			out = append(out, initialByte(arrayMajor, 1))
		}
		return append(out, initialByte(mapMajor, 0))
	}

	t.Run("nesting within the bound is read", func(t *testing.T) {
		var got node
		require.NoError(t, cborlite.Unmarshal(build(8), &got))
	})

	for _, depth := range []int{100, 1_000_000} {
		t.Run(fmt.Sprintf("declines %d levels instead of crashing", depth), func(t *testing.T) {
			var got node
			require.ErrorContains(t, cborlite.Unmarshal(build(depth), &got), "nested past")
		})
	}
}

func TestAFailedBuildInACycleLeavesNothingUsable(t *testing.T) {
	data := cborMap(cborText("Good"), head(uintMajor, 7), cborText("Bad"), head(uintMajor, 1))

	var outer cycleOuter
	require.Error(t, cborlite.Unmarshal(data, &outer))

	// cycleInner points back at cycleOuter, so its build saw the outer one half-built.
	var inner cycleInner
	require.Error(t, cborlite.Unmarshal(cborMap(cborText("Outer"), []byte{null}), &inner))
}

func TestDeclinesAreTellableApart(t *testing.T) {
	t.Run("bytes that do not match the target", func(t *testing.T) {
		type holder struct{ Value uint64 }

		err := cborlite.Unmarshal(cborMap(cborText("Value"), cborText("no")), &holder{})
		require.ErrorIs(t, err, cborlite.ErrShape)
		assert.NotErrorIs(t, err, cborlite.ErrUnsupportedType)
	})

	t.Run("a tag this package does not support", func(t *testing.T) {
		type holder struct {
			Value uint64 `cbor:",not_an_option"`
		}

		err := cborlite.Unmarshal(cborMap(), &holder{})
		require.ErrorIs(t, err, cborlite.ErrUnsupportedType)
		assert.NotErrorIs(t, err, cborlite.ErrShape)
	})
}
