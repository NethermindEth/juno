package sync

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/sync/pendingdata"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common/lru"
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

// Returns true if existing pendingData is valid for head and incoming is not richer than existing.
// Otherwise returns false.
func shouldPreservePendingData(
	existingPending core.PendingData,
	incomingPending core.PendingData,
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

	return (incomingB.Number < existingB.Number) ||
		(incomingB.Number == existingB.Number && incomingB.TransactionCount <= existingB.TransactionCount)
}

// StorePending stores a pending block given that it is for the next height
func (s *Synchronizer) StorePending(p *core.Pending) (bool, error) {
	if err := core.CheckBlockVersion(p.Block.ProtocolVersion); err != nil {
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
		return false, fmt.Errorf("store pending: %w", blockchain.ErrParentDoesNotMatchHead)
	}

	existingPtr := s.pendingData.Load()

	if existingPtr != nil && shouldPreservePendingData(*existingPtr, p, head) {
		// ignore the incoming pending if it has fewer transactions than the one we already have
		return false, nil
	}

	return s.pendingData.CompareAndSwap(existingPtr, utils.HeapPtr[core.PendingData](p)), nil
}

// UpdatePreLatestAttachment updates (or clears) the PreLatest attachment of the currently stored
// pre_confirmed at the given blockNumber by atomically swapping the stored pointer.
// Returns true if the store was updated, false if no matching pre_confirmed is stored
// or the attachment was already equal.
func (s *Synchronizer) UpdatePreLatestAttachment(blockNumber uint64, preLatest *core.PreLatest) bool {
	curPtr := s.pendingData.Load()
	if curPtr == nil || *curPtr == nil {
		return false
	}

	cur := *curPtr
	pc, ok := cur.(*core.PreConfirmed)
	if !ok {
		return false
	}

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

	return s.pendingData.CompareAndSwap(curPtr, utils.HeapPtr[core.PendingData](next))
}

// StorePreConfirmed stores a pre_confirmed block given that it is for the next height.
// If an equal-number block with >= txCount already exists, we do not overwrite it,
// but we allow updating the PreLatest attachment in-place via a CAS swap.
func (s *Synchronizer) StorePreConfirmed(p *core.PreConfirmed) (bool, error) {
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

	existingPtr := s.pendingData.Load()

	if existingPtr != nil && shouldPreservePendingData(*existingPtr, p, head) {
		_ = s.UpdatePreLatestAttachment(p.GetBlock().Number, p.PreLatest)
		return false, nil
	}

	return s.pendingData.CompareAndSwap(
		existingPtr,
		utils.HeapPtr[core.PendingData](p),
	), nil
}

func (s *Synchronizer) storeEmptyPending(latestHeader *core.Header) error {
	pending, err := pendingdata.MakeEmptyPendingForParent(s.blockchain, latestHeader)
	if err != nil {
		return err
	}

	_, err = s.StorePending(&pending)
	return err
}

// storeEmptyPreConfirmed creates a baseline pre_confirmed for head+1 and stores it.
// Pass preLatest to attach it to the baseline; pass nil to clear any attachment.
func (s *Synchronizer) storeEmptyPreConfirmed(
	latestHeader *core.Header,
	preLatest *core.PreLatest,
) error {
	preConfirmed, err := pendingdata.MakeEmptyPreConfirmedForParent(s.blockchain, latestHeader)
	if err != nil {
		return err
	}
	preConfirmed.WithPreLatest(preLatest)
	_, err = s.StorePreConfirmed(&preConfirmed)
	return err
}

func (s *Synchronizer) storeEmptyPendingData(lastHeader *core.Header) {
	needPreConfirmed, err := pendingdata.NeedsPreConfirmed(lastHeader.ProtocolVersion)
	if err != nil {
		s.log.Errorw("Failed to parse block version", "err", err)
		return
	}

	if needPreConfirmed {
		if err := s.storeEmptyPreConfirmed(lastHeader, nil); err != nil {
			s.log.Errorw("Failed to store empty pre_confirmed block", "number", lastHeader.Number)
		}
	} else {
		if err := s.storeEmptyPending(lastHeader); err != nil {
			s.log.Errorw("Failed to store empty pending block", "number", lastHeader.Number)
		}
	}
}

// pollPending periodically polls the pending block while at tip and forwards it to out.
func (s *Synchronizer) pollPending(ctx context.Context, out chan<- *core.Pending) {
	if s.pendingPollInterval == 0 {
		s.log.Infow("Pending block polling is disabled")
		return
	}

	ticker := time.NewTicker(s.pendingPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			highest := s.highestBlockHeader.Load()
			if highest == nil {
				// No highest known header; nothing to do yet.
				continue
			}

			head, err := s.blockchain.HeadsHeader()
			if err != nil {
				s.log.Debugw("Error while reading head header", "err", err)
				continue
			}

			if !s.isGreaterThanTip(head.Number + 1) {
				continue
			}

			pending, err := s.dataSource.BlockPending(ctx)
			if err != nil {
				s.log.Debugw("Error while trying to poll pending block", "err", err)
				continue
			}
			pending.Block.Number = head.Number + 1

			select {
			case out <- &pending:
				continue
			case <-ctx.Done():
				return
			}
		}
	}
}

// runPendingPhase processes pending blocks and empty pending baselines until protocol >= 0.14.0 is detected.
func (s *Synchronizer) runPendingPhase(ctx context.Context, headsSub *feed.Subscription[*core.Block]) bool {
	pendingCh := make(chan *core.Pending, 1)
	pendingCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go s.pollPending(pendingCtx, pendingCh)

	for {
		select {
		case <-ctx.Done():
			return false

		case head, ok := <-headsSub.Recv():
			if !ok {
				return false
			}
			need, err := pendingdata.NeedsPreConfirmed(head.ProtocolVersion)
			if err != nil {
				s.log.Debugw("Failed to parse protocol version", "err", err)
				continue
			}

			if need {
				return true // switch to pre_confirmed
			}

			if err := s.storeEmptyPending(head.Header); err != nil {
				s.log.Debugw("Error storing empty pending", "err", err)
			}

		case p := <-pendingCh:
			need, err := pendingdata.NeedsPreConfirmed(p.Block.ProtocolVersion)
			if err != nil {
				s.log.Debugw("Failed to parse pending protocol version", "err", err)
				continue
			}

			if need {
				return true // switch to pre_confirmed
			}

			if changed, err := s.StorePending(p); err != nil {
				s.log.Debugw("Error storing pending block", "err", err)
			} else if changed {
				s.pendingDataFeed.Send(p)
			}
		}
	}
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
	seenByParent *lru.BasicLRU[felt.Felt, *core.PreLatest],
	out chan<- *core.PreLatest,
) bool {
	pending, err := s.dataSource.BlockPending(ctx)
	if err != nil {
		s.log.Debugw("Error while trying to poll pre_latest block", "err", err)
		return false
	}

	preLatest := core.PreLatest(pending)

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
func (s *Synchronizer) pollPreLatest(ctx context.Context, out chan<- *core.PreLatest) {
	if s.pendingPollInterval == 0 {
		s.log.Infow("Pre-latest block polling is disabled")
		return
	}

	sub := s.newHeads.SubscribeKeepLast()
	defer sub.Unsubscribe()

	// Cache of pre-latest blocks keyed by the hash of their parent.
	// When we receive the head with this parent hash, we emit the cached pre-latest.
	seenByParent := lru.NewBasicLRU[felt.Felt, *core.PreLatest](preLatestCacheSize)

	ticker := time.NewTicker(s.pendingPollInterval)
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
				&seenByParent,
				out,
			)
		}
	}
}

// pollPreConfirmed polls for the current target pre_confirmed number at a fixed interval.
// The target is read from blockNumberToPoll and only polled while at tip.
// On success, the pre_confirmed is forwarded to out.
func (s *Synchronizer) pollPreConfirmed(
	ctx context.Context,
	blockNumberToPoll *atomic.Uint64,
	out chan<- *core.PreConfirmed,
) {
	if s.preConfirmedPollInterval == 0 {
		s.log.Infow("Pre-confirmed block polling is disabled")
		return
	}

	ticker := time.NewTicker(s.preConfirmedPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			targetBlockNum := blockNumberToPoll.Load()
			shouldPoll := targetBlockNum > 0 && s.isGreaterThanTip(targetBlockNum)
			if !shouldPoll {
				continue
			}

			preConfirmed, err := s.dataSource.PreConfirmedBlockByNumber(ctx, targetBlockNum)
			if err != nil {
				s.log.Debugw("Error while trying to poll pre_confirmed block", "err", err)
				continue
			}

			select {
			case out <- &preConfirmed:
				continue
			case <-ctx.Done():
				return
			}
		}
	}
}

// handleHeadInPreConfirmed processes a new head during the pre_confirmed phase.
// It computes nextHeight = head.Number + 1, clears any staged pre_latest if the
// head catches up to the current target, and stores an empty pre_confirmed when
// advancing.
func (s *Synchronizer) handleHeadInPreConfirmedPhase(
	head *core.Block,
	targetPreConfirmedNum *atomic.Uint64,
	stagedPreLatest *core.PreLatest,
) *core.PreLatest {
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
		s.log.Debugw("Error storing empty pre_confirmed (from head)", "err", err)
	}
	return nil
}

// handlePreLatest processes an incoming pre_latest during the pre_confirmed phase.
// If it raises the target, it stages the attachment and stores a baseline with it.
// Returns updated staged pre-latest.
func (s *Synchronizer) handlePreLatest(
	pl *core.PreLatest,
	targetPreConfirmedNum *atomic.Uint64,
	stagedPreLatest *core.PreLatest,
) *core.PreLatest {
	next := pl.Block.Number + 1
	if next <= targetPreConfirmedNum.Load() {
		return stagedPreLatest
	}

	targetPreConfirmedNum.Store(next)
	if err := s.storeEmptyPreConfirmed(pl.Block.Header, pl); err != nil {
		s.log.Debugw("Error storing empty pre_confirmed (with pre_latest)", "err", err)
	}

	s.preLatestDataFeed.Send(pl)
	return pl
}

// handlePreConfirmed finalises when the polled pre_confirmed equals the target.
// It attaches the staged pre_latest, stores it, and feeds if changed.
func (s *Synchronizer) handlePreConfirmed(
	pc *core.PreConfirmed,
	stagedPreLatest *core.PreLatest,
) {
	pc.WithPreLatest(stagedPreLatest)
	changed, err := s.StorePreConfirmed(pc)
	if err != nil {
		s.log.Debugw("Error while trying to store pre_confirmed block", "err", err)
		return
	}

	if changed {
		s.pendingDataFeed.Send(pc)
	}
}

// runPreConfirmedPhase coordinates baselines and final storage.
func (s *Synchronizer) runPreConfirmedPhase(ctx context.Context, headsSub *feed.Subscription[*core.Block]) {
	preLatestCh := make(chan *core.PreLatest, 1)
	preConfirmedCh := make(chan *core.PreConfirmed, 1)
	var preConfirmedBlockNumberToPoll atomic.Uint64

	go s.pollPreLatest(ctx, preLatestCh)
	go s.pollPreConfirmed(ctx, &preConfirmedBlockNumberToPoll, preConfirmedCh)

	var stagedPreLatest *core.PreLatest

	for {
		select {
		case <-ctx.Done():
			return

		case head, ok := <-headsSub.Recv():
			if !ok {
				return
			}

			stagedPreLatest = s.handleHeadInPreConfirmedPhase(
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

// pollPendingData runs synchronisation in two phases, switching when protocol >= v0.14.0:
//  1. Pending phase: store empty pending baselines and real pending blocks.
//  2. Pre_confirmed phase: manage baselines and pre_latest attachments and poll pre_confirmed.
func (s *Synchronizer) pollPendingData(ctx context.Context) {
	if s.pendingPollInterval == 0 || s.preConfirmedPollInterval == 0 {
		s.log.Infow("Pending data polling is disabled")
		return
	}

	headsSub := s.newHeads.SubscribeKeepLast()
	defer headsSub.Unsubscribe()

	// Phase 1: pending path
	switched := s.runPendingPhase(ctx, headsSub)
	if !switched {
		return
	}

	// Phase 2: pre_confirmed path
	s.runPreConfirmedPhase(ctx, headsSub)
}
