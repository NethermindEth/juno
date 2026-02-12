package driver

import (
	"context"
	gosync "sync"

	"github.com/NethermindEth/juno/consensus/proposal"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"go.uber.org/zap"
)

type CommitHook[V types.Hashable[H], H types.Hash] interface {
	OnCommit(context.Context, types.Height, V)
}

// CommitListener is a component that is used to notify different components that a new committed block is available.
type CommitListener[V types.Hashable[H], H types.Hash] interface {
	CommitHook[V, H]
	// Listen returns a channel that will receive committed blocks.
	// This is supposed to be used by the component that writes the committed blocks to the database.
	Listen() <-chan sync.CommittedBlock
}

type commitListener[V types.Hashable[H], H types.Hash] struct {
	log             utils.Logger
	proposalStore   *proposal.ProposalStore[H]
	postCommitHooks []CommitHook[V, H]
	commits         chan sync.CommittedBlock
}

func NewCommitListener[V types.Hashable[H], H types.Hash](
	log utils.Logger,
	proposalStore *proposal.ProposalStore[H],
	postCommitHooks ...CommitHook[V, H],
) CommitListener[V, H] {
	commits := make(chan sync.CommittedBlock)
	return &commitListener[V, H]{
		log:             log,
		proposalStore:   proposalStore,
		postCommitHooks: postCommitHooks,
		commits:         commits,
	}
}

func (b *commitListener[V, H]) OnCommit(ctx context.Context, height types.Height, value V) {
	buildResult := b.proposalStore.Get(value.Hash())
	if buildResult == nil {
		// todo(rdr): we can avoid using the ANY by writing some representation into Hash
		b.log.Error("failed to get build result", zap.Any("hash", value.Hash()))
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

	wg := gosync.WaitGroup{}
	for _, hook := range b.postCommitHooks {
		wg.Go(func() {
			hook.OnCommit(ctx, height, value)
		})
	}
	wg.Wait()
}

func (b *commitListener[V, H]) Listen() <-chan sync.CommittedBlock {
	return b.commits
}
