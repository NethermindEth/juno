package rpc

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
)

func TestCalculateFeeEstimate(t *testing.T) {
	l1GasPriceETH := new(felt.Felt).SetUint64(200)
	l2GasPriceETH := new(felt.Felt).SetUint64(70)
	l1GasPriceSTRK := new(felt.Felt).SetUint64(100)
	l2GasPriceSTRK := new(felt.Felt).SetUint64(50)
	l1DataGasPrice := &core.GasPrice{
		PriceInWei: new(felt.Felt).SetUint64(10),
		PriceInFri: new(felt.Felt).SetUint64(5),
	}
	header := &core.Header{
		L1GasPriceETH:  l1GasPriceETH,
		L2GasPriceETH:  l2GasPriceETH,
		L1GasPriceSTRK: l1GasPriceSTRK,
		L2GasPriceSTRK: l2GasPriceSTRK,
		L1DataGasPrice: l1DataGasPrice,
	}
	l1DataGas := uint64(500)
	overallFee := new(felt.Felt).SetUint64(6000)

	feeEstimate := calculateFeeEstimate(overallFee, l1DataGas, FRI, header)

	assert.Equal(t, l1GasPriceSTRK, feeEstimate.L1GasPrice)
	assert.Equal(t, l2GasPriceSTRK, feeEstimate.L2GasPrice)
	assert.Equal(t, l1DataGasPrice.PriceInFri, feeEstimate.L1DataGasPrice)
	assert.Equal(t, overallFee, feeEstimate.OverallFee)
	assert.Equal(t, FRI, *feeEstimate.Unit)
	assert.Equal(t, new(felt.Felt).SetUint64(35), feeEstimate.L1GasConsumed)
	assert.Equal(t, &felt.Zero, feeEstimate.L2GasConsumed)
}
