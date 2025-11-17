package sequencer_test

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/statetestutils"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/genesis"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/mocks"
	rpc "github.com/NethermindEth/juno/rpc/v8"
	"github.com/NethermindEth/juno/sequencer"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/ecdsa"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func getEmptySequencer(t *testing.T, blockTime time.Duration, seqAddr *felt.Felt) (sequencer.Sequencer, *blockchain.Blockchain) {
	t.Helper()
	testDB := memory.New()
	mockCtrl := gomock.NewController(t)
	mockVM := mocks.NewMockVM(mockCtrl)
	network := &utils.Mainnet
	bc := blockchain.New(testDB, network, statetestutils.UseNewState())
	emptyStateDiff := core.EmptyStateDiff()
	require.NoError(t, bc.StoreGenesis(&emptyStateDiff, nil))
	privKey, err := ecdsa.GenerateKey(rand.Reader)
	require.NoError(t, err)
	log := utils.NewNopZapLogger()
	p := mempool.New(memory.New(), bc, 1000, log)

	executor := builder.NewExecutor(bc, mockVM, log, false, false)
	testBuilder := builder.New(bc, executor)
	return sequencer.New(&testBuilder, p, seqAddr, privKey, blockTime, log), bc
}

// Sequencer contains prefunded accounts.
// We also return two invoke txns that are ready to be executed by the sequencer.
func getGenesisSequencer(
	t *testing.T,
	blockTime time.Duration,
	seqAddr *felt.Felt) (
	sequencer.Sequencer,
	*blockchain.Blockchain,
	*rpc.Handler,
	[2]rpc.BroadcastedTransaction,
) {
	t.Helper()
	// transfer tokens to 0x105
	invokeTxn := rpc.BroadcastedTransaction{ //nolint:dupl
		Transaction: rpc.Transaction{
			Type:          rpc.TxnInvoke,
			SenderAddress: felt.NewUnsafeFromString[felt.Felt]("0x101"),
			Version:       new(felt.Felt).SetUint64(1),
			MaxFee:        felt.NewUnsafeFromString[felt.Felt]("0xaeb1bacb2c"),
			Nonce:         new(felt.Felt).SetUint64(0),
			Signature: &[]*felt.Felt{
				felt.NewUnsafeFromString[felt.Felt]("0x239a9d44d7b7dd8d31ba0d848072c22643beb2b651d4e2cd8a9588a17fd6811"),
				felt.NewUnsafeFromString[felt.Felt]("0x6e7d805ee0cc02f3790ab65c8bb66b235341f97d22d6a9a47dc6e4fdba85972"),
			},
			CallData: &[]*felt.Felt{
				felt.NewUnsafeFromString[felt.Felt]("0x1"),
				felt.NewUnsafeFromString[felt.Felt]("0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7"),
				felt.NewUnsafeFromString[felt.Felt]("0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e"),
				felt.NewUnsafeFromString[felt.Felt]("0x3"),
				felt.NewUnsafeFromString[felt.Felt]("0x108"),
				felt.NewUnsafeFromString[felt.Felt]("0x1"),
				felt.NewUnsafeFromString[felt.Felt]("0x0"),
			},
		},
	}
	// transfer tokens to 0x106
	invokeTxn2 := rpc.BroadcastedTransaction{ //nolint:dupl
		Transaction: rpc.Transaction{
			Type:          rpc.TxnInvoke,
			SenderAddress: felt.NewUnsafeFromString[felt.Felt]("0x101"),
			Version:       new(felt.Felt).SetUint64(1),
			MaxFee:        felt.NewUnsafeFromString[felt.Felt]("0xaeb1bacb2c"),
			Nonce:         new(felt.Felt).SetUint64(1),
			Signature: &[]*felt.Felt{
				felt.NewUnsafeFromString[felt.Felt]("0x6012e655ac15a4ab973a42db121a2cb78d9807c5ff30aed74b70d32a682b083"),
				felt.NewUnsafeFromString[felt.Felt]("0xcd27013a24e143cc580ba788b14df808aefa135d8ed3aca297aa56aa632cb5"),
			},
			CallData: &[]*felt.Felt{
				felt.NewUnsafeFromString[felt.Felt]("0x1"),
				felt.NewUnsafeFromString[felt.Felt]("0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7"),
				felt.NewUnsafeFromString[felt.Felt]("0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e"),
				felt.NewUnsafeFromString[felt.Felt]("0x3"),
				felt.NewUnsafeFromString[felt.Felt]("0x109"),
				felt.NewUnsafeFromString[felt.Felt]("0x1"),
				felt.NewUnsafeFromString[felt.Felt]("0x0"),
			},
		},
	}

	testDB := memory.New()
	network := &utils.Mainnet
	bc := blockchain.New(testDB, network, statetestutils.UseNewState())
	log := utils.NewNopZapLogger()
	privKey, err := ecdsa.GenerateKey(rand.Reader)
	require.NoError(t, err)
	txnPool := mempool.New(testDB, bc, 1000, utils.NewNopZapLogger())

	genesisConfig, err := genesis.Read("../genesis/genesis_prefund_accounts.json")
	require.NoError(t, err)
	genesisConfig.Classes = []string{
		"../genesis/classes/strk.json", "../genesis/classes/account.json",
		"../genesis/classes/universaldeployer.json", "../genesis/classes/udacnt.json",
	}

	feeTokens := utils.DefaultFeeTokenAddresses
	chainInfo := vm.ChainInfo{
		ChainID:           network.L2ChainID,
		FeeTokenAddresses: feeTokens,
	}
	diff, classes, err := genesis.GenesisStateDiff(
		genesisConfig,
		vm.New(&chainInfo, false, log),
		bc.Network(),
		vm.DefaultMaxSteps,
		vm.DefaultMaxGas,
	)
	require.NoError(t, err)
	require.NoError(t, bc.StoreGenesis(&diff, classes))
	executor := builder.NewExecutor(bc, vm.New(&chainInfo, false, log), log, false, true)
	testBuilder := builder.New(bc, executor)
	rpcHandler := rpc.New(bc, nil, nil, utils.NewNopZapLogger()).WithMempool(txnPool)
	return sequencer.New(&testBuilder, txnPool, seqAddr, privKey, blockTime, log), bc, rpcHandler, [2]rpc.BroadcastedTransaction{invokeTxn, invokeTxn2}
}

func TestBuildEmptyBlocks(t *testing.T) {
	seqAddr := felt.NewUnsafeFromString[felt.Felt]("0xDEADBEEF")
	blockTime := 100 * time.Millisecond
	seq, bc := getEmptySequencer(t, blockTime, seqAddr)

	ctx, cancel := context.WithTimeout(t.Context(), 5*blockTime)
	defer cancel()
	require.NoError(t, seq.Run(ctx))

	height, err := bc.Height()
	require.NoError(t, err)
	require.GreaterOrEqual(t, height, uint64(0))
	for i := range height {
		block, err := bc.BlockByNumber(i + 1)
		require.NoError(t, err)
		require.Equal(t, seqAddr, block.SequencerAddress)
		require.Empty(t, block.Transactions)
		require.Empty(t, block.Receipts)
	}
}

func TestPrefundedAccounts(t *testing.T) {
	seqAddr := felt.NewUnsafeFromString[felt.Felt]("0xDEADBEEF")
	blockTime := 100 * time.Millisecond
	seq, bc, rpcHandler, txnsToExecute := getGenesisSequencer(t, blockTime, seqAddr)

	// Add txns to the mempool via RPC
	_, rpcErr := rpcHandler.AddTransaction(t.Context(), &txnsToExecute[0])
	require.Nil(t, rpcErr)
	_, rpcErr = rpcHandler.AddTransaction(t.Context(), &txnsToExecute[1])
	require.Nil(t, rpcErr)

	ctx, cancel := context.WithTimeout(t.Context(), 2*blockTime)
	defer cancel()
	require.NoError(t, seq.Run(ctx))

	height, err := bc.Height()
	require.NoError(t, err)

	expectedBalance := felt.NewUnsafeFromString[felt.Felt]("0x1")
	numExpectedBalance := 0
	foundExpectedNumAcntsWBalance := false
	for i := range height {
		su, err := bc.StateUpdateByNumber(i + 1)
		require.NoError(t, err)
		for _, store := range su.StateDiff.StorageDiffs {
			for _, val := range store {
				if val.Equal(expectedBalance) {
					numExpectedBalance++
				}
			}
		}
		if numExpectedBalance == len(txnsToExecute) {
			foundExpectedNumAcntsWBalance = true
			break
		}
	}
	require.True(t, foundExpectedNumAcntsWBalance)
}

// Example of how other tests can use the sequencer to execute txns in a controlled manner.
func TestRunOnce(t *testing.T) {
	seqAddr := felt.NewUnsafeFromString[felt.Felt]("0xDEADBEEF")
	blockTime := 100 * time.Millisecond
	seq, bc, rpcHandler, txnsToExecute := getGenesisSequencer(t, blockTime, seqAddr)

	// Build an empty block
	_, err := seq.RunOnce()
	require.NoError(t, err)

	// Add txns to the mempool via RPC
	_, rpcErr := rpcHandler.AddTransaction(t.Context(), &txnsToExecute[0])
	require.Nil(t, rpcErr)
	_, rpcErr = rpcHandler.AddTransaction(t.Context(), &txnsToExecute[1])
	require.Nil(t, rpcErr)

	// Build an non-empty block
	_, err = seq.RunOnce()
	require.NoError(t, err)

	block, err := bc.BlockByNumber(2)
	require.NoError(t, err)
	require.Equal(t, seqAddr, block.SequencerAddress)
	require.NotEmpty(t, block.Transactions)
	require.NotEmpty(t, block.Receipts)
}

func TestHelpers(t *testing.T) {
	seqAddr := felt.NewUnsafeFromString[felt.Felt]("0xDEADBEEF")
	blockTime := 100 * time.Millisecond
	seq, _, _, _ := getGenesisSequencer(t, blockTime, seqAddr) //nolint:dogsled

	_, err := seq.RunOnce()
	require.NoError(t, err)

	pending, err := seq.Pending()
	require.NoError(t, err)
	require.NotNil(t, pending)

	block := seq.PendingBlock()
	require.NotNil(t, block)

	state, closer, err := seq.PendingState()
	require.NoError(t, err)
	require.NotNil(t, state)
	require.NotNil(t, closer)
	require.NoError(t, closer())

	header := seq.HighestBlockHeader()
	require.Nil(t, header)

	num, err := seq.StartingBlockNumber()
	require.NoError(t, err)
	require.Equal(t, uint64(0), num)

	reorgSub := seq.SubscribeReorg()
	require.NotNil(t, reorgSub)
	require.NotNil(t, reorgSub.Subscription)

	newHeadsSub := seq.SubscribeNewHeads()
	require.NotNil(t, newHeadsSub)
	require.NotNil(t, newHeadsSub.Subscription)

	pendingDataSub := seq.SubscribePendingData()
	require.NotNil(t, pendingDataSub)
	require.NotNil(t, pendingDataSub.Subscription)
}
