// Package pruner implements a background service that bounds on-disk storage
// by deleting block data older than a configurable retention window.
//
// The retention floor is always anchored on the latest L1-confirmed head:
// the pruner keeps blocks in (l1Head - numRetainedBlocks, l2Head] and
// range-deletes everything below. Anchoring the floor on L1 — never on the
// local L2 head — guarantees pruned blocks are reorg-safe, since blocks at
// or below the L1-confirmed height cannot be reorged. The upper bound is
// the L2 head simply because that is the highest block the node has.
//
// The Pruner runs as a [service.Service]. It listens to two trigger feeds —
// new L2 heads and new L1 heads — to know when to re-evaluate the floor;
// the events themselves are only triggers, the retention bound is always
// recomputed from the L1 head.
package pruner

import (
	"context"
	"errors"
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

var _ service.Service = (*Pruner)(nil)

// defaultTargetBatchByteSize is the soft upper bound on accumulated batch
// bytes during a [PruneRange] sweep before the batch is flushed and a fresh
// one is started.
const defaultTargetBatchByteSize = 96 * db.Megabyte

// defaultL2HeadsPerPrune is how many L2 head events the L2 path coalesces
// before triggering a prune, so per-block deletes don't pile up tombstones
// during catch-up. Override via [WithL2HeadsPerPrune]. The L1-driven path
// resets the counter once it prunes, since its sweep from earliestPresent
// covers any leftover coalesced events.
const defaultL2HeadsPerPrune uint64 = 128

// Pruner deletes block data older than a retention window in response to
// new-head events. Construct via [New]; do not zero-value.
type Pruner struct {
	// numRetainedBlocks is the size of the retention window. The retention
	// floor (the lowest block we keep) is l1Head - numRetainedBlocks + 1,
	// so the pruner keeps blocks in (l1Head - numRetainedBlocks, l2Head]
	// and deletes everything below. Anchoring the floor on L1 (not on the
	// local L2 head) guarantees pruned blocks cannot be reorged.
	numRetainedBlocks uint64
	// pendingL2Heads counts how many L2 head events have advanced past the
	// retention floor since the last actual prune. Only touched from Run's
	// dispatch loop, so no synchronization needed.
	pendingL2Heads uint64
	// targetBatchByteSize is the flush threshold inside [PruneRange]'s
	// per-block loop. See [DefaultTargetBatchByteSize].
	targetBatchByteSize int
	// l2HeadsPerPrune is how many L2 head events the L2 path coalesces
	// before triggering a prune. See [defaultL2HeadsPerPrune].
	l2HeadsPerPrune uint64
	database        db.KeyValueStore
	// newHeadSub fires on each new L2 head; the event acts only as a
	// trigger to re-run the retention check. The retention floor itself is
	// always recomputed from the L1 head, never from this event's block.
	newHeadSub *feed.Subscription[*core.Block]
	// l1HeadSub fires on each new L1 head. L1 advances are what actually
	// move the retention floor, since the floor is always anchored on the
	// latest L1-confirmed height.
	l1HeadSub *feed.Subscription[*core.L1Head]
	listener  EventListener
	logger    log.StructuredLogger
}

type options struct {
	targetBatchByteSize int
	l2HeadsPerPrune     uint64
	listener            EventListener
}

type Option func(*options)

func WithListener(listener EventListener) Option {
	return func(o *options) {
		o.listener = listener
	}
}

func WithTargetBatchByteSize(size int) Option {
	return func(o *options) {
		o.targetBatchByteSize = size
	}
}

// WithL2HeadsPerPrune overrides how many L2 head events the L2 path
// coalesces before triggering a prune. See [defaultL2HeadsPerPrune].
func WithL2HeadsPerPrune(n uint64) Option {
	return func(o *options) {
		o.l2HeadsPerPrune = n
	}
}

// New constructs a Pruner. retainedBlocks sets the size of the retention
// window: the floor is anchored on L1, so the pruner keeps blocks in
// (l1Head - retainedBlocks, l2Head] and deletes everything below.
// newHeadSub and l1HeadSub are the two trigger feeds (see [Pruner]).
// Subscriptions are [feed.Subscription.Unsubscribe]'d when [Pruner.Run]
// returns.
func New(
	database db.KeyValueStore,
	retainedBlocks uint64,
	newHeadSub *feed.Subscription[*core.Block],
	l1HeadSub *feed.Subscription[*core.L1Head],
	logger log.StructuredLogger,
	opts ...Option,
) *Pruner {
	o := options{
		targetBatchByteSize: defaultTargetBatchByteSize,
		l2HeadsPerPrune:     defaultL2HeadsPerPrune,
		listener:            &SelectiveListener{},
	}
	for _, opt := range opts {
		opt(&o)
	}
	return &Pruner{
		numRetainedBlocks:   retainedBlocks,
		targetBatchByteSize: o.targetBatchByteSize,
		l2HeadsPerPrune:     o.l2HeadsPerPrune,
		database:            database,
		newHeadSub:          newHeadSub,
		l1HeadSub:           l1HeadSub,
		listener:            o.listener,
		logger:              logger,
	}
}

// Run drives the prune loop until ctx is cancelled. It dispatches each new
// L2 or L1 head event to the corresponding handler. Errors from a handler
// are logged but do not stop the loop — errors returned from service terminates the node.
func (p *Pruner) Run(ctx context.Context) error {
	defer p.newHeadSub.Unsubscribe()
	defer p.l1HeadSub.Unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return nil
		case block := <-p.newHeadSub.Recv():
			if err := p.onNewBlock(ctx, block); err != nil {
				p.listener.OnPruneError(err)
				p.logger.Error("Pruner.Run.onNewBlock", zap.Error(err))
			}
		case l1Head := <-p.l1HeadSub.Recv():
			if err := p.onNewL1Head(ctx, l1Head); err != nil {
				p.listener.OnPruneError(err)
				p.logger.Error("Pruner.Run.onNewL1Head", zap.Error(err))
			}
		}
	}
}

func (p *Pruner) onNewBlock(ctx context.Context, block *core.Block) error {
	l1Head, err := core.GetL1Head(p.database)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			// No L1 head yet — can't anchor the retention floor.
			return nil
		}
		return err
	}

	if l1Head.BlockNumber <= block.Number || block.Number < p.numRetainedBlocks {
		return nil
	}

	p.pendingL2Heads++
	if p.pendingL2Heads < p.l2HeadsPerPrune {
		return nil
	}
	p.pendingL2Heads = 0

	oldestToKeep := block.Number - p.numRetainedBlocks + 1

	return p.pruneUpto(ctx, oldestToKeep)
}

func (p *Pruner) onNewL1Head(ctx context.Context, l1Head *core.L1Head) error {
	chainHeight, err := core.GetChainHeight(p.database)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			// No chain height yet — nothing to prune against.
			return nil
		}
		return err
	}

	if l1Head.BlockNumber >= chainHeight || l1Head.BlockNumber < p.numRetainedBlocks {
		return nil
	}
	p.pendingL2Heads = 0

	oldestBlockToKeep := l1Head.BlockNumber - p.numRetainedBlocks + 1

	return p.pruneUpto(ctx, oldestBlockToKeep)
}

func (p *Pruner) pruneUpto(ctx context.Context, oldestBlockToKeep uint64) error {
	start := time.Now()

	blocksPruned, oldestKept, err := PruneUpto(
		ctx,
		p.database,
		oldestBlockToKeep,
		p.targetBatchByteSize,
	)
	if err != nil {
		return err
	}

	elapsed := time.Since(start)
	p.listener.OnPrune(oldestKept, blocksPruned, elapsed)
	p.logger.Info("Pruner",
		zap.Uint64("oldest_kept", oldestKept),
		zap.Uint64("block_pruned", blocksPruned),
		zap.Duration("elapsed", elapsed),
	)
	return nil
}
