package validator

import (
	"fmt"
	"slices"
	"testing"

	"github.com/NethermindEth/juno/adapters/consensus2p2p"
	"github.com/NethermindEth/juno/adapters/core2p2p"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/clients/feeder"
	starknetconsensus "github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/statetestutils"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/starknet"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"github.com/stretchr/testify/require"
)

type TestCase struct {
	Height       types.Height
	Round        types.Round
	ValidRound   types.Round
	Network      *utils.Network
	TxBatchCount int
}

type TestFixture struct {
	ProposalInit       *consensus.ProposalPart
	BlockInfo          *consensus.ProposalPart
	Transactions       []*consensus.ProposalPart
	ProposalCommitment *consensus.ProposalPart
	ProposalFin        *consensus.ProposalPart
	Proposal           *starknetconsensus.Proposal
	BuildResult        *builder.BuildResult
	Builder            *builder.Builder
	PreState           *builder.BuildState
}

func SetChainHeight(t *testing.T, database db.KeyValueStore, chainHeight types.Height) {
	require.NoError(t, database.Update(func(txn db.IndexedBatch) error {
		return core.WriteChainHeight(txn, uint64(chainHeight))
	}))
}

func LoadBlockDependencies(t *testing.T, database db.KeyValueStore, height types.Height, network *utils.Network) (headBlock, revealedBlock *core.Block) {
	var err error
	t.Helper()

	gw := adaptfeeder.New(feeder.NewTestClient(t, network))

	headBlock, err = gw.BlockByNumber(t.Context(), uint64(height-1))
	require.NoError(t, err)

	revealedBlock, err = gw.BlockByNumber(t.Context(), headBlock.Number-builder.BlockHashLag)
	require.NoError(t, err)

	err = database.Update(func(txn db.IndexedBatch) error {
		require.NoError(t, core.WriteBlockHeaderByNumber(txn, revealedBlock.Header))
		require.NoError(t, core.WriteBlockHeaderByNumber(txn, headBlock.Header))
		return nil
	})
	require.NoError(t, err)

	return headBlock, revealedBlock
}

func BuildTestFixture(
	t *testing.T,
	executor *mockExecutor,
	database db.KeyValueStore,
	testCase TestCase,
) TestFixture {
	t.Helper()

	client := feeder.NewTestClient(t, testCase.Network)
	gw := adaptfeeder.New(client)

	rawStateUpdate, err := client.StateUpdateWithBlock(t.Context(), fmt.Sprintf("%d", testCase.Height))
	require.NoError(t, err)

	stateUpdate, block, err := gw.StateUpdateWithBlock(t.Context(), uint64(testCase.Height))
	require.NoError(t, err)

	headBlock, revealedBlock := LoadBlockDependencies(t, database, testCase.Height, testCase.Network)

	proposer := &common.Address{Elements: ToBytes(*block.Header.SequencerAddress)}
	concatCounts := calculateConcatCounts(block, stateUpdate)
	totalGasConsumed := calculateTotalGasConsumed(rawStateUpdate)

	proposalInit := buildProposalInit(testCase, proposer)
	blockInfo := buildBlockInfo(testCase.Height, proposer, block)
	transactions := buildTransactions(t, gw, block, testCase.TxBatchCount)
	proposalCommitment := buildProposalCommitment(rawStateUpdate, block, headBlock, proposer, concatCounts, totalGasConsumed)
	proposalFin := buildProposalFin(block)

	buildResult := buildBuildResult(t, gw, block, stateUpdate, rawStateUpdate, concatCounts, totalGasConsumed)

	proposal := buildProposal(testCase.Round, testCase.ValidRound, block)

	preState := buildPreState(&buildResult, headBlock.Header, revealedBlock.Header)

	executor.RegisterBuildResult(&buildResult)

	builder := builder.New(
		blockchain.New(database, testCase.Network, statetestutils.UseNewState()),
		executor,
	)

	return TestFixture{
		ProposalInit:       &proposalInit,
		BlockInfo:          &blockInfo,
		Transactions:       transactions,
		ProposalCommitment: &proposalCommitment,
		ProposalFin:        &proposalFin,
		Proposal:           &proposal,
		BuildResult:        &buildResult,
		Builder:            &builder,
		PreState:           &preState,
	}
}

func buildProposalInit(
	testCase TestCase,
	proposer *common.Address,
) consensus.ProposalPart {
	var validRoundPtr *uint32
	if testCase.ValidRound != -1 {
		validRoundPtr = utils.HeapPtr(uint32(testCase.ValidRound))
	}

	return consensus.ProposalPart{
		Messages: &consensus.ProposalPart_Init{
			Init: &consensus.ProposalInit{
				BlockNumber: uint64(testCase.Height),
				Round:       uint32(testCase.Round),
				Proposer:    proposer,
				ValidRound:  validRoundPtr,
			},
		},
	}
}

func buildBlockInfo(height types.Height, proposer *common.Address, block *core.Block) consensus.ProposalPart {
	ethToStrkRate := new(felt.Felt).Div(block.L1DataGasPrice.PriceInFri, block.L1DataGasPrice.PriceInWei)
	return consensus.ProposalPart{
		Messages: &consensus.ProposalPart_BlockInfo{
			BlockInfo: &consensus.BlockInfo{
				BlockNumber:       uint64(height),
				Builder:           proposer,
				Timestamp:         block.Header.Timestamp,
				L2GasPriceFri:     core2p2p.AdaptUint128(block.L2GasPrice.PriceInFri),
				L1GasPriceWei:     core2p2p.AdaptUint128(block.L1GasPriceETH),
				L1DataGasPriceWei: core2p2p.AdaptUint128(block.L1DataGasPrice.PriceInWei),
				EthToStrkRate:     core2p2p.AdaptUint128(ethToStrkRate),
				L1DaMode:          common.L1DataAvailabilityMode(block.L1DAMode),
			},
		},
	}
}

func buildTransactions(t *testing.T, gw *adaptfeeder.Feeder, block *core.Block, txBatchCount int) []*consensus.ProposalPart {
	txBatchSize := (len(block.Transactions)-1)/txBatchCount + 1
	txBatches := []*consensus.ProposalPart{}

	for txs := range slices.Chunk(block.Transactions, txBatchSize) {
		transactions := make([]types.Transaction, len(txs))

		for i, tx := range txs {
			var class core.ClassDefinition
			var paidFeeOnL1 *felt.Felt
			switch tx := tx.(type) {
			case *core.DeclareTransaction:
				var err error
				class, err = gw.Class(t.Context(), tx.ClassHash)
				require.NoError(t, err)
			case *core.L1HandlerTransaction:
				paidFeeOnL1 = new(felt.Felt).SetUint64(1)
			}
			transactions[i] = types.Transaction{
				Transaction: tx,
				Class:       class,
				PaidFeeOnL1: paidFeeOnL1,
			}
		}

		batch, err := consensus2p2p.AdaptProposalTransaction(transactions)
		require.NoError(t, err)

		txBatches = append(txBatches, &consensus.ProposalPart{
			Messages: &consensus.ProposalPart_Transactions{
				Transactions: &batch,
			},
		})
	}

	return txBatches
}

func buildProposalCommitment(
	rawStateUpdate *starknet.StateUpdateWithBlock,
	block *core.Block,
	previousBlock *core.Block,
	proposer *common.Address,
	concatCounts felt.Felt,
	totalGasConsumed int,
) consensus.ProposalPart {
	// TODO: This is only available starting from starknet 0.14.0. We needs to implement the proper formula.
	nextL2GasPriceFri := new(felt.Felt).Add(block.L2GasPrice.PriceInFri, felt.One.Clone())
	return consensus.ProposalPart{
		Messages: &consensus.ProposalPart_Commitment{
			Commitment: &consensus.ProposalCommitment{
				BlockNumber:               block.Number,
				ParentCommitment:          &common.Hash{Elements: ToBytes(*block.Header.ParentHash)},
				Builder:                   proposer,
				Timestamp:                 block.Header.Timestamp,
				ProtocolVersion:           rawStateUpdate.Block.Version,
				OldStateRoot:              &common.Hash{Elements: ToBytes(*previousBlock.GlobalStateRoot)},
				VersionConstantCommitment: &common.Hash{Elements: ToBytes(felt.Zero)}, // TODO: Add version constant commitment
				StateDiffCommitment:       &common.Hash{Elements: ToBytes(*rawStateUpdate.Block.StateDiffCommitment)},
				TransactionCommitment:     &common.Hash{Elements: ToBytes(*rawStateUpdate.Block.TransactionCommitment)},
				EventCommitment:           &common.Hash{Elements: ToBytes(*rawStateUpdate.Block.EventCommitment)},
				ReceiptCommitment:         &common.Hash{Elements: ToBytes(*rawStateUpdate.Block.ReceiptCommitment)},
				ConcatenatedCounts:        &common.Felt252{Elements: ToBytes(concatCounts)},
				L1GasPriceFri:             core2p2p.AdaptUint128(block.L1GasPriceSTRK),
				L1DataGasPriceFri:         core2p2p.AdaptUint128(rawStateUpdate.Block.L1DataGasPrice.PriceInFri),
				L2GasPriceFri:             core2p2p.AdaptUint128(block.L2GasPrice.PriceInFri),
				L2GasUsed:                 &common.Uint128{Low: uint64(totalGasConsumed), High: 0},
				NextL2GasPriceFri:         core2p2p.AdaptUint128(nextL2GasPriceFri),
				L1DaMode:                  common.L1DataAvailabilityMode(block.L1DAMode),
			},
		},
	}
}

func buildProposalFin(block *core.Block) consensus.ProposalPart {
	return consensus.ProposalPart{
		Messages: &consensus.ProposalPart_Fin{
			Fin: &consensus.ProposalFin{
				ProposalCommitment: &common.Hash{Elements: ToBytes(*block.Hash)},
			},
		},
	}
}

func buildBuildResult(
	t *testing.T,
	gw *adaptfeeder.Feeder,
	block *core.Block,
	stateUpdate *core.StateUpdate,
	rawStateUpdate *starknet.StateUpdateWithBlock,
	concatCounts felt.Felt,
	totalGasConsumed int,
) builder.BuildResult {
	return builder.BuildResult{
		Preconfirmed: &core.PreConfirmed{
			Block:       block,
			StateUpdate: stateUpdate,
			NewClasses:  calculateNewClasses(t, gw, stateUpdate),
		},
		SimulateResult: &blockchain.SimulateResult{
			BlockCommitments: &core.BlockCommitments{
				TransactionCommitment: rawStateUpdate.Block.TransactionCommitment,
				EventCommitment:       rawStateUpdate.Block.EventCommitment,
				ReceiptCommitment:     rawStateUpdate.Block.ReceiptCommitment,
				StateDiffCommitment:   rawStateUpdate.Block.StateDiffCommitment,
			},
			ConcatCount: concatCounts,
		},
		L2GasConsumed: uint64(totalGasConsumed),
	}
}

func buildProposal(round, validRound types.Round, block *core.Block) starknetconsensus.Proposal {
	return starknetconsensus.Proposal{
		MessageHeader: starknetconsensus.MessageHeader{
			Height: types.Height(block.Number),
			Round:  round,
			Sender: starknetconsensus.Address(*block.SequencerAddress),
		},
		ValidRound: validRound,
		Value:      utils.HeapPtr(starknetconsensus.Value(*block.Hash)),
	}
}

func buildPreState(buildResult *builder.BuildResult, headBlockHeader, revealedBlockHeader *core.Header) builder.BuildState {
	strippedBlockHeader := *buildResult.Preconfirmed.Block.Header
	strippedBlockHeader.Hash = nil
	strippedBlockHeader.GlobalStateRoot = nil
	strippedBlockHeader.TransactionCount = 0
	strippedBlockHeader.EventCount = 0
	strippedBlockHeader.EventsBloom = nil
	strippedBlockHeader.Signatures = nil
	return builder.BuildState{
		Preconfirmed: &core.PreConfirmed{
			Block: &core.Block{
				Header:       &strippedBlockHeader,
				Transactions: []core.Transaction{},
				Receipts:     []*core.TransactionReceipt{},
			},
			StateUpdate: &core.StateUpdate{
				OldRoot:   headBlockHeader.GlobalStateRoot,
				StateDiff: utils.HeapPtr(core.EmptyStateDiff()),
			},
			NewClasses:            map[felt.Felt]core.ClassDefinition{},
			TransactionStateDiffs: []*core.StateDiff{},
			CandidateTxs:          []core.Transaction{},
		},
		L2GasConsumed:     0,
		RevealedBlockHash: revealedBlockHeader.Hash,
	}
}

func calculateTotalGasConsumed(rawStateUpdate *starknet.StateUpdateWithBlock) int {
	totalGasConsumed := 0
	for _, receipt := range rawStateUpdate.Block.Receipts {
		consumed := receipt.ExecutionResources.TotalGasConsumed
		totalGasConsumed += int(consumed.L1Gas + consumed.L2Gas + consumed.L1DataGas)
	}
	return totalGasConsumed
}

func calculateConcatCounts(block *core.Block, stateUpdate *core.StateUpdate) felt.Felt {
	return core.ConcatCounts(block.TransactionCount, block.EventCount, stateUpdate.StateDiff.Length(), block.L1DAMode)
}

func calculateNewClasses(
	t *testing.T,
	gw *adaptfeeder.Feeder,
	stateUpdate *core.StateUpdate,
) map[felt.Felt]core.ClassDefinition {
	newClasses := make(map[felt.Felt]core.ClassDefinition)
	for classHash := range stateUpdate.StateDiff.DeclaredV1Classes {
		class, err := gw.Class(t.Context(), &classHash)
		require.NoError(t, err)
		newClasses[classHash] = class
	}
	return newClasses
}
