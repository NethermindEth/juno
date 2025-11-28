package rpcv10_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
	"github.com/NethermindEth/juno/starknet"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/validator"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type expectedBlockTrace struct {
	blockHash   string
	blockNumber uint64
	wantTrace   []rpcv10.TracedBlockTransaction
}

// readTestData reads a JSON file from the testdata directory
// Returns the content as the specified type T
func readTestData[T any](t *testing.T, path string) T {
	var result T

	// Get the current test file directory using runtime.Caller
	_, testFile, _, ok := runtime.Caller(1)
	require.True(t, ok, "failed to get caller info")

	testDir := filepath.Dir(testFile)
	dataPath := filepath.Join(testDir, "testdata", path)

	content, err := os.ReadFile(dataPath)
	require.NoError(t, err)

	require.NoError(
		t,
		json.Unmarshal(content, &result),
	)

	return result
}

// readTestDataAsString reads a JSON file from the testdata directory
// and returns it as a string. Used for JSON format comparison.
func readTestDataAsString(t *testing.T, path string) string {
	t.Helper()

	_, testFile, _, ok := runtime.Caller(1)
	require.True(t, ok, "failed to get caller info")

	testDir := filepath.Dir(testFile)
	dataPath := filepath.Join(testDir, "testdata", path)

	content, err := os.ReadFile(dataPath)
	require.NoError(t, err)

	// Verify it's valid JSON before returning
	var data interface{}
	require.NoError(t, json.Unmarshal(content, &data), "Expected JSON file to contain valid JSON")

	return string(content)
}

func TestTraceFallback(t *testing.T) {
	t.Run("Goerli Integration", func(t *testing.T) {
		tests := map[string]expectedBlockTrace{
			"old block": {
				blockHash:   "0x3ae41b0f023e53151b0c8ab8b9caafb7005d5f41c9ab260276d5bdc49726279",
				blockNumber: 0,
				wantTrace: readTestData[[]rpcv10.TracedBlockTransaction](
					t,
					"traces/goerli_block_0.json",
				),
			},
			// The newer block still needs to have starknet_version <= 0.13.1 to be fetched from the feeder
			"newer block": {
				blockHash:   "0xe3828bd9154ab385e2cbb95b3b650365fb3c6a4321660d98ce8b0a9194f9a3",
				blockNumber: 300000,
				wantTrace: readTestData[[]rpcv10.TracedBlockTransaction](
					t,
					"traces/goerli_block_300_000.json",
				),
			},
		}

		AssertTracedBlockTransactions(t, &utils.Integration, tests)
	})

	t.Run("Sepolia", func(t *testing.T) {
		tests := map[string]expectedBlockTrace{
			"old block": {
				blockHash:   "0x37644818236ee05b7e3b180bed64ea70ee3dd1553ca334a5c2a290ee276f380",
				blockNumber: 3,
				wantTrace: readTestData[[]rpcv10.TracedBlockTransaction](
					t,
					"traces/sepolia_block_3.json",
				),
			},
			// The newer block still needs to have starknet_version <= 0.13.1 to be fetched from the feeder
			"newer block": {
				blockHash:   "0x733495d0744edd9785b400408fa87c8ad599f81859df544897f80a3fceab422",
				blockNumber: 40000,
				wantTrace: readTestData[[]rpcv10.TracedBlockTransaction](
					t,
					"traces/sepolia_block_40_000.json",
				),
			},
		}

		AssertTracedBlockTransactions(t, &utils.Sepolia, tests)
	})
}

func AssertTracedBlockTransactions(
	t *testing.T,
	n *utils.Network,
	tests map[string]expectedBlockTrace,
) {
	t.Helper()

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	client := feeder.NewTestClient(t, n)
	gateway := adaptfeeder.New(client)

	mockReader := mocks.NewMockReader(mockCtrl)

	mockReader.EXPECT().BlockByNumber(gomock.Any()).
		DoAndReturn(func(number uint64) (block *core.Block, err error) {
			block, err = gateway.BlockByNumber(t.Context(), number)

			// Simulate gas consumption in block receipts
			for _, receipt := range block.Receipts {
				receipt.ExecutionResources.TotalGasConsumed = &core.GasConsumed{
					L1Gas:     5,
					L2Gas:     10,
					L1DataGas: 15,
				}
			}
			return block, err
		}).AnyTimes()

	mockReader.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound).AnyTimes()
	mockReader.EXPECT().Network().Return(n).AnyTimes()

	for description, test := range tests {
		t.Run(description, func(t *testing.T) {
			handler := rpcv10.New(mockReader, nil, nil, nil)
			handler = handler.WithFeeder(client)
			blockID := rpcv9.BlockIDFromNumber(test.blockNumber)
			traces, httpHeader, err := handler.TraceBlockTransactions(t.Context(), &blockID)
			if n == &utils.Sepolia && description == "newer block" {
				// For the newer block test, we test 3 of the block traces (INVOKE, DEPLOY_ACCOUNT, DECLARE)
				traces = []rpcv10.TracedBlockTransaction{traces[0], traces[7], traces[11]}
			}
			require.Nil(t, err)
			require.Equal(t, httpHeader.Get(rpcv9.ExecutionStepsHeader), "0")
			require.Equal(t, test.wantTrace, traces)
		})
	}
}

func TestTraceBlockTransactionsReturnsError(t *testing.T) {
	t.Run("no feeder client set", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		mockReader := mocks.NewMockReader(mockCtrl)

		network := utils.Sepolia
		client := feeder.NewTestClient(t, &network)
		gateway := adaptfeeder.New(client)

		blockNumber := uint64(40000)

		mockReader.EXPECT().BlockByNumber(gomock.Any()).
			DoAndReturn(func(number uint64) (block *core.Block, err error) {
				return gateway.BlockByNumber(t.Context(), number)
			})
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound).AnyTimes()
		mockReader.EXPECT().Network().Return(&network)

		// No feeder client is set
		handler := rpcv10.New(mockReader, nil, nil, nil)

		blockID := rpcv9.BlockIDFromNumber(blockNumber)
		tracedBlocks, httpHeader, err := handler.TraceBlockTransactions(t.Context(), &blockID)

		require.Nil(t, tracedBlocks)
		require.Equal(t, rpccore.ErrInternal.Code, err.Code)
		assert.Equal(t, httpHeader.Get(rpcv9.ExecutionStepsHeader), "0")
	})
}

func TestTransactionTraceValidation(t *testing.T) {
	validInvokeTransactionTrace := rpcv10.TransactionTrace{
		Type:              rpcv9.TxnInvoke,
		ExecuteInvocation: &rpcv9.ExecuteInvocation{},
	}

	invalidInvokeTransactionTrace := rpcv10.TransactionTrace{
		Type: rpcv9.TxnInvoke,
	}

	validDeployAccountTransactionTrace := rpcv10.TransactionTrace{
		Type:                  rpcv9.TxnDeployAccount,
		ConstructorInvocation: &rpcv9.FunctionInvocation{},
	}

	invalidDeployAccountTransactionTrace := rpcv10.TransactionTrace{
		Type: rpcv9.TxnDeployAccount,
	}

	validL1HandlerTransactionTrace := rpcv10.TransactionTrace{
		Type: rpcv9.TxnL1Handler,
		FunctionInvocation: &rpcv9.ExecuteInvocation{
			FunctionInvocation: &rpcv9.FunctionInvocation{},
		},
	}

	validRevertedL1HandlerTransactionTrace := rpcv10.TransactionTrace{
		Type: rpcv9.TxnL1Handler,
		FunctionInvocation: &rpcv9.ExecuteInvocation{
			RevertReason: "Reverted",
		},
	}

	invalidL1HandlerTransactionTrace := rpcv10.TransactionTrace{
		Type: rpcv9.TxnL1Handler,
	}

	tests := []struct {
		name         string
		trace        rpcv10.TransactionTrace
		wantErr      bool
		expectedJSON string // Expected JSON output (empty if validation should fail)
	}{
		{
			name:         "valid INVOKE tx",
			trace:        validInvokeTransactionTrace,
			wantErr:      false,
			expectedJSON: readTestDataAsString(t, "traces/validation/valid_invoke_trace.json"),
		},
		{
			name:         "invalid INVOKE tx",
			trace:        invalidInvokeTransactionTrace,
			wantErr:      true,
			expectedJSON: "",
		},
		{
			name:         "valid DEPLOY_ACCOUNT tx",
			trace:        validDeployAccountTransactionTrace,
			wantErr:      false,
			expectedJSON: readTestDataAsString(t, "traces/validation/valid_deploy_account_trace.json"),
		},
		{
			name:         "invalid DEPLOY_ACCOUNT tx",
			trace:        invalidDeployAccountTransactionTrace,
			wantErr:      true,
			expectedJSON: "",
		},
		{
			name:         "valid L1_HANDLER tx",
			trace:        validL1HandlerTransactionTrace,
			wantErr:      false,
			expectedJSON: readTestDataAsString(t, "traces/validation/valid_l1_handler_trace.json"),
		},
		{
			name:         "valid L1_HANDLER tx reverted",
			trace:        validRevertedL1HandlerTransactionTrace,
			wantErr:      false,
			expectedJSON: readTestDataAsString(t, "traces/validation/valid_l1_handler_reverted_trace.json"),
		},
		{
			name:         "invalid L1_HANDLER tx",
			trace:        invalidL1HandlerTransactionTrace,
			wantErr:      true,
			expectedJSON: "",
		},
	}

	validate := validator.Validator()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validate.Struct(test.trace)

			if test.wantErr {
				assert.Error(t, err, "Expected validation to fail, but it passed")
			} else {
				assert.NoError(t, err, "Expected validation to pass, but it failed")

				jsonBytes, marshalErr := json.Marshal(test.trace)
				require.NoError(t, marshalErr)
				assert.JSONEq(t, test.expectedJSON, string(jsonBytes))
			}
		})
	}
}

func TestTraceTransaction(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	mockReader.EXPECT().Network().Return(&utils.Mainnet).AnyTimes()
	mockVM := mocks.NewMockVM(mockCtrl)
	handler := rpcv10.New(mockReader, mockSyncReader, mockVM, utils.NewNopZapLogger())

	t.Run("not found", func(t *testing.T) {
		t.Run("key not found", func(t *testing.T) {
			hash := felt.NewUnsafeFromString[felt.Felt]("0xBBBB")
			// Receipt() returns error related to db
			mockReader.EXPECT().Receipt(hash).Return(nil, nil, uint64(0), db.ErrKeyNotFound)
			preConfirmed := core.NewPreConfirmed(&core.Block{}, nil, nil, nil)
			mockSyncReader.EXPECT().PendingData().Return(
				&preConfirmed,
				nil,
			)

			trace, httpHeader, err := handler.TraceTransaction(t.Context(), hash)
			assert.Empty(t, trace)
			assert.Equal(t, rpccore.ErrTxnHashNotFound, err)
			assert.Equal(t, httpHeader.Get(rpcv9.ExecutionStepsHeader), "0")
		})

		t.Run("other error", func(t *testing.T) {
			hash := felt.NewUnsafeFromString[felt.Felt]("0xBBBB")
			// Receipt() returns some other error
			mockReader.EXPECT().Receipt(hash).Return(
				nil,
				nil,
				uint64(0),
				errors.New("database error"),
			)

			trace, httpHeader, err := handler.TraceTransaction(t.Context(), hash)
			assert.Empty(t, trace)
			assert.Equal(t, rpccore.ErrTxnHashNotFound, err)
			assert.Equal(t, httpHeader.Get(rpcv9.ExecutionStepsHeader), "0")
		})
	})
	t.Run("ok", func(t *testing.T) {
		hash := felt.NewUnsafeFromString[felt.Felt](
			"0x37b244ea7dc6b3f9735fba02d183ef0d6807a572dd91a63cc1b14b923c1ac0",
		)
		tx := &core.DeclareTransaction{
			TransactionHash: hash,
			ClassHash:       felt.NewUnsafeFromString[felt.Felt]("0x000000000"),
			Version:         new(core.TransactionVersion).SetUint64(1),
		}

		header := &core.Header{
			Hash:             felt.NewUnsafeFromString[felt.Felt]("0xCAFEBABE"),
			ParentHash:       felt.NewUnsafeFromString[felt.Felt]("0x0"),
			SequencerAddress: felt.NewUnsafeFromString[felt.Felt]("0X111"),
			L1GasPriceETH:    felt.NewUnsafeFromString[felt.Felt]("0x1"),
			ProtocolVersion:  "99.12.3",
			L1DAMode:         core.Calldata,
		}
		block := &core.Block{
			Header:       header,
			Transactions: []core.Transaction{tx},
		}
		declaredClass := &core.DeclaredClassDefinition{
			At:    3002,
			Class: &core.SierraClass{},
		}

		mockReader.EXPECT().Receipt(hash).Return(nil, header.Hash, header.Number, nil)
		mockReader.EXPECT().BlockByHash(header.Hash).Return(block, nil)

		mockReader.EXPECT().StateAtBlockHash(header.ParentHash).Return(nil, nopCloser, nil)
		headState := mocks.NewMockStateHistoryReader(mockCtrl)
		headState.EXPECT().Class(tx.ClassHash).Return(declaredClass, nil)
		mockReader.EXPECT().HeadState().Return(headState, nopCloser, nil)

		vmTrace := readTestData[vm.TransactionTrace](t, "traces/vm_transaction_trace.json")

		gc := []core.GasConsumed{{L1Gas: 2, L1DataGas: 3, L2Gas: 4}}
		overallFee := []*felt.Felt{felt.NewFromUint64[felt.Felt](1)}

		stepsUsed := uint64(123)
		stepsUsedStr := "123"

		mockVM.EXPECT().Execute(
			[]core.Transaction{tx},
			[]core.ClassDefinition{declaredClass.Class},
			[]*felt.Felt{},
			&vm.BlockInfo{Header: header},
			gomock.Any(),
			false,
			false,
			false,
			true,
			false,
			false).Return(vm.ExecutionResults{
			OverallFees: overallFee,
			GasConsumed: gc,
			Traces:      []vm.TransactionTrace{vmTrace},
			NumSteps:    stepsUsed,
		}, nil)

		trace, httpHeader, rpcErr := handler.TraceTransaction(t.Context(), hash)
		require.Nil(t, rpcErr)
		assert.Equal(t, httpHeader.Get(rpcv9.ExecutionStepsHeader), stepsUsedStr)

		vmTrace.ExecutionResources = &vm.ExecutionResources{
			L1Gas:     2,
			L1DataGas: 3,
			L2Gas:     4,
		}
		assert.Equal(t, rpcv10.AdaptVMTransactionTrace(&vmTrace), trace)
	})

	t.Run("pending block - starknet version < 0.14.0", func(t *testing.T) {
		hash := felt.NewUnsafeFromString[felt.Felt]("0xceb6a374aff2bbb3537cf35f50df8634b2354a21")
		tx := &core.DeclareTransaction{
			TransactionHash: hash,
			ClassHash:       felt.NewUnsafeFromString[felt.Felt]("0x000000000"),
			Version:         new(core.TransactionVersion).SetUint64(1),
		}

		header := &core.Header{
			ParentHash:       felt.NewUnsafeFromString[felt.Felt]("0x0"),
			SequencerAddress: felt.NewUnsafeFromString[felt.Felt]("0X111"),
			ProtocolVersion:  "99.12.3",
			L1DAMode:         core.Calldata,
			L1GasPriceETH:    felt.NewUnsafeFromString[felt.Felt]("0x1"),
		}
		require.Nil(t, header.Hash, "hash must be nil for pending block")

		block := &core.Block{
			Header:       header,
			Transactions: []core.Transaction{tx},
		}
		declaredClass := &core.DeclaredClassDefinition{
			At:    3002,
			Class: &core.SierraClass{},
		}

		mockReader.EXPECT().Receipt(hash).Return(nil, nil, uint64(0), db.ErrKeyNotFound)
		pendingStateDiff := core.EmptyStateDiff()
		pending := core.Pending{
			Block: block,
			StateUpdate: &core.StateUpdate{
				StateDiff: &pendingStateDiff,
			},
			NewClasses: map[felt.Felt]core.ClassDefinition{*tx.ClassHash: declaredClass.Class},
		}
		mockSyncReader.EXPECT().PendingData().Return(
			&pending,
			nil,
		).Times(2)
		headState := mocks.NewMockStateHistoryReader(mockCtrl)
		mockReader.EXPECT().StateAtBlockHash(header.ParentHash).
			Return(headState, nopCloser, nil).Times(2)

		vmTrace := readTestData[vm.TransactionTrace](t, "traces/vm_transaction_trace.json")

		gc := []core.GasConsumed{{L1Gas: 2, L1DataGas: 3, L2Gas: 4}}
		overallFee := []*felt.Felt{felt.NewFromUint64[felt.Felt](1)}

		stepsUsed := uint64(123)
		stepsUsedStr := "123"

		mockVM.EXPECT().Execute(
			[]core.Transaction{tx},
			[]core.ClassDefinition{declaredClass.Class},
			[]*felt.Felt{},
			&vm.BlockInfo{Header: header},
			gomock.Any(),
			false,
			false,
			false,
			true,
			false,
			false).
			Return(vm.ExecutionResults{
				OverallFees: overallFee,
				GasConsumed: gc,
				Traces:      []vm.TransactionTrace{vmTrace},
				NumSteps:    stepsUsed,
			}, nil)

		trace, httpHeader, rpcErr := handler.TraceTransaction(t.Context(), hash)
		require.Nil(t, rpcErr)
		assert.Equal(t, httpHeader.Get(rpcv9.ExecutionStepsHeader), stepsUsedStr)

		vmTrace.ExecutionResources = &vm.ExecutionResources{
			L1Gas:     2,
			L1DataGas: 3,
			L2Gas:     4,
		}
		assert.Equal(t, rpcv10.AdaptVMTransactionTrace(&vmTrace), trace)
	})

	t.Run("pre_confirmed block", func(t *testing.T) {
		hash := felt.NewUnsafeFromString[felt.Felt]("0xceb6a374aff2bbb3537cf35f50df8634b2354a21")
		tx := &core.InvokeTransaction{
			TransactionHash: hash,
			Version:         new(core.TransactionVersion).SetUint64(1),
		}

		header := &core.Header{
			Number:           1,
			SequencerAddress: felt.NewUnsafeFromString[felt.Felt]("0X111"),
			ProtocolVersion:  "99.12.3",
			L1DAMode:         core.Calldata,
			L1GasPriceETH:    felt.NewUnsafeFromString[felt.Felt]("0x1"),
		}
		require.Nil(t, header.Hash, "hash must be nil for pre_confirmed block")
		require.Nil(t, header.ParentHash, "ParentHash must be nil for pre_confirmed block")

		block := &core.Block{
			Header:       header,
			Transactions: []core.Transaction{tx},
		}

		mockReader.EXPECT().Receipt(hash).Return(nil, nil, uint64(0), db.ErrKeyNotFound)
		preConfirmedStateDiff := core.EmptyStateDiff()
		preConfirmed := core.PreConfirmed{
			Block: block,
			StateUpdate: &core.StateUpdate{
				StateDiff: &preConfirmedStateDiff,
			},
		}
		mockSyncReader.EXPECT().PendingData().Return(
			&preConfirmed,
			nil,
		)
		headState := mocks.NewMockStateHistoryReader(mockCtrl)
		mockReader.EXPECT().StateAtBlockNumber(header.Number-1).
			Return(headState, nopCloser, nil)

		vmTrace := readTestData[vm.TransactionTrace](t, "traces/vm_transaction_trace.json")

		gc := []core.GasConsumed{{L1Gas: 2, L1DataGas: 3, L2Gas: 4}}
		overallFee := []*felt.Felt{felt.NewFromUint64[felt.Felt](1)}

		stepsUsed := uint64(123)
		stepsUsedStr := "123"

		mockVM.EXPECT().Execute(
			[]core.Transaction{tx},
			nil,
			[]*felt.Felt{},
			&vm.BlockInfo{Header: header},
			gomock.Any(),
			false,
			false,
			false,
			true,
			false,
			false,
		).
			Return(vm.ExecutionResults{
				OverallFees: overallFee,
				GasConsumed: gc,
				Traces:      []vm.TransactionTrace{vmTrace},
				NumSteps:    stepsUsed,
			}, nil)

		trace, httpHeader, rpcErr := handler.TraceTransaction(t.Context(), hash)
		require.Nil(t, rpcErr)
		assert.Equal(t, httpHeader.Get(rpcv9.ExecutionStepsHeader), stepsUsedStr)

		vmTrace.ExecutionResources = &vm.ExecutionResources{
			L1Gas:     2,
			L1DataGas: 3,
			L2Gas:     4,
		}
		assert.Equal(t, rpcv10.AdaptVMTransactionTrace(&vmTrace), trace)
	})

	t.Run("pre_latest block", func(t *testing.T) {
		hash := felt.NewUnsafeFromString[felt.Felt]("0xceb6a374aff2bbb3537cf35f50df8634b2354a21")
		tx := &core.DeclareTransaction{
			TransactionHash: hash,
			ClassHash:       felt.NewUnsafeFromString[felt.Felt]("0x000000000"),
			Version:         new(core.TransactionVersion).SetUint64(1),
		}

		header := &core.Header{
			Number:           1,
			ParentHash:       felt.NewUnsafeFromString[felt.Felt]("0xFFFF"),
			SequencerAddress: felt.NewUnsafeFromString[felt.Felt]("0X111"),
			ProtocolVersion:  "99.12.3",
			L1DAMode:         core.Calldata,
			L1GasPriceETH:    felt.NewUnsafeFromString[felt.Felt]("0x1"),
		}
		require.Nil(t, header.Hash, "hash must be nil for pre_latest block")
		require.NotNil(t, header.ParentHash, "ParentHash must be nil for pre_latest block")

		block := &core.Block{
			Header:       header,
			Transactions: []core.Transaction{tx},
		}

		declaredClass := &core.DeclaredClassDefinition{
			At:    3002,
			Class: &core.SierraClass{},
		}
		preLatestStateDiff := core.EmptyStateDiff()
		preLatest := core.PreLatest{
			Block: block,
			StateUpdate: &core.StateUpdate{
				StateDiff: &preLatestStateDiff,
			},
			NewClasses: map[felt.Felt]core.ClassDefinition{*tx.ClassHash: declaredClass.Class},
		}

		preConfirmed := core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number: preLatest.Block.Number + 1,
				},
			},
		}
		mockSyncReader.EXPECT().PendingData().Return(
			preConfirmed.WithPreLatest(&preLatest),
			nil,
		)
		mockReader.EXPECT().Receipt(hash).Return(nil, nil, uint64(0), db.ErrKeyNotFound)
		headState := mocks.NewMockStateHistoryReader(mockCtrl)
		mockReader.EXPECT().StateAtBlockHash(preLatest.Block.ParentHash).
			Return(headState, nopCloser, nil)

		vmTrace := readTestData[vm.TransactionTrace](t, "traces/vm_transaction_trace.json")

		gc := []core.GasConsumed{{L1Gas: 2, L1DataGas: 3, L2Gas: 4}}
		overallFee := []*felt.Felt{felt.NewFromUint64[felt.Felt](1)}

		stepsUsed := uint64(123)
		stepsUsedStr := "123"

		mockVM.EXPECT().Execute(
			[]core.Transaction{tx},
			[]core.ClassDefinition{declaredClass.Class},
			[]*felt.Felt{},
			&vm.BlockInfo{Header: header},
			gomock.Any(),
			false,
			false,
			false,
			true,
			false,
			false,
		).
			Return(vm.ExecutionResults{
				OverallFees: overallFee,
				GasConsumed: gc,
				Traces:      []vm.TransactionTrace{vmTrace},
				NumSteps:    stepsUsed,
			}, nil)

		trace, httpHeader, rpcErr := handler.TraceTransaction(t.Context(), hash)
		require.Nil(t, rpcErr)
		assert.Equal(t, httpHeader.Get(rpcv9.ExecutionStepsHeader), stepsUsedStr)

		vmTrace.ExecutionResources = &vm.ExecutionResources{
			L1Gas:     2,
			L1DataGas: 3,
			L2Gas:     4,
		}
		assert.Equal(t, rpcv10.AdaptVMTransactionTrace(&vmTrace), trace)
	})

	t.Run("reverted INVOKE tx from feeder", func(t *testing.T) {
		n := &utils.Sepolia

		handler := rpcv10.New(mockReader, mockSyncReader, mockVM, utils.NewNopZapLogger())

		client := feeder.NewTestClient(t, n)
		handler.WithFeeder(client)
		gateway := adaptfeeder.New(client)

		// Tx at index 3 in the block
		revertedTxHash := felt.NewUnsafeFromString[felt.Felt](
			"0x2f00c7f28df2197196440747f97baa63d0851e3b0cfc2efedb6a88a7ef78cb1",
		)

		blockNumber := uint64(18)
		blockHash := felt.NewUnsafeFromString[felt.Felt](
			"0x5beb56c7d9a9fc066e695c3fc467f45532cace83d9979db4ccfd6b77ca476af",
		)

		mockReader.EXPECT().Receipt(revertedTxHash).Return(nil, blockHash, blockNumber, nil)
		mockReader.EXPECT().BlockByHash(blockHash).
			DoAndReturn(func(_ *felt.Felt) (block *core.Block, err error) {
				return gateway.BlockByNumber(t.Context(), blockNumber)
			})

		expectedRevertedTrace := readTestData[rpcv10.TransactionTrace](
			t,
			"traces/reverted_invoke_trace.json",
		)

		trace, httpHeader, err := handler.TraceTransaction(t.Context(), revertedTxHash)

		require.Nil(t, err)
		assert.Equal(t, "0", httpHeader.Get(rpcv9.ExecutionStepsHeader))
		assert.Equal(t, expectedRevertedTrace, trace)
	})
}

func TestTraceBlockTransactions(t *testing.T) {
	errTests := map[string]rpcv9.BlockID{
		"latest":        rpcv9.BlockIDLatest(),
		"hash":          rpcv9.BlockIDFromHash(felt.NewFromUint64[felt.Felt](1)),
		"number":        rpcv9.BlockIDFromNumber(2),
		"pre_confirmed": rpcv9.BlockIDPreConfirmed(),
		"l1_accepted":   rpcv9.BlockIDL1Accepted(),
	}

	for description, blockID := range errTests {
		t.Run(description, func(t *testing.T) {
			log := utils.NewNopZapLogger()
			n := &utils.Mainnet
			chain := blockchain.New(memory.New(), n)
			handler := rpcv10.New(chain, nil, nil, log)

			if description == "pre_confirmed" {
				mockCtrl := gomock.NewController(t)
				t.Cleanup(mockCtrl.Finish)

				update, httpHeader, rpcErr := handler.TraceBlockTransactions(t.Context(), &blockID)
				assert.Nil(t, update)
				assert.Equal(t, "0", httpHeader.Get(rpcv9.ExecutionStepsHeader))
				assert.Equal(t, rpccore.ErrCallOnPreConfirmed, rpcErr)
			} else {
				update, httpHeader, rpcErr := handler.TraceBlockTransactions(t.Context(), &blockID)
				assert.Nil(t, update)
				assert.Equal(t, "0", httpHeader.Get(rpcv9.ExecutionStepsHeader))
				assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
			}
		})
	}

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	n := &utils.Mainnet

	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	mockReader.EXPECT().Network().Return(n).AnyTimes()
	mockVM := mocks.NewMockVM(mockCtrl)
	log := utils.NewNopZapLogger()

	handler := rpcv10.New(mockReader, mockSyncReader, mockVM, log)

	t.Run("regular block", func(t *testing.T) {
		blockHash := felt.NewUnsafeFromString[felt.Felt](
			"0x37b244ea7dc6b3f9735fba02d183ef0d6807a572dd91a63cc1b14b923c1ac0",
		)
		tx := &core.DeclareTransaction{
			TransactionHash: felt.NewUnsafeFromString[felt.Felt]("0x000000001"),
			ClassHash:       felt.NewUnsafeFromString[felt.Felt]("0x000000000"),
		}

		header := &core.Header{
			Hash:             blockHash,
			ParentHash:       felt.NewUnsafeFromString[felt.Felt]("0x0"),
			Number:           0,
			SequencerAddress: felt.NewUnsafeFromString[felt.Felt]("0X111"),
			L1GasPriceETH:    felt.NewUnsafeFromString[felt.Felt]("0x777"),
			ProtocolVersion:  "99.12.3",
		}
		block := &core.Block{
			Header:       header,
			Transactions: []core.Transaction{tx},
		}
		declaredClass := &core.DeclaredClassDefinition{
			At:    3002,
			Class: &core.SierraClass{},
		}

		mockReader.EXPECT().BlockByHash(blockHash).Return(block, nil)

		mockReader.EXPECT().StateAtBlockHash(header.ParentHash).Return(nil, nopCloser, nil)
		headState := mocks.NewMockStateHistoryReader(mockCtrl)
		headState.EXPECT().Class(tx.ClassHash).Return(declaredClass, nil)
		mockReader.EXPECT().HeadState().Return(headState, nopCloser, nil)

		vmTrace := readTestData[vm.TransactionTrace](t, "traces/vm_block_trace.json")

		stepsUsed := uint64(123)
		stepsUsedStr := "123"

		mockVM.EXPECT().Execute(
			[]core.Transaction{tx},
			[]core.ClassDefinition{declaredClass.Class},
			[]*felt.Felt{},
			&vm.BlockInfo{Header: header},
			gomock.Any(),
			false,
			false,
			false,
			true,
			false,
			false).Return(vm.ExecutionResults{
			OverallFees:      nil,
			DataAvailability: []core.DataAvailability{{}, {}},
			GasConsumed:      []core.GasConsumed{{}, {}},
			Traces:           []vm.TransactionTrace{vmTrace},
			NumSteps:         stepsUsed,
		}, nil)

		expectedTrace := rpcv10.AdaptVMTransactionTrace(&vmTrace)
		expectedResult := []rpcv10.TracedBlockTransaction{
			{
				TransactionHash: tx.Hash(),
				TraceRoot:       &expectedTrace,
			},
		}

		blockID := rpcv9.BlockIDFromHash(blockHash)
		result, httpHeader, rpcErr := handler.TraceBlockTransactions(t.Context(), &blockID)
		require.Nil(t, rpcErr)
		assert.Equal(t, httpHeader.Get(rpcv9.ExecutionStepsHeader), stepsUsedStr)
		assert.Equal(t, expectedResult, result)
	})
}

func TestAdaptVMTransactionTrace(t *testing.T) {
	t.Run("successfully adapt INVOKE trace from vm", func(t *testing.T) {
		fromAddr := felt.NewUnsafeFromString[felt.Felt](
			"0x4c5772d1914fe6ce891b64eb35bf3522aeae1315647314aac58b01137607f3f",
		)
		toAddr := felt.NewUnsafeFromString[felt.Address](
			"0x540552aae708306346466633036396334303062342d24292eadbdc777db86e5",
		)

		payload0 := &felt.Zero
		payload1 := felt.NewUnsafeFromString[felt.Felt]("0x5ba586f822ce9debae27fa04a3e71721fdc90ff")
		payload2 := felt.NewFromUint64[felt.Felt](0x455448)
		payload3 := felt.NewFromUint64[felt.Felt](0x31da07977d000)
		payload4 := &felt.Zero

		vmTrace := vm.TransactionTrace{
			Type: vm.TxnInvoke,
			ValidateInvocation: &vm.FunctionInvocation{
				Messages: []vm.OrderedL2toL1Message{
					{
						Order: 0,
						From:  fromAddr,
						To:    toAddr,
						Payload: []*felt.Felt{
							payload0,
							payload1,
							payload2,
							payload3,
							payload4,
						},
					},
				},
				ExecutionResources: &vm.ExecutionResources{
					L1Gas:     1,
					L1DataGas: 2,
					L2Gas:     3,
					ComputationResources: vm.ComputationResources{
						Steps:        1,
						MemoryHoles:  2,
						Pedersen:     3,
						RangeCheck:   4,
						Bitwise:      5,
						Ecdsa:        6,
						EcOp:         7,
						Keccak:       8,
						Poseidon:     9,
						SegmentArena: 10,
						AddMod:       11,
						MulMod:       12,
						RangeCheck96: 13,
						Output:       14,
					},
					DataAvailability: &vm.DataAvailability{
						L1Gas:     1,
						L1DataGas: 2,
					},
				},
			},
			FeeTransferInvocation: &vm.FunctionInvocation{},
			ExecuteInvocation: &vm.ExecuteInvocation{
				RevertReason:       "",
				FunctionInvocation: &vm.FunctionInvocation{},
			},
			ConstructorInvocation: &vm.FunctionInvocation{},
			FunctionInvocation:    &vm.ExecuteInvocation{},
			StateDiff: &vm.StateDiff{ //nolint:dupl // false positive with rpcv10.StateDiff
				StorageDiffs: []vm.StorageDiff{
					{
						Address: felt.Zero,
						StorageEntries: []vm.Entry{
							{
								Key:   felt.Zero,
								Value: felt.Zero,
							},
						},
					},
				},
				Nonces: []vm.Nonce{
					{
						ContractAddress: felt.Zero,
						Nonce:           felt.Zero,
					},
				},
				DeployedContracts: []vm.DeployedContract{
					{
						Address:   felt.Zero,
						ClassHash: felt.Zero,
					},
				},
				DeprecatedDeclaredClasses: []*felt.Felt{
					&felt.Zero,
				},
				DeclaredClasses: []vm.DeclaredClass{
					{
						ClassHash:         felt.Zero,
						CompiledClassHash: felt.Zero,
					},
				},
				ReplacedClasses: []vm.ReplacedClass{
					{
						ContractAddress: felt.Zero,
						ClassHash:       felt.Zero,
					},
				},
				MigratedCompiledClasses: []vm.MigratedCompiledClass{
					{
						ClassHash:         felt.SierraClassHash(felt.Zero),
						CompiledClassHash: felt.CasmClassHash(felt.Zero),
					},
				},
			},
		}

		expectedAdaptedTrace := rpcv10.TransactionTrace{
			Type: rpcv9.TxnInvoke,
			ValidateInvocation: &rpcv9.FunctionInvocation{
				Calls:  []rpcv9.FunctionInvocation{},
				Events: []rpcv6.OrderedEvent{},
				Messages: []rpcv6.OrderedL2toL1Message{
					{
						Order: 0,
						From:  fromAddr,
						// todo(rdr): we shouldn't need this conversion but the right fix is
						//            refactor which is a whole stream of work on itself
						To: (*felt.Felt)(toAddr),
						Payload: []*felt.Felt{
							payload0,
							payload1,
							payload2,
							payload3,
							payload4,
						},
					},
				},
				ExecutionResources: &rpcv9.InnerExecutionResources{
					L1Gas: 1,
					L2Gas: 3,
				},
				IsReverted: false,
			},
			FeeTransferInvocation: &rpcv9.FunctionInvocation{
				Calls:      []rpcv9.FunctionInvocation{},
				Events:     []rpcv6.OrderedEvent{},
				Messages:   []rpcv6.OrderedL2toL1Message{},
				IsReverted: false,
			},
			ExecuteInvocation: &rpcv9.ExecuteInvocation{
				RevertReason: "",
				FunctionInvocation: &rpcv9.FunctionInvocation{
					Calls:      []rpcv9.FunctionInvocation{},
					Events:     []rpcv6.OrderedEvent{},
					Messages:   []rpcv6.OrderedL2toL1Message{},
					IsReverted: false,
				},
			},
			StateDiff: &rpcv10.StateDiff{ //nolint:dupl // false positive with vm.StateDiff
				StorageDiffs: []rpcv6.StorageDiff{
					{
						Address: felt.Zero,
						StorageEntries: []rpcv6.Entry{
							{
								Key:   felt.Zero,
								Value: felt.Zero,
							},
						},
					},
				},
				Nonces: []rpcv6.Nonce{
					{
						ContractAddress: felt.Zero,
						Nonce:           felt.Zero,
					},
				},
				DeployedContracts: []rpcv6.DeployedContract{
					{
						Address:   felt.Zero,
						ClassHash: felt.Zero,
					},
				},
				DeprecatedDeclaredClasses: []*felt.Felt{
					&felt.Zero,
				},
				DeclaredClasses: []rpcv6.DeclaredClass{
					{
						ClassHash:         felt.Zero,
						CompiledClassHash: felt.Zero,
					},
				},
				ReplacedClasses: []rpcv6.ReplacedClass{
					{
						ContractAddress: felt.Zero,
						ClassHash:       felt.Zero,
					},
				},
				MigratedCompiledClasses: []rpcv10.MigratedCompiledClass{
					{
						ClassHash:         felt.SierraClassHash(felt.Zero),
						CompiledClassHash: felt.CasmClassHash(felt.Zero),
					},
				},
			},
		}

		assert.Equal(t, expectedAdaptedTrace, rpcv10.AdaptVMTransactionTrace(&vmTrace))
	})

	t.Run("successfully adapt DEPLOY_ACCOUNT tx from vm", func(t *testing.T) {
		vmTrace := vm.TransactionTrace{
			Type:                  vm.TxnDeployAccount,
			ValidateInvocation:    &vm.FunctionInvocation{},
			FeeTransferInvocation: &vm.FunctionInvocation{},
			ExecuteInvocation: &vm.ExecuteInvocation{
				RevertReason:       "",
				FunctionInvocation: &vm.FunctionInvocation{},
			},
			ConstructorInvocation: &vm.FunctionInvocation{},
			FunctionInvocation:    &vm.ExecuteInvocation{},
		}

		expectedAdaptedTrace := rpcv10.TransactionTrace{
			Type: rpcv9.TxnDeployAccount,
			ValidateInvocation: &rpcv9.FunctionInvocation{
				Calls:    []rpcv9.FunctionInvocation{},
				Events:   []rpcv6.OrderedEvent{},
				Messages: []rpcv6.OrderedL2toL1Message{},
			},
			FeeTransferInvocation: &rpcv9.FunctionInvocation{
				Calls:    []rpcv9.FunctionInvocation{},
				Events:   []rpcv6.OrderedEvent{},
				Messages: []rpcv6.OrderedL2toL1Message{},
			},
			ConstructorInvocation: &rpcv9.FunctionInvocation{
				Calls:    []rpcv9.FunctionInvocation{},
				Events:   []rpcv6.OrderedEvent{},
				Messages: []rpcv6.OrderedL2toL1Message{},
			},
		}

		adaptedTrace := rpcv10.AdaptVMTransactionTrace(&vmTrace)

		require.Equal(t, expectedAdaptedTrace, adaptedTrace)
	})

	t.Run("successfully adapt L1_HANDLER tx from vm", func(t *testing.T) {
		vmTrace := vm.TransactionTrace{
			Type:                  vm.TxnL1Handler,
			ValidateInvocation:    &vm.FunctionInvocation{},
			FeeTransferInvocation: &vm.FunctionInvocation{},
			ExecuteInvocation: &vm.ExecuteInvocation{
				RevertReason:       "",
				FunctionInvocation: &vm.FunctionInvocation{},
			},
			ConstructorInvocation: &vm.FunctionInvocation{},
			FunctionInvocation: &vm.ExecuteInvocation{
				FunctionInvocation: &vm.FunctionInvocation{},
			},
		}

		expectedAdaptedTrace := rpcv10.TransactionTrace{
			Type: rpcv9.TxnL1Handler,
			FunctionInvocation: &rpcv9.ExecuteInvocation{
				RevertReason: "",
				FunctionInvocation: &rpcv9.FunctionInvocation{
					Calls:      []rpcv9.FunctionInvocation{},
					Events:     []rpcv6.OrderedEvent{},
					Messages:   []rpcv6.OrderedL2toL1Message{},
					IsReverted: false,
				},
			},
		}

		adaptedTrace := rpcv10.AdaptVMTransactionTrace(&vmTrace)

		require.Equal(t, expectedAdaptedTrace, adaptedTrace)
	})
}

func TestAdaptFeederBlockTrace(t *testing.T) {
	t.Run("nil block trace", func(t *testing.T) {
		block := &core.Block{}

		res, err := rpcv10.AdaptFeederBlockTrace(block, nil)
		require.Nil(t, res)
		require.Nil(t, err)
	})

	t.Run("inconsistent block and blockTrace", func(t *testing.T) {
		block := &core.Block{
			Transactions: []core.Transaction{&core.InvokeTransaction{}},
		}
		blockTrace := &starknet.BlockTrace{}

		res, err := rpcv10.AdaptFeederBlockTrace(block, blockTrace)
		require.Nil(t, res)
		require.Equal(t, errors.New("mismatched number of txs and traces"), err)
	})

	t.Run("L1_HANDLER tx gets successfully adapted", func(t *testing.T) {
		block := &core.Block{
			Transactions: []core.Transaction{&core.L1HandlerTransaction{}},
		}
		blockTrace := &starknet.BlockTrace{
			Traces: []starknet.TransactionTrace{
				{
					TransactionHash:       *felt.NewFromUint64[felt.Felt](1),
					FeeTransferInvocation: &starknet.FunctionInvocation{},
					ValidateInvocation:    &starknet.FunctionInvocation{},
					FunctionInvocation: &starknet.FunctionInvocation{
						Events: []starknet.OrderedEvent{{
							Order: 1,
							Keys:  []felt.Felt{*felt.NewFromUint64[felt.Felt](2)},
							Data:  []felt.Felt{*felt.NewFromUint64[felt.Felt](3)},
						}},
						ExecutionResources: starknet.ExecutionResources{
							TotalGasConsumed: &starknet.GasConsumed{
								L1Gas:     10,
								L2Gas:     11,
								L1DataGas: 12,
							},
						},
					},
				},
			},
		}

		expectedAdaptedTrace := []rpcv10.TracedBlockTransaction{
			{
				TransactionHash: felt.NewFromUint64[felt.Felt](1),
				TraceRoot: &rpcv10.TransactionTrace{
					Type: rpcv9.TxnL1Handler,
					FunctionInvocation: &rpcv9.ExecuteInvocation{
						RevertReason: "",
						FunctionInvocation: &rpcv9.FunctionInvocation{
							Calls: []rpcv9.FunctionInvocation{},
							Events: []rpcv6.OrderedEvent{{
								Order: 1,
								Keys:  []*felt.Felt{felt.NewFromUint64[felt.Felt](2)},
								Data:  []*felt.Felt{felt.NewFromUint64[felt.Felt](3)},
							}},
							Messages: []rpcv6.OrderedL2toL1Message{},
							ExecutionResources: &rpcv9.InnerExecutionResources{
								L1Gas: 10,
								L2Gas: 11,
							},
						},
					},
				},
			},
		}

		res, err := rpcv10.AdaptFeederBlockTrace(block, blockTrace)
		require.Nil(t, err)
		require.Equal(t, expectedAdaptedTrace, res)
	})

	t.Run("INVOKE tx gets successfully adapted (with revert error)", func(t *testing.T) {
		block := &core.Block{
			Transactions: []core.Transaction{&core.InvokeTransaction{}},
		}
		blockTrace := &starknet.BlockTrace{
			Traces: []starknet.TransactionTrace{
				{
					TransactionHash:       *felt.NewFromUint64[felt.Felt](1),
					FeeTransferInvocation: &starknet.FunctionInvocation{},
					ValidateInvocation:    &starknet.FunctionInvocation{},
					// When revert error, feeder trace has no FunctionInvocation only RevertError is set
					RevertError: "some error",
				},
			},
		}

		expectedAdaptedTrace := []rpcv10.TracedBlockTransaction{
			{
				TransactionHash: felt.NewFromUint64[felt.Felt](1),
				TraceRoot: &rpcv10.TransactionTrace{
					Type: rpcv9.TxnInvoke,
					FeeTransferInvocation: &rpcv9.FunctionInvocation{
						Calls:              []rpcv9.FunctionInvocation{},
						Events:             []rpcv6.OrderedEvent{},
						Messages:           []rpcv6.OrderedL2toL1Message{},
						ExecutionResources: &rpcv9.InnerExecutionResources{},
					},
					ValidateInvocation: &rpcv9.FunctionInvocation{
						Calls:              []rpcv9.FunctionInvocation{},
						Events:             []rpcv6.OrderedEvent{},
						Messages:           []rpcv6.OrderedL2toL1Message{},
						ExecutionResources: &rpcv9.InnerExecutionResources{},
					},
					ExecuteInvocation: &rpcv9.ExecuteInvocation{
						RevertReason: "some error",
					},
				},
			},
		}

		res, err := rpcv10.AdaptFeederBlockTrace(block, blockTrace)
		require.Nil(t, err)
		require.Equal(t, expectedAdaptedTrace, res)
	})
}

func TestCall(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockVM := mocks.NewMockVM(mockCtrl)
	handler := rpcv10.New(mockReader, nil, mockVM, utils.NewNopZapLogger())

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		blockID := rpcv9.BlockIDLatest()
		res, rpcErr := handler.Call(&rpcv9.FunctionCall{}, &blockID)
		require.Nil(t, res)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(nil, nil, db.ErrKeyNotFound)

		blockID := rpcv9.BlockIDFromHash(&felt.Zero)
		res, rpcErr := handler.Call(&rpcv9.FunctionCall{}, &blockID)
		require.Nil(t, res)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(nil, nil, db.ErrKeyNotFound)

		blockID := rpcv9.BlockIDFromNumber(0)
		res, rpcErr := handler.Call(&rpcv9.FunctionCall{}, &blockID)
		require.Nil(t, res)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	mockState := mocks.NewMockStateHistoryReader(mockCtrl)

	t.Run("call - unknown contract", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockReader.EXPECT().HeadsHeader().Return(new(core.Header), nil)
		mockState.EXPECT().ContractClassHash(&felt.Zero).Return(felt.Zero, errors.New("unknown contract"))

		blockID := rpcv9.BlockIDLatest()
		res, rpcErr := handler.Call(&rpcv9.FunctionCall{}, &blockID)
		require.Nil(t, res)
		assert.Equal(t, rpccore.ErrContractNotFound, rpcErr)
	})

	t.Run("ok", func(t *testing.T) {
		handler = handler.WithCallMaxSteps(1337).WithCallMaxGas(1338)

		contractAddr := felt.NewFromUint64[felt.Felt](1)
		selector := felt.NewFromUint64[felt.Felt](2)
		classHash := felt.NewFromUint64[felt.Felt](3)
		calldata := []felt.Felt{
			*felt.NewFromUint64[felt.Felt](4),
			*felt.NewFromUint64[felt.Felt](5),
		}
		expectedRes := vm.CallResult{Result: []*felt.Felt{
			felt.NewFromUint64[felt.Felt](6),
			felt.NewFromUint64[felt.Felt](7),
		}}

		headsHeader := &core.Header{
			Number:    9,
			Timestamp: 101,
		}

		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockReader.EXPECT().HeadsHeader().Return(headsHeader, nil)
		mockState.EXPECT().ContractClassHash(contractAddr).Return(*classHash, nil)
		mockVM.EXPECT().Call(
			&vm.CallInfo{
				ContractAddress: contractAddr,
				ClassHash:       classHash,
				Selector:        selector,
				Calldata:        calldata,
			},
			&vm.BlockInfo{
				Header: headsHeader,
			},
			gomock.Any(),
			uint64(1337),
			uint64(1338),
			true,
			false,
		).Return(expectedRes, nil)

		blockID := rpcv9.BlockIDLatest()
		res, rpcErr := handler.Call(
			&rpcv9.FunctionCall{
				ContractAddress:    *contractAddr,
				EntryPointSelector: *selector,
				Calldata:           rpcv9.CalldataInputs{Data: calldata},
			},
			&blockID,
		)
		require.Nil(t, rpcErr)
		require.Equal(t, expectedRes.Result, res)
	})

	t.Run("entrypoint not found error", func(t *testing.T) {
		handler = handler.WithCallMaxSteps(1337).WithCallMaxGas(1338)

		contractAddr := felt.NewFromUint64[felt.Felt](1)
		selector := felt.NewFromUint64[felt.Felt](2)
		classHash := felt.NewFromUint64[felt.Felt](3)
		calldata := []felt.Felt{*felt.NewFromUint64[felt.Felt](4)}
		expectedRes := vm.CallResult{
			Result: []*felt.Felt{
				felt.NewUnsafeFromString[felt.Felt](rpccore.EntrypointNotFoundFelt),
			},
			ExecutionFailed: true,
		}
		expectedErr := rpccore.ErrEntrypointNotFoundV0_10

		headsHeader := &core.Header{
			Number:    9,
			Timestamp: 101,
		}

		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockReader.EXPECT().HeadsHeader().Return(headsHeader, nil)
		mockState.EXPECT().ContractClassHash(contractAddr).Return(*classHash, nil)
		mockVM.EXPECT().Call(
			&vm.CallInfo{
				ContractAddress: contractAddr,
				ClassHash:       classHash,
				Selector:        selector,
				Calldata:        calldata,
			},
			&vm.BlockInfo{
				Header: headsHeader,
			},
			gomock.Any(),
			uint64(1337),
			uint64(1338),
			true,
			false,
		).Return(expectedRes, nil)

		blockID := rpcv9.BlockIDLatest()
		res, rpcErr := handler.Call(&rpcv9.FunctionCall{
			ContractAddress:    *contractAddr,
			EntryPointSelector: *selector,
			Calldata:           rpcv9.CalldataInputs{Data: calldata},
		},
			&blockID,
		)
		require.Nil(t, res)
		require.Equal(t, rpcErr, expectedErr)
	})

	t.Run("execution failed with execution failure and empty result", func(t *testing.T) {
		handler = handler.WithCallMaxSteps(1337).WithCallMaxGas(1338)

		contractAddr := felt.NewFromUint64[felt.Felt](1)
		selector := felt.NewFromUint64[felt.Felt](2)
		classHash := felt.NewFromUint64[felt.Felt](3)
		calldata := []felt.Felt{*felt.NewFromUint64[felt.Felt](4)}
		expectedRes := vm.CallResult{
			ExecutionFailed: true,
		}
		expectedErr := rpcv9.MakeContractError(json.RawMessage(""))

		headsHeader := &core.Header{
			Number:    9,
			Timestamp: 101,
		}

		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockReader.EXPECT().HeadsHeader().Return(headsHeader, nil)
		mockState.EXPECT().ContractClassHash(contractAddr).Return(*classHash, nil)
		mockVM.EXPECT().Call(
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
		).Return(expectedRes, nil)

		blockID := rpcv9.BlockIDLatest()
		res, rpcErr := handler.Call(
			&rpcv9.FunctionCall{
				ContractAddress:    *contractAddr,
				EntryPointSelector: *selector,
				Calldata:           rpcv9.CalldataInputs{Data: calldata},
			},
			&blockID,
		)
		require.Nil(t, res)
		require.Equal(t, expectedErr, rpcErr)
	})
}
