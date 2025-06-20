package proposer

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/genesis"
	"github.com/NethermindEth/juno/mempool"
	rpc "github.com/NethermindEth/juno/rpc/v8"
	"github.com/NethermindEth/juno/sequencer"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/ecdsa"
	"github.com/stretchr/testify/require"
)

// Create a builder, with a block which includes fee tokens, and some pre-funded accounts.
// This allows us to execute real txns in the tests.
func getCustomBuilder(t *testing.T, seqAddr *felt.Felt) (*builder.Builder, *rpc.Handler, *mempool.Pool) {
	t.Helper()

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
	testBuilder := builder.New(bc, vm.New(false, log), log, true)
	// We use the sequencer to build a non-empty blockchain
	seq := sequencer.New(&testBuilder, txnPool, seqAddr, privKey, blockTime, log)
	rpcHandler := rpc.New(bc, nil, nil, "", log).WithMempool(txnPool)
	_, err = seq.RunOnce()
	require.NoError(t, err)

	return &testBuilder, rpcHandler, txnPool
}

func TestEmptyProposal(t *testing.T) {
	proposerAddr := new(felt.Felt).SetUint64(123123)
	builder, _, mempool := getCustomBuilder(t, proposerAddr)
	proposer := New(builder, mempool, proposerAddr, utils.NewNopZapLogger())

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
	builder, rpcHandler, mempool := getCustomBuilder(t, proposerAddr)
	proposer := New(builder, mempool, proposerAddr, utils.NewNopZapLogger())

	// Step 1: ProposalInit()
	pInit, err := proposer.ProposalInit()
	require.NoError(t, err)
	require.NotEmpty(t, pInit)

	// Add txn to the mempool (send tokens to "0x102")
	daModeL1 := rpc.DAModeL1
	invokeTxn := rpc.BroadcastedTransaction{
		Transaction: rpc.Transaction{
			Type:          rpc.TxnInvoke,
			Hash:          utils.HexToFelt(t, "0x76f781334b478792e443af1e632eb7ecc82cdb4ce4337ad90f24bd6481a8c02"),
			SenderAddress: utils.HexToFelt(t, "0x29abab2cdf9cf0daad14b926b27744882b8e76a5ae0d33e0d3d9698ec2c4c22"),
			Version:       utils.HexToFelt(t, "0x3"),
			Nonce:         new(felt.Felt).SetUint64(0),
			Signature: &[]*felt.Felt{
				utils.HexToFelt(t, "0x1d2cab7be6f8675d2ca5365fde15bdefaab45e18d10cc5a70a2584debea89a"),
				utils.HexToFelt(t, "0x68cf661a6d90bc9ecb9da41deba26c9961def7757605e25a9d29cd83e942cb2"),
			},
			CallData: &[]*felt.Felt{
				utils.HexToFelt(t, "0x1"),
				utils.HexToFelt(t, "0x4718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d"),
				utils.HexToFelt(t, "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e"),
				utils.HexToFelt(t, "0x3"),
				utils.HexToFelt(t, "0x7a168e677e301b1334408040cd61acdcfc8bf40f460be7dd4d36b4cde4ab628"),
				utils.HexToFelt(t, "0x1158e460913d00000"),
				utils.HexToFelt(t, "0x0"),
			},
			ResourceBounds: &rpc.ResourceBoundsMap{
				L1Gas: &rpc.ResourceBounds{
					MaxAmount:       utils.HexToFelt(t, "0x123"),
					MaxPricePerUnit: utils.HexToFelt(t, "0x2d79883d2000"),
				},
				L2Gas: &rpc.ResourceBounds{
					MaxAmount:       utils.HexToFelt(t, "0x124"),
					MaxPricePerUnit: utils.HexToFelt(t, "0x1800"),
				},
				L1DataGas: &rpc.ResourceBounds{
					MaxAmount:       utils.HexToFelt(t, "0x125"),
					MaxPricePerUnit: utils.HexToFelt(t, "0x1800"),
				},
			},
			Tip:                   utils.HexToFelt(t, "0"),
			PaymasterData:         &[]*felt.Felt{},
			AccountDeploymentData: &[]*felt.Felt{},
			NonceDAMode:           &daModeL1,
			FeeDAMode:             &daModeL1,
		},
	}
	_, rpcErr := rpcHandler.AddTransaction(t.Context(), invokeTxn)
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
