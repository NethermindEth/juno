package rpcv9_test

import (
	"encoding/json"
	"errors"
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	rpc "github.com/NethermindEth/juno/rpc/v9"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSimulateTransactions(t *testing.T) {
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
	defaultMockBehavior := func(mockReader *mocks.MockReader, _ *mocks.MockVM, mockState *mocks.MockStateHistoryReader) {
		mockReader.EXPECT().Network().Return(n)
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockReader.EXPECT().HeadsHeader().Return(headsHeader, nil)
	}
	tests := []struct {
		name            string
		stepsUsed       uint64
		err             *jsonrpc.Error
		mockBehavior    func(*mocks.MockReader, *mocks.MockVM, *mocks.MockStateHistoryReader)
		simulationFlags []rpcv6.SimulationFlag
		simulatedTxs    []rpc.SimulatedTransaction
	}{
		{ //nolint:dupl
			name:      "ok with zero values, skip fee",
			stepsUsed: 123,
			mockBehavior: func(mockReader *mocks.MockReader, mockVM *mocks.MockVM, mockState *mocks.MockStateHistoryReader) {
				defaultMockBehavior(mockReader, mockVM, mockState)
				mockVM.EXPECT().Execute([]core.Transaction{}, nil, []*felt.Felt{}, &vm.BlockInfo{
					Header: headsHeader,
				}, mockState, n, true, false, false, true, true).
					Return(vm.ExecutionResults{
						OverallFees:      []*felt.Felt{},
						DataAvailability: []core.DataAvailability{},
						GasConsumed:      []core.GasConsumed{},
						Traces:           []vm.TransactionTrace{},
						NumSteps:         uint64(123),
					}, nil)
			},
			simulationFlags: []rpcv6.SimulationFlag{rpcv6.SkipFeeChargeFlag},
			simulatedTxs:    []rpc.SimulatedTransaction{},
		},
		{ //nolint:dupl
			name:      "ok with zero values, skip validate",
			stepsUsed: 123,
			mockBehavior: func(mockReader *mocks.MockReader, mockVM *mocks.MockVM, mockState *mocks.MockStateHistoryReader) {
				defaultMockBehavior(mockReader, mockVM, mockState)
				mockVM.EXPECT().Execute([]core.Transaction{}, nil, []*felt.Felt{}, &vm.BlockInfo{
					Header: headsHeader,
				}, mockState, n, false, true, false, true, true).
					Return(vm.ExecutionResults{
						OverallFees:      []*felt.Felt{},
						DataAvailability: []core.DataAvailability{},
						GasConsumed:      []core.GasConsumed{},
						Traces:           []vm.TransactionTrace{},
						NumSteps:         uint64(123),
					}, nil)
			},
			simulationFlags: []rpcv6.SimulationFlag{rpcv6.SkipValidateFlag},
			simulatedTxs:    []rpc.SimulatedTransaction{},
		},
		{
			name: "transaction execution error",
			mockBehavior: func(mockReader *mocks.MockReader, mockVM *mocks.MockVM, mockState *mocks.MockStateHistoryReader) {
				defaultMockBehavior(mockReader, mockVM, mockState)
				mockVM.EXPECT().Execute([]core.Transaction{}, nil, []*felt.Felt{}, &vm.BlockInfo{
					Header: headsHeader,
				}, mockState, n, false, true, false, true, true).
					Return(vm.ExecutionResults{}, vm.TransactionExecutionError{
						Index: 44,
						Cause: json.RawMessage("oops"),
					})
			},
			simulationFlags: []rpcv6.SimulationFlag{rpcv6.SkipValidateFlag},
			err: rpccore.ErrTransactionExecutionError.CloneWithData(rpc.TransactionExecutionErrorData{
				TransactionIndex: 44,
				ExecutionError:   json.RawMessage("oops"),
			}),
		},
		{
			name: "inconsistent lengths error",
			mockBehavior: func(mockReader *mocks.MockReader, mockVM *mocks.MockVM, mockState *mocks.MockStateHistoryReader) {
				defaultMockBehavior(mockReader, mockVM, mockState)
				mockVM.EXPECT().Execute([]core.Transaction{}, nil, []*felt.Felt{}, &vm.BlockInfo{
					Header: headsHeader,
				}, mockState, n, false, true, false, true, true).
					Return(vm.ExecutionResults{
						OverallFees:      []*felt.Felt{&felt.Zero},
						DataAvailability: []core.DataAvailability{{L1Gas: 0}, {L1Gas: 0}},
						GasConsumed:      []core.GasConsumed{{L1Gas: 0, L1DataGas: 0, L2Gas: 0}},
						Traces:           []vm.TransactionTrace{{}},
						NumSteps:         uint64(0),
					}, nil)
			},
			simulationFlags: []rpcv6.SimulationFlag{rpcv6.SkipValidateFlag},
			err: rpccore.ErrInternal.CloneWithData(errors.New(
				"inconsistent lengths: 1 overall fees, 1 traces, 1 gas consumed, 2 data availability, 0 txns",
			)),
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

			test.mockBehavior(mockReader, mockVM, mockState)
			handler := rpc.New(mockReader, nil, mockVM, utils.NewNopZapLogger())

			blockID := blockIDLatest(t)
			simulatedTxs, httpHeader, err := handler.SimulateTransactions(
				&blockID,
				[]rpc.BroadcastedTransaction{},
				test.simulationFlags,
			)
			if test.err != nil {
				require.Equal(t, test.err, err)
				require.Nil(t, simulatedTxs)
				return
			}
			require.Nil(t, err)
			require.Equal(
				t,
				httpHeader.Get(rpc.ExecutionStepsHeader),
				strconv.FormatUint(test.stepsUsed, 10),
			)
			require.Equal(t, test.simulatedTxs, simulatedTxs)
		})
	}
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

	version3 := felt.FromUint64(3)

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

			handler := rpc.New(mockReader, nil, mockVM, utils.NewNopZapLogger())

			blockID := blockIDLatest(t)
			_, _, err := handler.SimulateTransactions(
				&blockID,
				test.transactions,
				[]rpcv6.SimulationFlag{},
			)
			if test.err != nil {
				require.Equal(t, test.err, err)
				return
			}
			require.Nil(t, err)
		})
	}
}
