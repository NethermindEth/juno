package sequencer

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

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

// We use a custom chain to test the consensus logic.
// The reason is that our current blockifier version is incompatible
// with early mainnet/sepolia blocks. And we can't execute blocks at the chain
// head without access to the state. To get around this, a custom chain was used
// in these tests.
func GetCustomBC(t *testing.T, seqAddr *felt.Felt) (*builder.Builder, *blockchain.Blockchain, *core.Header) {
	t.Helper()

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

	expectedExnsInBlock := []rpc.BroadcastedTransaction{invokeTxn}
	testDB := memory.New()
	network := &utils.Mainnet
	bc := blockchain.New(testDB, network)
	log := utils.NewNopZapLogger()

	privKey, err := ecdsa.GenerateKey(rand.Reader)
	require.NoError(t, err)
	p := mempool.New(testDB, bc, 1024, utils.NewNopZapLogger()) //nolint:mnd

	genesisConfig, err := genesis.Read("../../genesis/genesis_prefund_accounts.json")
	require.NoError(t, err)
	genesisConfig.Classes = []string{
		"../../genesis/classes/strk.json", "../../genesis/classes/account.json",
		"../../genesis/classes/universaldeployer.json", "../../genesis/classes/udacnt.json",
	}
	diff, classes, err := genesis.GenesisStateDiff(genesisConfig, vm.New(false, log), bc.Network(), 40000000) //nolint:mnd
	require.NoError(t, err)
	require.NoError(t, bc.StoreGenesis(&diff, classes))
	blockTime := 100 * time.Millisecond
	testBuilder := builder.New(bc, vm.New(false, log), log, false)
	// We use the sequencer to build a non-empty blockchain
	seq := New(&testBuilder, p, seqAddr, privKey, blockTime, log)
	rpcHandler := rpc.New(bc, nil, nil, "", log).WithMempool(p)
	for i := range expectedExnsInBlock {
		_, rpcErr := rpcHandler.AddTransaction(t.Context(), expectedExnsInBlock[i])
		require.Nil(t, rpcErr)
	}
	head, err := seq.RunOnce()
	require.NoError(t, err)
	return &testBuilder, bc, head
}

// Execute a single block. Useful for tests.
func (s *Sequencer) RunOnce() (*core.Header, error) {
	if err := s.builder.ClearPending(); err != nil {
		s.log.Errorw("clearing pending", "err", err)
	}

	if err := s.builder.InitPendingBlock(s.sequencerAddress); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if pErr := s.listenPool(ctx); pErr != nil {
		if pErr != mempool.ErrTxnPoolEmpty {
			s.log.Warnw("listening pool", "err", pErr)
		}
	}

	pending, err := s.Pending()
	if err != nil {
		s.log.Infof("Failed to get pending block")
	}
	if err := s.builder.Finalise(pending, utils.Sign(s.privKey), s.privKey); err != nil {
		return nil, err
	}
	s.log.Infof("Finalised new block")
	if s.plugin != nil {
		err := s.plugin.NewBlock(pending.Block, pending.StateUpdate, pending.NewClasses)
		if err != nil {
			s.log.Errorw("error sending new block to plugin", err)
		}
	}
	// push the new head to the feed
	s.subNewHeads.Send(pending.Block)

	if err := s.builder.ClearPending(); err != nil {
		return nil, err
	}
	if err := s.builder.InitPendingBlock(s.sequencerAddress); err != nil {
		return nil, err
	}
	return pending.Block.Header, nil
}
