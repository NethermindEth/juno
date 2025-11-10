package rpcv6_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	rpccore "github.com/NethermindEth/juno/rpc/rpccore"
	rpc "github.com/NethermindEth/juno/rpc/v6"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

//nolint:dupl
func TestSimulateTransactions(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	n := &utils.Mainnet

	mockReader := mocks.NewMockReader(mockCtrl)
	mockReader.EXPECT().Network().Return(n).AnyTimes()
	mockVM := mocks.NewMockVM(mockCtrl)
	handler := rpc.New(mockReader, nil, mockVM, n, utils.NewNopZapLogger())

	mockState := mocks.NewMockStateHistoryReader(mockCtrl)
	mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil).AnyTimes()
	headsHeader := &core.Header{
		SequencerAddress: n.BlockHashMetaInfo.FallBackSequencerAddress,
	}
	mockReader.EXPECT().HeadsHeader().Return(headsHeader, nil).AnyTimes()
	nilTxns := make([]core.Transaction, 0)
	t.Run("ok with zero values, skip fee", func(t *testing.T) {
		stepsUsed := uint64(123)
		mockVM.EXPECT().Execute(nilTxns, nil, []*felt.Felt{}, &vm.BlockInfo{
			Header: headsHeader,
		}, mockState, true, false, false, false, true, false).
			Return(vm.ExecutionResults{
				OverallFees:      []*felt.Felt{},
				DataAvailability: []core.DataAvailability{},
				Traces:           []vm.TransactionTrace{},
				NumSteps:         stepsUsed,
			}, nil)

		_, err := handler.SimulateTransactions(
			rpc.BlockID{Latest: true},
			rpc.BroadcastedTransactionInputs{},
			[]rpc.SimulationFlag{rpc.SkipFeeChargeFlag},
		)
		require.Nil(t, err)
	})

	t.Run("ok with zero values, skip validate", func(t *testing.T) {
		stepsUsed := uint64(123)
		mockVM.EXPECT().Execute(nilTxns, nil, []*felt.Felt{}, &vm.BlockInfo{
			Header: headsHeader,
		}, mockState, false, true, false, false, true, false).
			Return(vm.ExecutionResults{
				OverallFees:      []*felt.Felt{},
				DataAvailability: []core.DataAvailability{},
				Traces:           []vm.TransactionTrace{},
				NumSteps:         stepsUsed,
			}, nil)

		_, err := handler.SimulateTransactions(
			rpc.BlockID{Latest: true},
			rpc.BroadcastedTransactionInputs{},
			[]rpc.SimulationFlag{rpc.SkipValidateFlag},
		)
		require.Nil(t, err)
	})

	t.Run("transaction execution error", func(t *testing.T) {
		mockVM.EXPECT().Execute(nilTxns, nil, []*felt.Felt{}, &vm.BlockInfo{
			Header: headsHeader,
		}, mockState, false, true, false, false, true, false).
			Return(vm.ExecutionResults{}, vm.TransactionExecutionError{
				Index: 44,
				Cause: json.RawMessage("oops"),
			})

		_, err := handler.SimulateTransactions(
			rpc.BlockID{Latest: true},
			rpc.BroadcastedTransactionInputs{},
			[]rpc.SimulationFlag{rpc.SkipValidateFlag},
		)
		require.Equal(t, rpccore.ErrTransactionExecutionError.CloneWithData(rpc.TransactionExecutionErrorData{
			TransactionIndex: 44,
			ExecutionError:   json.RawMessage("oops"),
		}), err)

		mockVM.EXPECT().Execute(nilTxns, nil, []*felt.Felt{}, &vm.BlockInfo{
			Header: headsHeader,
		}, mockState, false, true, false, false, true, false).
			Return(vm.ExecutionResults{}, vm.TransactionExecutionError{
				Index: 44,
				Cause: json.RawMessage("oops"),
			})

		_, err = handler.SimulateTransactions(
			rpc.BlockID{Latest: true},
			rpc.BroadcastedTransactionInputs{},
			[]rpc.SimulationFlag{rpc.SkipValidateFlag},
		)
		require.Equal(t, rpccore.ErrTransactionExecutionError.CloneWithData(rpc.TransactionExecutionErrorData{
			TransactionIndex: 44,
			ExecutionError:   json.RawMessage("oops"),
		}), err)
	})

	t.Run("incosistant length error", func(t *testing.T) {
		mockVM.EXPECT().Execute([]core.Transaction{}, nil, []*felt.Felt{}, &vm.BlockInfo{
			Header: headsHeader,
		}, mockState, false, true, false, false, true, false).
			Return(vm.ExecutionResults{
				OverallFees:      []*felt.Felt{&felt.Zero},
				DataAvailability: []core.DataAvailability{{L1Gas: 0}, {L1Gas: 0}},
				GasConsumed:      []core.GasConsumed{{L1Gas: 0, L1DataGas: 0, L2Gas: 0}},
				Traces:           []vm.TransactionTrace{{}},
				NumSteps:         uint64(0),
			}, nil)

		_, err := handler.SimulateTransactions(
			rpc.BlockID{Latest: true},
			rpc.BroadcastedTransactionInputs{},
			[]rpc.SimulationFlag{rpc.SkipValidateFlag},
		)
		require.Equal(t, rpccore.ErrInternal.CloneWithData(errors.New(
			"inconsistent lengths: 1 overall fees, 1 traces, 1 gas consumed, 2 data availability, 0 txns",
		)), err)
	})
}

func TestSimulateTransactionsShouldErrorWithoutSenderAddressOrResourceBounds(t *testing.T) {
	t.Parallel()
	n := &utils.Mainnet
	headsHeader := &core.Header{
		SequencerAddress: n.BlockHashMetaInfo.FallBackSequencerAddress,
		L1GasPriceETH:    &felt.Zero,
		L1GasPriceSTRK:   &felt.Zero,
		L1DAMode:         0,
		L1DataGasPrice: &core.GasPrice{
			PriceInWei: &felt.Zero,
			PriceInFri: &felt.Zero,
		},
		L2GasPrice: &core.GasPrice{
			PriceInWei: &felt.Zero,
			PriceInFri: &felt.Zero,
		},
	}

	version3 := felt.FromUint64[felt.Felt](3)

	tests := []struct {
		name         string
		transactions []rpc.BroadcastedTransaction
		err          *jsonrpc.Error
	}{
		{
			name: "declare transaction without sender address",
			transactions: []rpc.BroadcastedTransaction{
				{
					Transaction: rpc.Transaction{
						Version: &version3,
						Type:    rpc.TxnDeclare,
					},
				},
			},
			err: jsonrpc.Err(jsonrpc.InvalidParams, "sender_address is required for this transaction type"),
		},
		{
			name: "declare transaction without resource bounds",
			transactions: []rpc.BroadcastedTransaction{
				{
					Transaction: rpc.Transaction{
						Version:       &version3,
						Type:          rpc.TxnDeclare,
						SenderAddress: &felt.Zero,
					},
				},
			},
			err: jsonrpc.Err(jsonrpc.InvalidParams, "resource_bounds is required for this transaction type"),
		},
		{
			name: "invoke transaction without sender address",
			transactions: []rpc.BroadcastedTransaction{
				{
					Transaction: rpc.Transaction{
						Version: &version3,
						Type:    rpc.TxnInvoke,
					},
				},
			},
			err: jsonrpc.Err(jsonrpc.InvalidParams, "sender_address is required for this transaction type"),
		},
		{
			name: "invoke transaction without resource bounds",
			transactions: []rpc.BroadcastedTransaction{
				{
					Transaction: rpc.Transaction{
						Version:       &version3,
						Type:          rpc.TxnInvoke,
						SenderAddress: &felt.Zero,
					},
				},
			},
			err: jsonrpc.Err(jsonrpc.InvalidParams, "resource_bounds is required for this transaction type"),
		},
		{
			name: "deploy account transaction without resource bounds",
			transactions: []rpc.BroadcastedTransaction{
				{
					Transaction: rpc.Transaction{
						Version: &version3,
						Type:    rpc.TxnDeployAccount,
					},
				},
			},
			err: jsonrpc.Err(jsonrpc.InvalidParams, "resource_bounds is required for this transaction type"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockReader := mocks.NewMockReader(mockCtrl)
			mockVM := mocks.NewMockVM(mockCtrl)
			mockState := mocks.NewMockStateHistoryReader(mockCtrl)

			mockReader.EXPECT().Network().Return(n)
			mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
			mockReader.EXPECT().HeadsHeader().Return(headsHeader, nil)

			handler := rpc.New(mockReader, nil, mockVM, n, utils.NewNopZapLogger())

			_, err := handler.SimulateTransactions(
				rpc.BlockID{Latest: true},
				rpc.BroadcastedTransactionInputs{Data: test.transactions},
				[]rpc.SimulationFlag{},
			)
			if test.err != nil {
				require.Equal(t, test.err, err)
				return
			}
			require.Nil(t, err)
		})
	}
}
