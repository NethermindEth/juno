package proposer

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/genesis"
	"github.com/NethermindEth/juno/mempool"
	rpc "github.com/NethermindEth/juno/rpc/v8"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/ecdsa"
	"github.com/stretchr/testify/require"
)

// Create a builder, with a block which includes fee tokens, and some pre-funded accounts.
// This allows us to execute real txns in the tests.
func getCustomBuilder(t *testing.T, seqAddr *felt.Felt) (*builder.Builder, *rpc.Handler) {
	protocolVersion := semver.New(0, 13, 2, "", "")
	t.Helper()

	invokeTxn := mempool.BroadcastedTransaction{
		Transaction: &core.InvokeTransaction{
			TransactionHash: utils.HexToFelt(t, "0x3ecb47a4945b98115f404c5fd9893f624c0066a164a2ac0ac53bbfc5fef3485"),
			SenderAddress:   utils.HexToFelt(t, "0x406a8f52e741619b17410fc90774e4b36f968e1a71ae06baacfe1f55d987923"),
			Version:         new(core.TransactionVersion).SetUint64(1),
			MaxFee:          utils.HexToFelt(t, "0xaeb1bacb2c"),
			Nonce:           new(felt.Felt).SetUint64(0),
			TransactionSignature: []*felt.Felt{
				utils.HexToFelt(t, "0x239a9d44d7b7dd8d31ba0d848072c22643beb2b651d4e2cd8a9588a17fd6811"),
				utils.HexToFelt(t, "0x6e7d805ee0cc02f3790ab65c8bb66b235341f97d22d6a9a47dc6e4fdba85972"),
			},
			CallData: []*felt.Felt{
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
	network := &utils.Mainnet
	testDB := memory.New()

	bc := blockchain.New(testDB, network)
	log := utils.NewNopZapLogger()

	privKey, err := ecdsa.GenerateKey(rand.Reader)
	require.NoError(t, err)
	txnPool := mempool.New(testDB, bc, 1024, utils.NewNopZapLogger())

	genesisConfig, err := genesis.Read("../../genesis/genesis_prefund_accounts.json")
	require.NoError(t, err)
	genesisConfig.Classes = []string{
		"../../genesis/classes/strk.json", "../../genesis/classes/account.json",
		"../../genesis/classes/universaldeployer.json", "../../genesis/classes/udacnt.json",
	}
	diff, classes, err := genesis.GenesisStateDiff(genesisConfig, vm.New(false, log), bc.Network(), 40000000) //nolint:gomnd
	require.NoError(t, err)
	require.NoError(t, bc.StoreGenesis(&diff, classes))

	blockTime := 100 * time.Millisecond
	testBuilder := builder.New(privKey, seqAddr, bc, vm.New(false, log), blockTime, txnPool, log, false, testDB, *protocolVersion)
	rpcHandler := rpc.New(bc, nil, nil, "", log).WithMempool(txnPool)

	// Build the block
	require.NoError(t, testBuilder.ClearPending())
	require.NoError(t, testBuilder.InitPendingBlock())
	err = testBuilder.ExecuteTxns([]mempool.BroadcastedTransaction{invokeTxn})
	require.NoError(t, err)
	require.NoError(t, testBuilder.Finalise(nil))
	require.True(t, testBuilder.MempoolIsEmpty())
	return &testBuilder, rpcHandler
}

func TestEmptyProposal(t *testing.T) {
	proposerAddr := new(felt.Felt).SetUint64(123123)
	builder, _ := getCustomBuilder(t, proposerAddr)
	proposer := New(builder)

	// Step 1: ProposalInit()
	pInit, err := proposer.ProposalInit()
	require.NoError(t, err)
	require.NotEmpty(t, pInit)

	// Step 2: BlockInfo() returns (zero,false)
	blockInfoTimeout := 100 * time.Millisecond
	ctx, cancel := context.WithTimeout(t.Context(), blockInfoTimeout)
	_, blockNonEmpty := proposer.BlockInfo(ctx)
	cancel()
	require.False(t, blockNonEmpty)

	// Step 3: ProposalCommitment()
	pCommitment, err := proposer.ProposalCommitment()
	require.NoError(t, err)
	require.NotEmpty(t, pCommitment)

	// Step 4: ProposalFin()
	pFin, err := proposer.ProposalFin()
	require.NoError(t, err)
	require.NotEmpty(t, pFin)
}

func TestNonEmptyProposal(t *testing.T) {
	proposerAddr := new(felt.Felt).SetUint64(123123)
	builder, rpcHandler := getCustomBuilder(t, proposerAddr)
	proposer := New(builder)

	// Step 1: ProposalInit()
	pInit, err := proposer.ProposalInit()
	require.NoError(t, err)
	require.NotEmpty(t, pInit)

	// Add txn to the mempool (send tokens to "0x102")
	txn := rpc.BroadcastedTransaction{
		Transaction: rpc.Transaction{
			Type:          rpc.TxnInvoke,
			Hash:          utils.HexToFelt(t, "0x722e584df0c18fcda54552ae5055f6c1fda331c4ae5de7ec5fc0376ae8b9a7f"),
			SenderAddress: utils.HexToFelt(t, "0x0406a8f52e741619b17410fc90774e4b36f968e1a71ae06baacfe1f55d987923"),
			Version:       utils.HexToFelt(t, "1"),
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
	_, rpcErr := rpcHandler.AddTransaction(t.Context(), txn)
	require.Nil(t, rpcErr)

	// Step 2: BlockInfo() returns (zero,false)
	blockInfoTimeout := 100 * time.Millisecond
	ctx, cancel := context.WithTimeout(t.Context(), blockInfoTimeout)
	defer cancel() // We should return without the timeout
	blockInfo, blockNonEmpty := proposer.BlockInfo(ctx)
	require.True(t, blockNonEmpty)
	require.NotEmpty(t, blockInfo)

	// Step 4: Txns
	txnExecutionTimeout := 1000 * time.Millisecond
	ctxTxn, cancelTxn := context.WithTimeout(t.Context(), txnExecutionTimeout)
	go func() {
		time.Sleep(txnExecutionTimeout)
		cancelTxn()
	}()
	numTxnsProcessed := 0
	txnChan := proposer.Txns(ctxTxn)
	for txn := range txnChan {
		numTxnsProcessed++
		require.NotEmpty(t, txn)
	}
	require.Equal(t, numTxnsProcessed, 1)

	// Step 5: ProposalCommitment()
	pCommitment, err := proposer.ProposalCommitment()
	require.NoError(t, err)
	require.NotEmpty(t, pCommitment)

	// Step 6: ProposalFin()
	pFin, err := proposer.ProposalFin()
	require.NoError(t, err)
	require.NotEmpty(t, pFin)
}
