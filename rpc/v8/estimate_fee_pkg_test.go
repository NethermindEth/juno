package rpcv8

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
)

func TestFeeEstimateToV0_7(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		assert.Equal(t, FeeEstimateV0_7{}, feeEstimateToV0_7(FeeEstimate{}))
	})

	t.Run("full", func(t *testing.T) {
		gasConsumed := new(felt.Felt).SetUint64(1)
		gasPrice := new(felt.Felt).SetUint64(2)
		dataGasConsumed := new(felt.Felt).SetUint64(3)
		dataGasPrice := new(felt.Felt).SetUint64(4)
		overallFee := new(felt.Felt).SetUint64(5)
		unit := WEI
		assert.Equal(
			t,
			FeeEstimateV0_7{
				GasConsumed:     gasConsumed,
				GasPrice:        gasPrice,
				DataGasConsumed: dataGasConsumed,
				DataGasPrice:    dataGasPrice,
				OverallFee:      overallFee,
				Unit:            &unit,
			},
			feeEstimateToV0_7(FeeEstimate{
				L1GasConsumed:     gasConsumed,
				L1GasPrice:        gasPrice,
				L2GasConsumed:     new(felt.Felt).SetUint64(6),
				L2GasPrice:        new(felt.Felt).SetUint64(7),
				L1DataGasConsumed: dataGasConsumed,
				L1DataGasPrice:    dataGasPrice,
				OverallFee:        overallFee,
				Unit:              &unit,
			}))
	})
}
