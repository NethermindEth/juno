package sync

import (
	"context"
	"errors"
	"time"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils/lru"
	"go.uber.org/zap"
)

const (
	preLatestCacheSize = 10
)

// isGreaterThanTip reports whether the given number is at or beyond the highest known header.
func (s *Synchronizer) isGreaterThanTip(blockNumber uint64) bool {
	highest := s.highestBlockHeader.Load()
	if highest == nil {
		return false
	}
	return highest.Number < blockNumber
}

// storeEmptyPreConfirmed creates a baseline pre_confirmed at parent.Number+1
// and stores it under the real chain head.
//
// parent is the header the empty pre_confirmed sits directly above and is
// only used to populate the block (sequencer, gas prices, number). It is
// either the chain head (when preLatest is nil) or the pre-latest block
// header (when preLatest is non-nil and the pre_confirmed lives at
// real_head+2).
func (s *Synchronizer) storeEmptyPreConfirmed(
	parent *core.Header,
	preLatest *pending.PreLatest,
) error {
	preConfirmed, err := MakeEmptyPreConfirmedForParent(s.blockchain, parent)
	if err != nil {
		return err
	}
	preConfirmed.WithPreLatest(preLatest)

	head, err := s.blockchain.HeadsHeader()
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			return err
		}
		head = nil
	}

	_, err = s.preConfirmed.StorePreConfirmedForHead(&preConfirmed, head)
	return err
}

// handleTickerPreLatest polls a pre-latest once and either:
//   - emits it to out and returns true when if delivered,
//   - caches it by its ParentHash for a future head and returns false,
//   - returns false on errors or context cancellation.
//
// Caller should invoke only when at tip and not yet delivered for the current head.
func (s *Synchronizer) handleTickerPreLatest(
	ctx context.Context,
	currentHead *core.Block,
	seenByParent *lru.SimpleCache[felt.Felt, *pending.PreLatest],
	out chan<- *pending.PreLatest,
) bool {
	preLatest, err := s.dataSource.BlockPreLatest(ctx)
	if err != nil {
		s.logger.Debug("Error while trying to poll pre_latest block", zap.Error(err))
		return false
	}

	if !preLatest.Block.ParentHash.Equal(currentHead.Hash) {
		seenByParent.Add(*preLatest.Block.ParentHash, &preLatest)
		return false
	}

	preLatest.Block.Number = currentHead.Number + 1

	select {
	case <-ctx.Done():
		return false
	case out <- &preLatest:
		return true
	}
}

// pollPreLatest fetches at most one pre-latest per head while at tip and forwards it to out.
// It avoids duplicate deliveries. If a fetched pre-latest corresponds to a future head,
// it is cached keyed by ParentHash and emitted immediately when that head arrives.
func (s *Synchronizer) pollPreLatest(ctx context.Context, out chan<- *pending.PreLatest) {
	if s.preLatestPollInterval == 0 {
		s.logger.Info("Pre-latest block polling is disabled")
		return
	}

	sub := s.newHeads.SubscribeKeepLast()
	defer sub.Unsubscribe()

	// Cache of pre-latest blocks keyed by the hash of their parent.
	// When we receive the head with this parent hash, we emit the cached pre-latest.
	seenByParent := lru.NewSimple[felt.Felt, *pending.PreLatest](preLatestCacheSize)

	ticker := time.NewTicker(s.preLatestPollInterval)
	defer ticker.Stop()

	var (
		currentHead      *core.Block
		deliveredForHead bool // whether we've already emitted a pre-latest for the current head
	)

	for {
		select {
		case <-ctx.Done():
			return

		case head, ok := <-sub.Recv():
			if !ok {
				// Subscription closed; nothing more to do.
				return
			}

			currentHead = head
			deliveredForHead = false

			// If we already cached a pre-latest for this new head (by its parent hash),
			// emit it immediately and mark as delivered.
			if pl, hit := seenByParent.Get(*currentHead.Hash); hit {
				seenByParent.Remove(*currentHead.Hash)
				pl.Block.Number = currentHead.Number + 1

				select {
				case <-ctx.Done():
					return
				case out <- pl:
					deliveredForHead = true
				}
			}

		case <-ticker.C:
			// We only poll when:
			//   - we have a head
			//   - we are at the tip (or caught up) relative to highest known header
			//   - we have not yet delivered a pre-latest for this head
			shouldPoll := currentHead != nil && s.isGreaterThanTip(currentHead.Header.Number+1) && !deliveredForHead
			if !shouldPoll {
				continue
			}

			deliveredForHead = s.handleTickerPreLatest(
				ctx,
				currentHead,
				seenByParent,
				out,
			)
		}
	}
}

// preConfirmedPoll bundles a wire update with the block number it was fetched
// and the transaction count the poll was based on. baseTxCount echoes the
// knownTransactionCount sent to the server so the consumer can assert the
// stored pre_confirmed still has that many transactions before merging a
// Delta — guarding against duplicate appends when two polls race and observe
// the same base before either has been applied.
type preConfirmedPoll struct {
	update      starknet.PreConfirmedUpdate
	blockNumber uint64
	baseTxCount uint64
}

// pollPreConfirmed polls the feeder for the current pre_confirmed at a fixed
// interval. The poll target and the (identifier, txCount) hints are all read
// from a single atomic load of the stored pre_confirmed, so the three values
// are necessarily coherent. Each poll echoes the stored pre_confirmed's
// identifier and transaction count so the server can return a no-change
// marker, a delta of appended transactions, or a fresh full block when the
// round identifier no longer matches. Polling fires only while at tip
// (stored number is strictly greater than the highest known header). On
// success, the resulting update is forwarded to out.
func (s *Synchronizer) pollPreConfirmed(
	ctx context.Context,
	out chan<- preConfirmedPoll,
) {
	if s.preConfirmedPollInterval == 0 {
		s.logger.Info("Pre-confirmed block polling is disabled")
		return
	}

	ticker := time.NewTicker(s.preConfirmedPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			current := s.preConfirmed.ReadUnsafe()
			if current == nil || !s.isGreaterThanTip(current.Block.Number) {
				continue
			}
			txCount := uint64(len(current.Block.Transactions))

			update, err := s.dataSource.PreConfirmedBlockByNumber(
				ctx, current.Block.Number, current.BlockIdentifier, txCount,
			)
			if err != nil {
				const msg = "polling pre-confirmed block"
				if errors.Is(err, feeder.ErrInvalidFeederResponse) {
					s.logger.Error(msg, zap.Error(err))
					continue
				}
				s.logger.Debug(msg, zap.Error(err), zap.Uint64("block_number", current.Block.Number))
				continue
			}

			select {
			case out <- preConfirmedPoll{
				update:      update,
				blockNumber: current.Block.Number,
				baseTxCount: txCount,
			}:
				continue
			case <-ctx.Done():
				return
			}
		}
	}
}

// handleHead processes a new head.
// It computes nextHeight = head.Number + 1, clears any staged pre_latest if the
// head catches up to the current target, and stores an empty pre_confirmed when
// advancing. The "current target" is implicit in the stored pre_confirmed's
// block number — the baseline write itself publishes the new target.
func (s *Synchronizer) handleHead(
	head *core.Block,
	stagedPreLatest *pending.PreLatest,
) *pending.PreLatest {
	next := head.Number + 1
	var targetNum uint64
	if current := s.preConfirmed.ReadUnsafe(); current != nil {
		targetNum = current.Block.Number
	}

	if next < targetNum {
		return stagedPreLatest
	}

	if next == targetNum {
		s.preConfirmed.UpdatePreLatestAttachment(targetNum, nil)
		return nil
	}

	if err := s.storeEmptyPreConfirmed(head.Header, nil); err != nil {
		s.logger.Debug("Error storing empty pre_confirmed (from head)", zap.Error(err))
	}
	return nil
}

// handlePreLatest processes an incoming pre_latest.
// If it raises the target, it stages the attachment and stores a baseline with it.
// Returns updated staged pre-latest.
func (s *Synchronizer) handlePreLatest(
	pl *pending.PreLatest,
	stagedPreLatest *pending.PreLatest,
) *pending.PreLatest {
	next := pl.Block.Number + 1
	var targetNum uint64
	if current := s.preConfirmed.ReadUnsafe(); current != nil {
		targetNum = current.Block.Number
	}
	if next <= targetNum {
		return stagedPreLatest
	}

	if err := s.storeEmptyPreConfirmed(pl.Block.Header, pl); err != nil {
		s.logger.Debug("Error storing empty pre_confirmed (with pre_latest)", zap.Error(err))
	}

	s.preLatestDataFeed.Send(pl)
	return pl
}

// handlePreConfirmed reconciles a polled pre_confirmed update with the stored
// pre_confirmed.
func (s *Synchronizer) handlePreConfirmed(
	poll preConfirmedPoll,
	stagedPreLatest *pending.PreLatest,
) {
	head, err := s.blockchain.HeadsHeader()
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		s.logger.Debug("Error while loading head for pre_confirmed apply", zap.Error(err))
		return
	}

	if err != nil {
		head = nil
	}

	applied, err := s.preConfirmed.ApplyUpdate(
		poll.update,
		poll.blockNumber,
		poll.baseTxCount,
		head,
		stagedPreLatest,
	)
	if err != nil {
		s.logger.Debug("Error while applying pre_confirmed update", zap.Error(err))
		return
	}

	if applied != nil {
		s.preConfirmedDataFeed.Send(applied)
	}
}

// pollPendingData coordinates pre_latest and pre_confirmed polling.
func (s *Synchronizer) pollPendingData(ctx context.Context) {
	if s.preLatestPollInterval == 0 || s.preConfirmedPollInterval == 0 {
		s.logger.Info("Pending data polling is disabled")
		return
	}

	headsSub := s.newHeads.SubscribeKeepLast()
	defer headsSub.Unsubscribe()

	preLatestCh := make(chan *pending.PreLatest, 1)
	preConfirmedCh := make(chan preConfirmedPoll, 1)

	go s.pollPreLatest(ctx, preLatestCh)
	go s.pollPreConfirmed(ctx, preConfirmedCh)

	var stagedPreLatest *pending.PreLatest

	for {
		select {
		case <-ctx.Done():
			return

		case head, ok := <-headsSub.Recv():
			if !ok {
				return
			}

			stagedPreLatest = s.handleHead(head, stagedPreLatest)
		case pl := <-preLatestCh:
			stagedPreLatest = s.handlePreLatest(pl, stagedPreLatest)
		case pc := <-preConfirmedCh:
			s.handlePreConfirmed(pc, stagedPreLatest)
		}
	}
}
