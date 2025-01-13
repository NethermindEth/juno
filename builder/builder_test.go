package builder_test

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/NethermindEth/juno/adapters/vm2core"
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

func TestSign(t *testing.T) {
	testDB := pebble.NewMemTest(t)
	mockCtrl := gomock.NewController(t)
	mockVM := mocks.NewMockVM(mockCtrl)
	bc := blockchain.New(testDB, &utils.Integration, nil)
	seqAddr := utils.HexToFelt(t, "0xDEADBEEF")
	privKey, err := ecdsa.GenerateKey(rand.Reader)
	require.NoError(t, err)
	p := mempool.New(pebble.NewMemTest(t))
	testBuilder := builder.New(privKey, seqAddr, bc, mockVM, 0, p, utils.NewNopZapLogger(), false, testDB)

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
	txnReceipt := vm.TransactionReceipt{Gas: vm.GasConsumed{L1Gas: 1, L1DataGas: 2}}
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
			MemoryHoles:      17,
			Steps:            700,
			TotalGasConsumed: &core.GasConsumed{L1Gas: 1, L1DataGas: 2},
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
	got := vm2core.Receipt(want.Fee, want.FeeUnit, want.TransactionHash, trace, &txnReceipt)
	require.Equal(t, want, got)
}

func TestBuildTwoEmptyBlocks(t *testing.T) {
	testDB := pebble.NewMemTest(t)
	mockCtrl := gomock.NewController(t)
	mockVM := mocks.NewMockVM(mockCtrl)
	bc := blockchain.New(testDB, &utils.Integration, nil)
	require.NoError(t, bc.StoreGenesis(core.EmptyStateDiff(), nil))
	seqAddr := utils.HexToFelt(t, "0xDEADBEEF")
	privKey, err := ecdsa.GenerateKey(rand.Reader)
	require.NoError(t, err)
	p := mempool.New(pebble.NewMemTest(t))
	minHeight := uint64(2)
	testBuilder := builder.New(privKey, seqAddr, bc, mockVM, time.Millisecond, p, utils.NewNopZapLogger(), false, testDB)

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

func TestPrefundedAccounts(t *testing.T) {
	// transfer tokens to 0x101
	invokeTxn := rpc.BroadcastedTransaction{ //nolint:dupl
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
	// transfer tokens to 0x102
	invokeTxn2 := rpc.BroadcastedTransaction{ //nolint:dupl
		Transaction: rpc.Transaction{
			Type:          rpc.TxnInvoke,
			SenderAddress: utils.HexToFelt(t, "0x0406a8f52e741619b17410fc90774e4b36f968e1a71ae06baacfe1f55d987923"),
			Version:       new(felt.Felt).SetUint64(1),
			MaxFee:        utils.HexToFelt(t, "0xaeb1bacb2c"),
			Nonce:         new(felt.Felt).SetUint64(1),
			Signature: &[]*felt.Felt{
				utils.HexToFelt(t, "0x6012e655ac15a4ab973a42db121a2cb78d9807c5ff30aed74b70d32a682b083"),
				utils.HexToFelt(t, "0xcd27013a24e143cc580ba788b14df808aefa135d8ed3aca297aa56aa632cb5"),
			},
			CallData: &[]*felt.Felt{
				utils.HexToFelt(t, "0x1"),
				utils.HexToFelt(t, "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7"),
				utils.HexToFelt(t, "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e"),
				utils.HexToFelt(t, "0x3"),
				utils.HexToFelt(t, "0x102"),
				utils.HexToFelt(t, "0x12345678"),
				utils.HexToFelt(t, "0x0"),
			},
		},
	}

	expectedExnsInBlock := []rpc.BroadcastedTransaction{invokeTxn, invokeTxn2}
	testDB := pebble.NewMemTest(t)
	network := &utils.Mainnet
	bc := blockchain.New(testDB, network, nil)
	log := utils.NewNopZapLogger()
	seqAddr := utils.HexToFelt(t, "0xDEADBEEF")
	privKey, err := ecdsa.GenerateKey(rand.Reader)
	require.NoError(t, err)
	p := mempool.New(pebble.NewMemTest(t))

	genesisConfig, err := genesis.Read("../genesis/genesis_prefund_accounts.json")
	require.NoError(t, err)
	genesisConfig.Classes = []string{
		"../genesis/classes/strk.json", "../genesis/classes/account.json",
		"../genesis/classes/universaldeployer.json", "../genesis/classes/udacnt.json",
	}
	diff, classes, err := genesis.GenesisStateDiff(genesisConfig, vm.New(false, log), bc.Network(), 40000000) //nolint:gomnd
	require.NoError(t, err)
	require.NoError(t, bc.StoreGenesis(diff, classes))
	testBuilder := builder.New(privKey, seqAddr, bc, vm.New(false, log), 1000*time.Millisecond, p, log, false, testDB)
	rpcHandler := rpc.New(bc, nil, nil, "", log).WithMempool(p)
	for _, txn := range expectedExnsInBlock {
		rpcHandler.AddTransaction(context.Background(), txn)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()
	require.NoError(t, testBuilder.Run(ctx))

	height, err := bc.Height()
	require.NoError(t, err)
	for i := uint64(0); i < height; i++ {
		block, err := bc.BlockByNumber(i + 1)
		require.NoError(t, err)
		if block.TransactionCount != 0 {
			require.Equal(t, len(expectedExnsInBlock), int(block.TransactionCount), "Failed to find correct number of transactions in the block")
		}
	}

	expectedBalance := new(felt.Felt).Add(utils.HexToFelt(t, "0x56bc75e2d63100000"), utils.HexToFelt(t, "0x12345678"))
	foundExpectedBalance := false
	numExpectedBalance := 0
	for i := uint64(0); i < height; i++ {
		su, err := bc.StateUpdateByNumber(i + 1)
		require.NoError(t, err)
		for _, store := range su.StateDiff.StorageDiffs {
			for _, val := range store {
				if val.Equal(expectedBalance) {
					foundExpectedBalance = true
					numExpectedBalance++
				}
			}
		}
		if foundExpectedBalance {
			break
		}
	}
	require.Equal(t, len(expectedExnsInBlock), numExpectedBalance, "Accounts don't have the expected balance")
	require.True(t, foundExpectedBalance)
}

func TestShadowSepolia(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	snData := mocks.NewMockStarknetData(mockCtrl)
	network := &utils.Sepolia
	testDB := pebble.NewMemTest(t)
	bc := blockchain.New(testDB, network, nil)
	p := mempool.New(pebble.NewMemTest(t))
	log := utils.NewNopZapLogger()
	vmm := vm.New(false, log)
	seqAddr := utils.HexToFelt(t, "0xDEADBEEF")
	privKey, err := ecdsa.GenerateKey(rand.Reader)
	require.NoError(t, err)

	blockTime := time.Second
	testBuilder := builder.NewShadow(privKey, seqAddr, bc, vmm, blockTime, p, log, snData, testDB).WithSyncToBlock(0)
	gw := adaptfeeder.New(feeder.NewTestClient(t, network))

	const numTestBlocks = 3 // Note: depends on the number of blocks that the buidler syncStores (see Run())
	var blocks [numTestBlocks]*core.Block
	for i := 0; i < numTestBlocks; i++ {
		block, err2 := gw.BlockByNumber(context.Background(), uint64(i))
		require.NoError(t, err2)
		blocks[i] = block
		su, err2 := gw.StateUpdate(context.Background(), uint64(i))
		require.NoError(t, err2)
		snData.EXPECT().BlockByNumber(context.Background(), uint64(i)).Return(block, nil)
		snData.EXPECT().StateUpdate(context.Background(), uint64(i)).Return(su, nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), numTestBlocks*blockTime)
	defer cancel()

	snData.EXPECT().BlockLatest(gomock.Any()).Return(&core.Block{Header: &core.Header{Number: numTestBlocks}}, nil).Times(3)

	classHashes := []string{
		"0x5c478ee27f2112411f86f207605b2e2c58cdb647bac0df27f660ef2252359c6",
		"0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3",
		"0x1b661756bf7d16210fc611626e1af4569baa1781ffc964bd018f4585ae241c1",
		"0x4f23a756b221f8ce46b72e6a6b10ee7ee6cf3b59790e76e02433104f9a8c5d1",
	}
	for _, hash := range classHashes {
		classHash := utils.HexToFelt(t, hash)
		class, err2 := gw.Class(context.Background(), classHash)
		require.NoError(t, err2)
		snData.EXPECT().Class(context.Background(), classHash).Return(class, nil)
	}
	err = testBuilder.Run(ctx)
	require.NoError(t, err)
	head, err := bc.Head()
	require.NoError(t, err)
	require.Equal(t, uint64(2), head.Number)
	runTest := func(t *testing.T, wantBlockNum uint64, wantBlock *core.Block) {
		gotBlock, err := bc.BlockByNumber(wantBlockNum)
		require.NoError(t, err)
		require.Equal(t, wantBlock.Number, gotBlock.Number)
		require.Equal(t, wantBlock.TransactionCount, gotBlock.TransactionCount, "TransactionCount diff")
		require.Equal(t, wantBlock.GlobalStateRoot.String(), gotBlock.GlobalStateRoot.String(), "GlobalStateRoot diff")
	}
	for i := range numTestBlocks {
		runTest(t, uint64(i), blocks[i])
	}
}
