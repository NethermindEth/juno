package builder_test

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/mocks"
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
	blockInfo := vm.BlockInfo{
		Header: b.Header,
	}
	paidFeesOnL1 := []*felt.Felt{}
	mockVM.EXPECT().Execute([]core.Transaction{userTxn.Transaction},
		[]core.Class{userTxn.DeclaredClass}, paidFeesOnL1, blockInfo, gomock.Any(), utils.Integration, false, false, false, false).Return(nil, nil, nil)
	assert.NoError(t, testBuilder.ValidateAgainstPendingState(&userTxn))

	require.NoError(t, bc.Store(b, &core.BlockCommitments{}, su, nil))

	mockVM.EXPECT().Execute([]core.Transaction{userTxn.Transaction},
		[]core.Class{userTxn.DeclaredClass}, paidFeesOnL1, blockInfo, gomock.Any(), utils.Integration, false, false, false, false).Return(nil, nil, nil)
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
				}},
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
	require.NoError(t, bc.StoreGenesis(core.EmptyStateDiff(), nil))
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

		mockVM.EXPECT().Execute([]core.Transaction{invokeTxn}, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), false, false, false, false).Return(
			[]*felt.Felt{&felt.Zero}, []vm.TransactionTrace{
				{},
			}, executionErr,
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
