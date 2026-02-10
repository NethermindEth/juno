package rpcv10_test

import (
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
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
	defaultMockBehavior := func(
		mockReader *mocks.MockReader,
		_ *mocks.MockVM,
		mockState *mocks.MockStateHistoryReader,
	) {
		mockReader.EXPECT().Network().Return(n)
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockReader.EXPECT().HeadsHeader().Return(headsHeader, nil)
	}
	tests := []struct {
		name            string
		stepsUsed       uint64
		err             *jsonrpc.Error
		mockBehavior    func(*mocks.MockReader, *mocks.MockVM, *mocks.MockStateHistoryReader)
		simulationFlags []rpcv10.SimulationFlag
		simulatedTxs    []rpcv10.SimulatedTransaction
	}{
		{ //nolint:dupl // test data
			name:      "ok with zero values, skip fee",
			stepsUsed: 123,
			mockBehavior: func(
				mockReader *mocks.MockReader,
				mockVM *mocks.MockVM,
				mockState *mocks.MockStateHistoryReader,
			) {
				defaultMockBehavior(mockReader, mockVM, mockState)
				mockVM.EXPECT().Execute([]core.Transaction{}, nil, []*felt.Felt{}, &vm.BlockInfo{
					Header: headsHeader,
				}, mockState, true, false, false, true, true, false, false).
					Return(vm.ExecutionResults{
						OverallFees:      []*felt.Felt{},
						DataAvailability: []core.DataAvailability{},
						GasConsumed:      []core.GasConsumed{},
						Traces:           []vm.TransactionTrace{},
						NumSteps:         uint64(123),
					}, nil)
			},
			simulationFlags: []rpcv10.SimulationFlag{rpcv10.SkipFeeChargeFlag},
			simulatedTxs:    []rpcv10.SimulatedTransaction{},
		},
		{ //nolint:dupl // test data
			name:      "ok with zero values, skip validate",
			stepsUsed: 123,
			mockBehavior: func(
				mockReader *mocks.MockReader,
				mockVM *mocks.MockVM,
				mockState *mocks.MockStateHistoryReader,
			) {
				defaultMockBehavior(mockReader, mockVM, mockState)
				mockVM.EXPECT().Execute([]core.Transaction{}, nil, []*felt.Felt{}, &vm.BlockInfo{
					Header: headsHeader,
				}, mockState, false, true, false, true, true, false, false).
					Return(vm.ExecutionResults{
						OverallFees:      []*felt.Felt{},
						DataAvailability: []core.DataAvailability{},
						GasConsumed:      []core.GasConsumed{},
						Traces:           []vm.TransactionTrace{},
						NumSteps:         uint64(123),
					}, nil)
			},
			simulationFlags: []rpcv10.SimulationFlag{rpcv10.SkipValidateFlag},
			simulatedTxs:    []rpcv10.SimulatedTransaction{},
		},
		{
			name: "transaction execution error",
			mockBehavior: func(
				mockReader *mocks.MockReader,
				mockVM *mocks.MockVM,
				mockState *mocks.MockStateHistoryReader,
			) {
				defaultMockBehavior(mockReader, mockVM, mockState)
				mockVM.EXPECT().Execute([]core.Transaction{}, nil, []*felt.Felt{}, &vm.BlockInfo{
					Header: headsHeader,
				}, mockState, false, true, false, true, true, false, false).
					Return(vm.ExecutionResults{}, vm.TransactionExecutionError{
						Index: 44,
						Cause: json.RawMessage("oops"),
					})
			},
			simulationFlags: []rpcv10.SimulationFlag{rpcv10.SkipValidateFlag},
			err: rpccore.ErrTransactionExecutionError.CloneWithData(rpcv9.TransactionExecutionErrorData{
				TransactionIndex: 44,
				ExecutionError:   json.RawMessage("oops"),
			}),
		},
		{
			name: "inconsistent lengths error",
			mockBehavior: func(
				mockReader *mocks.MockReader,
				mockVM *mocks.MockVM,
				mockState *mocks.MockStateHistoryReader,
			) {
				defaultMockBehavior(mockReader, mockVM, mockState)
				mockVM.EXPECT().Execute([]core.Transaction{}, nil, []*felt.Felt{}, &vm.BlockInfo{
					Header: headsHeader,
				}, mockState, false, true, false, true, true, false, false).
					Return(vm.ExecutionResults{
						OverallFees:      []*felt.Felt{&felt.Zero},
						DataAvailability: []core.DataAvailability{{L1Gas: 0}, {L1Gas: 0}},
						GasConsumed:      []core.GasConsumed{{L1Gas: 0, L1DataGas: 0, L2Gas: 0}},
						Traces:           []vm.TransactionTrace{{}},
						NumSteps:         uint64(0),
					}, nil)
			},
			simulationFlags: []rpcv10.SimulationFlag{rpcv10.SkipValidateFlag},
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
			handler := rpcv10.New(mockReader, nil, mockVM, utils.NewNopZapLogger())

			blockID := rpcv9.BlockIDLatest()
			simulatedTxs, httpHeader, err := handler.SimulateTransactions(
				t.Context(),
				&blockID,
				rpcv9.BroadcastedTransactionInputs{},
				test.simulationFlags,
			)
			if test.err != nil {
				require.Equal(t, test.err, err)
				require.Nil(t, simulatedTxs.SimulatedTransactions)
				return
			}
			require.Nil(t, err)
			require.Equal(
				t,
				httpHeader.Get(rpcv9.ExecutionStepsHeader),
				strconv.FormatUint(test.stepsUsed, 10),
			)
			require.Equal(t, test.simulatedTxs, simulatedTxs.SimulatedTransactions)
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

	version3 := felt.FromUint64[felt.Felt](3)

	tests := []struct {
		name         string
		transactions []rpcv9.BroadcastedTransaction
		err          *jsonrpc.Error
	}{
		{
			name: "declare transaction without sender address",
			transactions: []rpcv9.BroadcastedTransaction{
				{
					Transaction: rpcv9.Transaction{
						Version: &version3,
						Type:    rpcv9.TxnDeclare,
					},
				},
			},
			err: jsonrpc.Err(
				jsonrpc.InvalidParams,
				"sender_address is required for this transaction type",
			),
		},
		{
			name: "declare transaction without resource bounds",
			transactions: []rpcv9.BroadcastedTransaction{
				{
					Transaction: rpcv9.Transaction{
						Version:       &version3,
						Type:          rpcv9.TxnDeclare,
						SenderAddress: &felt.Zero,
					},
				},
			},
			err: jsonrpc.Err(
				jsonrpc.InvalidParams,
				"resource_bounds is required for this transaction type",
			),
		},
		{
			name: "invoke transaction without sender address",
			transactions: []rpcv9.BroadcastedTransaction{
				{
					Transaction: rpcv9.Transaction{
						Version: &version3,
						Type:    rpcv9.TxnInvoke,
					},
				},
			},
			err: jsonrpc.Err(
				jsonrpc.InvalidParams,
				"sender_address is required for this transaction type",
			),
		},
		{
			name: "invoke transaction without resource bounds",
			transactions: []rpcv9.BroadcastedTransaction{
				{
					Transaction: rpcv9.Transaction{
						Version:       &version3,
						Type:          rpcv9.TxnInvoke,
						SenderAddress: &felt.Zero,
					},
				},
			},
			err: jsonrpc.Err(
				jsonrpc.InvalidParams,
				"resource_bounds is required for this transaction type",
			),
		},
		{
			name: "deploy account transaction without resource bounds",
			transactions: []rpcv9.BroadcastedTransaction{
				{
					Transaction: rpcv9.Transaction{
						Version: &version3,
						Type:    rpcv9.TxnDeployAccount,
					},
				},
			},
			err: jsonrpc.Err(
				jsonrpc.InvalidParams,
				"resource_bounds is required for this transaction type",
			),
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

			handler := rpcv10.New(mockReader, nil, mockVM, utils.NewNopZapLogger())

			blockID := rpcv9.BlockIDLatest()
			_, _, err := handler.SimulateTransactions(
				t.Context(),
				&blockID,
				rpcv9.BroadcastedTransactionInputs{Data: test.transactions},
				[]rpcv10.SimulationFlag{},
			)
			if test.err != nil {
				require.Equal(t, test.err, err)
				return
			}
			require.Nil(t, err)
		})
	}
}

type initialReadsTestCase struct {
	name                 string
	traceFlags           []rpcv10.TraceFlag
	simulationFlags      []rpcv10.SimulationFlag
	initialReads         *vm.InitialReads
	expectedInitialReads *rpcv10.InitialReads
}

// initialReadsTestCases returns the shared test cases for testing RETURN_INITIAL_READS flag.
// These test cases are used by both TestSimulateTransactionsWithReturnInitialReads
// and TestTraceBlockTransactionsWithReturnInitialReads since they test the same
// initial reads behaviour.
func initialReadsTestCases() []initialReadsTestCase {
	addr := felt.FromUint64[felt.Address](123)
	key := felt.FromUint64[felt.Felt](456)
	value := felt.FromUint64[felt.Felt](789)

	return []initialReadsTestCase{
		{
			name:            "with flag and non-empty initial reads",
			traceFlags:      []rpcv10.TraceFlag{rpcv10.TraceReturnInitialReadsFlag},
			simulationFlags: []rpcv10.SimulationFlag{rpcv10.ReturnInitialReadsFlag},
			initialReads: &vm.InitialReads{
				Storage: []vm.InitialReadsStorageEntry{
					{ContractAddress: addr, Key: key, Value: value},
				},
				Nonces:            []vm.InitialReadsNonceEntry{},
				ClassHashes:       []vm.InitialReadsClassHashEntry{},
				DeclaredContracts: []vm.InitialReadsDeclaredContractEntry{},
			},
			expectedInitialReads: &rpcv10.InitialReads{
				Storage:           []rpcv10.StorageEntry{{ContractAddress: addr, Key: key, Value: value}},
				Nonces:            []rpcv10.NonceEntry{},
				ClassHashes:       []rpcv10.ClassHashEntry{},
				DeclaredContracts: []rpcv10.DeclaredContractEntry{},
			},
		},
		{
			name:            "with flag but empty initial reads",
			traceFlags:      []rpcv10.TraceFlag{rpcv10.TraceReturnInitialReadsFlag},
			simulationFlags: []rpcv10.SimulationFlag{rpcv10.ReturnInitialReadsFlag},
			initialReads: &vm.InitialReads{
				Storage:           []vm.InitialReadsStorageEntry{},
				Nonces:            []vm.InitialReadsNonceEntry{},
				ClassHashes:       []vm.InitialReadsClassHashEntry{},
				DeclaredContracts: []vm.InitialReadsDeclaredContractEntry{},
			},
			expectedInitialReads: &rpcv10.InitialReads{
				Storage:           []rpcv10.StorageEntry{},
				Nonces:            []rpcv10.NonceEntry{},
				ClassHashes:       []rpcv10.ClassHashEntry{},
				DeclaredContracts: []rpcv10.DeclaredContractEntry{},
			},
		},
		{
			name:            "without flag",
			traceFlags:      []rpcv10.TraceFlag{},
			simulationFlags: []rpcv10.SimulationFlag{},
			initialReads: &vm.InitialReads{
				Storage: []vm.InitialReadsStorageEntry{
					{ContractAddress: addr, Key: key, Value: value},
				},
				Nonces:            []vm.InitialReadsNonceEntry{},
				ClassHashes:       []vm.InitialReadsClassHashEntry{},
				DeclaredContracts: []vm.InitialReadsDeclaredContractEntry{},
			},
			expectedInitialReads: nil,
		},
		{
			name:                 "with flag but VM returns no initial reads",
			traceFlags:           []rpcv10.TraceFlag{rpcv10.TraceReturnInitialReadsFlag},
			simulationFlags:      []rpcv10.SimulationFlag{rpcv10.ReturnInitialReadsFlag},
			initialReads:         nil,
			expectedInitialReads: nil,
		},
	}
}

func TestSimulateTransactionsWithReturnInitialReads(t *testing.T) {
	t.Parallel()
	n := &utils.Mainnet
	headsHeader := &core.Header{
		SequencerAddress: n.BlockHashMetaInfo.FallBackSequencerAddress,
		L1GasPriceETH:    &felt.Zero,
		L1GasPriceSTRK:   &felt.Zero,
		L1DAMode:         0,
		L1DataGasPrice:   &core.GasPrice{PriceInWei: &felt.Zero, PriceInFri: &felt.Zero},
		L2GasPrice:       &core.GasPrice{PriceInWei: &felt.Zero, PriceInFri: &felt.Zero},
	}

	testCases := initialReadsTestCases()

	for _, test := range testCases {
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

			returnInitialReads := slices.Contains(test.traceFlags, rpcv10.TraceReturnInitialReadsFlag)

			version3 := felt.FromUint64[felt.Felt](3)
			senderAddr := felt.FromUint64[felt.Felt](1)
			nonce := felt.FromUint64[felt.Felt](1)
			broadcastedTxns := []rpcv9.BroadcastedTransaction{{
				Transaction: rpcv9.Transaction{
					Version:       &version3,
					Type:          rpcv9.TxnInvoke,
					SenderAddress: &senderAddr,
					Nonce:         &nonce,
					Tip:           &felt.Zero,
					CallData:      &[]*felt.Felt{},
					Signature:     &[]*felt.Felt{&felt.Zero},
					ResourceBounds: &rpcv9.ResourceBoundsMap{
						L1Gas: &rpcv9.ResourceBounds{
							MaxAmount:       felt.NewFromUint64[felt.Felt](1000),
							MaxPricePerUnit: &felt.Zero,
						},
						L1DataGas: &rpcv9.ResourceBounds{
							MaxAmount:       felt.NewFromUint64[felt.Felt](1000),
							MaxPricePerUnit: &felt.Zero,
						},
						L2Gas: &rpcv9.ResourceBounds{MaxAmount: &felt.Zero, MaxPricePerUnit: &felt.Zero},
					},
				},
			}}

			mockVM.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), mockState,
				false, false, false, true, true, false, returnInitialReads,
			).Return(vm.ExecutionResults{
				OverallFees:      []*felt.Felt{&felt.Zero},
				DataAvailability: []core.DataAvailability{{L1Gas: 0}},
				GasConsumed:      []core.GasConsumed{{L1Gas: 0, L1DataGas: 0, L2Gas: 0}},
				Traces:           []vm.TransactionTrace{{}},
				NumSteps:         100,
				InitialReads:     test.initialReads,
			}, nil)

			handler := rpcv10.New(mockReader, nil, mockVM, utils.NewNopZapLogger())

			blockID := rpcv9.BlockIDLatest()
			simulatedTxs, _, err := handler.SimulateTransactions(
				t.Context(),
				&blockID,
				rpcv9.BroadcastedTransactionInputs{Data: broadcastedTxns},
				test.simulationFlags,
			)
			require.Nil(t, err)

			require.Equal(t, test.expectedInitialReads, simulatedTxs.InitialReads)
		})
	}
}
