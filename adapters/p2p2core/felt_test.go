package p2p2core

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

func TestAdaptNilReturnsNil(t *testing.T) {
	assert.Nil(t, AdaptHash(nil))
	assert.Nil(t, AdaptAddress(nil))
	assert.Nil(t, AdaptFelt(nil))
}

func TestAdaptUint128(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		u128 := AdaptUint128(nil)
		assert.Nil(t, u128)
	})
	t.Run("non-nil", func(t *testing.T) {
		cases := []struct {
			Low, High uint64
			Expect    *felt.Felt
		}{
			{32, 64, utils.HexToFelt(t, "0x400000000000000020")},
		}

		for _, c := range cases {
			result := AdaptUint128(&spec.Uint128{
				Low:  c.Low,
				High: c.High,
			})
			assert.Equal(t, c.Expect, result, "expected %v actual %v", c.Expect, result)
		}
	})
}
