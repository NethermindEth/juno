package proposer

import (
	"context"

	"github.com/NethermindEth/juno/consensus/types"
)

// Proposer defines the interface for constructing a block proposal.
//
// The block proposal process has two flows:
//
// 1. **Empty Block Flow**:
//   - This flow is used when the mempool is empty at the time ProposalInit() is called.
//   - Sequence: ProposalInit() → ProposalCommitment() → ProposalFin()
//
// 2. **Non-Empty Block Flow**:
//   - This flow is used when the mempool contains transactions at the time ProposalInit() is called.
//   - Sequence: ProposalInit() → BlockInfo() → Txns() [can be called multiple times] →
//     ProposalCommitment() → ProposalFin()
//
// By determining the mempool state at ProposalInit(), we can decide which flow to follow,
// simplifying the overall block proposal process.

//go:generate mockgen -destination=../mocks/mock_proposer.go -package=mocks github.com/NethermindEth/juno/consensus/Proposer Proposer
type Proposer interface {
	ProposalInit() (types.ProposalInit, bool, error) // Bool to indicate if we will propose an empty block or not
	BlockInfo() (types.BlockInfo, error)
	Txns() ([]types.Transaction, error)
	ProposalCommitment(ctx context.Context) (types.ProposalCommitment, error)
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

func (p *proposer) ProposalInit() (types.ProposalInit, bool, error) {
	// Todo: implement

	isEmpty := false
	return types.ProposalInit{}, isEmpty, nil
}

func (p *proposer) BlockInfo() (types.BlockInfo, error) {
	// Todo: implement
	return types.BlockInfo{}, nil
}

func (p *proposer) Txns() ([]types.Transaction, error) {
	// Todo: implement
	return []types.Transaction{}, nil
}

func (p *proposer) ProposalCommitment(ctx context.Context) (types.ProposalCommitment, error) {
	// Todo: implement
	select {
	case <-ctx.Done():
		return types.ProposalCommitment{}, ctx.Err()
	case <-p.commitmentReady:
		return p.commitment, nil
	}
}

func (p *proposer) ProposalFin() (types.ProposalFin, error) {
	// Todo: implement
	return types.ProposalFin{}, nil
}
