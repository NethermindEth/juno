package builder

import (
	"errors"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"

	"github.com/NethermindEth/juno/mempool"
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

func (b *Builder) Network() *utils.Network {
	return b.blockchain.Network()
}

func (b *Builder) Finalise(preconfirmed *core.PreConfirmed, signer utils.BlockSignFunc, privateKey *ecdsa.PrivateKey) error {
	return b.blockchain.Finalise(preconfirmed.Block, preconfirmed.StateUpdate, preconfirmed.NewClasses, signer)
}

func (b *Builder) InitPreconfirmedBlock(params *BuildParams) (*BuildState, error) {
	header, err := b.blockchain.HeadsHeader()
	if err != nil {
		return nil, err
	}

	revealedBlockHash, err := b.getRevealedBlockHash(header.Number)
	if err != nil {
		return nil, err
	}

	preconfirmedBlock := core.Block{
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
	newClasses := make(map[felt.Felt]core.ClassDefinition)
	emptyStateDiff := core.EmptyStateDiff()
	su := core.StateUpdate{
		OldRoot:   header.GlobalStateRoot,
		StateDiff: &emptyStateDiff,
	}
	preconfirmed := core.PreConfirmed{
		Block:                 &preconfirmedBlock,
		StateUpdate:           &su,
		NewClasses:            newClasses,
		TransactionStateDiffs: []*core.StateDiff{},
		CandidateTxs:          []core.Transaction{},
	}

	return &BuildState{
		Preconfirmed:      &preconfirmed,
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

func (b *Builder) PendingState(
	buildState *BuildState,
) (core.CommonStateReader, func() error, error) {
	if buildState.Preconfirmed == nil {
		return nil, nil, core.ErrPendingDataNotFound
	}

	headState, headCloser, err := b.blockchain.HeadState()
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove the state closer once we refactor the state
	return core.NewPendingState(
			buildState.Preconfirmed.StateUpdate.StateDiff,
			buildState.Preconfirmed.NewClasses,
			headState,
		),
		headCloser, nil
}

func (b *Builder) RunTxns(state *BuildState, txns []mempool.BroadcastedTransaction) error {
	if len(txns) == 0 {
		return nil
	}
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
		Preconfirmed:   state.Preconfirmed,
		SimulateResult: &simulatedResult,
		L2GasConsumed:  state.L2GasConsumed,
	}

	return buildResult, nil
}
