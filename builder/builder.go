package builder

import (
	"errors"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/ecdsa"
)

const (
	BlockHashLag          = uint64(10)
	NumTxnsToBatchExecute = 10
)

var (
	ErrPendingParentHash   = errors.New("pending block parent hash does not match chain head")
	CurrentStarknetVersion = semver.MustParse("0.14.0")
)

type BuildParams struct {
	Builder           felt.Felt
	Timestamp         uint64
	L2GasPriceFRI     felt.Felt
	L1GasPriceWEI     felt.Felt
	L1DataGasPriceWEI felt.Felt
	EthToStrkRate     felt.Felt
	L1DAMode          core.L1DAMode
}

type Builder struct {
	// Builder dependencies
	executor   Executor
	blockchain *blockchain.Blockchain
}

func New(
	bc *blockchain.Blockchain,
	executor Executor,
) Builder {
	return Builder{
		blockchain: bc,
		executor:   executor,
	}
}

func (b *Builder) Finalise(pending *sync.Pending, signer utils.BlockSignFunc, privateKey *ecdsa.PrivateKey) error {
	return b.blockchain.Finalise(pending.Block, pending.StateUpdate, pending.NewClasses, signer)
}

func (b *Builder) InitPendingBlock(params *BuildParams) (*BuildState, error) {
	header, err := b.blockchain.HeadsHeader()
	if err != nil {
		return nil, err
	}

	revealedBlockHash, err := b.getRevealedBlockHash(header.Number)
	if err != nil {
		return nil, err
	}

	pendingBlock := core.Block{
		Header: &core.Header{
			Hash:             nil, // To be set after finishing execution
			ParentHash:       header.Hash,
			Number:           header.Number + 1,
			GlobalStateRoot:  nil, // To be set after finishing execution
			SequencerAddress: &params.Builder,
			TransactionCount: 0, // To be updated during transaction execution
			EventCount:       0, // To be updated during transaction execution
			Timestamp:        params.Timestamp,
			ProtocolVersion:  CurrentStarknetVersion.String(),
			EventsBloom:      nil, // To be set after finishing execution
			L1GasPriceETH:    &params.L1GasPriceWEI,
			Signatures:       nil, // To be set after finishing execution
			L1GasPriceSTRK:   new(felt.Felt).Mul(&params.L1GasPriceWEI, &params.EthToStrkRate),
			L1DAMode:         params.L1DAMode,
			L1DataGasPrice: &core.GasPrice{
				PriceInWei: &params.L1DataGasPriceWEI,
				PriceInFri: new(felt.Felt).Mul(&params.L1DataGasPriceWEI, &params.EthToStrkRate),
			},
			L2GasPrice: &core.GasPrice{
				PriceInWei: new(felt.Felt).Div(&params.L2GasPriceFRI, &params.EthToStrkRate),
				PriceInFri: &params.L2GasPriceFRI,
			},
		},
		Transactions: []core.Transaction{},
		Receipts:     []*core.TransactionReceipt{},
	}
	newClasses := make(map[felt.Felt]core.Class)
	emptyStateDiff := core.EmptyStateDiff()
	su := core.StateUpdate{
		StateDiff: &emptyStateDiff,
	}
	pending := sync.Pending{
		Block:       &pendingBlock,
		StateUpdate: &su,
		NewClasses:  newClasses,
	}

	return &BuildState{
		Pending:           &pending,
		RevealedBlockHash: revealedBlockHash,
		L2GasConsumed:     0,
	}, nil
}

func (b *Builder) getRevealedBlockHash(blockHeight uint64) (*felt.Felt, error) {
	if blockHeight < BlockHashLag {
		return nil, nil
	}

	header, err := b.blockchain.BlockHeaderByNumber(blockHeight - BlockHashLag)
	if err != nil {
		return nil, err
	}
	return header.Hash, nil
}

func (b *Builder) PendingState(buildState *BuildState) (state.StateReader, error) {
	if buildState.Pending == nil {
		return nil, sync.ErrPendingBlockNotFound
	}

	headState, err := b.blockchain.HeadState()
	if err != nil {
		return nil, err
	}

	return sync.NewPendingState(buildState.Pending.StateUpdate.StateDiff, buildState.Pending.NewClasses, headState), nil
}

func (b *Builder) RunTxns(state *BuildState, txns []mempool.BroadcastedTransaction) error {
	return b.executor.RunTxns(state, txns)
}

func (b *Builder) Finish(state *BuildState) (BuildResult, error) {
	simulatedResult, err := b.executor.Finish(state)
	if err != nil {
		return BuildResult{}, err
	}

	if simulatedResult.ConcatCount.IsZero() {
		simulatedResult.BlockCommitments = &core.BlockCommitments{
			TransactionCommitment: new(felt.Felt).SetUint64(0),
			EventCommitment:       new(felt.Felt).SetUint64(0),
			ReceiptCommitment:     new(felt.Felt).SetUint64(0),
			StateDiffCommitment:   new(felt.Felt).SetUint64(0),
		}
	}

	// Todo: we ignore some values until the spec is Finalised: VersionConstantCommitment, NextL2GasPriceFRI
	buildResult := BuildResult{
		Pending:        state.Pending,
		SimulateResult: &simulatedResult,
		L2GasConsumed:  state.L2GasConsumed,
	}

	return buildResult, nil
}
