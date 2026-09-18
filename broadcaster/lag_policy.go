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
// Stateful per subscription. A lag before any event has been delivered is skipped.
func LagPolicyBlockReplay(
	reader BlockByNumberReader, logger log.StructuredLogger,
) LagPolicy[*core.Block] {
	return func(seq iter.Seq[ring.EventOrLag[*core.Block]]) iter.Seq[*core.Block] {
		var lastNumber uint64
		yielded := false
		return func(yield func(*core.Block) bool) {
			for ev := range seq {
				if block, ok := ev.AsEvent(); ok {
					if !yield(block) {
						return
					}
					lastNumber, yielded = block.Number, true
					continue
				}

				lag, _ := ev.AsLag()
				logger.Warn("broadcaster subscriber lagged; recovering missed blocks from db",
					zap.Uint64("missedSeq", lag.MissedSeq),
					zap.Uint64("nextSeq", lag.NextSeq),
				)
				// No anchor yet, or a degenerate range: nothing to map onto.
				if !yielded || lag.NextSeq <= lag.MissedSeq {
					continue
				}

				dropped := lag.NextSeq - lag.MissedSeq
				for offset := range dropped {
					number := lastNumber + 1 + offset
					recovered, err := reader.BlockByNumber(number)
					if err != nil {
						// Block not yet persisted (chain head runs ahead of commit) or a
						// read error: log and skip; the live stream still flows.
						logger.Warn("block replay could not recover block",
							zap.Uint64("number", number),
							zap.Error(err),
						)
						continue
					}
					if !yield(recovered) {
						return
					}
				}
				// Advance by the full dropped count even if some fetches failed, so the
				// next live block (delivered at NextSeq) stays aligned.
				lastNumber += dropped
			}
		}
	}
}
