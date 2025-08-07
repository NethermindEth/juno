package driver

import (
	"context"

	"github.com/NethermindEth/juno/consensus/p2p"
	"github.com/NethermindEth/juno/consensus/proposal"
	"github.com/NethermindEth/juno/consensus/proposer"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
)

// CommitListener is a component that is used to notify different components that a new committed block is available.
type CommitListener[V types.Hashable[H], H types.Hash, A types.Addr] interface {
	// Commit is called by Tendermint when a block has been decided on and can be committed to the DB.
	Commit(context.Context, types.Height, V)
	// Listen returns a channel that will receive committed blocks.
	// This is supposed to be used by the component that writes the committed blocks to the database.
	Listen() <-chan sync.CommittedBlock
}

type commitListener[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	log           utils.Logger
	proposalStore *proposal.ProposalStore[H]
	proposer      proposer.Proposer[V, H]
	p2p           p2p.P2P[V, H, A]
	commits       chan sync.CommittedBlock
}

func NewCommitListener[V types.Hashable[H], H types.Hash, A types.Addr](
	log utils.Logger,
	proposalStore *proposal.ProposalStore[H],
	proposer proposer.Proposer[V, H],
	p2p p2p.P2P[V, H, A],
) CommitListener[V, H, A] {
	commits := make(chan sync.CommittedBlock)
	return &commitListener[V, H, A]{
		log:           log,
		proposalStore: proposalStore,
		proposer:      proposer,
		p2p:           p2p,
		commits:       commits,
	}
}

func (b *commitListener[V, H, A]) Commit(ctx context.Context, height types.Height, value V) {
	buildResult := b.proposalStore.Get(value.Hash())
	if buildResult == nil {
		b.log.Errorw("failed to get build result", "hash", value.Hash())
		return
	}

	committedBlock := sync.CommittedBlock{
		Block:       buildResult.Preconfirmed.Block,
		StateUpdate: buildResult.Preconfirmed.StateUpdate,
		NewClasses:  buildResult.Preconfirmed.NewClasses,
		Persisted:   make(chan struct{}),
	}

	select {
	case <-ctx.Done():
		return
	case b.commits <- committedBlock:
	}

	select {
	case <-ctx.Done():
		return
	case <-committedBlock.Persisted:
	}

	b.proposer.OnCommit(ctx, height, value)
	b.p2p.OnCommit(ctx, height, value)
}

func (b *commitListener[V, H, A]) Listen() <-chan sync.CommittedBlock {
	return b.commits
}
