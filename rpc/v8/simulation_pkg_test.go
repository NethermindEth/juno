package rpcv8

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/require"
)

//nolint:dupl
func TestCreateSimulatedTransactions(t *testing.T) {
	executionResults := vm.ExecutionResults{
		OverallFees: []*felt.Felt{new(felt.Felt).SetUint64(10), new(felt.Felt).SetUint64(20)},
		DataAvailability: []core.DataAvailability{
			{L1Gas: 5, L1DataGas: 2},
			{L1Gas: 6, L1DataGas: 3},
		},
		GasConsumed: []core.GasConsumed{
			{L1Gas: 100, L1DataGas: 50, L2Gas: 200},
			{L1Gas: 150, L1DataGas: 70, L2Gas: 250},
		},
		Traces:   []vm.TransactionTrace{{}, {}},
		NumSteps: 0,
	}
	txns := []core.Transaction{
		&core.InvokeTransaction{
			Version: new(core.TransactionVersion).SetUint64(1),
		},
		&core.InvokeTransaction{
			Version: new(core.TransactionVersion).SetUint64(3),
		},
	}
	header := &core.Header{
		L1GasPriceETH:  new(felt.Felt).SetUint64(1),
		L1GasPriceSTRK: new(felt.Felt).SetUint64(2),
		L2GasPrice: &core.GasPrice{
			PriceInWei: new(felt.Felt).SetUint64(3),
			PriceInFri: new(felt.Felt).SetUint64(4),
		},
		L1DataGasPrice: &core.GasPrice{
			PriceInWei: new(felt.Felt).SetUint64(5),
			PriceInFri: new(felt.Felt).SetUint64(6),
		},
	}

	// Successful case
	simTxs, err := createSimulatedTransactions(executionResults, txns, header)
	require.NoError(t, err)
	require.Len(t, simTxs, 2)
	expected := []SimulatedTransaction{
		{
			TransactionTrace: &vm.TransactionTrace{
				ExecutionResources: &vm.ExecutionResources{
					L1Gas:     100,
					L1DataGas: 50,
					L2Gas:     200,
					DataAvailability: &vm.DataAvailability{
						L1Gas:     5,
						L1DataGas: 2,
					},
				},
			},
			FeeEstimation: FeeEstimate{
				L1GasConsumed:     new(felt.Felt).SetUint64(100),
				L1GasPrice:        new(felt.Felt).SetUint64(1),
				L2GasConsumed:     new(felt.Felt).SetUint64(200),
				L2GasPrice:        new(felt.Felt).SetUint64(3),
				L1DataGasConsumed: new(felt.Felt).SetUint64(50),
				L1DataGasPrice:    new(felt.Felt).SetUint64(5),
				OverallFee:        new(felt.Felt).SetUint64(10),
				Unit:              utils.Ptr(WEI),
			},
		},
		{
			TransactionTrace: &vm.TransactionTrace{
				ExecutionResources: &vm.ExecutionResources{
					L1Gas:     150,
					L1DataGas: 70,
					L2Gas:     250,
					DataAvailability: &vm.DataAvailability{
						L1Gas:     6,
						L1DataGas: 3,
					},
				},
			},
			FeeEstimation: FeeEstimate{
				L1GasConsumed:     new(felt.Felt).SetUint64(150),
				L1GasPrice:        new(felt.Felt).SetUint64(2),
				L2GasConsumed:     new(felt.Felt).SetUint64(250),
				L2GasPrice:        new(felt.Felt).SetUint64(4),
				L1DataGasConsumed: new(felt.Felt).SetUint64(70),
				L1DataGasPrice:    new(felt.Felt).SetUint64(6),
				OverallFee:        new(felt.Felt).SetUint64(20),
				Unit:              utils.Ptr(FRI),
			},
		},
	}
	require.Equal(t, expected, simTxs)

	// Edge case: Inconsistent lengths
	executionResults.Traces = []vm.TransactionTrace{{}}
	simTxs, err = createSimulatedTransactions(executionResults, txns, header)
	require.Error(t, err)
	require.Contains(t, err.Error(), "inconsistent lengths")
	require.Empty(t, simTxs)

	// Edge case: Empty input
	simTxs, err = createSimulatedTransactions(vm.ExecutionResults{}, []core.Transaction{}, header)
	require.NoError(t, err)
	require.Empty(t, simTxs)
}

func TestHandleExecutionError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		jsonRPCError *jsonrpc.Error
	}{
		{
			name:         "Resource Busy Error",
			err:          utils.ErrResourceBusy,
			jsonRPCError: rpccore.ErrInternal.CloneWithData(rpccore.ThrottledVMErr),
		},
		{
			name: "Transaction Execution Error",
			err: &vm.TransactionExecutionError{
				Index: 0,
				Cause: errors.New("some error"),
			},
			jsonRPCError: &jsonrpc.Error{
				Code:    rpccore.ErrUnexpectedError.Code,
				Message: rpccore.ErrUnexpectedError.Message,
				Data:    "execute transaction #0: some error",
			},
		},
		{
			name:         "Unexpected Error",
			err:          errors.New("unexpected error"),
			jsonRPCError: rpccore.ErrUnexpectedError.CloneWithData("unexpected error"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.jsonRPCError, handleExecutionError(test.err))
		})
	}
}
