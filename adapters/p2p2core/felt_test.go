package p2p2core

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/common"
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
			{32, 64, felt.NewUnsafeFromString[felt.Felt]("0x400000000000000020")},
		}

		for _, c := range cases {
			result := AdaptUint128(&common.Uint128{
				Low:  c.Low,
				High: c.High,
			})
			assert.Equal(t, c.Expect, result, "expected %v actual %v", c.Expect, result)
		}
	})
}
