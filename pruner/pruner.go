// Package pruner implements a background service that bounds on-disk storage
// by deleting block data older than a configurable retention window.
//
// The retention floor is anchored on the lower of the L1-confirmed head
// and the local L2 head: floor = min(l1Head, l2Head) - numRetainedBlocks.
// In normal operation L1 lags L2, so the floor tracks L1; during catch-up
// where L2 is behind L1, we conservatively anchor on L2 instead. Either
// way the floor stays at or below L1, so pruned blocks cannot be reorged.
// The upper bound is the L2 head simply because that is the highest block
// the node has.
//
// In addition to the block-count floor, the pruner enforces an optional
// wallclock minimum-age floor (see [WithMinAge]): blocks whose on-chain
// timestamp is younger than minAge are protected from pruning. The
// combined floor is min(standardFloor, minAgeFloor). The floor is
// suppressed in the L2 path during deep catch-up sync, where incoming
// blocks carry historical timestamps.
//
// The Pruner runs as a [service.Service]. It listens to two trigger feeds —
// new L2 heads and new L1 heads — to know when to re-evaluate the floor;
// the events themselves are only triggers, the retention bound is always
// recomputed from the L1 head.
package pruner

import (
	"context"
	"errors"
	"fmt"
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

// defaultFloorTickInterval is how often the min-age floor is refreshed
// in production. Tighter than minAge so the floor's drift between
// refreshes is bounded by this constant rather than by the (possibly
// large) minAge window. Overridable via [WithFloorTickInterval]
const defaultFloorTickInterval = 15 * time.Minute

// Pruner deletes block data older than a retention window in response to
// new-head events. Construct via [New]; do not zero-value.
type Pruner struct {
	// numRetainedBlocks is the number of blocks retained below the
	// retention pivot (= min(l1Head, l2Head); see package doc). The floor
	// (lowest block we keep) is pivot - numRetainedBlocks and the pruner
	// keeps blocks in [pivot - numRetainedBlocks, l2Head]. The pivot
	// stays at or below L1, so pruned blocks cannot be reorged.
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
	// minAge is the wallclock retention window: blocks whose on-chain
	// timestamp is younger than minAge are protected from pruning.
	// Zero disables the wallclock floor.
	minAge time.Duration
	// floorTickInterval is how often latestSampledHeight is refreshed.
	floorTickInterval time.Duration
	// latestSampledHeight is the lowest block with Timestamp >= now-minAge,
	// re-derived each floorTickInterval tick.
	latestSampledHeight uint64
	database            db.KeyValueStore
	// newHeadSub fires on each new L2 head. During catch-up (L2 < L1) it
	// drives the floor from this event's block number; otherwise the L1
	// path drives the floor and this event acts only as a trigger.
	newHeadSub *feed.Subscription[*core.Block]
	// l1HeadSub fires on each new L1 head. In normal operation (L1 < L2)
	// L1 advances move the retention floor; during catch-up this path
	// short-circuits and the L2 path drives the floor instead.
	l1HeadSub *feed.Subscription[*core.L1Head]
	listener  EventListener
	logger    log.StructuredLogger
}

type options struct {
	targetBatchByteSize int
	l2HeadsPerPrune     uint64
	minAge              time.Duration
	floorTickInterval   time.Duration
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

// WithMinAge enables the wallclock minimum-age floor: blocks whose
// on-chain timestamp is younger than duration are protected from pruning,
// in addition to the standard --prune-mode block-count floor.
// Zero disables the floor.
func WithMinAge(duration time.Duration) Option {
	return func(o *options) {
		o.minAge = duration
	}
}

// WithFloorTickInterval overrides how often the min-age floor is
// refreshed via binary search. Smaller values tighten the
// floor's drift bound at the cost of more frequent refreshes.
func WithFloorTickInterval(duration time.Duration) Option {
	return func(o *options) {
		o.floorTickInterval = duration
	}
}

// New constructs a Pruner. retainedBlocks is the number of blocks
// retained below the retention pivot (= min(l1Head, l2Head); the pivot
// itself is always retained), so the pruner keeps blocks in
// [pivot - retainedBlocks, l2Head] and deletes everything below.
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
		floorTickInterval:   defaultFloorTickInterval,
		listener:            &SelectiveListener{},
	}
	for _, opt := range opts {
		opt(&o)
	}
	return &Pruner{
		numRetainedBlocks:   retainedBlocks,
		targetBatchByteSize: o.targetBatchByteSize,
		l2HeadsPerPrune:     o.l2HeadsPerPrune,
		minAge:              o.minAge,
		floorTickInterval:   o.floorTickInterval,
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
// After some time without L1 heads received, warnings are logged periodically. With no L1
// heads the pruner cannot work.
func (p *Pruner) Run(ctx context.Context) error {
	defer p.newHeadSub.Unsubscribe()
	defer p.l1HeadSub.Unsubscribe()

	const defaultStalenessTick = 24 * time.Hour
	const periodicStalenessTick = 1 * time.Hour
	staleTicker := time.NewTicker(defaultStalenessTick)
	defer staleTicker.Stop()

	// minAge == 0 disables the wallclock floor; a nil channel never
	// fires in select, so the ticker case is effectively absent.
	var sampleTickerC <-chan time.Time
	if p.minAge > 0 {
		if err := p.seedFloor(); err != nil {
			return fmt.Errorf("pruner: seed minimum-age floor: %w", err)
		}
		sampleTicker := time.NewTicker(p.floorTickInterval)
		defer sampleTicker.Stop()
		sampleTickerC = sampleTicker.C
	}

	for {
		select {
		case <-ctx.Done():
			return nil

		case block := <-p.newHeadSub.Recv():
			if err := p.onNewBlock(ctx, block); err != nil {
				p.listener.OnPruneError(err)
				p.logger.Error("on new L2 block",
					zap.Uint64("num", block.Number),
					zap.Error(err),
				)
			}

		case l1Head := <-p.l1HeadSub.Recv():
			if err := p.onNewL1Head(ctx, l1Head); err != nil {
				p.listener.OnPruneError(err)
				p.logger.Error("on new L1 Head",
					zap.Uint64("num", l1Head.BlockNumber),
					zap.Error(err),
				)
			}
			staleTicker.Reset(defaultStalenessTick)

		case <-sampleTickerC:
			if err := p.sampleHeight(); err != nil {
				// seedFloor above already established a usable baseline;
				// transient tick failures leave the previous sample in
				// place and we retry next tick.
				p.logger.Warn("min-retention height sample failed", zap.Error(err))
			}

		case <-staleTicker.C:
			p.listener.OnL1Stale()
			p.logger.Warn("no L1 head received in more than 24 hours. " +
				"Pruning is paused and disk usage will slowly grow until L1 head delivery resumes",
			)
			staleTicker.Reset(periodicStalenessTick)
		}
	}
}

// ErrNoBlockInWindow is returned by [FindOldestBlockAtOrAfter] when no
// block in [lower, upper] has Timestamp >= cutoff.
var ErrNoBlockInWindow = errors.New("no block in window")

// FindOldestBlockAtOrAfter binary-searches block numbers in [lower, upper]
// for the smallest one whose Header.Timestamp is at or after cutoff.
// The caller must pass a range of fully-retained blocks.
func FindOldestBlockAtOrAfter(
	database db.KeyValueStore,
	lower,
	upper uint64,
	cutoff time.Time,
) (uint64, error) {
	if lower > upper {
		return 0, ErrNoBlockInWindow
	}
	cutoffUnix := uint64(cutoff.Unix())
	low, high := lower, upper+1
	for low < high {
		mid := low + (high-low)/2
		header, err := core.GetBlockHeaderByNumber(database, mid)
		if err != nil {
			return 0, err
		}
		if header.Timestamp < cutoffUnix {
			low = mid + 1
		} else {
			high = mid
		}
	}
	if low > upper {
		return 0, ErrNoBlockInWindow
	}
	return low, nil
}

// sampleHeight (re)computes the wallclock floor
func (p *Pruner) sampleHeight() error {
	height, err := core.GetChainHeight(p.database)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil
		}
		return err
	}
	cutoff := time.Now().Add(-p.minAge)
	// Reuse the previous floor as the lower bound: the cutoff only
	// advances, so the new floor can't drop below it.
	floor, err := FindOldestBlockAtOrAfter(p.database, p.latestSampledHeight, height, cutoff)
	if err != nil {
		if errors.Is(err, ErrNoBlockInWindow) {
			// No block qualifies (deep catch-up or chain younger than minAge).
			p.latestSampledHeight = height
			return nil
		}
		return err
	}
	p.latestSampledHeight = floor
	return nil
}

// seedFloor anchors the per-tick search's lower bound to the oldest
// retained block, then runs the first sampleHeight.
func (p *Pruner) seedFloor() error {
	oldest, err := OldestRetainedBlock(p.database)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			// Assumes empty DB
			return nil
		}
		return err
	}
	p.latestSampledHeight = oldest
	return p.sampleHeight()
}

// applyTimeFloor returns the lower of standardFloor and the wallclock floor,
// since a smaller oldestBlockToKeep retains more blocks.
func (p *Pruner) applyTimeFloor(standardFloor uint64) uint64 {
	if p.minAge == 0 {
		return standardFloor
	}
	return min(p.latestSampledHeight, standardFloor)
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

	standardFloor := block.Number - p.numRetainedBlocks
	// Skip the wallclock floor during deep catch-up: an ancient on-chain
	// timestamp means our sample reflects sync-recency, not wallclock-recency.
	oldestToKeep := standardFloor
	if p.minAge > 0 && withinTimeWindow(block.Timestamp, p.minAge) {
		oldestToKeep = p.applyTimeFloor(standardFloor)
	}

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

	oldestBlockToKeep := p.applyTimeFloor(l1Head.BlockNumber - p.numRetainedBlocks)

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

// withinTimeWindow reports whether the Unix-seconds timestamp ts is no
// older than window when measured from wallclock now.
func withinTimeWindow(ts uint64, window time.Duration) bool {
	return ts >= uint64(time.Now().Add(-window).Unix())
}
