package rpcv10

import (
	"encoding/json"
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

func TestCreateSimulatedTransactions(t *testing.T) {
	executionResults := vm.ExecutionResults{
		OverallFees: []*felt.Felt{
			felt.NewFromUint64[felt.Felt](10),
			felt.NewFromUint64[felt.Felt](20),
		},
		DataAvailability: []core.DataAvailability{
			{L1Gas: 5, L1DataGas: 2},
			{L1Gas: 6, L1DataGas: 3},
		},
		GasConsumed: []core.GasConsumed{
			{L1Gas: 100, L1DataGas: 50, L2Gas: 200},
			{L1Gas: 150, L1DataGas: 70, L2Gas: 250},
		},
		Traces:   []vm.TransactionTrace{{Type: vm.TxnL1Handler}, {}},
		NumSteps: 0,
	}

	txns := []core.Transaction{
		&core.L1HandlerTransaction{
			Version: new(core.TransactionVersion).SetUint64(3),
		},
		&core.InvokeTransaction{
			Version: new(core.TransactionVersion).SetUint64(3),
		},
	}

	header := &core.Header{
		L1GasPriceETH:  felt.NewFromUint64[felt.Felt](1),
		L1GasPriceSTRK: felt.NewFromUint64[felt.Felt](2),
		L2GasPrice: &core.GasPrice{
			PriceInWei: felt.NewFromUint64[felt.Felt](3),
			PriceInFri: felt.NewFromUint64[felt.Felt](4),
		},
		L1DataGasPrice: &core.GasPrice{
			PriceInWei: felt.NewFromUint64[felt.Felt](5),
			PriceInFri: felt.NewFromUint64[felt.Felt](6),
		},
	}

	// Successful case
	simTxs, err := createSimulatedTransactions(&executionResults, txns, header)
	require.NoError(t, err)
	require.Len(t, simTxs, 2)
	expected := []SimulatedTransaction{
		{
			TransactionTrace: &TransactionTrace{
				Type: TransactionType(vm.TxnL1Handler),
				ExecutionResources: &ExecutionResources{
					InnerExecutionResources: InnerExecutionResources{
						L1Gas: 100,
						L2Gas: 200,
					},
					L1DataGas: 50,
				},
			},
			FeeEstimation: FeeEstimate{
				L1GasConsumed:     felt.NewFromUint64[felt.Felt](100),
				L1GasPrice:        felt.NewFromUint64[felt.Felt](1),
				L2GasConsumed:     felt.NewFromUint64[felt.Felt](200),
				L2GasPrice:        felt.NewFromUint64[felt.Felt](3),
				L1DataGasConsumed: felt.NewFromUint64[felt.Felt](50),
				L1DataGasPrice:    felt.NewFromUint64[felt.Felt](5),
				OverallFee:        felt.NewFromUint64[felt.Felt](10),
				Unit:              utils.HeapPtr(WEI),
			},
		},
		{
			TransactionTrace: &TransactionTrace{
				ExecutionResources: &ExecutionResources{
					InnerExecutionResources: InnerExecutionResources{
						L1Gas: 150,
						L2Gas: 250,
					},
					L1DataGas: 70,
				},
			},
			FeeEstimation: FeeEstimate{
				L1GasConsumed:     felt.NewFromUint64[felt.Felt](150),
				L1GasPrice:        felt.NewFromUint64[felt.Felt](2),
				L2GasConsumed:     felt.NewFromUint64[felt.Felt](250),
				L2GasPrice:        felt.NewFromUint64[felt.Felt](4),
				L1DataGasConsumed: felt.NewFromUint64[felt.Felt](70),
				L1DataGasPrice:    felt.NewFromUint64[felt.Felt](6),
				OverallFee:        felt.NewFromUint64[felt.Felt](20),
				Unit:              utils.HeapPtr(FRI),
			},
		},
	}
	require.Equal(t, expected, simTxs)

	// Edge case: Inconsistent lengths
	executionResults.Traces = []vm.TransactionTrace{{}}
	simTxs, err = createSimulatedTransactions(&executionResults, txns, header)
	require.Error(t, err)
	require.Contains(t, err.Error(), "inconsistent lengths")
	require.Empty(t, simTxs)

	// Edge case: Empty input
	simTxs, err = createSimulatedTransactions(
		&vm.ExecutionResults{},
		[]core.Transaction{},
		header,
	)
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
				Cause: json.RawMessage("some error"),
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
