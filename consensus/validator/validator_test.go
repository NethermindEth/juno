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
	qwe := utils.HexToFelt(t, "0x29abab2cdf9cf0daad14b926b27744882b8e76a5ae0d33e0d3d9698ec2c4c22")
	diff.Nonces[*qwe] = utils.HexToFelt(t, "0x1")
	require.NoError(t, bc.StoreGenesis(&diff, classes))
	blockTime := 100 * time.Millisecond
	testBuilder := builder.New(bc, vm.New(false, log), log, false)
	// We use the sequencer to build a non-empty blockchain
	seq := sequencer.New(&testBuilder, p, seqAddr, privKey, blockTime, log)

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
		// Other fields are set to 0 as expected by the validator
		ConcatenatedCounts:    *new(felt.Felt).SetUint64(0),
		StateDiffCommitment:   *new(felt.Felt).SetUint64(0),
		TransactionCommitment: *new(felt.Felt).SetUint64(0),
		EventCommitment:       *new(felt.Felt).SetUint64(0),
		ReceiptCommitment:     *new(felt.Felt).SetUint64(0),
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
	invokeTxn := core.InvokeTransaction{
		TransactionHash: utils.HexToFelt(t, "0x76f781334b478792e443af1e632eb7ecc82cdb4ce4337ad90f24bd6481a8c02"),
		SenderAddress:   utils.HexToFelt(t, "0x29abab2cdf9cf0daad14b926b27744882b8e76a5ae0d33e0d3d9698ec2c4c22"),
		Version:         new(core.TransactionVersion).SetUint64(3),
		Nonce:           new(felt.Felt).SetUint64(1),
		TransactionSignature: []*felt.Felt{
			utils.HexToFelt(t, "0x1d2cab7be6f8675d2ca5365fde15bdefaab45e18d10cc5a70a2584debea89a"),
			utils.HexToFelt(t, "0x68cf661a6d90bc9ecb9da41deba26c9961def7757605e25a9d29cd83e942cb2"),
		},
		CallData: []*felt.Felt{
			utils.HexToFelt(t, "0x1"),
			utils.HexToFelt(t, "0x4718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d"),
			utils.HexToFelt(t, "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e"),
			utils.HexToFelt(t, "0x3"),
			utils.HexToFelt(t, "0x7a168e677e301b1334408040cd61acdcfc8bf40f460be7dd4d36b4cde4ab628"),
			utils.HexToFelt(t, "0x1158e460913d00000"),
			utils.HexToFelt(t, "0x0"),
		},
		ResourceBounds: map[core.Resource]core.ResourceBounds{
			core.ResourceL1Gas: core.ResourceBounds{
				MaxAmount:       1800,
				MaxPricePerUnit: utils.HexToFelt(t, "0x2d79883d2000"),
			},
			core.ResourceL2Gas: core.ResourceBounds{
				MaxAmount:       0,
				MaxPricePerUnit: utils.HexToFelt(t, "0x0"),
			},
			core.ResourceL1DataGas: core.ResourceBounds{
				MaxAmount:       0,
				MaxPricePerUnit: utils.HexToFelt(t, "0x0"),
			},
		},
		Tip:                   utils.HexToUint64(t, "0x9184e72a000"),
		PaymasterData:         []*felt.Felt{},
		AccountDeploymentData: []*felt.Felt{},
		NonceDAMode:           core.DAModeL1,
		FeeDAMode:             core.DAModeL1,
	}
	require.NoError(t, validator.TransactionBatch([]types.Transaction{{Transaction: &invokeTxn}}))

	nonEmptyCommitment := types.ProposalCommitment{
		BlockNumber:      head.Number + 1,
		Builder:          *proposerAddr,
		ParentCommitment: *head.Hash,
		Timestamp:        blockInfo.Timestamp,
		ProtocolVersion:  *blockchain.SupportedStarknetVersion,

		StateDiffCommitment:   *utils.HexToFelt(t, "0x1d551d2dece3472b9fa78b06d970e38476e744b9befcef719dcb69f120f879e"),
		TransactionCommitment: *utils.HexToFelt(t, "0x7b26ea9d61b4fa058a2a32bd16e99c085526e1a45f16bfd223456ad2a3edf11"),
		EventCommitment:       *utils.HexToFelt(t, "0x369d1bd8c4ff269e967ef05bbfdbe2e3bb9d190f3e8d5cdd7f8543f9e38803d"),
		ReceiptCommitment:     *utils.HexToFelt(t, "0x565be83c9bd120647734f31b9d3c9842f639c094d814cc4728bddd66cfac18e"),
		ConcatenatedCounts:    *utils.HexToFelt(t, "0x1000000000000000100000000000000038000000000000000"),
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

// TODO: Write tests to actually test the `ProposalCommitment` function.
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
	t.Run("ValidCommitment", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		err := compareProposalCommitment(expected, p)
		require.NoError(t, err)
	})

	t.Run("MismatchedBlockNumber", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		p.BlockNumber = blockNumber + 1
		err := compareProposalCommitment(expected, p)
		expectedErr := fmt.Sprintf("block number mismatch: proposal=%d header=%d", p.BlockNumber, h.Number)
		require.EqualError(t, err, expectedErr)
		p.BlockNumber = blockNumber
	})

	t.Run("MismatchedParentCommitment", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		p.ParentCommitment = *utils.HexToFelt(t, "222")
		require.Error(t, compareProposalCommitment(expected, p))
	})

	t.Run("FutureTimestamp", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		p.Timestamp = 2000
		require.Error(t, compareProposalCommitment(expected, p))
	})

	t.Run("UnsupportedProtocolVersion", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		p.ProtocolVersion = *semver.New(0, 123, 0, "", "")
		require.Error(t, compareProposalCommitment(expected, p))
	})

	t.Run("MismatchedConcatenatedCounts", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		p.ConcatenatedCounts = *utils.HexToFelt(t, "2")
		require.Error(t, compareProposalCommitment(expected, p))
	})

	t.Run("MismatchedStateDiffCommitment", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		other := &felt.Felt{}
		other.SetUint64(99)
		p.StateDiffCommitment = *other
		require.Error(t, compareProposalCommitment(expected, p))
	})

	t.Run("MismatchedTransactionCommitment", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		other := &felt.Felt{}
		other.SetUint64(99)
		p.TransactionCommitment = *other
		require.Error(t, compareProposalCommitment(expected, p))
	})

	t.Run("MismatchedEventCommitment", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		other := &felt.Felt{}
		other.SetUint64(99)
		p.EventCommitment = *other
		require.Error(t, compareProposalCommitment(expected, p))
	})

	t.Run("MismatchedReceiptCommitment", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		other := &felt.Felt{}
		other.SetUint64(99)
		p.ReceiptCommitment = *other
		require.Error(t, compareProposalCommitment(expected, p))
	})

	t.Run("MismatchedL1DAMode", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		p.L1DAMode = core.Calldata
		require.Error(t, compareProposalCommitment(expected, p))
	})
}
