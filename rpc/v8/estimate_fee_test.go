package rpcv8_test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpc "github.com/NethermindEth/juno/rpc/v8"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	snUtils "github.com/NethermindEth/starknet.go/utils"
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

func TestCairo(t *testing.T) {
	chain, accountAddr, deployerAddr := testStorage(t, "0.13.4")

	snClass, compliedClass, bsClass := classFromFile(t, "../../cairo/bs/target/dev/bs_HelloStarknet.contract_class.json")
	bsClassHash, err := bsClass.Hash()
	require.NoError(t, err)

	contractClass, err := json.Marshal(snClass)
	require.NoError(t, err)

	coreCompiledClass, err := sn2core.AdaptCompiledClass(compliedClass)
	require.NoError(t, err)
	compliedClassHash := coreCompiledClass.Hash()

	declareTxn := rpc.BroadcastedTransaction{
		Transaction: rpc.Transaction{
			Type:              rpc.TxnDeclare,
			Version:           new(felt.Felt).SetUint64(3),
			Nonce:             &felt.Zero,
			ClassHash:         bsClassHash,
			SenderAddress:     accountAddr,
			Signature:         &[]*felt.Felt{},
			CompiledClassHash: compliedClassHash,
			ResourceBounds: &map[rpc.Resource]rpc.ResourceBounds{
				rpc.ResourceL1Gas: {
					MaxAmount:       &felt.Zero,
					MaxPricePerUnit: &felt.Zero,
				},
				rpc.ResourceL2Gas: {
					MaxAmount:       &felt.Zero,
					MaxPricePerUnit: &felt.Zero,
				},
				rpc.ResourceL1DataGas: {
					MaxAmount:       &felt.Zero,
					MaxPricePerUnit: &felt.Zero,
				},
			},
			Tip:                   &felt.Zero,
			PaymasterData:         &[]*felt.Felt{},
			AccountDeploymentData: &[]*felt.Felt{},
			NonceDAMode:           utils.HeapPtr(rpc.DAModeL1),
			FeeDAMode:             utils.HeapPtr(rpc.DAModeL1),
		},
		ContractClass: contractClass,
	}

	deployTxn := rpc.BroadcastedTransaction{
		Transaction: rpc.Transaction{
			Type:          rpc.TxnInvoke,
			Version:       new(felt.Felt).SetUint64(3),
			Nonce:         new(felt.Felt).SetUint64(1),
			SenderAddress: accountAddr,
			Signature:     &[]*felt.Felt{},
			CallData: &[]*felt.Felt{
				new(felt.Felt).SetUint64(1),
				deployerAddr,
				// Entry point selector for the called contract
				new(felt.Felt).SetBigInt(snUtils.GetSelectorFromName("deploy_contract")),
				// Length of the call data for the called contract
				new(felt.Felt).SetUint64(4),
				// classHash
				bsClassHash,
				// salt
				&felt.Zero,
				// unique
				&felt.Zero,
				// calldata_len
				&felt.Zero,
			},
			ResourceBounds: &map[rpc.Resource]rpc.ResourceBounds{
				rpc.ResourceL1Gas: {
					MaxAmount:       &felt.Zero,
					MaxPricePerUnit: &felt.Zero,
				},
				rpc.ResourceL2Gas: {
					MaxAmount:       &felt.Zero,
					MaxPricePerUnit: &felt.Zero,
				},
				rpc.ResourceL1DataGas: {
					MaxAmount:       &felt.Zero,
					MaxPricePerUnit: &felt.Zero,
				},
			},
			Tip:                   &felt.Zero,
			PaymasterData:         &[]*felt.Felt{},
			AccountDeploymentData: &[]*felt.Felt{},
			NonceDAMode:           utils.HeapPtr(rpc.DAModeL1),
			FeeDAMode:             utils.HeapPtr(rpc.DAModeL1),
		},
		ContractClass: contractClass,
		PaidFeeOnL1:   &felt.Felt{},
	}

	testDB := pebble.NewMemTest(t)
	txn, err := testDB.NewTransaction(true)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, txn.Discard())
	})

	virtualMachine := vm.New(false, nil)
	handler := rpc.New(chain, &sync.NoopSynchronizer{}, virtualMachine, "", nil)

	_, _, jsonErr := handler.EstimateFee([]rpc.BroadcastedTransaction{declareTxn, deployTxn}, []rpc.SimulationFlag{rpc.SkipValidateFlag}, rpc.BlockID{Latest: true})
	executionError, ok := jsonErr.Data.(rpc.TransactionExecutionErrorData)
	require.True(t, ok)
	fmt.Println(string(executionError.ExecutionError))
	require.Nil(t, jsonErr)
}

func testStorage(t *testing.T, protocolVersion string) (*blockchain.Blockchain, *felt.Felt, *felt.Felt) {
	t.Helper()

	testDB := pebble.NewMemTest(t)
	chain := blockchain.New(testDB, &utils.Sepolia)

	genesis := &core.Block{
		Header: &core.Header{
			Number:          0,
			Timestamp:       0,
			ProtocolVersion: protocolVersion,
			Hash:            utils.HexToFelt(t, "0xb00"),
			ParentHash:      &felt.Zero,
		},
	}
	genesisStateUpdate := &core.StateUpdate{
		BlockHash: genesis.Header.Hash,
		NewRoot:   &felt.Zero,
		OldRoot:   &felt.Zero,
		StateDiff: &core.StateDiff{},
	}

	require.NoError(t, chain.Store(genesis, &core.BlockCommitments{}, genesisStateUpdate, nil))

	accountAddr := new(felt.Felt).SetUint64(0xc01)
	_, _, accountClass := classFromFile(t, "../../cairo/account/target/dev/account_AccountUpgradeable.contract_class.json")
	accountClassHash, err := accountClass.Hash()
	require.NoError(t, err)

	deployerAddr := new(felt.Felt).SetUint64(0xc02)
	_, _, delployerClass := classFromFile(t, "../../cairo/universal_deployer/target/dev/universal_deployer_UniversalDeployer.contract_class.json")
	delployerClassHash, err := delployerClass.Hash()
	require.NoError(t, err)

	stateUpdate := &core.StateUpdate{
		OldRoot: &felt.Zero,
		NewRoot: utils.HexToFelt(t, "0x1d653077282c72d3177d8339ceb8de61b9b4f5621317da6a2e6dc24ce32a6e3"),
		StateDiff: &core.StateDiff{
			DeployedContracts: map[felt.Felt]*felt.Felt{
				*accountAddr:  accountClassHash,
				*deployerAddr: delployerClassHash,
			},
		},
	}
	newClasses := map[felt.Felt]core.Class{
		*accountClassHash:   accountClass,
		*delployerClassHash: delployerClass,
	}
	block := &core.Block{
		Header: &core.Header{
			Hash:             utils.HexToFelt(t, "0xb01"),
			ParentHash:       genesis.Header.Hash,
			Number:           1,
			GlobalStateRoot:  stateUpdate.NewRoot,
			SequencerAddress: utils.HexToFelt(t, "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8"),
			Timestamp:        1,
			ProtocolVersion:  protocolVersion,
			L1GasPriceETH:    utils.HexToFelt(t, "0x1"),
			L1GasPriceSTRK:   utils.HexToFelt(t, "0x2"),
			L1DAMode:         core.Blob,
			L1DataGasPrice:   &core.GasPrice{PriceInWei: utils.HexToFelt(t, "0x2"), PriceInFri: utils.HexToFelt(t, "0x2")},
			L2GasPrice:       &core.GasPrice{PriceInWei: utils.HexToFelt(t, "0x1"), PriceInFri: utils.HexToFelt(t, "0x1")},
		},
	}

	require.NoError(t, chain.Store(block, &core.BlockCommitments{}, stateUpdate, newClasses))

	return chain, accountAddr, deployerAddr
}

func classFromFile(t *testing.T, path string) (*starknet.SierraDefinition, *starknet.CompiledClass, *core.Cairo1Class) {
	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()

	snClass := new(starknet.SierraDefinition)
	require.NoError(t, json.NewDecoder(file).Decode(snClass))

	compliedClass, err := compiler.Compile(snClass)
	require.NoError(t, err)

	class, err := sn2core.AdaptCairo1Class(snClass, compliedClass)
	require.NoError(t, err)

	return snClass, compliedClass, class
}
