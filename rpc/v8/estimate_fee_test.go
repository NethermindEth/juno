package rpcv8_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpc "github.com/NethermindEth/juno/rpc/v8"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestEstimateFee(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	n := &utils.Mainnet

	mockReader := mocks.NewMockReader(mockCtrl)
	mockReader.EXPECT().Network().Return(n).AnyTimes()
	mockVM := mocks.NewMockVM(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpc.New(mockReader, nil, mockVM, "", log)

	mockState := mocks.NewMockStateHistoryReader(mockCtrl)
	mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil).AnyTimes()
	mockReader.EXPECT().HeadsHeader().Return(&core.Header{}, nil).AnyTimes()

	blockInfo := vm.BlockInfo{Header: &core.Header{}}
	t.Run("ok with zero values", func(t *testing.T) {
		mockVM.EXPECT().Execute([]core.Transaction{}, nil, []*felt.Felt{}, &blockInfo, mockState, n, true, false, true, true).
			Return(vm.ExecutionResults{
				OverallFees:      []*felt.Felt{},
				DataAvailability: []core.DataAvailability{},
				GasConsumed:      []core.GasConsumed{},
				Traces:           []vm.TransactionTrace{},
				NumSteps:         uint64(123),
			}, nil)

		_, httpHeader, err := handler.EstimateFee([]rpc.BroadcastedTransaction{}, []rpc.SimulationFlag{}, rpc.BlockID{Latest: true})
		require.Nil(t, err)
		assert.Equal(t, httpHeader.Get(rpc.ExecutionStepsHeader), "123")
	})

	t.Run("ok with zero values, skip validate", func(t *testing.T) {
		mockVM.EXPECT().Execute([]core.Transaction{}, nil, []*felt.Felt{}, &blockInfo, mockState, n, true, true, true, true).
			Return(vm.ExecutionResults{
				OverallFees:      []*felt.Felt{},
				DataAvailability: []core.DataAvailability{},
				GasConsumed:      []core.GasConsumed{},
				Traces:           []vm.TransactionTrace{},
				NumSteps:         uint64(123),
			}, nil)

		_, httpHeader, err := handler.EstimateFee([]rpc.BroadcastedTransaction{}, []rpc.SimulationFlag{rpc.SkipValidateFlag}, rpc.BlockID{Latest: true})
		require.Nil(t, err)
		assert.Equal(t, httpHeader.Get(rpc.ExecutionStepsHeader), "123")
	})

	t.Run("transaction execution error", func(t *testing.T) {
		mockVM.EXPECT().Execute([]core.Transaction{}, nil, []*felt.Felt{}, &blockInfo, mockState, n, true, true, true, true).
			Return(vm.ExecutionResults{}, vm.TransactionExecutionError{
				Index: 44,
				Cause: json.RawMessage("oops"),
			})

		_, httpHeader, err := handler.EstimateFee([]rpc.BroadcastedTransaction{}, []rpc.SimulationFlag{rpc.SkipValidateFlag}, rpc.BlockID{Latest: true})
		require.Equal(t, rpccore.ErrTransactionExecutionError.CloneWithData(rpc.TransactionExecutionErrorData{
			TransactionIndex: 44,
			ExecutionError:   json.RawMessage("oops"),
		}), err)
		require.Equal(t, httpHeader.Get(rpc.ExecutionStepsHeader), "0")
	})

	t.Run("transaction with invalid contract class", func(t *testing.T) {
		toFelt := func(hex string) *felt.Felt {
			return utils.HexToFelt(t, hex)
		}
		invalidTx := rpc.BroadcastedTransaction{
			Transaction: rpc.Transaction{
				Type:          rpc.TxnDeclare,
				Version:       toFelt("0x1"),
				Nonce:         toFelt("0x0"),
				MaxFee:        toFelt("0x1"),
				SenderAddress: toFelt("0x2"),
				Signature: &[]*felt.Felt{
					toFelt("0x123"),
				},
			},
			ContractClass: json.RawMessage(`{}`),
		}
		_, _, err := handler.EstimateFee([]rpc.BroadcastedTransaction{invalidTx}, []rpc.SimulationFlag{}, rpc.BlockID{Latest: true})
		expectedErr := &jsonrpc.Error{
			Code:    jsonrpc.InvalidParams,
			Message: "Invalid Params",
			Data:    "invalid program",
		}
		require.Equal(t, expectedErr, err)
	})
}

type executionError struct {
	ClassHash       string `json:"class_hash"`
	ContractAddress string `json:"contract_address"`
	Error           any    `json:"error"`
	Selector        string `json:"selector"`
}

// Manually obtained from VM logs
// Global variable because
// unparam: `createInvokeTransaction` - `deployedContractAddress` always receives `deployedContractAddr`

// From versioned constants
var executeEntryPointSelector = "0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad"

// func TestEstimateFeeWithVM(t *testing.T) {
// 	// Get blockchain with predeployed account and deployer contracts
// 	chain, accountAddr, accountClassHash, deployerAddr, _ := blockchain.NewTestBlockchain(t, "0.13.4")
//
// 	// Get binary search contract
// 	snClass, compliedClass, bsClass := blockchain.ClassFromFile(t, "../../cairo/target/dev/juno_HelloStarknet.contract_class.json")
// 	bsClassHash, err := bsClass.Hash()
// 	require.NoError(t, err)
//
// 	contractClass, err := json.Marshal(snClass)
// 	require.NoError(t, err)
//
// 	coreCompiledClass, err := sn2core.AdaptCompiledClass(compliedClass)
// 	require.NoError(t, err)
// 	compliedClassHash := coreCompiledClass.Hash()
//
// 	// Declare and deploy binary search contract
// 	declareTxn := createDeclareTransaction(t, accountAddr, bsClassHash, compliedClassHash, contractClass)
// 	deployTxn := createDeployTransaction(t, accountAddr, deployerAddr, bsClassHash, contractClass)
//
// 	// Call test_redeposits entry point
// 	validEntryPoint := crypto.StarknetKeccak([]byte("test_redeposits"))
// 	invokeTxn := createInvokeTransaction(t, "0x2",
// 		accountAddr, validEntryPoint, "0x7",
// 	)
//
// 	virtualMachine := vm.New(false, nil)
// 	handler := rpc.New(chain, &sync.NoopSynchronizer{}, virtualMachine, "", nil)
//
// 	declareExpected := createFeeEstimate(t,
// 		"0x0", "0x2",
// 		"0x423cb80", "0x1",
// 		"0xc0", "0x2",
// 		"0x423cd00", rpc.FRI,
// 	)
// 	deployExpected := createFeeEstimate(t,
// 		"0x0", "0x2",
// 		"0xfdcc5", "0x1",
// 		"0xe0", "0x2",
// 		"0xe6d5c", rpc.FRI,
// 	)
// 	invokeExpected := createFeeEstimate(t,
// 		"0x0", "0x2",
// 		"0xbe18b", "0x1",
// 		"0x80", "0x2",
// 		"0xace0a", rpc.FRI,
// 	)
//
// 	invalidEntryPoint := crypto.StarknetKeccak([]byte("invalid_entry_point"))
//
// 	tests := []struct {
// 		name                    string
// 		broadcastedTransactions []rpc.BroadcastedTransaction
// 		jsonErr                 *jsonrpc.Error
// 		expected                []rpc.FeeEstimate
// 	}{
// 		{
// 			name:                    "binary search ok",
// 			broadcastedTransactions: []rpc.BroadcastedTransaction{declareTxn, deployTxn, invokeTxn},
// 			expected:                []rpc.FeeEstimate{declareExpected, deployExpected, invokeExpected},
// 		},
// 		{
// 			name: "invalid entry point",
// 			broadcastedTransactions: []rpc.BroadcastedTransaction{
// 				declareTxn, deployTxn,
// 				createInvokeTransaction(t, "0x2",
// 					accountAddr, invalidEntryPoint, "0x7",
// 				),
// 			},
// 			jsonErr: rpccore.ErrTransactionExecutionError.CloneWithData(
// 				rpc.TransactionExecutionErrorData{
// 					TransactionIndex: 2,
// 					ExecutionError: mustMarshal(t, executionError{
// 						ClassHash:       accountClassHash.String(),
// 						ContractAddress: accountAddr.String(),
// 						Error: executionError{
// 							ClassHash:       accountClassHash.String(),
// 							ContractAddress: accountAddr.String(),
// 							Error: executionError{
// 								ClassHash:       bsClassHash.String(),
// 								ContractAddress: deployedContractAddress,
// 								Error:           rpccore.EntrypointNotFoundFelt + " ('ENTRYPOINT_NOT_FOUND')",
// 								Selector:        invalidEntryPoint.String(),
// 							},
// 							Selector: executeEntryPointSelector,
// 						},
// 						Selector: executeEntryPointSelector,
// 					}),
// 				},
// 			),
// 		},
// 		{
// 			name: "max gas exceeded",
// 			broadcastedTransactions: []rpc.BroadcastedTransaction{
// 				declareTxn, deployTxn,
// 				createInvokeTransaction(t, "0x2",
// 					accountAddr, validEntryPoint, "0x186A0", // 100000
// 				),
// 			},
// 			jsonErr: rpccore.ErrTransactionExecutionError.CloneWithData(
// 				rpc.TransactionExecutionErrorData{
// 					TransactionIndex: 2,
// 					ExecutionError: json.RawMessage(
// 						`"Transaction ran out of gas during simulation"`,
// 					),
// 				},
// 			),
// 		},
// 		{
// 			name: "gas limit exceeded",
// 			broadcastedTransactions: []rpc.BroadcastedTransaction{
// 				declareTxn, deployTxn,
// 				createInvokeTransaction(t, "0x2",
// 					accountAddr, validEntryPoint, "0x64", // 100
// 				),
// 			},
// 			jsonErr: rpccore.ErrTransactionExecutionError.CloneWithData(
// 				rpc.TransactionExecutionErrorData{
// 					TransactionIndex: 2,
// 					ExecutionError: mustMarshal(t, executionError{
// 						ClassHash:       accountClassHash.String(),
// 						ContractAddress: accountAddr.String(),
// 						Error: executionError{
// 							ClassHash:       accountClassHash.String(),
// 							ContractAddress: accountAddr.String(),
// 							Error:           "0x4f7574206f6620676173 ('Out of gas')",
// 							Selector:        executeEntryPointSelector,
// 						},
// 						Selector: executeEntryPointSelector,
// 					}),
// 				},
// 			),
// 		},
// 	}
//
// 	for _, test := range tests {
// 		t.Run(test.name, func(t *testing.T) {
// 			feeEstimate, _, jsonErr := handler.EstimateFee(
// 				test.broadcastedTransactions,
// 				[]rpc.SimulationFlag{rpc.SkipValidateFlag},
// 				rpc.BlockID{Latest: true},
// 			)
//
// 			if test.jsonErr != nil {
// 				require.Equal(t, test.jsonErr, jsonErr, handleJSONError(t, jsonErr))
// 				return
// 			}
//
// 			require.Equal(t, test.expected, feeEstimate)
// 		})
// 	}
// }

func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}

func createDeclareTransaction(t *testing.T,
	accountAddr, bsClassHash, compliedClassHash *felt.Felt,
	contractClass []byte,
) rpc.BroadcastedTransaction {
	return rpc.BroadcastedTransaction{
		Transaction: rpc.Transaction{
			Type:              rpc.TxnDeclare,
			Version:           utils.HexToFelt(t, "0x3"),
			Nonce:             &felt.Zero,
			ClassHash:         bsClassHash,
			SenderAddress:     accountAddr,
			Signature:         &[]*felt.Felt{},
			CompiledClassHash: compliedClassHash,
			ResourceBounds: utils.HeapPtr(createResourceBounds(t,
				"0x0", "0x0",
				"0x0", "0x0",
				"0x0", "0x0",
			)),
			Tip:                   &felt.Zero,
			PaymasterData:         &[]*felt.Felt{},
			AccountDeploymentData: &[]*felt.Felt{},
			NonceDAMode:           utils.HeapPtr(rpc.DAModeL1),
			FeeDAMode:             utils.HeapPtr(rpc.DAModeL1),
		},
		ContractClass: contractClass,
	}
}

func createDeployTransaction(t *testing.T,
	accountAddr, deployerAddr, bsClassHash *felt.Felt,
	contractClass []byte,
) rpc.BroadcastedTransaction {
	return rpc.BroadcastedTransaction{
		Transaction: rpc.Transaction{
			Type:          rpc.TxnInvoke,
			Version:       utils.HexToFelt(t, "0x3"),
			Nonce:         utils.HexToFelt(t, "0x1"),
			SenderAddress: accountAddr,
			Signature:     &[]*felt.Felt{},
			CallData: &[]*felt.Felt{
				utils.HexToFelt(t, "0x1"),
				deployerAddr,
				// Entry point selector for the called contract
				crypto.StarknetKeccak([]byte("deploy_contract")),
				// Length of the call data for the called contract
				utils.HexToFelt(t, "0x4"),
				// classHash
				bsClassHash,
				// salt
				&felt.Zero,
				// unique
				&felt.Zero,
				// calldata_len
				&felt.Zero,
			},
			ResourceBounds: utils.HeapPtr(createResourceBounds(t,
				"0x0", "0x2",
				"0xfdcc5", "0x1",
				"0xe0", "0x2",
			)),
			Tip:                   &felt.Zero,
			PaymasterData:         &[]*felt.Felt{},
			AccountDeploymentData: &[]*felt.Felt{},
			NonceDAMode:           utils.HeapPtr(rpc.DAModeL1),
			FeeDAMode:             utils.HeapPtr(rpc.DAModeL1),
		},
		ContractClass: contractClass,
		PaidFeeOnL1:   &felt.Felt{},
	}
}

func createInvokeTransaction(t *testing.T,
	accountAddr, entryPointSelector,
	nonce, deployedContractAddress, depth *felt.Felt,
) rpc.BroadcastedTransaction {
	return rpc.BroadcastedTransaction{
		Transaction: rpc.Transaction{
			Type:          rpc.TxnInvoke,
			Version:       utils.HexToFelt(t, "0x3"),
			Nonce:         nonce,
			SenderAddress: accountAddr,
			Signature:     &[]*felt.Felt{},
			CallData: &[]*felt.Felt{
				utils.HexToFelt(t, "0x1"),
				// Address of the deployed test contract
				deployedContractAddress,
				// Entry point selector for the called contract
				entryPointSelector,
				// Length of the call data for the called contract
				utils.HexToFelt(t, "0x1"),
				depth,
			},
			ResourceBounds: utils.HeapPtr(createResourceBounds(t,
				"0x0", "0x2",
				"0xbe18b", "0x1",
				"0x80", "0x2",
			)),
			Tip:                   &felt.Zero,
			PaymasterData:         &[]*felt.Felt{},
			AccountDeploymentData: &[]*felt.Felt{},
			NonceDAMode:           utils.HeapPtr(rpc.DAModeL2),
			FeeDAMode:             utils.HeapPtr(rpc.DAModeL2),
		},
		PaidFeeOnL1: &felt.Felt{},
	}
}

func createResourceBounds(t *testing.T,
	l1GasAmount, l1GasPrice,
	l2GasAmount, l2GasPrice,
	l1DataGasAmount, l1DataGasPrice string,
) map[rpc.Resource]rpc.ResourceBounds {
	t.Helper()
	return map[rpc.Resource]rpc.ResourceBounds{
		rpc.ResourceL1Gas: {
			MaxAmount:       utils.HexToFelt(t, l1GasAmount),
			MaxPricePerUnit: utils.HexToFelt(t, l1GasPrice),
		},
		rpc.ResourceL2Gas: {
			MaxAmount:       utils.HexToFelt(t, l2GasAmount),
			MaxPricePerUnit: utils.HexToFelt(t, l2GasPrice),
		},
		rpc.ResourceL1DataGas: {
			MaxAmount:       utils.HexToFelt(t, l1DataGasAmount),
			MaxPricePerUnit: utils.HexToFelt(t, l1DataGasPrice),
		},
	}
}

func createFeeEstimate(t *testing.T,
	l1GasConsumed, l1GasPrice,
	l2GasConsumed, l2GasPrice,
	l1DataGasConsumed, l1DataGasPrice,
	overallFee string, unit rpc.FeeUnit,
) rpc.FeeEstimate {
	return rpc.FeeEstimate{
		L1GasConsumed:     utils.HexToFelt(t, l1GasConsumed),
		L1GasPrice:        utils.HexToFelt(t, l1GasPrice),
		L2GasConsumed:     utils.HexToFelt(t, l2GasConsumed),
		L2GasPrice:        utils.HexToFelt(t, l2GasPrice),
		L1DataGasConsumed: utils.HexToFelt(t, l1DataGasConsumed),
		L1DataGasPrice:    utils.HexToFelt(t, l1DataGasPrice),
		OverallFee:        utils.HexToFelt(t, overallFee),
		Unit:              utils.HeapPtr(unit),
	}
}

func handleJSONError(t *testing.T, jsonErr *jsonrpc.Error) string {
	if jsonErr != nil && jsonErr.Data != nil {
		executionErr, ok := jsonErr.Data.(rpc.TransactionExecutionErrorData)
		require.True(t, ok, jsonErr)
		return string(executionErr.ExecutionError)
	}
	return ""
}

func TestEstimateFeeWithVMInvoke(t *testing.T) {
	// Get blockchain with predeployed account and deployer contracts
	chain := blockchain.NewTestBlockchain(t, "0.13.4")
	accountAddr := chain.AccountAddress()
	accountClassHash := chain.AccountClassHash()

	// Predeploy binary search contract
	addr := utils.HexToFelt(t, "0xd")

	class := blockchain.NewClass(t, "../../cairo/target/dev/juno_HelloStarknet.contract_class.json")
	class.AddAccount(addr, nil)

	chain.PredeployContracts(t, []*blockchain.TestClass{class})

	validEntryPoint := crypto.StarknetKeccak([]byte("test_redeposits"))
	invalidEntryPoint := crypto.StarknetKeccak([]byte("invalid_entry_point"))

	validDepth := utils.HexToFelt(t, "0x7")

	virtualMachine := vm.New(false, nil)
	handler := rpc.New(chain, &sync.NoopSynchronizer{}, virtualMachine, "", nil)

	tests := []struct {
		name                    string
		broadcastedTransactions []rpc.BroadcastedTransaction
		jsonErr                 *jsonrpc.Error
		expected                []rpc.FeeEstimate
	}{
		{
			name: "binary search ok",
			broadcastedTransactions: []rpc.BroadcastedTransaction{
				createInvokeTransaction(t, accountAddr, validEntryPoint, &felt.Zero,
					addr, validDepth,
				),
			},
			expected: []rpc.FeeEstimate{
				createFeeEstimate(t,
					"0x0", "0x2",
					"0xbe18b", "0x1",
					"0x80", "0x2",
					"0xace0a", rpc.FRI,
				),
			},
		},
		{
			name: "invalid entry point",
			broadcastedTransactions: []rpc.BroadcastedTransaction{
				createInvokeTransaction(t, accountAddr, invalidEntryPoint, &felt.Zero,
					addr, validDepth,
				),
			},
			jsonErr: rpccore.ErrTransactionExecutionError.CloneWithData(
				rpc.TransactionExecutionErrorData{
					TransactionIndex: 0,
					ExecutionError: mustMarshal(t, executionError{
						ClassHash:       accountClassHash.String(),
						ContractAddress: accountAddr.String(),
						Error: executionError{
							ClassHash:       accountClassHash.String(),
							ContractAddress: accountAddr.String(),
							Error: executionError{
								ClassHash:       chain.ClassHashByAddress(addr).String(),
								ContractAddress: addr.String(),
								Error:           rpccore.EntrypointNotFoundFelt + " ('ENTRYPOINT_NOT_FOUND')",
								Selector:        invalidEntryPoint.String(),
							},
							Selector: executeEntryPointSelector,
						},
						Selector: executeEntryPointSelector,
					}),
				},
			),
		},
		{
			name: "max gas exceeded",
			broadcastedTransactions: []rpc.BroadcastedTransaction{
				createInvokeTransaction(t, accountAddr, validEntryPoint, &felt.Zero,
					addr, utils.HexToFelt(t, "0x186A0"), // 100000
				),
			},
			jsonErr: rpccore.ErrTransactionExecutionError.CloneWithData(
				rpc.TransactionExecutionErrorData{
					TransactionIndex: 0,
					ExecutionError: json.RawMessage(
						`"Transaction ran out of gas during simulation"`,
					),
				},
			),
		},
		{
			name: "gas limit exceeded",
			broadcastedTransactions: []rpc.BroadcastedTransaction{
				createInvokeTransaction(t, accountAddr, validEntryPoint, &felt.Zero,
					addr, utils.HexToFelt(t, "0x64"), // 100
				),
			},
			jsonErr: rpccore.ErrTransactionExecutionError.CloneWithData(
				rpc.TransactionExecutionErrorData{
					TransactionIndex: 0,
					ExecutionError: mustMarshal(t, executionError{
						ClassHash:       accountClassHash.String(),
						ContractAddress: accountAddr.String(),
						Error: executionError{
							ClassHash:       accountClassHash.String(),
							ContractAddress: accountAddr.String(),
							Error:           "0x4f7574206f6620676173 ('Out of gas')",
							Selector:        executeEntryPointSelector,
						},
						Selector: executeEntryPointSelector,
					}),
				},
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			feeEstimate, _, jsonErr := handler.EstimateFee(
				test.broadcastedTransactions,
				[]rpc.SimulationFlag{rpc.SkipValidateFlag},
				rpc.BlockID{Latest: true},
			)

			if test.jsonErr != nil {
				require.Equal(t, test.jsonErr, jsonErr,
					fmt.Sprintf("expected: %v\n, got: %v\n",
						handleJSONError(t, test.jsonErr),
						handleJSONError(t, jsonErr),
					),
				)
				return
			}

			require.Equal(t, test.expected, feeEstimate)
		})
	}
}
