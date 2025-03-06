package rpcv8_test

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"testing"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
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
			Version:           utils.HexToFelt(t, "0x3"),
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
			ResourceBounds: &map[rpc.Resource]rpc.ResourceBounds{
				rpc.ResourceL1Gas: {
					MaxAmount:       &felt.Zero,
					MaxPricePerUnit: &felt.Zero,
				},
				rpc.ResourceL2Gas: {
					MaxAmount:       utils.HexToFelt(t, "0xfdcc5"),
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

	invokeTxn := rpc.BroadcastedTransaction{
		Transaction: rpc.Transaction{
			Type:          rpc.TxnInvoke,
			Version:       utils.HexToFelt(t, "0x3"),
			Nonce:         utils.HexToFelt(t, "0x2"),
			SenderAddress: accountAddr,
			Signature:     &[]*felt.Felt{},
			CallData: &[]*felt.Felt{
				utils.HexToFelt(t, "0x1"),
				// Address of the deployed test contract
				utils.HexToFelt(t, "0x5715f1e9c6ab89b6624fa464b02acd3edc054cc6add51305affce40aa781d08"),
				// Entry point selector for the called contract
				crypto.StarknetKeccak([]byte("test_redeposits")),
				// Length of the call data for the called contract
				utils.HexToFelt(t, "0x1"),
				utils.HexToFelt(t, "0x7"),
			},
			ResourceBounds: &map[rpc.Resource]rpc.ResourceBounds{
				rpc.ResourceL1Gas: {
					MaxAmount:       new(felt.Felt).SetUint64(50),
					MaxPricePerUnit: new(felt.Felt).SetUint64(1000),
				},
				rpc.ResourceL2Gas: {
					MaxAmount:       new(felt.Felt).SetUint64(800_000),
					MaxPricePerUnit: new(felt.Felt).SetUint64(1000),
				},
				rpc.ResourceL1DataGas: {
					MaxAmount:       new(felt.Felt).SetUint64(100),
					MaxPricePerUnit: new(felt.Felt).SetUint64(1000),
				},
			},
			Tip:                   &felt.Zero,
			PaymasterData:         &[]*felt.Felt{},
			AccountDeploymentData: &[]*felt.Felt{},
			NonceDAMode:           utils.HeapPtr(rpc.DAModeL2),
			FeeDAMode:             utils.HeapPtr(rpc.DAModeL2),
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

	feeEstimate, _, jsonErr := handler.EstimateFee(
		[]rpc.BroadcastedTransaction{declareTxn, deployTxn, invokeTxn},
		[]rpc.SimulationFlag{rpc.SkipValidateFlag},
		rpc.BlockID{Latest: true},
	)
	require.Nil(t, jsonErr)

	declareExpected := rpc.FeeEstimate{
		L1GasConsumed:     utils.HexToFelt(t, "0x0"),
		L1GasPrice:        utils.HexToFelt(t, "0x2"),
		L2GasConsumed:     utils.HexToFelt(t, "0x4237680"),
		L2GasPrice:        utils.HexToFelt(t, "0x1"),
		L1DataGasConsumed: utils.HexToFelt(t, "0xc0"),
		L1DataGasPrice:    utils.HexToFelt(t, "0x2"),
		OverallFee:        utils.HexToFelt(t, "0x4237800"),
		Unit:              utils.HeapPtr(rpc.FRI),
	}
	deployExpected := rpc.FeeEstimate{
		L1GasConsumed:     utils.HexToFelt(t, "0x0"),
		L1GasPrice:        utils.HexToFelt(t, "0x2"),
		L2GasConsumed:     utils.HexToFelt(t, "0xfdcc5"),
		L2GasPrice:        utils.HexToFelt(t, "0x1"),
		L1DataGasConsumed: utils.HexToFelt(t, "0xe0"),
		L1DataGasPrice:    utils.HexToFelt(t, "0x2"),
		OverallFee:        utils.HexToFelt(t, "0xe6d5c"),
		Unit:              utils.HeapPtr(rpc.FRI),
	}
	invokeExpected := rpc.FeeEstimate{
		L1GasConsumed:     utils.HexToFelt(t, "0x0"),
		L1GasPrice:        utils.HexToFelt(t, "0x2"),
		L2GasConsumed:     utils.HexToFelt(t, "0xbe18b"),
		L2GasPrice:        utils.HexToFelt(t, "0x1"),
		L1DataGasConsumed: utils.HexToFelt(t, "0x80"),
		L1DataGasPrice:    utils.HexToFelt(t, "0x2"),
		OverallFee:        utils.HexToFelt(t, "0xace0a"),
		Unit:              utils.HeapPtr(rpc.FRI),
	}

	require.Equal(t, []rpc.FeeEstimate{declareExpected, deployExpected, invokeExpected}, feeEstimate)
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
		BlockHash: genesis.Hash,
		NewRoot:   &felt.Zero,
		OldRoot:   &felt.Zero,
		StateDiff: &core.StateDiff{},
	}

	require.NoError(t, chain.Store(genesis, &core.BlockCommitments{}, genesisStateUpdate, nil))

	accountAddr := utils.HexToFelt(t, "0xc01")
	_, _, accountClass := classFromFile(t, "../../cairo/account/target/dev/account_AccountUpgradeable.contract_class.json")
	accountClassHash, err := accountClass.Hash()
	require.NoError(t, err)

	deployerAddr := utils.HexToFelt(t, "0xc02")
	_, _, delployerClass := classFromFile(t, "../../cairo/universal_deployer/target/dev/universal_deployer_UniversalDeployer.contract_class.json")
	delployerClassHash, err := delployerClass.Hash()
	require.NoError(t, err)

	ethFeeTokenAddr := utils.HexToFelt(t, "0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7")
	strkFeeTokenAddr := utils.HexToFelt(t, "0x04718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d")
	_, _, erc20Class := classFromFile(t, "../../cairo/erc20/target/dev/erc20_ERC20Upgradeable.contract_class.json")
	erc20ClassHash, err := erc20Class.Hash()
	require.NoError(t, err)

	accountBalanceKey := fromNameAndKey(t, "ERC20_balances", accountAddr)
	fmt.Printf("accountBalanceKey: %s\n", accountBalanceKey.String())

	stateUpdate := &core.StateUpdate{
		OldRoot: &felt.Zero,
		NewRoot: utils.HexToFelt(t, "0x5403e8bee8d88c4a36879d7236988aeb0b2eb62df16426150181df76b5872af"),
		StateDiff: &core.StateDiff{
			DeployedContracts: map[felt.Felt]*felt.Felt{
				*accountAddr:      accountClassHash,
				*deployerAddr:     delployerClassHash,
				*ethFeeTokenAddr:  erc20ClassHash,
				*strkFeeTokenAddr: erc20ClassHash,
			},
			StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
				*ethFeeTokenAddr: {
					*accountBalanceKey: utils.HexToFelt(t, "0x10000000000000000000000000000"),
				},
				*strkFeeTokenAddr: {
					*accountBalanceKey: utils.HexToFelt(t, "0x10000000000000000000000000000"),
				},
			},
		},
	}
	newClasses := map[felt.Felt]core.Class{
		*accountClassHash:   accountClass,
		*delployerClassHash: delployerClass,
		*erc20ClassHash:     erc20Class,
	}
	block := &core.Block{
		Header: &core.Header{
			Hash:             utils.HexToFelt(t, "0xb01"),
			ParentHash:       genesis.Hash,
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

func fromNameAndKey(t *testing.T, name string, key *felt.Felt) *felt.Felt {
	t.Helper()
	intermediate := crypto.StarknetKeccak([]byte(name))
	byteArr := crypto.Pedersen(intermediate, key).Bytes()
	value := new(big.Int).SetBytes(byteArr[:])

	maxAddr, ok := new(big.Int).SetString("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff00", 0)
	require.True(t, ok)

	value = value.Rem(value, maxAddr)

	return new(felt.Felt).SetBigInt(value)
}
