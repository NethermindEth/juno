package rpc

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

func TestEstimateFeeMarshalJson(t *testing.T) {
	tests := []struct {
		name string
		f    FeeEstimate
		want []byte
	}{
		{ //nolint:dupl
			name: "V0_7",
			f: FeeEstimate{
				L1GasConsumed:     new(felt.Felt).SetUint64(1),
				L1GasPrice:        new(felt.Felt).SetUint64(2),
				L2GasConsumed:     new(felt.Felt).SetUint64(3),
				L2GasPrice:        new(felt.Felt).SetUint64(4),
				L1DataGasConsumed: new(felt.Felt).SetUint64(5),
				L1DataGasPrice:    new(felt.Felt).SetUint64(6),
				OverallFee:        new(felt.Felt).SetUint64(7),
				Unit:              utils.Ptr(WEI),
				rpcVersion:        V0_7,
			},
			want: []byte(`{"gas_consumed":"0x1","gas_price":"0x2","data_gas_consumed":"0x5","data_gas_price":"0x6","overall_fee":"0x7","unit":"WEI"}`),
		},
		{ //nolint:dupl
			name: "V0_8",
			f: FeeEstimate{
				L1GasConsumed:     new(felt.Felt).SetUint64(8),
				L1GasPrice:        new(felt.Felt).SetUint64(9),
				L2GasConsumed:     new(felt.Felt).SetUint64(10),
				L2GasPrice:        new(felt.Felt).SetUint64(11),
				L1DataGasConsumed: new(felt.Felt).SetUint64(12),
				L1DataGasPrice:    new(felt.Felt).SetUint64(13),
				OverallFee:        new(felt.Felt).SetUint64(14),
				Unit:              utils.Ptr(WEI),
				rpcVersion:        V0_8,
			},
			want: []byte(`{"l1_gas_consumed":"0x8","l1_gas_price":"0x9","l2_gas_consumed":"0xa","l2_gas_price":"0xb","l1_data_gas_consumed":"0xc","l1_data_gas_price":"0xd","overall_fee":"0xe","unit":"WEI"}`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.f.MarshalJSON()
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
