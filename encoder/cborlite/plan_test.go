package cborlite_test

import (
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
		data := cborMap(cborText("gasprice"), []byte{0x07})

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
		data := cborMap(cborText("Price"), []byte{0x07})

		var got tagged
		require.NoError(t, cborlite.Unmarshal(data, &got))
		assert.Nil(t, got.Price, "an unknown key is skipped, which is how a typo goes quiet")
	})

	t.Run("an empty tag name keeps the Go field name", func(t *testing.T) {
		type tagged struct {
			Value uint64 `cbor:",omitempty"`
		}

		var got tagged
		require.NoError(t, cborlite.Unmarshal(cborMap(cborText("Value"), []byte{0x07}), &got))
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
	err := cborlite.Unmarshal(cborMap(cborText("Low"), []byte{0x01}), &got)
	assert.ErrorContains(t, err, "embedded")
}

// A pair of types that reach each other, so two plans are under construction at once and
// the cache has to hand back the right one. Local types cannot do this: a type declared in
// a function body cannot name one declared after it.
type (
	outerCycle struct{ Inner []innerCycle }
	innerCycle struct {
		Back  []outerCycle
		Value uint64
	}
)

// TestUnmarshalSelfReferentialStruct covers the cycle guard in the plan cache. Building a
// plan descends into every field, and a field of the type being built descends into it
// again, so without the half-built plan being handed back the build never returns and the
// process dies on a full stack rather than reporting anything.
//
// core.SegmentLengths is this shape and it is read on the getClass path, so the guard runs
// in production. Nothing else here would notice it going: every other test names a type
// that terminates.
func TestUnmarshalSelfReferentialStruct(t *testing.T) {
	t.Run("through a slice of itself", func(t *testing.T) {
		type node struct {
			Children []node
			Length   uint64
		}

		data := cborMap(
			cborText("Length"), []byte{0x01},
			cborText("Children"), cborArray(
				cborMap(cborText("Length"), []byte{0x02}),
				cborMap(
					cborText("Length"), []byte{0x03},
					cborText("Children"), cborArray(cborMap(cborText("Length"), []byte{0x04})),
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

	// A pointer field descends eagerly too, so it reaches the guard by its own route.
	t.Run("through a pointer to itself", func(t *testing.T) {
		type node struct {
			Next  *node
			Value uint64
		}

		data := cborMap(
			cborText("Value"), []byte{0x01},
			cborText("Next"), cborMap(
				cborText("Value"), []byte{0x02},
				cborText("Next"), []byte{cborlite.Null},
			),
		)

		var got node
		require.NoError(t, cborlite.Unmarshal(data, &got))
		assert.Equal(t, node{Value: 1, Next: &node{Value: 2}}, got)
	})

	t.Run("through another type that points back", func(t *testing.T) {
		data := cborMap(cborText("Inner"), cborArray(
			cborMap(
				cborText("Value"), []byte{0x07},
				cborText("Back"), cborArray(cborMap(cborText("Inner"), cborArray())),
			),
		))

		var got outerCycle
		require.NoError(t, cborlite.Unmarshal(data, &got))
		assert.Equal(t, outerCycle{Inner: []innerCycle{{
			Value: 7,
			Back:  []outerCycle{{Inner: []innerCycle{}}},
		}}}, got)
	})
}

// TestNullWhereAStructIsExpected covers the one kind that cannot be absent. A pointer, a
// slice, a map and an interface field all read null as their zero value, and a struct
// field refuses it instead of coming back silently empty.
func TestNullWhereAStructIsExpected(t *testing.T) {
	type inner struct{ Value uint64 }
	type outer struct {
		Nested inner
		After  uint64
	}

	data := cborMap(
		cborText("Nested"), []byte{cborlite.Null},
		cborText("After"), []byte{0x07},
	)

	var got outer
	require.ErrorContains(t, cborlite.Unmarshal(data, &got), "Nested")
}
