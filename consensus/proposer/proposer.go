package proposer

import (
	"context"
	"errors"
	"time"

	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/utils"
)

const NumTxnsToBatchExecute = 10 // TODO: Make this configurable

// Proposer defines the interface for constructing a block proposal in two possible flows:
//
// 1. **Empty Block Flow**
//    - Used when no transactions are available.
//    - Sequence: ProposalInit() → ProposalCommitment() → ProposalFin()
//
// 2. **Non-Empty Block Flow**
//    - Used when transactions are present in the mempool.
//    - Sequence: ProposalInit() → BlockInfo(ctx) → Txns(ctx) → ProposalCommitment() → ProposalFin()
//
// Method semantics:
// - ProposalInit and ProposalFin are synchronous.
// - BlockInfo(ctx) blocks until the mempool is non-empty or the context expires, and indicates if the block is empty.
// - Txns(ctx) streams executed transaction batches until the context is cancelled.
// - ProposalCommitment blocks until the proposal is finalised.
//
// The caller is responsible for coordinating the flow and enforcing timeouts using context.
// All blocking methods should be passed a context with an appropriate deadline to avoid indefinite blocking.
//
// Example usage:
//
//		proposalInit, err := proposer.ProposalInit()
//		...
//
//		blockInfo, isEmpty := proposer.BlockInfo(ctx)
//		if !isEmpty {
//			for res := range proposer.Txns(ctx) {
//				if res.Err != nil {
//					// handle error
//					break
//				}
//				// handle res.Txns
//			}
//		}
//		...
//
//		commitment, err := proposer.ProposalCommitment()
//		...
//
//		fin, err := proposer.ProposalFin()
//		...

//go:generate mockgen -destination=../mocks/mock_proposer.go -package=mocks github.com/NethermindEth/juno/consensus/Proposer Proposer
type Proposer interface {
	ProposalInit() (types.ProposalInit, error)

	// Blocks until BlockInfo is ready, then returns it.
	BlockInfo(ctx context.Context) (types.BlockInfo, bool)

	// Returns a channel that emits transaction batches (can emit multiple times).
	Txns(ctx context.Context) <-chan []types.Transaction

	// Blocks until the proposal commitment is ready or context is cancelled.
	ProposalCommitment() (types.ProposalCommitment, error)

	ProposalFin() (types.ProposalFin, error)
}

type proposer struct {
	sequencerAddress *felt.Felt
	builder          *builder.Builder
	buildState       *builder.BuildState
	mempool          *mempool.Pool
	log              utils.Logger
}

func New(
	b *builder.Builder,
	mempool *mempool.Pool,
	sequencerAddress *felt.Felt,
	log utils.Logger,
) Proposer {
	return &proposer{
		sequencerAddress: sequencerAddress,
		builder:          b,
		buildState:       &builder.BuildState{},
		mempool:          mempool,
		log:              log,
	}
}

func (p *proposer) getDefaultBuildParams() builder.BuildParams {
	return builder.BuildParams{
		Builder:           *p.sequencerAddress,
		Timestamp:         uint64(time.Now().Unix()),
		L2GasPriceFRI:     felt.One,  // TODO: Implement this properly
		L1GasPriceWEI:     felt.One,  // TODO: Implement this properly
		L1DataGasPriceWEI: felt.One,  // TODO: Implement this properly
		EthToStrkRate:     felt.One,  // TODO: Implement this properly
		L1DAMode:          core.Blob, // TODO: Implement this properly
	}
}

func (p *proposer) ProposalInit() (types.ProposalInit, error) {
	err := p.buildState.ClearPending()
	if err != nil {
		return types.ProposalInit{}, err
	}

	buildParams := p.getDefaultBuildParams()

	p.buildState, err = p.builder.InitPendingBlock(&buildParams)
	if err != nil {
		return types.ProposalInit{}, err
	}

	return types.ProposalInit{
		BlockNum: types.Height(p.buildState.PendingBlock().Number),
		Proposer: *p.buildState.PendingBlock().SequencerAddress,
	}, nil
}

// BlockInfo waits until the mempool has transactions and returns block metadata.
// If the context is cancelled or times out first, it returns a zero-value BlockInfo
// and false. Without a context deadline, this method may block indefinitely.
// The boolean return indicates whether the block is non-empty.
func (p *proposer) BlockInfo(ctx context.Context) (types.BlockInfo, bool) {
	// Check if there are any txns to create a non-empty block, with
	// a timeout set by the context
	ticker := time.NewTicker(100 * time.Millisecond) //nolint:mnd
	defer ticker.Stop()
	for {
		if p.mempool.Len() != 0 {
			break
		}

		select {
		case <-ctx.Done():
			return types.BlockInfo{}, false
		case <-ticker.C:
		}
	}

	pBlock := p.buildState.PendingBlock()

	return types.BlockInfo{
		BlockNumber:   pBlock.Number,
		Builder:       *pBlock.SequencerAddress,
		Timestamp:     uint64(time.Now().Unix()),
		L2GasPriceFRI: *pBlock.L2GasPrice.PriceInFri,
		L1GasPriceWEI: *pBlock.L1GasPriceETH,
		EthToStrkRate: *new(felt.Felt).SetUint64(1), // TODO: Implement this properly
		L1DAMode:      core.Blob,
	}, true
}

// Txns streams executed transaction batches until the context is cancelled or an error occurs.
// Returns a channel of TxnBatchResult, which includes either a batch of transactions or an error.
// The channel is closed when the stream ends.
func (p *proposer) Txns(ctx context.Context) <-chan []types.Transaction { // Todo: consider back pressure
	out := make(chan []types.Transaction)

	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			txns, err := p.mempool.PopBatch(NumTxnsToBatchExecute)
			if err != nil {
				if errors.Is(err, mempool.ErrTxnPoolEmpty) {
					time.Sleep(100 * time.Millisecond) //nolint:mnd
					continue
				}
				p.log.Errorw("failed to pop transactions from the mempool", "err", err)
				return
			}

			if err := p.builder.RunTxns(p.buildState, txns); err != nil {
				p.log.Errorw("failed to execute transactions", "err", err)
				return
			}
			adaptedTxns := make([]types.Transaction, len(txns))
			for i := range adaptedTxns {
				adaptedTxns[i] = types.Transaction{
					Transaction: txns[i].Transaction,
					Class:       txns[i].DeclaredClass,
					PaidFeeOnL1: txns[i].PaidFeeOnL1,
				}
			}
			out <- adaptedTxns
		}
	}()

	return out
}

func (p *proposer) ProposalCommitment() (types.ProposalCommitment, error) {
	buildResult, err := p.builder.Finish(p.buildState)
	if err != nil {
		return types.ProposalCommitment{}, err
	}

	proposalCommitment, err := buildResult.ProposalCommitment()
	if err != nil {
		return types.ProposalCommitment{}, err
	}

	return proposalCommitment, nil
}

// ProposalFin() returns the block hash of the pending block
func (p *proposer) ProposalFin() (types.ProposalFin, error) {
	pblock := p.buildState.PendingBlock()
	return types.ProposalFin(*pblock.Hash), nil
}
