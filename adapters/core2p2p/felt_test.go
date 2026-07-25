package core2p2p

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdaptFeltSlice(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		assert.Nil(t, AdaptFeltSlice(nil))
	})
	t.Run("converts elements", func(t *testing.T) {
		one := felt.FromUint64[felt.Felt](1)
		two := felt.FromUint64[felt.Felt](2)

		result := AdaptFeltSlice([]felt.Felt{one, two})
		require.Len(t, result, 2)
		assert.Equal(t, one.Marshal(), result[0].Elements)
		assert.Equal(t, two.Marshal(), result[1].Elements)
	})
}

func TestAdaptAccountSignature(t *testing.T) {
	one := felt.FromUint64[felt.Felt](1)
	two := felt.FromUint64[felt.Felt](2)

	sig := AdaptAccountSignature([]felt.Felt{one, two})
	require.Len(t, sig.Parts, 2)
	assert.Equal(t, one.Marshal(), sig.Parts[0].Elements)
	assert.Equal(t, two.Marshal(), sig.Parts[1].Elements)
}
