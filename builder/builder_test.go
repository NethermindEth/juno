package builder_test

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/genesis"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/mocks"
	rpc "github.com/NethermindEth/juno/rpc/v8"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/ecdsa"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func waitFor(t *testing.T, timeout time.Duration, check func() bool) {
	t.Helper()

	const numPolls = 4
	pollInterval := timeout / numPolls

	for range numPolls {
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

func waitForTxns(t *testing.T, bc blockchain.Reader, timeout time.Duration, targetTxnNumber uint64) {
	waitFor(t, timeout, func() bool {
		curBlockNumber, err := bc.Height()
		require.NoError(t, err)
		cumTxns := uint64(0)
		for i := range curBlockNumber {
			block, err := bc.BlockByNumber(i)
			require.NoError(t, err)
			cumTxns += block.TransactionCount
			if cumTxns >= targetTxnNumber {
				return true
			}
		}
		return false
	})
}
func TestSign(t *testing.T) {
	testDB := pebble.NewMemTest(t)
	mockCtrl := gomock.NewController(t)
	mockVM := mocks.NewMockVM(mockCtrl)
	bc := blockchain.New(testDB, &utils.Integration)
	seqAddr := utils.HexToFelt(t, "0xDEADBEEF")
	privKey, err := ecdsa.GenerateKey(rand.Reader)
	require.NoError(t, err)
	p, closer := mempool.New(pebble.NewMemTest(t), bc, 1000, utils.NewNopZapLogger())
	testBuilder := builder.New(privKey, seqAddr, bc, mockVM, 0, p, utils.NewNopZapLogger(), false, testDB, closer)

	_, err = testBuilder.Sign(new(felt.Felt), new(felt.Felt))
	require.NoError(t, err)
	// We don't check the signature since the private key generation is not deterministic.
}

func TestBuildTwoEmptyBlocks(t *testing.T) {
	testDB := pebble.NewMemTest(t)
	mockCtrl := gomock.NewController(t)
	mockVM := mocks.NewMockVM(mockCtrl)
	bc := blockchain.New(testDB, &utils.Integration)
	emptyStateDiff := core.EmptyStateDiff()
	require.NoError(t, bc.StoreGenesis(&emptyStateDiff, nil))
	seqAddr := utils.HexToFelt(t, "0xDEADBEEF")
	privKey, err := ecdsa.GenerateKey(rand.Reader)
	require.NoError(t, err)
	p, closer := mempool.New(pebble.NewMemTest(t), bc, 1000, utils.NewNopZapLogger())

	minHeight := uint64(2)
	testBuilder := builder.New(privKey, seqAddr, bc, mockVM, time.Millisecond, p, utils.NewNopZapLogger(), false, testDB, closer)

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		waitForBlock(t, bc, time.Second, minHeight)
		cancel()
	}()
	require.NoError(t, testBuilder.Run(ctx))

	height, err := bc.Height()
	require.NoError(t, err)
	require.GreaterOrEqual(t, height, minHeight)
	for i := range height {
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
	bc := blockchain.New(testDB, network)
	log := utils.NewNopZapLogger()
	seqAddr := utils.HexToFelt(t, "0xDEADBEEF")
	privKey, err := ecdsa.GenerateKey(rand.Reader)
	require.NoError(t, err)
	p, mempoolCloser := mempool.New(testDB, bc, 1000, utils.NewNopZapLogger())

	genesisConfig, err := genesis.Read("../genesis/genesis_prefund_accounts.json")
	require.NoError(t, err)
	genesisConfig.Classes = []string{
		"../genesis/classes/strk.json", "../genesis/classes/account.json",
		"../genesis/classes/universaldeployer.json", "../genesis/classes/udacnt.json",
	}
	diff, classes, err := genesis.GenesisStateDiff(genesisConfig, vm.New(false, log), bc.Network(), 40000000) //nolint:gomnd
	require.NoError(t, err)
	require.NoError(t, bc.StoreGenesis(&diff, classes))
	testBuilder := builder.New(privKey, seqAddr, bc, vm.New(false, log), 1000*time.Millisecond, p, log, false, testDB, mempoolCloser)
	rpcHandler := rpc.New(bc, nil, nil, "", log).WithMempool(p)
	for _, txn := range expectedExnsInBlock {
		rpcHandler.AddTransaction(t.Context(), txn)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 1500*time.Millisecond)
	go func() {
		waitForTxns(t, bc, 4*time.Second, 2)
		cancel()
	}()
	require.NoError(t, testBuilder.Run(ctx))

	height, err := bc.Height()
	require.NoError(t, err)
	for i := range height {
		block, err := bc.BlockByNumber(i + 1)
		require.NoError(t, err)
		if block.TransactionCount != 0 {
			require.Equal(t, len(expectedExnsInBlock), int(block.TransactionCount), "Failed to find correct number of transactions in the block")
		}
	}

	expectedBalance := new(felt.Felt).Add(utils.HexToFelt(t, "0x56bc75e2d63100000"), utils.HexToFelt(t, "0x12345678"))
	foundExpectedBalance := false
	numExpectedBalance := 0
	for i := range height {
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
