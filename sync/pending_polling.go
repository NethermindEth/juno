package sync

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils/lru"
	"go.uber.org/zap"
)

const (
	preLatestCacheSize = 10
	// Per-fetch cap so a hung feeder can't hold the running guard for the
	// feeder client's full retry budget (~20s). TODO: consider exposing as flag.
	preConfirmedFetchTimeout = 2 * time.Second
)

// isGreaterThanTip reports whether the given number is at or beyond the highest known header.
func (s *Synchronizer) isGreaterThanTip(blockNumber uint64) bool {
	highest := s.highestBlockHeader.Load()
	if highest == nil {
		return false
	}
	return highest.Number < blockNumber
}

// Returns true if existing preConfirmed is valid for head and incoming is not richer than existing.
// Otherwise returns false.
func shouldPreservePreConfirmed(
	existingPending *pending.PreConfirmed,
	incomingPending *pending.PreConfirmed,
	head *core.Header,
) bool {
	if existingPending == nil {
		return false
	}

	if !existingPending.Validate(head) {
		return false
	}

	existingB := existingPending.GetBlock()
	incomingB := incomingPending.GetBlock()

	if incomingB.Number > existingB.Number {
		return false
	}

	if incomingB.Number == existingB.Number {
		if incomingPending.BlockIdentifier != existingPending.BlockIdentifier {
			return false
		}
		if incomingB.TransactionCount > existingB.TransactionCount {
			return false
		}
	}

	return true
}

// UpdatePreLatestAttachment updates (or clears) the PreLatest attachment of the currently stored
// pre_confirmed at the given blockNumber by atomically swapping the stored pointer.
// Returns true if the store was updated, false if no matching pre_confirmed is stored
// or the attachment was already equal.
func (s *Synchronizer) UpdatePreLatestAttachment(
	blockNumber uint64,
	preLatest *pending.PreLatest,
) bool {
	pc := s.preConfirmed.Load()

	if pc == nil || pc.Block == nil || pc.Block.Number != blockNumber {
		// nil or different height stored; do not touch.
		return false
	}

	if pc.PreLatest == preLatest {
		// No change.
		return false
	}

	// Copy and update attachment on the copy.
	next := pc.Copy()
	next.WithPreLatest(preLatest)

	return s.preConfirmed.CompareAndSwap(pc, next)
}

// StorePreConfirmed stores a pre_confirmed block given that it is for the next height.
// If an equal-number block with >= txCount already exists, we do not overwrite it,
// but we allow updating the PreLatest attachment in-place via a CAS swap.
func (s *Synchronizer) StorePreConfirmed(p *pending.PreConfirmed) (bool, error) {
	if err := core.CheckBlockVersion(p.GetBlock().ProtocolVersion); err != nil {
		return false, err
	}

	head, err := s.blockchain.HeadsHeader()
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			return false, err
		}
		head = nil
	}

	if !p.Validate(head) {
		return false, errors.New("store pre_confirmed not valid for parent")
	}

	existingPtr := s.preConfirmed.Load()

	if existingPtr != nil && shouldPreservePreConfirmed(existingPtr, p, head) {
		_ = s.UpdatePreLatestAttachment(p.GetBlock().Number, p.PreLatest)
		return false, nil
	}

	return s.preConfirmed.CompareAndSwap(existingPtr, p), nil
}

// storeEmptyPreConfirmed creates a baseline pre_confirmed for head+1 and stores it.
// Pass preLatest to attach it to the baseline; pass nil to clear any attachment.
func (s *Synchronizer) storeEmptyPreConfirmed(
	latestHeader *core.Header,
	preLatest *pending.PreLatest,
) error {
	preConfirmed, err := MakeEmptyPreConfirmedForParent(s.blockchain, latestHeader)
	if err != nil {
		return err
	}
	preConfirmed.WithPreLatest(preLatest)
	_, err = s.StorePreConfirmed(&preConfirmed)
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

// requestPreConfirmedRefresh wakes the polling goroutine. Drops the request
// if a fetch is already running: that fetch already serves it.
func (s *Synchronizer) requestPreConfirmedRefresh() {
	if s.preConfirmedFetching.Load() {
		return
	}
	select {
	case s.preConfirmedTrigger <- struct{}{}:
	default:
	}
}

// pollPreConfirmed is request-driven: each trigger from requestPreConfirmedRefresh
// runs one fetch against blockNumberToPoll and forwards the update to out. The
// ticker is a fallback so passive subscribers still get updates when no RPC
// traffic drives refreshes.
func (s *Synchronizer) pollPreConfirmed(
	ctx context.Context,
	blockNumberToPoll *atomic.Uint64,
	out chan<- *pending.PreConfirmedUpdate,
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
		case <-s.preConfirmedTrigger:
		case <-ticker.C:
		}

		// 0 means "no target set yet".
		targetBlockNum := blockNumberToPoll.Load()
		if targetBlockNum == 0 || !s.isGreaterThanTip(targetBlockNum) {
			continue
		}

		blockIdentifier := feeder.PreConfirmedBlankIdentifier
		var knownTransactionCount uint64
		if currentPreConf := s.preConfirmed.Load(); currentPreConf != nil {
			blockIdentifier = currentPreConf.BlockIdentifier
			knownTransactionCount = uint64(len(currentPreConf.Block.Transactions))
		}

		update, err := s.fetchPreConfirmed(ctx, targetBlockNum, blockIdentifier, knownTransactionCount)
		if err != nil {
			s.logger.Debug("Error while trying to poll pre_confirmed block", zap.Error(err))
			continue
		}

		select {
		case out <- &update:
		case <-ctx.Done():
			return
		}
	}
}

// fetchPreConfirmed runs one fetch under the running guard and the per-fetch
// timeout. Drains a buffered trigger before releasing the guard so a refresh
// arriving during this call doesn't fire a back-to-back fetch.
func (s *Synchronizer) fetchPreConfirmed(
	ctx context.Context,
	blockNum uint64,
	blockIdentifier string,
	knownTransactionCount uint64,
) (pending.PreConfirmedUpdate, error) {
	s.preConfirmedFetching.Store(true)
	defer func() {
		select {
		case <-s.preConfirmedTrigger:
		default:
		}
		s.preConfirmedFetching.Store(false)
	}()

	fetchCtx, cancel := context.WithTimeout(ctx, preConfirmedFetchTimeout)
	defer cancel()

	return s.dataSource.PreConfirmedBlockByNumber(
		fetchCtx, blockNum, blockIdentifier, knownTransactionCount,
	)
}

// handleHead processes a new head.
// It computes nextHeight = head.Number + 1, clears any staged pre_latest if the
// head catches up to the current target, and stores an empty pre_confirmed when
// advancing.
func (s *Synchronizer) handleHead(
	head *core.Block,
	targetPreConfirmedNum *atomic.Uint64,
	stagedPreLatest *pending.PreLatest,
) *pending.PreLatest {
	next := head.Number + 1
	targetNum := targetPreConfirmedNum.Load()
	if next < targetNum {
		return stagedPreLatest
	}

	if next == targetNum {
		s.UpdatePreLatestAttachment(targetNum, nil)
		return nil
	}

	targetPreConfirmedNum.Store(next)
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
	targetPreConfirmedNum *atomic.Uint64,
	stagedPreLatest *pending.PreLatest,
) *pending.PreLatest {
	next := pl.Block.Number + 1
	if next <= targetPreConfirmedNum.Load() {
		return stagedPreLatest
	}

	targetPreConfirmedNum.Store(next)
	if err := s.storeEmptyPreConfirmed(pl.Block.Header, pl); err != nil {
		s.logger.Debug("Error storing empty pre_confirmed (with pre_latest)", zap.Error(err))
	}

	s.preLatestDataFeed.Send(pl)
	return pl
}

// handlePreConfirmed reconciles a polled pre_confirmed update with the stored
// pre_confirmed. No-change updates are dropped silently. Full updates replace
// the stored block. Delta updates are applied as an append onto the stored
// block; if the stored identifier has drifted from the update's identifier the
// delta is dropped and the next poll will return Full.
func (s *Synchronizer) handlePreConfirmed(
	update *pending.PreConfirmedUpdate,
	stagedPreLatest *pending.PreLatest,
) {
	var nextPreConfirmed *pending.PreConfirmed

	currentPreConfirmed := s.preConfirmed.Load()
	if currentPreConfirmed == nil {
		s.logger.Debug("No pre_confirmed stored")
		return
	}

	switch update.Mode {
	case pending.PreConfirmedNoChange:
		nextPreConfirmed = currentPreConfirmed.Copy()

	case pending.PreConfirmedFull:
		nextPreConfirmed = update.FullBlock

	case pending.PreConfirmedDelta:
		existing := currentPreConfirmed.Copy()
		if existing.BlockIdentifier != update.BlockIdentifier {
			// Stored identifier drifted; drop. Next poll will return Full.
			nextPreConfirmed = existing
			break
		}
		merged := existing.ApplyDelta(
			update.AppendTransactions,
			update.AppendReceipts,
			update.AppendStateDiffs,
			update.BlockIdentifier,
		)
		nextPreConfirmed = merged
	}

	nextPreConfirmed.WithPreLatest(stagedPreLatest)
	changed, err := s.StorePreConfirmed(nextPreConfirmed)
	if err != nil {
		s.logger.Debug("Error while trying to store pre_confirmed block", zap.Error(err))
		return
	}

	if changed {
		s.preConfirmedDataFeed.Send(nextPreConfirmed)
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
	preConfirmedCh := make(chan *pending.PreConfirmedUpdate, 1)
	var preConfirmedBlockNumberToPoll atomic.Uint64

	go s.pollPreLatest(ctx, preLatestCh)
	go s.pollPreConfirmed(ctx, &preConfirmedBlockNumberToPoll, preConfirmedCh)

	var stagedPreLatest *pending.PreLatest

	for {
		select {
		case <-ctx.Done():
			return

		case head, ok := <-headsSub.Recv():
			if !ok {
				return
			}

			stagedPreLatest = s.handleHead(
				head,
				&preConfirmedBlockNumberToPoll,
				stagedPreLatest,
			)
		case pl := <-preLatestCh:
			stagedPreLatest = s.handlePreLatest(
				pl,
				&preConfirmedBlockNumberToPoll,
				stagedPreLatest,
			)
		case pc := <-preConfirmedCh:
			s.handlePreConfirmed(pc, stagedPreLatest)
		}
	}
}
