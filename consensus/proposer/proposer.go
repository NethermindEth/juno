package proposer

import (
	"context"

	"github.com/NethermindEth/juno/consensus/types"
)

// Proposer defines the interface for constructing a block proposal in two possible flows:
//
// 1. **Empty Block Flow**
//   - Used when no transactions are available for inclusion.
//   - Sequence: ProposalInit() → ProposalCommitment() → ProposalFin()
//
// 2. **Non-Empty Block Flow**
//   - Used when transactions are available in the mempool.
//   - Sequence: ProposalInit() → BlockInfo() → Txns() [can emit multiple times] →
//     ProposalCommitment() → ProposalFin()
//
// All methods (except ProposalInit and ProposalFin) are asynchronous:
// - BlockInfo sends a single BlockInfo value to the provided channel when available.
// - Txns may send multiple batches of transactions to the provided channel.
// - ProposalCommitment sends a single commitment once the block is finalized and ready.
//
// The caller controls channel setup and lifetime, allowing integration with select-based
// event loops or reactive flows.
//
// The caller is responsible for managing the channels and coordinating the flow using
// a select loop. Example usage:
//
//	blockInfoCh := make(chan types.BlockInfo, 1)
//	txnsCh := make(chan []types.Transaction, 1)
//	commitmentCh := make(chan types.ProposalCommitment, 1)
//
//	proposer.BlockInfo(blockInfoCh)
//	proposer.Txns(txnsCh)
//	proposer.ProposalCommitment(ctx, commitmentCh)
//
//	for {
//	    select {
//	    case info := <-blockInfoCh:
//	        // handle BlockInfo (only once)
//
//	    case txs := <-txnsCh:
//	        // handle incoming txns (can occur multiple times)
//
//	    case commitment := <-commitmentCh:
//	        // handle commitment and finalize
//	        fin, err := proposer.ProposalFin()
//	        ...
//	        return
//	    }
//	}
//
//go:generate mockgen -destination=../mocks/mock_proposer.go -package=mocks github.com/NethermindEth/juno/consensus/Proposer Proposer
type Proposer interface {
	ProposalInit() (types.ProposalInit, error)
	BlockInfo(out chan<- types.BlockInfo)                                        // sends once
	Txns(out chan<- []types.Transaction)                                         // sends multiple times
	ProposalCommitment(ctx context.Context, out chan<- types.ProposalCommitment) // sends once
	ProposalFin() (types.ProposalFin, error)
}

type proposer struct {
	commitment      types.ProposalCommitment
	commitmentReady chan struct{} // triggers when the final block is executed/simulated
}

func New() Proposer {
	return &proposer{
		commitmentReady: make(chan struct{}),
	}
}

func (p *proposer) ProposalInit() (types.ProposalInit, error) {
	//Todo: implement
	return types.ProposalInit{}, nil
}

func (p *proposer) BlockInfo(out chan<- types.BlockInfo) {
	//Todo: implement
}

func (p *proposer) Txns(out chan<- []types.Transaction) {
	// Todo: implement
}

func (p *proposer) ProposalCommitment(ctx context.Context, out chan<- types.ProposalCommitment) {
	//Todo: implement
}

func (p *proposer) ProposalFin() (types.ProposalFin, error) {
	return types.ProposalFin{}, nil
}
