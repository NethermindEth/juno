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

func TestValidateAgainstPendingState(t *testing.T) {
	testDB := pebble.NewMemTest(t)
	mockCtrl := gomock.NewController(t)
	mockVM := mocks.NewMockVM(mockCtrl)
	bc := blockchain.New(testDB, &utils.Integration)
	seqAddr := utils.HexToFelt(t, "0xDEADBEEF")
	testBuilder := builder.New(nil, seqAddr, bc, mockVM, 0, utils.NewNopZapLogger())

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
	testBuilder := builder.New(privKey, seqAddr, bc, mockVM, 0, utils.NewNopZapLogger())

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

type finisher struct {
	i      uint64
	min    uint64
	cancel context.CancelFunc
}

func newFinisher(min uint64, cancel context.CancelFunc) *finisher {
	return &finisher{
		min:    min,
		cancel: cancel,
	}
}

func (f *finisher) OnBlockFinalised(_ *core.Header) {
	if f.i < f.min {
		f.i++
	} else {
		f.cancel()
	}
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

	ctx, cancel := context.WithCancel(context.Background())
	minHeight := uint64(2)
	testBuilder := builder.New(privKey, seqAddr, bc, mockVM, time.Millisecond, utils.NewNopZapLogger()).WithEventListener(&builder.SelectiveListener{
		OnBlockFinalisedCb: newFinisher(minHeight, cancel).OnBlockFinalised,
	})
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
