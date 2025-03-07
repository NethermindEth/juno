package rpcv8_test

import (
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpc "github.com/NethermindEth/juno/rpc/v8"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/utils/testutils"
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

func TestEstimateFeeWithVM(t *testing.T) {
	chain, accountAddr, deployerAddr := testutils.TestBlockchain(t, "0.13.4")

	snClass, compliedClass, bsClass := testutils.ClassFromFile(t, "../../cairo/target/dev/juno_HelloStarknet.contract_class.json")
	bsClassHash, err := bsClass.Hash()
	require.NoError(t, err)

	contractClass, err := json.Marshal(snClass)
	require.NoError(t, err)

	coreCompiledClass, err := sn2core.AdaptCompiledClass(compliedClass)
	require.NoError(t, err)
	compliedClassHash := coreCompiledClass.Hash()

	declareTxn := createDeclareTransaction(t, accountAddr, bsClassHash, compliedClassHash, contractClass)
	deployTxn := createDeployTransaction(t, accountAddr, deployerAddr, bsClassHash, contractClass)
	invokeTxn := createInvokeTransaction(t, accountAddr)

	virtualMachine := vm.New(false, nil)
	handler := rpc.New(chain, &sync.NoopSynchronizer{}, virtualMachine, "", nil)

	feeEstimate, _, jsonErr := handler.EstimateFee(
		[]rpc.BroadcastedTransaction{declareTxn, deployTxn, invokeTxn},
		[]rpc.SimulationFlag{rpc.SkipValidateFlag},
		rpc.BlockID{Latest: true},
	)
	handleJSONError(t, jsonErr)

	declareExpected := createFeeEstimate(t,
		"0x0", "0x2",
		"0x423cb80", "0x1",
		"0xc0", "0x2",
		"0x423cd00", rpc.FRI,
	)
	deployExpected := createFeeEstimate(t,
		"0x0", "0x2",
		"0xfdcc5", "0x1",
		"0xe0", "0x2",
		"0xe6d5c", rpc.FRI,
	)
	invokeExpected := createFeeEstimate(t,
		"0x0", "0x2",
		"0xbe18b", "0x1",
		"0x80", "0x2",
		"0xace0a", rpc.FRI,
	)

	require.Equal(t,
		[]rpc.FeeEstimate{declareExpected, deployExpected, invokeExpected},
		feeEstimate,
	)
}

func createDeclareTransaction(t *testing.T, accountAddr, bsClassHash, compliedClassHash *felt.Felt, contractClass []byte) rpc.BroadcastedTransaction {
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

func createInvokeTransaction(t *testing.T, accountAddr *felt.Felt) rpc.BroadcastedTransaction {
	return rpc.BroadcastedTransaction{
		Transaction: rpc.Transaction{
			Type:          rpc.TxnInvoke,
			Version:       utils.HexToFelt(t, "0x3"),
			Nonce:         utils.HexToFelt(t, "0x2"),
			SenderAddress: accountAddr,
			Signature:     &[]*felt.Felt{},
			CallData: &[]*felt.Felt{
				utils.HexToFelt(t, "0x1"),
				// Address of the deployed test contract
				utils.HexToFelt(t, "0x16d24ca6289c75b6c7f4de3030f1f1641d73b555372421d47f2696916050b01"),
				// Entry point selector for the called contract
				crypto.StarknetKeccak([]byte("test_redeposits")),
				// Length of the call data for the called contract
				utils.HexToFelt(t, "0x1"),
				utils.HexToFelt(t, "0x7"),
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

func handleJSONError(t *testing.T, jsonErr *jsonrpc.Error) {
	if jsonErr != nil {
		if jsonErr.Data != nil {
			executionErr, ok := jsonErr.Data.(rpc.TransactionExecutionErrorData)
			require.True(t, ok, jsonErr)
			t.Fatal(string(executionErr.ExecutionError))
		}
		t.Fatal(jsonErr)
	}
}
