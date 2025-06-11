package proposer

import (
	"context"
	"errors"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mempool"
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
	Txns(ctx context.Context) <-chan TxnBatchResult

	// Blocks until the proposal commitment is ready or context is cancelled.
	ProposalCommitment() (types.ProposalCommitment, error)

	ProposalFin() (types.ProposalFin, error)
}

type TxnBatchResult struct {
	Txns []types.Transaction
	Err  error
}

type proposer struct {
	sequencerAddress *felt.Felt
	builder          *builder.Builder
	mempool          *mempool.Pool
}

func New(builder *builder.Builder, mempool *mempool.Pool, sequencerAddress *felt.Felt) Proposer {
	return &proposer{
		sequencerAddress: sequencerAddress,
		builder:          builder,
		mempool:          mempool,
	}
}

func (p *proposer) ProposalInit() (types.ProposalInit, error) {
	if err := p.builder.ClearPending(); err != nil {
		return types.ProposalInit{}, err
	}

	if err := p.builder.InitPendingBlock(p.sequencerAddress); err != nil {
		return types.ProposalInit{}, err
	}
	pendingBlock := p.builder.PendingBlock()

	return types.ProposalInit{
		BlockNum: pendingBlock.Number,
		Proposer: *pendingBlock.SequencerAddress,
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

	pBlock := p.builder.PendingBlock()

	return types.BlockInfo{
		BlockNumber:   pBlock.Number,
		Builder:       *pBlock.SequencerAddress,
		Timestamp:     uint64(time.Now().Unix()),
		L2GasPriceFRI: *pBlock.L2GasPrice.PriceInFri,
		L1GasPriceWEI: *pBlock.L1GasPriceETH,
		EthToStrkRate: *new(felt.Felt).SetUint64(1), // Todo: SN plan to drop this
		L1DAMode:      core.Blob,
	}, true
}

// Txns streams executed transaction batches until the context is cancelled or an error occurs.
// Returns a channel of TxnBatchResult, which includes either a batch of transactions or an error.
// The channel is closed when the stream ends.
func (p *proposer) Txns(ctx context.Context) <-chan TxnBatchResult { // Todo: consider back pressure
	out := make(chan TxnBatchResult)

	go func() {
		defer close(out)
		for {
			txns, err := p.mempool.PopBatch(NumTxnsToBatchExecute)
			if err != nil {
				if errors.Is(err, mempool.ErrTxnPoolEmpty) {
					time.Sleep(100 * time.Millisecond) //nolint:mnd
					continue
				}
				out <- TxnBatchResult{Err: err}
				return
			}

			if err := p.builder.ExecuteTxns(txns); err != nil {
				out <- TxnBatchResult{Err: err}
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
			out <- TxnBatchResult{Txns: adaptedTxns}
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()

	return out
}

func (p *proposer) ProposalCommitment() (types.ProposalCommitment, error) {
	commitments, concatCommitment, err := p.builder.ExecutePending()
	if err != nil {
		return types.ProposalCommitment{}, err
	}

	pending, err := p.builder.Pending()
	if err != nil {
		return types.ProposalCommitment{}, err
	}

	version, err := semver.NewVersion(pending.Block.ProtocolVersion)
	if err != nil {
		return types.ProposalCommitment{}, err
	}

	// Todo: we ignore some values until the spec is Finalised: VersionConstantCommitment, NextL2GasPriceFRI
	return types.ProposalCommitment{
		BlockNumber:           pending.Block.Number,
		Builder:               *pending.Block.SequencerAddress,
		ParentCommitment:      *pending.Block.ParentHash,
		Timestamp:             pending.Block.Timestamp,
		ProtocolVersion:       *version,
		OldStateRoot:          *pending.StateUpdate.OldRoot,
		StateDiffCommitment:   *commitments.StateDiffCommitment,
		TransactionCommitment: *commitments.TransactionCommitment,
		EventCommitment:       *commitments.EventCommitment,
		ReceiptCommitment:     *commitments.ReceiptCommitment,
		ConcatenatedCounts:    *concatCommitment,
		L1GasPriceFRI:         *pending.Block.L1GasPriceSTRK,
		L1DataGasPriceFRI:     *pending.Block.L1DataGasPrice.PriceInFri,
		L2GasPriceFRI:         *pending.Block.L2GasPrice.PriceInFri,
		L2GasUsed:             *new(felt.Felt).SetUint64(p.builder.L2GasConsumed()),
		L1DAMode:              core.Blob,
	}, nil
}

// ProposalFin() returns the block hash of the pending block
func (p *proposer) ProposalFin() (types.ProposalFin, error) {
	pblock := p.builder.PendingBlock()
	return types.ProposalFin(*pblock.Hash), nil
}
