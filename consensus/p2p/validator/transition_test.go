package validator

import (
	"crypto/rand"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/statetestutils"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/genesis"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/sequencer"
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
func getBuilder(t *testing.T, seqAddr *felt.Felt) (*builder.Builder, *core.Header) {
	t.Helper()
	testDB := memory.New()
	network := &utils.Mainnet
	bc := blockchain.New(testDB, network, statetestutils.UseNewState())
	log := utils.NewNopZapLogger()

	privKey, err := ecdsa.GenerateKey(rand.Reader)
	require.NoError(t, err)
	p := mempool.New(testDB, bc, 1000, utils.NewNopZapLogger())

	genesisConfig, err := genesis.Read("../../../genesis/genesis_prefund_accounts.json")
	require.NoError(t, err)
	genesisConfig.Classes = []string{
		"../../../genesis/classes/strk.json", "../../../genesis/classes/account.json",
		"../../../genesis/classes/universaldeployer.json", "../../../genesis/classes/udacnt.json",
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
	blockTime := 100 * time.Millisecond
	executor := builder.NewExecutor(bc, vm.New(&chainInfo, false, log), log, false, true)
	testBuilder := builder.New(bc, executor)
	// We use the sequencer to build a non-empty blockchain
	seq := sequencer.New(&testBuilder, p, seqAddr, privKey, blockTime, log)
	head, err := seq.RunOnce()
	require.NoError(t, err)
	return &testBuilder, head
}

// This test assumes Sepolia block 0 has been committed, and now
// the Proposer proposes an empty block for block 1
func TestEmptyProposal(t *testing.T) {
	proposerAddr := felt.NewUnsafeFromString[felt.Felt]("0xabcdef")
	testBuilder, head := getBuilder(t, proposerAddr)

	initialState := InitialState{}
	transition := NewTransition(testBuilder)

	// Step 1: ProposalInit
	proposalInit := types.ProposalInit{
		BlockNum: types.Height(head.Number + 1),
		Proposer: *proposerAddr,
	}
	awaitingBlockInfoOrCommitmentState, err := transition.OnProposalInit(t.Context(), &initialState, &proposalInit)
	require.NoError(t, err)

	emptyCommitment := types.ProposalCommitment{
		BlockNumber:      head.Number + 1,
		ParentCommitment: *head.Hash,
		Builder:          *proposerAddr,
		Timestamp:        0,
		ProtocolVersion:  *builder.CurrentStarknetVersion,
		OldStateRoot:     *felt.NewUnsafeFromString[felt.Felt]("0x7629d7aa2c2ae74781626790ab75feb3306b79a41a917bcb923596d12af7f72"),
		// Other fields are set to 0 as expected by the validator
		ConcatenatedCounts:    *new(felt.Felt).SetUint64(0),
		StateDiffCommitment:   *new(felt.Felt).SetUint64(0),
		TransactionCommitment: *new(felt.Felt).SetUint64(0),
		EventCommitment:       *new(felt.Felt).SetUint64(0),
		ReceiptCommitment:     *new(felt.Felt).SetUint64(0),
	}

	// Step 2: ProposalCommitment
	awaitingProposalFinState, err := transition.OnEmptyBlockCommitment(t.Context(), awaitingBlockInfoOrCommitmentState, &emptyCommitment)
	require.NoError(t, err)

	// Step 3: ProposalFin
	// Note: this commitment depends on the SupportedStarknetVersion, so block1Hash test should be updated whenever
	// we update SupportedStarknetVersion
	block1Hash, err := new(felt.Felt).SetString("0x3521bcbcf5ab2b01702e73a2b2726b8303aa035dc2cb5b1e5aefca34e7585a5")
	require.NoError(t, err)
	proposalFin := types.ProposalFin(*block1Hash)
	_, err = transition.OnProposalFin(t.Context(), awaitingProposalFinState, &proposalFin)
	require.NoError(t, err)
}

// This test uses a custom blockchain, with predefined accounts and token contracts.
// The proposer proposes a valid non-empty block with a single invoke transaction.
// The validator should re-execute it, and come to agreement on the resulting commitments.
func TestProposal(t *testing.T) {
	proposerAddr := felt.NewUnsafeFromString[felt.Felt]("0xDEADBEEF")
	b, head := getBuilder(t, proposerAddr)
	transition := NewTransition(b)
	initialState := InitialState{}

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
	awaitingBlockInfoOrCommitmentState, err := transition.OnProposalInit(t.Context(), &initialState, &proposalInit)
	require.NoError(t, err)

	// Step 2: BlockInfo
	blockInfo := types.BlockInfo{
		BlockNumber:       head.Number + 1,
		Builder:           *proposerAddr,
		Timestamp:         1700474724,
		L2GasPriceFRI:     felt.FromUint64[felt.Felt](l2GasPriceFri),
		L1GasPriceWEI:     felt.FromUint64[felt.Felt](l1GasPriceWei),
		L1DataGasPriceWEI: felt.FromUint64[felt.Felt](l1DataGasPriceWei),
		EthToStrkRate:     felt.FromUint64[felt.Felt](ethToStrkRate),
		L1DAMode:          core.Blob,
	}
	receivingTransactionsState, err := transition.OnBlockInfo(t.Context(), awaitingBlockInfoOrCommitmentState, &blockInfo)
	require.NoError(t, err)

	// Step 3: TransactionBatch
	// Invoke txn: transfer tokens to account "0x102"
	invokeTxn := core.InvokeTransaction{
		TransactionHash: felt.NewUnsafeFromString[felt.Felt]("0x631a5ed85f6758233b9286092f5eacbad90bdfb94a8fdfaab8c31e631232992"),
		SenderAddress:   felt.NewUnsafeFromString[felt.Felt]("0x101"),
		Version:         new(core.TransactionVersion).SetUint64(3),
		Nonce:           new(felt.Felt).SetUint64(0),
		TransactionSignature: []*felt.Felt{
			felt.NewUnsafeFromString[felt.Felt]("0xa678c78ff34d4a0ccd5063318265d60e233445782892b40e019bf4556e57c0"),
			felt.NewUnsafeFromString[felt.Felt]("0x234470d2c4f6dc6f8e38adf1992cda3969119f62f25941b8bfb4ccd50b5c823"),
		},
		CallData: []*felt.Felt{
			felt.NewUnsafeFromString[felt.Felt]("0x1"),
			felt.NewUnsafeFromString[felt.Felt]("0x4718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d"),
			felt.NewUnsafeFromString[felt.Felt]("0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e"),
			felt.NewUnsafeFromString[felt.Felt]("0x3"),
			felt.NewUnsafeFromString[felt.Felt]("0x105"),
			felt.NewUnsafeFromString[felt.Felt]("0x1234"),
			felt.NewUnsafeFromString[felt.Felt]("0x0"),
		},
		ResourceBounds: map[core.Resource]core.ResourceBounds{
			core.ResourceL1Gas: {
				MaxAmount:       4,
				MaxPricePerUnit: felt.NewFromUint64[felt.Felt](l1GasPriceFri + 1),
			},
			core.ResourceL2Gas: {
				MaxAmount:       520000,
				MaxPricePerUnit: felt.NewFromUint64[felt.Felt](l2GasPriceFri + 1),
			},
			core.ResourceL1DataGas: {
				MaxAmount:       296,
				MaxPricePerUnit: felt.NewFromUint64[felt.Felt](l1DataGasPriceFri + 1),
			},
		},
		Tip:                   0x0,
		PaymasterData:         []*felt.Felt{},
		AccountDeploymentData: []*felt.Felt{},
		NonceDAMode:           core.DAModeL1,
		FeeDAMode:             core.DAModeL1,
	}

	receivingTransactionsState, err = transition.OnTransactions(
		t.Context(),
		receivingTransactionsState,
		[]types.Transaction{{Transaction: &invokeTxn}},
	)
	require.NoError(t, err)

	nonEmptyCommitment := types.ProposalCommitment{
		BlockNumber:      head.Number + 1,
		Builder:          *proposerAddr,
		ParentCommitment: *head.Hash,
		Timestamp:        blockInfo.Timestamp,
		ProtocolVersion:  *builder.CurrentStarknetVersion,

		OldStateRoot:          *felt.NewUnsafeFromString[felt.Felt]("0x7629d7aa2c2ae74781626790ab75feb3306b79a41a917bcb923596d12af7f72"),
		StateDiffCommitment:   *felt.NewUnsafeFromString[felt.Felt]("0x16d4498d26bdb6828f7401f8bd8cffb76d49a6be573a30961d0752af4772728"),
		TransactionCommitment: *felt.NewUnsafeFromString[felt.Felt]("0x1286e8721df29411c3f24c8decdef473e1ca758a87ab8d8a4e99ff7511c6fcd"),
		EventCommitment:       *felt.NewUnsafeFromString[felt.Felt]("0x44a528b33c19728469ae78c616070c5a609b57250c630391e64a9532d4bbac9"),
		ReceiptCommitment:     *felt.NewUnsafeFromString[felt.Felt]("0x4e31a7f4aad47f4d3a2e176961c0ccb7ef90f52159da68edd52c9fc0739eb4c"),
		ConcatenatedCounts:    *felt.NewUnsafeFromString[felt.Felt]("0x1000000000000000100000000000000038000000000000000"),
		L1GasPriceFRI:         felt.FromUint64[felt.Felt](l1GasPriceFri),
		L1DataGasPriceFRI:     felt.FromUint64[felt.Felt](l1DataGasPriceFri),
		L2GasPriceFRI:         blockInfo.L2GasPriceFRI,
		L2GasUsed:             *felt.NewUnsafeFromString[felt.Felt]("0x9dbc0"),
		L1DAMode:              blockInfo.L1DAMode,
	}
	awaitingProposalFinState, err := transition.OnProposalCommitment(t.Context(), receivingTransactionsState, &nonEmptyCommitment)
	require.NoError(t, err)

	// Step 5: ProposalFin
	proposalFin := types.ProposalFin(*felt.NewUnsafeFromString[felt.Felt]("0x33104073e2e1af67a09ce406ba489fd00592aa406fba60d6a8a8959f7613080"))
	_, err = transition.OnProposalFin(t.Context(), awaitingProposalFinState, &proposalFin)
	require.NoError(t, err)
}
