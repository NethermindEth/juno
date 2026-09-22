package broadcaster

import (
	"iter"

	"github.com/NethermindEth/juno/broadcaster/broadcast/ring"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

// WithLagObserver taps lag notifications for observation (metrics, logging, ...),
// then delegates to policy. It composes with any policy — e.g.
// WithLagObserver(observe, LagPolicyBlockReplay(reader, logger)) records lag and
// still recovers. observe is called with the lag envelope's sequence numbers; the
// gap size is nextSeq-missedSeq. broadcaster stays free of any metrics dependency:
// callers pass a plain callback.
//
// Note: only KindBroadcast surfaces lag, so observe only fires there; KindFeed
// drops silently and ignores lag policies entirely.
func WithLagObserver[T any](
	observe func(missedSeq, nextSeq uint64), policy LagPolicy[T],
) LagPolicy[T] {
	return func(seq iter.Seq[ring.EventOrLag[T]]) iter.Seq[T] {
		tapped := func(yield func(ring.EventOrLag[T]) bool) {
			for ev := range seq {
				if lag, ok := ev.AsLag(); ok {
					observe(lag.MissedSeq, lag.NextSeq)
				}
				if !yield(ev) {
					return
				}
			}
		}
		return policy(tapped)
	}
}

// LagPolicyDrop yields events and silently drops lag notifications. Matches
// feed.Feed's behavior for slow subscribers; use only when the caller has
// no recovery to do and lag is truly tolerable.
func LagPolicyDrop[T any](seq iter.Seq[ring.EventOrLag[T]]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for ev := range seq {
			if v, ok := ev.AsEvent(); ok {
				if !yield(v) {
					return
				}
			}
		}
	}
}

// LagPolicyLog yields events and logs each lag notification at warn level
// with the missed and next-available sequence numbers.
func LagPolicyLog[T any](logger log.StructuredLogger) LagPolicy[T] {
	return func(seq iter.Seq[ring.EventOrLag[T]]) iter.Seq[T] {
		return func(yield func(T) bool) {
			for ev := range seq {
				if v, ok := ev.AsEvent(); ok {
					if !yield(v) {
						return
					}
					continue
				}
				if lag, ok := ev.AsLag(); ok {
					logger.Warn("broadcaster subscriber lagged",
						zap.Uint64("missedSeq", lag.MissedSeq),
						zap.Uint64("nextSeq", lag.NextSeq),
					)
				}
			}
		}
	}
}

// BlockByNumberReader recovers a block by its height, so a lagged subscriber can
// be caught up from durable storage. blockchain.Reader satisfies this directly.
type BlockByNumberReader interface {
	BlockByNumber(number uint64) (*core.Block, error)
}

// LagPolicyBlockReplay forwards live blocks and, on a lag notification, recovers
// the dropped blocks from durable storage.
//
// Stateful per subscription. Lag is mapped onto block numbers from the last delivered
// block; a lag before any event is held and replayed in front of the first block,
// whose number then anchors the range.
//
// Reorgs: the live path is unaffected, since the next block re-anchors the mapping and
// the reorg itself is announced on the sync's reorg feed. A lag before the first block
// replays the canonical chain from the database and is reorg-safe; its range is clamped
// at genesis because the missed count exceeds the height gap by the reorg depth. A lag
// after the anchor maps missed sequences onto numbers above the old anchor: numbers past
// the new head are logged and skipped, and the replacement blocks between the fork point
// and the anchor are not re-sent.
func LagPolicyBlockReplay(
	reader BlockByNumberReader, logger log.StructuredLogger,
) LagPolicy[*core.Block] {
	return func(seq iter.Seq[ring.EventOrLag[*core.Block]]) iter.Seq[*core.Block] {
		var lastDeliveredNumber uint64
		anchored := false
		// Missed count accumulated from lags seen before the first block anchors us.
		var pendingMissed uint64
		return func(yield func(*core.Block) bool) {
			for event := range seq {
				if block, ok := event.AsEvent(); ok {
					if !anchored && pendingMissed > 0 {
						// The missed count exceeds the height gap by the depth of any reorg in
						// it, so clamp at genesis instead of underflowing.
						fromBlock := block.Number - min(pendingMissed, block.Number)
						if !replayBlocks(reader, logger, fromBlock, block.Number, yield) {
							return
						}
						pendingMissed = 0
					}
					if !yield(block) {
						return
					}
					lastDeliveredNumber, anchored = block.Number, true
					continue
				}

				lag, _ := event.AsLag()
				logger.Warn("broadcaster subscriber lagged; recovering missed blocks from db",
					zap.Uint64("missedSeq", lag.MissedSeq),
					zap.Uint64("nextSeq", lag.NextSeq),
				)
				// The ring guarantees NextSeq > MissedSeq: a slot is only overwritten a
				// full capacity later, so the resume point is past the missed sequence.
				missed := lag.NextSeq - lag.MissedSeq
				if !anchored {
					pendingMissed += missed
					continue
				}
				fromBlock := lastDeliveredNumber + 1
				toBlockExclusive := fromBlock + missed
				if !replayBlocks(reader, logger, fromBlock, toBlockExclusive, yield) {
					return
				}
				// Advance past the whole missed range even if some fetches failed, so the
				// next live block (delivered at NextSeq) stays aligned.
				lastDeliveredNumber = toBlockExclusive - 1
			}
		}
	}
}

// replayBlocks yields blocks from fromBlock up to but not including toBlockExclusive,
// skipping any it cannot read; it returns false once the consumer stops.
func replayBlocks(
	reader BlockByNumberReader,
	logger log.StructuredLogger,
	fromBlock,
	toBlockExclusive uint64,
	yield func(*core.Block) bool,
) bool {
	for number := fromBlock; number < toBlockExclusive; number++ {
		block, err := reader.BlockByNumber(number)
		if err != nil {
			// Producers commit before they broadcast, so a miss means the chain reorged
			// across the lag (the number is above the new head) or the read failed;
			// log and skip, the live stream still flows.
			logger.Warn("block replay could not recover block",
				zap.Uint64("number", number),
				zap.Error(err),
			)
			continue
		}
		if !yield(block) {
			return false
		}
	}
	return true
}
