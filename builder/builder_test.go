package builder_test

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/genesis"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/ecdsa"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func waitFor(t *testing.T, timeout time.Duration, check func() bool) {
	t.Helper()

	const numPolls = 4
	pollInterval := timeout / numPolls

	for i := 0; i < numPolls; i++ {
		if check() {
			return
		}
		time.Sleep(pollInterval)
	}
	require.Equal(t, true, false, "reached timeout")
}

func waitForBlock(t *testing.T, bc blockchain.Reader, timeout time.Duration, targetBlockNumber uint64) {
	waitFor(t, timeout, func() bool {
		curBlockNumber, err := bc.Height()
		require.NoError(t, err)
		return curBlockNumber >= targetBlockNumber
	})
}

func waitForTxns(t *testing.T, bc blockchain.Reader, timeout time.Duration, txns []*felt.Felt) {
	waitFor(t, timeout, func() bool {
		for _, txnHash := range txns {
			_, _, _, err := bc.Receipt(txnHash)
			if err != nil {
				return false
			}
		}
		return true
	})
}

func TestValidateAgainstPendingState(t *testing.T) {
	testDB := pebble.NewMemTest(t)
	mockCtrl := gomock.NewController(t)
	mockVM := mocks.NewMockVM(mockCtrl)
	bc := blockchain.New(testDB, &utils.Integration)
	seqAddr := utils.HexToFelt(t, "0xDEADBEEF")
	p := mempool.New(pebble.NewMemTest(t))
	testBuilder := builder.New(nil, seqAddr, bc, mockVM, 0, p, utils.NewNopZapLogger())

	client := feeder.NewTestClient(t, &utils.Integration)
	gw := adaptfeeder.New(client)

	su, b, err := gw.StateUpdateWithBlock(context.Background(), 0)
	require.NoError(t, err)

	require.NoError(t, bc.StorePending(&blockchain.Pending{
		Block:       b,
		StateUpdate: su,
	}))

	userTxn := mempool.BroadcastedTransaction{
		Transaction: &core.InvokeTransaction{
			TransactionHash: utils.HexToFelt(t, "0x1337"),
		},
		DeclaredClass: &core.Cairo0Class{
			Program: "best program",
		},
	}
	pendingBlock, err := bc.Pending()
	require.NoError(t, err)
	blockInfo := &vm.BlockInfo{
		Header: &core.Header{
			Number:           pendingBlock.Block.Number,
			Timestamp:        pendingBlock.Block.Timestamp,
			SequencerAddress: seqAddr,
			GasPrice:         pendingBlock.Block.GasPrice,
			GasPriceSTRK:     pendingBlock.Block.GasPriceSTRK,
		},
	}

	mockVM.EXPECT().Execute([]core.Transaction{userTxn.Transaction},
		[]core.Class{userTxn.DeclaredClass}, []*felt.Felt{}, gomock.Any(),
		gomock.Any(), &utils.Integration, false, false, false, false).DoAndReturn(
		func(txns []core.Transaction, classes []core.Class, felts []*felt.Felt, info *vm.BlockInfo, any interface{}, integration *utils.Network, b1, b2, b3, b4 bool) ([]*core.Event, []*core.Event, []*core.Event, error) {
			// Check all fields of info except for Timestamp
			if info.Header.Number != blockInfo.Header.Number {
				return nil, nil, nil, fmt.Errorf("unexpected BlockInfo: Number. Expected %v, got %v", blockInfo.Header.Number, info.Header.Number)
			}
			if info.Header.SequencerAddress.String() != blockInfo.Header.SequencerAddress.String() {
				return nil, nil, nil, fmt.Errorf("unexpected BlockInfo: SequencerAddress. Expected %v, got %v", blockInfo.Header.SequencerAddress, info.Header.SequencerAddress)
			}
			if info.Header.GasPrice != blockInfo.Header.GasPrice {
				return nil, nil, nil, fmt.Errorf("unexpected BlockInfo: GasPrice. Expected %v, got %v", blockInfo.Header.GasPrice, info.Header.GasPrice)
			}
			if info.Header.GasPriceSTRK != blockInfo.Header.GasPriceSTRK {
				return nil, nil, nil, fmt.Errorf("unexpected BlockInfo: GasPriceSTRK. Expected %v, got %v", blockInfo.Header.GasPriceSTRK, info.Header.GasPriceSTRK)
			}

			// Call the real Execute function
			return nil, nil, nil, nil
		})
	assert.NoError(t, testBuilder.ValidateAgainstPendingState(&userTxn))

	blockInfo.Header.Number += 1

	require.NoError(t, bc.Store(b, &core.BlockCommitments{}, su, nil))
	mockVM.EXPECT().Execute([]core.Transaction{userTxn.Transaction},
		[]core.Class{userTxn.DeclaredClass}, []*felt.Felt{}, gomock.Any(),
		gomock.Any(), &utils.Integration, false, false, false, false).DoAndReturn(
		func(txns []core.Transaction, classes []core.Class, felts []*felt.Felt, info *vm.BlockInfo, any interface{}, integration *utils.Network, b1, b2, b3, b4 bool) ([]*core.Event, []*core.Event, []*core.Event, error) {
			if info.Header.Number != blockInfo.Header.Number {
				return nil, nil, nil, fmt.Errorf("unexpected BlockInfo: Number. Expected %v, got %v", blockInfo.Header.Number, info.Header.Number)
			}
			if info.Header.SequencerAddress.String() != blockInfo.Header.SequencerAddress.String() {
				return nil, nil, nil, fmt.Errorf("unexpected BlockInfo: SequencerAddress. Expected %v, got %v", blockInfo.Header.SequencerAddress, info.Header.SequencerAddress)
			}
			if info.Header.GasPrice != blockInfo.Header.GasPrice {
				return nil, nil, nil, fmt.Errorf("unexpected BlockInfo: GasPrice. Expected %v, got %v", blockInfo.Header.GasPrice, info.Header.GasPrice)
			}
			if info.Header.GasPriceSTRK != blockInfo.Header.GasPriceSTRK {
				return nil, nil, nil, fmt.Errorf("unexpected BlockInfo: GasPriceSTRK. Expected %v, got %v", blockInfo.Header.GasPriceSTRK, info.Header.GasPriceSTRK)
			}

			return nil, nil, nil, errors.New("oops")
		})
	assert.EqualError(t, testBuilder.ValidateAgainstPendingState(&userTxn), "oops")
}

func TestSign(t *testing.T) {
	testDB := pebble.NewMemTest(t)
	mockCtrl := gomock.NewController(t)
	mockVM := mocks.NewMockVM(mockCtrl)
	bc := blockchain.New(testDB, &utils.Integration)
	seqAddr := utils.HexToFelt(t, "0xDEADBEEF")
	privKey, err := ecdsa.GenerateKey(rand.Reader)
	require.NoError(t, err)
	p := mempool.New(pebble.NewMemTest(t))
	testBuilder := builder.New(privKey, seqAddr, bc, mockVM, 0, p, utils.NewNopZapLogger())

	_, err = testBuilder.Sign(new(felt.Felt), new(felt.Felt))
	require.NoError(t, err)
	// We don't check the signature since the private key generation is not deterministic.
}

func TestReceipt(t *testing.T) {
	trace := &vm.TransactionTrace{
		ConstructorInvocation: &vm.FunctionInvocation{
			ContractAddress: *utils.HexToFelt(t, "0xa1"),
			Events: []vm.OrderedEvent{
				{
					Order: 0,
					Keys:  []*felt.Felt{},
					Data:  []*felt.Felt{},
				},
			},
			ExecutionResources: &vm.ExecutionResources{
				ComputationResources: vm.ComputationResources{
					Pedersen:     4,
					RangeCheck:   4,
					Bitwise:      4,
					Ecdsa:        4,
					EcOp:         4,
					Keccak:       3,
					Poseidon:     2,
					SegmentArena: 1,
					MemoryHoles:  10,
					Steps:        400,
				},
			},
			Messages: []vm.OrderedL2toL1Message{
				{
					Order:   0,
					To:      "0xa2",
					Payload: []*felt.Felt{utils.HexToFelt(t, "0xa3")},
				},
			},
		},
		ValidateInvocation: &vm.FunctionInvocation{
			ContractAddress: *utils.HexToFelt(t, "0xDEADBEEF2"),
			Events:          []vm.OrderedEvent{},
			ExecutionResources: &vm.ExecutionResources{
				ComputationResources: vm.ComputationResources{
					Pedersen:    2,
					RangeCheck:  2,
					Bitwise:     2,
					Ecdsa:       1,
					MemoryHoles: 5,
					Steps:       200,
				},
			},
			Messages: []vm.OrderedL2toL1Message{
				{
					Order:   1,
					To:      "0xDEADBEEF3",
					Payload: []*felt.Felt{utils.HexToFelt(t, "0xDEADBEEF4")},
				},
			},
		},
		ExecuteInvocation: &vm.ExecuteInvocation{
			RevertReason: "oops",
		},
		FeeTransferInvocation: &vm.FunctionInvocation{
			ContractAddress: *utils.HexToFelt(t, "0xe1"),
			Events:          []vm.OrderedEvent{},
			ExecutionResources: &vm.ExecutionResources{
				ComputationResources: vm.ComputationResources{
					Pedersen:    1,
					RangeCheck:  1,
					MemoryHoles: 2,
					Steps:       100,
				},
			},
			Messages: []vm.OrderedL2toL1Message{
				{
					Order:   3,
					To:      "0xe2",
					Payload: []*felt.Felt{utils.HexToFelt(t, "0xe3")},
				},
			},
		},
	}
	want := &core.TransactionReceipt{
		Fee:     utils.HexToFelt(t, "0xDEADBEEF1"),
		FeeUnit: core.STRK,
		Events: []*core.Event{
			{
				From: utils.HexToFelt(t, "0xa1"),
				Keys: []*felt.Felt{},
				Data: []*felt.Felt{},
			},
		},
		ExecutionResources: &core.ExecutionResources{
			BuiltinInstanceCounter: core.BuiltinInstanceCounter{
				Pedersen:     7,
				RangeCheck:   7,
				Bitwise:      6,
				Output:       0,
				Ecsda:        5,
				EcOp:         4,
				Keccak:       3,
				Poseidon:     2,
				SegmentArena: 1,
			},
			MemoryHoles: 17,
			Steps:       700,
		},
		L1ToL2Message: nil,
		L2ToL1Message: []*core.L2ToL1Message{
			{
				From:    utils.HexToFelt(t, "0xa1"),
				To:      common.HexToAddress("0xa2"),
				Payload: []*felt.Felt{utils.HexToFelt(t, "0xa3")},
			},
			{
				From:    utils.HexToFelt(t, "0xDEADBEEF2"),
				To:      common.HexToAddress("0xDEADBEEF3"),
				Payload: []*felt.Felt{utils.HexToFelt(t, "0xDEADBEEF4")},
			},
			{
				From:    utils.HexToFelt(t, "0xe1"),
				To:      common.HexToAddress("0xe2"),
				Payload: []*felt.Felt{utils.HexToFelt(t, "0xe3")},
			},
		},
		TransactionHash: utils.HexToFelt(t, "0x1337"),
		Reverted:        true,
		RevertReason:    "oops",
	}
	got := builder.Receipt(want.Fee, want.FeeUnit, want.TransactionHash, trace)
	require.Equal(t, want, got)
}

func TestBuildTwoEmptyBlocks(t *testing.T) {
	testDB := pebble.NewMemTest(t)
	mockCtrl := gomock.NewController(t)
	mockVM := mocks.NewMockVM(mockCtrl)
	bc := blockchain.New(testDB, &utils.Integration)
	require.NoError(t, bc.StoreGenesis(core.EmptyStateDiff(), nil))
	seqAddr := utils.HexToFelt(t, "0xDEADBEEF")
	privKey, err := ecdsa.GenerateKey(rand.Reader)
	require.NoError(t, err)
	p := mempool.New(pebble.NewMemTest(t))
	minHeight := uint64(2)
	testBuilder := builder.New(privKey, seqAddr, bc, mockVM, time.Millisecond, p, utils.NewNopZapLogger())

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		waitForBlock(t, bc, time.Second, minHeight)
		cancel()
	}()
	require.NoError(t, testBuilder.Run(ctx))

	height, err := bc.Height()
	require.NoError(t, err)
	require.GreaterOrEqual(t, height, minHeight)
	for i := uint64(0); i < height; i++ {
		block, err := bc.BlockByNumber(i + 1)
		require.NoError(t, err)
		require.Equal(t, seqAddr, block.SequencerAddress)
		require.Empty(t, block.Transactions)
		require.Empty(t, block.Receipts)
	}
}

func TestBuildBlocks(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	bc := blockchain.New(pebble.NewMemTest(t), &utils.Integration)
	require.NoError(t, bc.StoreGenesis(core.EmptyStateDiff(), map[felt.Felt]core.Class{}))
	mockVM := mocks.NewMockVM(mockCtrl)

	seqAddr := utils.HexToFelt(t, "0xDEADBEEF")
	privKey, err := ecdsa.GenerateKey(rand.Reader)
	require.NoError(t, err)
	p := mempool.New(pebble.NewMemTest(t))
	testBuilder := builder.New(privKey, seqAddr, bc, mockVM, time.Millisecond, p, utils.NewNopZapLogger())

	txnHashes := []*felt.Felt{}
	for i := uint64(0); i < 100; i++ {
		invokeTxn := &core.InvokeTransaction{
			TransactionHash: new(felt.Felt).SetUint64(i),
			Version:         new(core.TransactionVersion),
		}
		require.NoError(t, p.Push(&mempool.BroadcastedTransaction{
			Transaction: invokeTxn,
		}))

		var executionErr error
		if i%10 == 0 {
			executionErr = vm.TransactionExecutionError{}
		} else {
			txnHashes = append(txnHashes, invokeTxn.TransactionHash)
		}

		mockVM.EXPECT().Execute([]core.Transaction{invokeTxn}, gomock.Any(), gomock.Any(), gomock.Any(),
			gomock.Any(), gomock.Any(), false, false, false, false).Return(
			[]*felt.Felt{&felt.Zero}, []*felt.Felt{}, []vm.TransactionTrace{{StateDiff: &vm.StateDiff{}}}, executionErr,
		)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		waitForTxns(t, bc, time.Second, txnHashes)
		cancel()
	}()
	require.NoError(t, testBuilder.Run(ctx))

	var totalTxns uint64
	height, err := bc.Height()
	require.NoError(t, err)
	for i := uint64(0); i < height; i++ {
		block, err := bc.BlockByNumber(i + 1)
		require.NoError(t, err)
		totalTxns += block.TransactionCount
	}
	require.Equal(t, uint64(90), totalTxns)
}

func TestSepoliaBootstrap(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	bc := blockchain.New(pebble.NewMemTest(t), &utils.Sepolia)
	snData := mocks.NewMockStarknetData(mockCtrl)
	p := mempool.New(pebble.NewMemTest(t))
	vmm := vm.New(utils.NewNopZapLogger())
	seqAddr := utils.HexToFelt(t, "0xDEADBEEF")
	privKey, err := ecdsa.GenerateKey(rand.Reader)
	require.NoError(t, err)

	blockTime := time.Second
	testBuilder := builder.New(privKey, seqAddr, bc, vmm, blockTime, p, utils.NewNopZapLogger()).
		WithBootstrapToBlock(2).
		WithStarknetData(snData).
		WithBootstrap(true)

	client := feeder.NewTestClient(t, &utils.Sepolia)
	gw := adaptfeeder.New(client)

	var i uint64
	var block *core.Block
	var err2 error
	for i = 0; i < 2; i++ {
		block, err2 = gw.BlockByNumber(context.Background(), i)
		require.NoError(t, err2)

		su, err2 := gw.StateUpdate(context.Background(), i)
		require.NoError(t, err2)

		snData.EXPECT().BlockByNumber(context.Background(), i).Return(block, nil)
		snData.EXPECT().StateUpdate(context.Background(), i).Return(su, nil)
	}
	classHashes := []string{
		"0x5c478ee27f2112411f86f207605b2e2c58cdb647bac0df27f660ef2252359c6",
		"0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3",
		"0x1b661756bf7d16210fc611626e1af4569baa1781ffc964bd018f4585ae241c1",
	}

	for _, hash := range classHashes {
		classHash := utils.HexToFelt(t, hash)
		class, err2 := gw.Class(context.Background(), classHash)
		require.NoError(t, err2)
		snData.EXPECT().Class(context.Background(), classHash).Return(class, nil)
	}

	t.Run("Bootstrap blocks 0 and 1 + Run block 2", func(t *testing.T) {
		block, err := gw.BlockByNumber(context.Background(), 2)
		require.NoError(t, err)
		txns := block.Transactions
		var mempoolTxns []*mempool.BroadcastedTransaction
		for _, txn := range txns {
			switch tx := txn.(type) {
			case *core.DeployTransaction, *core.DeployAccountTransaction, *core.InvokeTransaction, *core.L1HandlerTransaction:
				mempoolTxns = append(mempoolTxns,
					&mempool.BroadcastedTransaction{
						Transaction: tx,
					})
			case *core.DeclareTransaction:
				class, err2 := gw.Class(context.Background(), tx.ClassHash)
				require.NoError(t, err2)
				mempoolTxns = append(mempoolTxns, &mempool.BroadcastedTransaction{
					Transaction:   tx,
					DeclaredClass: class,
				})
			default:
				require.Error(t, errors.New("unknown transaction type"))
			}
		}

		for _, txn := range mempoolTxns {
			err = p.Push(txn)
			require.NoError(t, err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*blockTime)
		defer cancel()
		err = testBuilder.Run(ctx)
		require.NoError(t, err)
		head, err := bc.BlockByNumber(1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), head.Number)
		require.Equal(t, block.TransactionCount, head.TransactionCount, "TransactionCount diff")
		require.Equal(t, block.GlobalStateRoot.String(), head.GlobalStateRoot.String(), "GlobalStateRoot diff")
		head, err = bc.Head()
		require.NoError(t, err)
		require.Equal(t, block.Number, head.Number)
		require.Equal(t, block.TransactionCount, head.TransactionCount)
		require.Equal(t, head.GlobalStateRoot, head.GlobalStateRoot)
	})
}

func TestPrefundedAccounts(t *testing.T) {
	network := &utils.Mainnet
	bc := blockchain.New(pebble.NewMemTest(t), network)
	log := utils.NewNopZapLogger()
	seqAddr := utils.HexToFelt(t, "0xDEADBEEF")
	privKey, err := ecdsa.GenerateKey(rand.Reader)
	require.NoError(t, err)
	p := mempool.New(pebble.NewMemTest(t))

	genesisConfig, err := genesis.Read("../genesis/genesis_prefund_accounts.json")
	require.NoError(t, err)
	genesisConfig.Classes = []string{"../genesis/classes/strk.json", "../genesis/classes/account.json"}
	diff, classes, err := genesis.GenesisStateDiff(genesisConfig, vm.New(log), bc.Network())
	require.NoError(t, err)
	require.NoError(t, bc.StoreGenesis(diff, classes))

	testBuilder := builder.New(privKey, seqAddr, bc, vm.New(log), 100*time.Millisecond, p, log).WithPrefundAccounts(true)
	rpcHandler := rpc.New(bc, nil, nil, "", log).WithMempool(p)

	// transfer tokens to 0x101
	invokeTxn := rpc.BroadcastedTransaction{
		Transaction: rpc.Transaction{
			Type:          rpc.TxnInvoke,
			SenderAddress: utils.HexToFelt(t, "0x406a8f52e741619b17410fc90774e4b36f968e1a71ae06baacfe1f55d987923"),
			Version:       new(felt.Felt).SetUint64(1),
			MaxFee:        utils.HexToFelt(t, "0xaeb1bacb2c"),
			Nonce:         new(felt.Felt).SetUint64(0),
			Signature: &[]*felt.Felt{
				utils.HexToFelt(t, "0x239a9d44d7b7dd8d31ba0d848072c22643beb2b651d4e2cd8a9588a17fd6811"),
				utils.HexToFelt(t, "0x6e7d805ee0cc02f3790ab65c8bb66b235341f97d22d6a9a47dc6e4fdba85972"),
			},
			CallData: &[]*felt.Felt{
				utils.HexToFelt(t, "0x1"),
				utils.HexToFelt(t, "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7"),
				utils.HexToFelt(t, "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e"),
				utils.HexToFelt(t, "0x3"),
				utils.HexToFelt(t, "0x101"),
				utils.HexToFelt(t, "0x12345678"),
				utils.HexToFelt(t, "0x0"),
			},
		},
	}
	invokeTxn2 := invokeTxn
	invokeTxn2.Nonce = new(felt.Felt).SetUint64(1)
	invokeTxn2.Signature = &[]*felt.Felt{
		utils.HexToFelt(t, "0x1dd3f747f9d9c05e29c08acc6f2c27fcb6e00fcea9f192d367b900347a08c25"),
		utils.HexToFelt(t, "0x46aabe43c9174e84b00b50258ed25086091d969fa12dc1f3d64f93114694b9"),
	}

	addTransactionsAndRunTest := func(t *testing.T, txns ...rpc.BroadcastedTransaction) (uint64, *felt.Felt, bool) {
		for _, txn := range txns {
			rpcHandler.AddTransaction(context.Background(), txn)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 1300*time.Millisecond)
		defer cancel()
		require.NoError(t, testBuilder.Run(ctx))

		height, err := bc.Height()
		require.NoError(t, err)
		expectedBalance := new(felt.Felt).Add(utils.HexToFelt(t, "0x123456789123"), utils.HexToFelt(t, "0x12345678"))
		if len(txns) > 1 {
			expectedBalance = new(felt.Felt).Add(expectedBalance, utils.HexToFelt(t, "0x12345678"))
		}

		var foundNumTxnsInBlock uint64
		for i := uint64(0); i < height; i++ {
			block, err := bc.BlockByNumber(i + 1)
			require.NoError(t, err)
			if block.TransactionCount != 0 {
				foundNumTxnsInBlock += block.TransactionCount
			}
		}
		require.NotEqual(t, 0, foundNumTxnsInBlock, "Failed to find any transactions in the block")

		foundExpectedBalance := false
		for i := uint64(0); i < height; i++ {
			su, err := bc.StateUpdateByNumber(i + 1)
			require.NoError(t, err)
			for _, store := range su.StateDiff.StorageDiffs {
				for _, val := range store {
					if val.Equal(expectedBalance) {
						foundExpectedBalance = true
					}
				}
			}
			if foundExpectedBalance {
				break
			}
		}

		return foundNumTxnsInBlock, expectedBalance, foundExpectedBalance
	}

	t.Run("single transaction in the block", func(t *testing.T) {
		foundNumTxnsInBlock, _, foundExpectedBalance := addTransactionsAndRunTest(t, invokeTxn)
		require.Equal(t, uint64(1), foundNumTxnsInBlock)
		require.True(t, foundExpectedBalance)
	})

	t.Run("two transactions in the same block", func(t *testing.T) {
		foundNumTxnsInBlock, _, foundExpectedBalance := addTransactionsAndRunTest(t, invokeTxn, invokeTxn2)
		require.Equal(t, uint64(2), foundNumTxnsInBlock)
		require.True(t, foundExpectedBalance)
	})
}
