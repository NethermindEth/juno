package felt_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
	"github.com/stretchr/testify/require"
)

func TestIsZero(t *testing.T) {
	var val feltoid
	require.True(t, felt.IsZero(&val))

	val[0] = 1
	require.False(t, felt.IsZero(&val))
}

func TestEqual(t *testing.T) {
	base := [4]uint64{1, 2, 3, 4}
	v1 := feltoid(base)
	v2 := feltoid(base)
	require.True(t, felt.Equal(&v1, &v2))

	v2[0] += 100
	require.False(t, felt.Equal(&v1, &v2))
}

// The helpers must agree with the field element's own comparisons on every input.
func TestHelpersMatchElement(t *testing.T) {
	values := []felt.Felt{
		{},
		felt.One[felt.Felt](),
		felt.FromUint64[felt.Felt](1 << 40),
		felt.Random[felt.Felt](),
		felt.Random[felt.Felt](),
	}
	for _, a := range values {
		elementA := fp.Element(a)
		require.Equal(t, elementA.IsZero(), felt.IsZero(&a))
		for _, b := range values {
			elementB := fp.Element(b)
			require.Equal(t, elementA.Equal(&elementB), felt.Equal(&a, &b))
		}
	}
}

func TestCastPointerSlice(t *testing.T) {
	t.Run("nil stays nil", func(t *testing.T) {
		require.Nil(t, felt.CastPointerSlice[felt.ClassHash]([]*felt.Felt(nil)))
	})

	t.Run("shares the backing array", func(t *testing.T) {
		first, second := felt.FromUint64[felt.Felt](1), felt.FromUint64[felt.Felt](2)
		felts := []*felt.Felt{&first, &second}

		classHashes := felt.CastPointerSlice[felt.ClassHash](felts)
		require.Len(t, classHashes, 2)
		require.Same(t, &first, (*felt.Felt)(classHashes[0]))
		require.Same(t, &second, (*felt.Felt)(classHashes[1]))

		classHashes[0], classHashes[1] = classHashes[1], classHashes[0]
		require.Same(t, &second, felts[0])
	})
}

func TestCastSlice(t *testing.T) {
	t.Run("nil stays nil", func(t *testing.T) {
		require.Nil(t, felt.CastSlice[felt.Address]([]felt.Felt(nil)))
	})

	t.Run("shares the backing array", func(t *testing.T) {
		felts := []felt.Felt{felt.FromUint64[felt.Felt](1), felt.FromUint64[felt.Felt](2)}

		addresses := felt.CastSlice[felt.Address](felts)
		want := []felt.Address{felt.FromUint64[felt.Address](1), felt.FromUint64[felt.Address](2)}
		require.Equal(t, want, addresses)

		addresses[0] = felt.FromUint64[felt.Address](7)
		require.Equal(t, felt.FromUint64[felt.Felt](7), felts[0])
	})
}
