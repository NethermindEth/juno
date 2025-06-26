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
	require.NoError(t, bc.StoreGenesis(&diff, classes))
	blockTime := 100 * time.Millisecond
	executor := builder.NewExecutor(bc, vm.New(false, log), log, false, true)
	testBuilder := builder.New(bc, executor)
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
	executor := builder.NewExecutor(chain, vm, log, false, false)
	testBuilder := builder.New(chain, executor)

	validator := New[value, felt.Felt, felt.Felt](&testBuilder)

	// Step 1: ProposalInit
	proposalInit := types.ProposalInit{
		BlockNum: types.Height(head.Number + 1),
		Proposer: *proposerAddr,
	}
	require.NoError(t, validator.ProposalInit(&proposalInit))

	emptyCommitment := types.ProposalCommitment{
		BlockNumber:      head.Number + 1,
		ParentCommitment: *head.Hash,
		Builder:          *proposerAddr,
		Timestamp:        0,
		ProtocolVersion:  *builder.CurrentStarknetVersion,
		OldStateRoot:     *utils.HexToFelt(t, "0x7629d7aa2c2ae74781626790ab75feb3306b79a41a917bcb923596d12af7f72"),
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
	block1Hash, err := new(felt.Felt).SetString("0x707b1c40b82913d91d7cde74107be9cfa48b9080c49e95dd0ba7b00cd1125c")
	require.NoError(t, err)
	proposalFin := types.ProposalFin(*block1Hash)
	require.NoError(t, validator.ProposalFin(proposalFin))
}

// This test uses a custom blockchain, with predefined accounts and token contracts.
// The proposer proposes a valid non-empty block with a single invoke transaction.
// The validator should re-execute it, and come to agreement on the resulting commitments.
func TestProposal(t *testing.T) {
	proposerAddr := utils.HexToFelt(t, "0xDEADBEEF")
	b, _, head := getCustomBC(t, proposerAddr)
	validator := New[value, felt.Felt, felt.Felt](b)

	l2GasPriceFri := uint64(10)
	l1GasPriceWei := uint64(11)
	l1DataGasPriceWei := uint64(12)
	ethToStrkRate := uint64(13)
	l1GasPriceFri := l1GasPriceWei * ethToStrkRate
	l1DataGasPriceFri := l1DataGasPriceWei * ethToStrkRate

	// Step 1: ProposalInit
	proposalInit := types.ProposalInit{
		BlockNum: types.Height(head.Number + 1),
		Proposer: *proposerAddr,
	}
	require.NoError(t, validator.ProposalInit(&proposalInit))

	// Step 2: BlockInfo
	blockInfo := types.BlockInfo{
		BlockNumber:       head.Number + 1,
		Builder:           *proposerAddr,
		Timestamp:         1700474724,
		L2GasPriceFRI:     felt.FromUint64(l2GasPriceFri),
		L1GasPriceWEI:     felt.FromUint64(l1GasPriceWei),
		L1DataGasPriceWEI: felt.FromUint64(l1DataGasPriceWei),
		EthToStrkRate:     felt.FromUint64(ethToStrkRate),
		L1DAMode:          core.Blob,
	}
	require.NoError(t, validator.BlockInfo(&blockInfo))

	// Step 3: TransactionBatch
	// Invoke txn: transfer tokens to account "0x102"
	invokeTxn := core.InvokeTransaction{
		TransactionHash: utils.HexToFelt(t, "0x631a5ed85f6758233b9286092f5eacbad90bdfb94a8fdfaab8c31e631232992"),
		SenderAddress:   utils.HexToFelt(t, "0x101"),
		Version:         new(core.TransactionVersion).SetUint64(3),
		Nonce:           new(felt.Felt).SetUint64(0),
		TransactionSignature: []*felt.Felt{
			utils.HexToFelt(t, "0xa678c78ff34d4a0ccd5063318265d60e233445782892b40e019bf4556e57c0"),
			utils.HexToFelt(t, "0x234470d2c4f6dc6f8e38adf1992cda3969119f62f25941b8bfb4ccd50b5c823"),
		},
		CallData: []*felt.Felt{
			utils.HexToFelt(t, "0x1"),
			utils.HexToFelt(t, "0x4718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d"),
			utils.HexToFelt(t, "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e"),
			utils.HexToFelt(t, "0x3"),
			utils.HexToFelt(t, "0x105"),
			utils.HexToFelt(t, "0x1234"),
			utils.HexToFelt(t, "0x0"),
		},
		ResourceBounds: map[core.Resource]core.ResourceBounds{
			core.ResourceL1Gas: {
				MaxAmount:       4,
				MaxPricePerUnit: utils.HeapPtr(felt.FromUint64(l1GasPriceFri + 1)),
			},
			core.ResourceL2Gas: {
				MaxAmount:       500000,
				MaxPricePerUnit: utils.HeapPtr(felt.FromUint64(l2GasPriceFri + 1)),
			},
			core.ResourceL1DataGas: {
				MaxAmount:       296,
				MaxPricePerUnit: utils.HeapPtr(felt.FromUint64(l1DataGasPriceFri + 1)),
			},
		},
		Tip:                   utils.HexToUint64(t, "0x0"),
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
		ProtocolVersion:  *builder.CurrentStarknetVersion,

		OldStateRoot:          *utils.HexToFelt(t, "0x7629d7aa2c2ae74781626790ab75feb3306b79a41a917bcb923596d12af7f72"),
		StateDiffCommitment:   *utils.HexToFelt(t, "0x4cef1c56b11255e30104420e8af41aff2ffe015ddd1eebc7acb28c42ffc3a0"),
		TransactionCommitment: *utils.HexToFelt(t, "0x1286e8721df29411c3f24c8decdef473e1ca758a87ab8d8a4e99ff7511c6fcd"),
		EventCommitment:       *utils.HexToFelt(t, "0x7e7140ab1993f5f3b9050104fcf65b462a6d9e5d1b4aa5d7fb29a177e5f960e"),
		ReceiptCommitment:     *utils.HexToFelt(t, "0x644551c17735bb6679bc4782fc95154d013b73b359bb95b33ea26f170d03cf1"),
		ConcatenatedCounts:    *utils.HexToFelt(t, "0x1000000000000000100000000000000038000000000000000"),
		L1GasPriceFRI:         felt.FromUint64(l1GasPriceFri),
		L1DataGasPriceFRI:     felt.FromUint64(l1DataGasPriceFri),
		L2GasPriceFRI:         blockInfo.L2GasPriceFRI,
		L2GasUsed:             *utils.HexToFelt(t, "0x93f80"),
		L1DAMode:              blockInfo.L1DAMode,
	}
	require.NoError(t, validator.ProposalCommitment(&nonEmptyCommitment))

	// Step 5: ProposalFin
	proposalFin := types.ProposalFin(*utils.HexToFelt(t, "0x302789a08a44329b02a6dc13fe837f9719fdce7607d3bd5c872d5ca9b5cdbbd"))
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
			ProtocolVersion:       *builder.CurrentStarknetVersion,
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
		ProtocolVersion:  builder.CurrentStarknetVersion.String(),
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

	t.Run("MismatchedConcatenatedCounts", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		p.ConcatenatedCounts = *utils.HexToFelt(t, "2")
		require.Error(t, compareProposalCommitment(expected, p))
	})

	t.Run("MismatchedL1GasPriceFRI", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		p.L1GasPriceFRI = felt.FromUint64(3)
		require.Error(t, compareProposalCommitment(expected, p))
	})

	t.Run("MismatchedL1DataGasPriceFRI", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		p.L1DataGasPriceFRI = felt.FromUint64(4)
		require.Error(t, compareProposalCommitment(expected, p))
	})

	t.Run("MismatchedL2GasPriceFRI", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		p.L2GasPriceFRI = felt.FromUint64(5)
		require.Error(t, compareProposalCommitment(expected, p))
	})

	t.Run("MismatchedL2GasUsed", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		p.L2GasUsed = felt.FromUint64(6)
		require.Error(t, compareProposalCommitment(expected, p))
	})

	t.Run("MismatchedL1DAMode", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		p.L1DAMode = core.Calldata
		require.Error(t, compareProposalCommitment(expected, p))
	})
}
