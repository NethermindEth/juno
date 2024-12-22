package starknet_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/stretchr/testify/assert"
)

func TestL2GasPrice(t *testing.T) {
	t.Run("L2GasPrice is not set", func(t *testing.T) {
		block := starknet.Block{}
		assert.Equal(t, &felt.Zero, block.L2GasPriceETH())
		assert.Equal(t, &felt.Zero, block.L2GasPriceSTRK())
	})

	t.Run("L2GasPrice is set", func(t *testing.T) {
		gasPriceWei := new(felt.Felt).SetUint64(100)
		gasPriceFri := new(felt.Felt).SetUint64(50)
		block := starknet.Block{
			L2GasPrice: &starknet.GasPrice{
				PriceInWei: gasPriceWei,
				PriceInFri: gasPriceFri,
			},
		}
		assert.Equal(t, gasPriceWei, block.L2GasPriceETH())
		assert.Equal(t, gasPriceFri, block.L2GasPriceSTRK())
	})
}
