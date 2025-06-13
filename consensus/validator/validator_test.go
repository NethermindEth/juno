package validator

import (
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
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

type value uint64

func (v value) Hash() felt.Felt {
	return *new(felt.Felt).SetUint64(uint64(v))
}

// We use a custom chain to test the consensus logic.
// The reason is that our current blockifier version is incompatible
// with early mainnet/sepolia blocks. And we can't execute blocks at the chain
// head without access to the state. To get around this, a custom chain was used
// in these tests.
func getCustomBC(t *testing.T, seqAddr *felt.Felt) (*builder.Builder, *blockchain.Blockchain, *core.Header) {
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
	p := mempool.New(testDB, bc, 1000, utils.NewNopZapLogger())

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
	testBuilder := builder.New(bc, vm.New(false, log), log, false)
	// We use the sequencer to build a non-empty blockchain
	seq := sequencer.New(&testBuilder, p, seqAddr, privKey, blockTime, log)
	rpcHandler := rpc.New(bc, nil, nil, "", log).WithMempool(p)
	for i := range expectedExnsInBlock {
		_, rpcErr := rpcHandler.AddTransaction(t.Context(), expectedExnsInBlock[i])
		require.Nil(t, rpcErr)
	}
	head, err := seq.RunOnce()
	require.NoError(t, err)
	return &testBuilder, bc, head
}

// This test assumes Sepolia block 0 has been committed, and now
// the Proposer proposes an empty block for block 1
func TestEmptyProposal(t *testing.T) {
	proposerAddr := utils.HexToFelt(t, "0xabcdef")
	_, chain, head := getCustomBC(t, proposerAddr)
	log := utils.NewNopZapLogger()
	vm := vm.New(false, log)
	testBuilder := builder.New(chain, vm, log, false)

	validator := New[value, felt.Felt, felt.Felt](&testBuilder)

	// Step 1: ProposalInit
	proposalInit := types.ProposalInit{
		BlockNum: head.Number + 1,
		Proposer: *proposerAddr,
	}
	require.NoError(t, validator.ProposalInit(&proposalInit))

	emptyCommitment := types.ProposalCommitment{
		BlockNumber:      head.Number + 1,
		ParentCommitment: *head.Hash,
		Builder:          *proposerAddr,
		Timestamp:        0,
		ProtocolVersion:  *blockchain.SupportedStarknetVersion,
	}

	// Step 2: ProposalCommitment
	require.NoError(t, validator.ProposalCommitment(&emptyCommitment))

	// Step 3: ProposalFin
	// Note: this commitment depends on the SupportedStarknetVersion, so block1Hash test should be updated whenever
	// we update SupportedStarknetVersion
	block1Hash, err := new(felt.Felt).SetString("0x27a852d180c77e0b7bc2b3e9b635b625db3ae48f2c469c93930c1889ae2d09f")
	require.NoError(t, err)
	proposalFin := types.ProposalFin(*block1Hash)
	require.NoError(t, validator.ProposalFin(proposalFin))
}

// This test uses a custom blockchain, with predefined accounts and token contracts.
// The proposer proposes a valid non-empty block with a single invoke transaction.
// The validator should re-execute it, and come to agreement on the resulting commitments.
func TestProposal(t *testing.T) {
	proposerAddr := utils.HexToFelt(t, "0xDEADBEEF")
	builder, _, head := getCustomBC(t, proposerAddr)
	validator := New[value, felt.Felt, felt.Felt](builder)

	// Step 1: ProposalInit
	proposalInit := types.ProposalInit{
		BlockNum: head.Number + 1,
		Proposer: *proposerAddr,
	}
	require.NoError(t, validator.ProposalInit(&proposalInit))

	// Step 2: BlockInfo
	blockInfo := types.BlockInfo{
		BlockNumber:       head.Number + 1,
		Builder:           *proposerAddr,
		Timestamp:         1700474724,
		L2GasPriceFRI:     *new(felt.Felt).SetUint64(10),
		L1GasPriceWEI:     *new(felt.Felt).SetUint64(11),
		L1DataGasPriceWEI: *new(felt.Felt).SetUint64(12),
		EthToStrkRate:     *new(felt.Felt).SetUint64(13),
		L1DAMode:          core.Blob,
	}
	validator.BlockInfo(&blockInfo)

	// Step 3: TransactionBatch
	// Invoke txn: transfer tokens to account "0x102"
	invokeTxn2 := core.InvokeTransaction{
		TransactionHash: utils.HexToFelt(t, "0x722e584df0c18fcda54552ae5055f6c1fda331c4ae5de7ec5fc0376ae8b9a7f"),
		SenderAddress:   utils.HexToFelt(t, "0x0406a8f52e741619b17410fc90774e4b36f968e1a71ae06baacfe1f55d987923"),
		Version:         new(core.TransactionVersion).SetUint64(1),
		MaxFee:          utils.HexToFelt(t, "0xaeb1bacb2c"),
		Nonce:           new(felt.Felt).SetUint64(1),
		TransactionSignature: []*felt.Felt{
			utils.HexToFelt(t, "0x6012e655ac15a4ab973a42db121a2cb78d9807c5ff30aed74b70d32a682b083"),
			utils.HexToFelt(t, "0xcd27013a24e143cc580ba788b14df808aefa135d8ed3aca297aa56aa632cb5"),
		},
		CallData: []*felt.Felt{
			utils.HexToFelt(t, "0x1"),
			utils.HexToFelt(t, "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7"),
			utils.HexToFelt(t, "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e"),
			utils.HexToFelt(t, "0x3"),
			utils.HexToFelt(t, "0x102"),
			utils.HexToFelt(t, "0x12345678"),
			utils.HexToFelt(t, "0x0"),
		},
	}
	require.NoError(t, validator.TransactionBatch([]types.Transaction{{Index: 0, Transaction: &invokeTxn2}}))

	nonEmptyCommitment := types.ProposalCommitment{
		BlockNumber:      head.Number + 1,
		Builder:          *proposerAddr,
		ParentCommitment: *head.Hash,
		Timestamp:        blockInfo.Timestamp,
		ProtocolVersion:  *blockchain.SupportedStarknetVersion,

		StateDiffCommitment:   *utils.HexToFelt(t, "0x3248f1e62ba170555f7caa03c6f6c89843d5bfdafbf16384210544ef0b548e1"),
		TransactionCommitment: *utils.HexToFelt(t, "0x4ba493c0b6605d0a7af00e6d401e937989192bb10ba3cc940ee509fee3e664b"),
		EventCommitment:       *utils.HexToFelt(t, "0x366f7f8dd503ee94f626be1575fbd579692bb990be92b6c5b65b98a6c4faa9a"),
		ReceiptCommitment:     *utils.HexToFelt(t, "0x513b7abd6c2952930a937580e14a05b0cdd1c69b570862194a023bf85090464"),
		ConcatenatedCounts:    *utils.HexToFelt(t, "0x1000000000000000300000000000000048000000000000000"),
		L1DataGasPriceFRI:     *new(felt.Felt).SetUint64(1),
		L2GasPriceFRI:         blockInfo.L2GasPriceFRI,
		L1DAMode:              blockInfo.L1DAMode,
	}
	require.NoError(t, validator.ProposalCommitment(&nonEmptyCommitment))

	// Step 5: ProposalFin
	// The custom chain may generate a different number of blocks before we run this test, which means
	// the proposed block hash can't be known ahead of time.
	// Ideally we would check against Sepolia/Mainnet, but this isn't possible yet.
	proposedBlock := builder.PendingBlock()
	proposalFin := types.ProposalFin(*proposedBlock.Hash)
	require.NoError(t, validator.ProposalFin(proposalFin))
}

func TestCompareFeltField(t *testing.T) {
	name := "test-field"

	a := new(felt.Felt).SetUint64(12345)
	b := new(felt.Felt).SetUint64(12345)
	c := new(felt.Felt).SetUint64(67890)

	t.Run("EqualFields", func(t *testing.T) {
		err := compareFeltField(name, a, b)
		if err != nil {
			t.Errorf("expected no error for equal fields, got: %v", err)
		}
	})

	t.Run("UnequalFields", func(t *testing.T) {
		err := compareFeltField(name, a, c)
		if err == nil {
			t.Errorf("expected error for unequal fields, got nil")
		} else {
			expected := fmt.Sprintf("%s commitment mismatch: proposal=%s commitments=%s", name, a, c)
			if err.Error() != expected {
				t.Errorf("unexpected error message: got %q, want %q", err.Error(), expected)
			}
		}
	})
}

func TestCompareProposalCommitment(t *testing.T) {
	proposer := utils.HexToFelt(t, "1")

	stateDiffCommitment := new(felt.Felt).SetUint64(1)
	transactionCommitment := new(felt.Felt).SetUint64(2)
	eventCommitment := new(felt.Felt).SetUint64(3)
	receiptCommitment := new(felt.Felt).SetUint64(4)

	blockNumber := uint64(100)

	newDefaultProposalCommitment := func() *types.ProposalCommitment {
		return &types.ProposalCommitment{
			BlockNumber:           blockNumber,
			ParentCommitment:      *new(felt.Felt).SetUint64(111),
			Builder:               *proposer,
			Timestamp:             1000,
			ProtocolVersion:       *blockchain.SupportedStarknetVersion,
			ConcatenatedCounts:    *new(felt.Felt).SetUint64(1),
			StateDiffCommitment:   *stateDiffCommitment,
			TransactionCommitment: *transactionCommitment,
			EventCommitment:       *eventCommitment,
			ReceiptCommitment:     *receiptCommitment,
			L1DAMode:              core.Blob,
		}
	}

	h := &core.Header{
		Number:           blockNumber,
		ParentHash:       utils.HexToFelt(t, "111"),
		SequencerAddress: proposer,
		Timestamp:        1000,
		ProtocolVersion:  blockchain.SupportedStarknetVersion.String(),
		L1DAMode:         core.Blob,
	}

	c := &core.BlockCommitments{
		StateDiffCommitment:   stateDiffCommitment,
		TransactionCommitment: transactionCommitment,
		EventCommitment:       eventCommitment,
		ReceiptCommitment:     receiptCommitment,
	}

	concatCount := utils.HexToFelt(t, "1")
	t.Run("ValidCommitment", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		err := compareProposalCommitment(p, h, c, concatCount)
		require.NoError(t, err)
	})

	t.Run("MismatchedBlockNumber", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		p.BlockNumber = blockNumber + 1
		err := compareProposalCommitment(p, h, c, concatCount)
		expected := fmt.Sprintf("block number mismatch: proposal=%d header=%d", p.BlockNumber, h.Number)
		require.EqualError(t, err, expected)
		p.BlockNumber = blockNumber
	})

	t.Run("MismatchedParentCommitment", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		p.ParentCommitment = *utils.HexToFelt(t, "222")
		require.Error(t, compareProposalCommitment(p, h, c, concatCount))
	})

	t.Run("FutureTimestamp", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		p.Timestamp = 2000
		require.Error(t, compareProposalCommitment(p, h, c, concatCount))
	})

	t.Run("UnsupportedProtocolVersion", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		p.ProtocolVersion = *semver.New(0, 123, 0, "", "")
		require.Error(t, compareProposalCommitment(p, h, c, concatCount))
	})

	t.Run("MismatchedConcatenatedCounts", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		p.ConcatenatedCounts = *utils.HexToFelt(t, "2")
		require.Error(t, compareProposalCommitment(p, h, c, concatCount))
	})

	t.Run("MismatchedStateDiffCommitment", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		other := &felt.Felt{}
		other.SetUint64(99)
		p.StateDiffCommitment = *other
		require.Error(t, compareProposalCommitment(p, h, c, concatCount))
	})

	t.Run("MismatchedTransactionCommitment", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		other := &felt.Felt{}
		other.SetUint64(99)
		p.TransactionCommitment = *other
		require.Error(t, compareProposalCommitment(p, h, c, concatCount))
	})

	t.Run("MismatchedEventCommitment", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		other := &felt.Felt{}
		other.SetUint64(99)
		p.EventCommitment = *other
		require.Error(t, compareProposalCommitment(p, h, c, concatCount))
	})

	t.Run("MismatchedReceiptCommitment", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		other := &felt.Felt{}
		other.SetUint64(99)
		p.ReceiptCommitment = *other
		require.Error(t, compareProposalCommitment(p, h, c, concatCount))
	})

	t.Run("MismatchedL1DAMode", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		p.L1DAMode = core.Calldata
		require.Error(t, compareProposalCommitment(p, h, c, concatCount))
	})
}
