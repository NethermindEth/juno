package sync

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common/lru"
)

const (
	preLatestCacheSize = 10
	blockHashLag       = 10
)

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

	if *preLatest.Block.ParentHash != *currentHead.Hash {
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
// The target is read from s.targetPreConfirmedNum and only polled while at tip.
// On success, the pre_confirmed is forwarded to out.
func (s *Synchronizer) pollPreConfirmed(ctx context.Context, out chan<- *core.PreConfirmed) {
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
			targetNum := s.targetPreConfirmedNum.Load()
			shouldPoll := targetNum > 0 && s.isGreaterThanTip(targetNum)
			if !shouldPoll {
				continue
			}

			preConfirmed, err := s.dataSource.PreConfirmedBlockByNumber(ctx, targetNum)
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
			need, err := needsPreConfirmed(head.ProtocolVersion)
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
			need, err := needsPreConfirmed(p.Block.ProtocolVersion)
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

// handleHeadInPreConfirmed processes a new head during the pre_confirmed phase.
// It computes nextHeight = head.Number + 1, clears any staged pre_latest if the
// head catches up to the current target, and stores an empty pre_confirmed when
// advancing. Returns updated (targetNum, stagedPreLatest).
func (s *Synchronizer) handleHeadInPreConfirmedPhase(
	head *core.Block,
	targetPreConfirmedNum uint64,
	stagedPreLatest *core.PreLatest,
) (uint64, *core.PreLatest) {
	next := head.Number + 1
	if next < targetPreConfirmedNum {
		return targetPreConfirmedNum, stagedPreLatest
	}

	if next == targetPreConfirmedNum {
		s.UpdatePreLatestAttachment(targetPreConfirmedNum, nil)
		return targetPreConfirmedNum, nil
	}

	s.targetPreConfirmedNum.Store(next)
	if err := s.storeEmptyPreConfirmed(head.Header, nil); err != nil {
		s.log.Debugw("Error storing empty pre_confirmed (from head)", "err", err)
	}
	return next, nil
}

// handlePreLatest processes an incoming pre_latest during the pre_confirmed phase.
// If it raises the target, it stages the attachment and stores a baseline with it.
// Returns updated (targetNum, stagedPreLatest).
func (s *Synchronizer) handlePreLatest(
	pl *core.PreLatest,
	targetPreConfirmedNum uint64,
	stagedPreLatest *core.PreLatest,
) (uint64, *core.PreLatest) {
	next := pl.Block.Number + 1
	if next <= targetPreConfirmedNum {
		return targetPreConfirmedNum, stagedPreLatest
	}

	s.targetPreConfirmedNum.Store(next)
	if err := s.storeEmptyPreConfirmed(pl.Block.Header, pl); err != nil {
		s.log.Debugw("Error storing empty pre_confirmed (with pre_latest)", "err", err)
	}
	return next, pl
}

// handlePreConfirmed finalises when the polled pre_confirmed equals the target.
// It attaches the staged pre_latest (if still valid), stores it, and feeds if changed.
func (s *Synchronizer) handlePreConfirmed(
	pc *core.PreConfirmed,
	targetNum uint64,
	stagedPreLatest *core.PreLatest,
) {
	if pc.Block.Number != targetNum {
		return
	}

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

// runPreConfirmedPhase coordinates baselines, pre_latest attachments, and final storage.
// Key rule: if head for target-1 arrives (i.e., head.Number+1 == targetNum) after staging a pre_latest,
// we clear the staged attachment and also clear the stored baseline’s attachment (if present).
func (s *Synchronizer) runPreConfirmedPhase(ctx context.Context, headsSub *feed.Subscription[*core.Block]) {
	preLatestCh := make(chan *core.PreLatest, 1)
	preConfirmedCh := make(chan *core.PreConfirmed, 1)

	go s.pollPreLatest(ctx, preLatestCh)
	go s.pollPreConfirmed(ctx, preConfirmedCh)

	var (
		targetPreConfirmedNum uint64
		stagedPreLatest       *core.PreLatest
	)

	for {
		select {
		case <-ctx.Done():
			return

		case head, ok := <-headsSub.Recv():
			if !ok {
				return
			}

			targetPreConfirmedNum, stagedPreLatest = s.handleHeadInPreConfirmedPhase(
				head,
				targetPreConfirmedNum,
				stagedPreLatest,
			)
		case pl := <-preLatestCh:
			targetPreConfirmedNum, stagedPreLatest = s.handlePreLatest(
				pl,
				targetPreConfirmedNum,
				stagedPreLatest,
			)
		case pc := <-preConfirmedCh:
			s.handlePreConfirmed(pc, targetPreConfirmedNum, stagedPreLatest)
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
	if !switched || ctx.Err() != nil {
		return
	}

	// Phase 2: pre_confirmed path
	s.runPreConfirmedPhase(ctx, headsSub)
}

// StorePending stores a pending block given that it is for the next height
func (s *Synchronizer) StorePending(p *core.Pending) (bool, error) {
	err := blockchain.CheckBlockVersion(p.Block.ProtocolVersion)
	if err != nil {
		return false, err
	}

	expectedParentHash := new(felt.Felt)
	h, err := s.blockchain.HeadsHeader()
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return false, err
	} else if err == nil {
		expectedParentHash = h.Hash
	}

	if !expectedParentHash.Equal(p.Block.ParentHash) {
		return false, fmt.Errorf("store pending: %w", blockchain.ErrParentDoesNotMatchHead)
	}

	if existingPending, err := s.PendingData(); err == nil {
		if existingPending.GetBlock().TransactionCount >= p.Block.TransactionCount {
			// ignore the incoming pending if it has fewer transactions than the one we already have
			return false, nil
		}
	} else if !errors.Is(err, ErrPendingBlockNotFound) {
		return false, err
	}

	s.pendingData.Store(utils.HeapPtr[core.PendingData](p))

	return true, nil
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
		// A pending is stored or another type; nothing to do.
		return false
	}
	if pc.Block == nil || pc.Block.Number != blockNumber {
		// Different height stored; do not touch.
		return false
	}

	if pc.PreLatest == preLatest {
		// No change.
		return false
	}

	// Clone and update attachment on the clone.
	next := pc.Copy()
	next.WithPreLatest(preLatest)

	return s.pendingData.CompareAndSwap(curPtr, utils.HeapPtr[core.PendingData](next))
}

// StorePreConfirmed stores a pre_confirmed block given that it is for the next height.
// If an equal-number block with >= txCount already exists, we do not overwrite it,
// but we allow updating the PreLatest attachment in-place via a CAS swap.
func (s *Synchronizer) StorePreConfirmed(p *core.PreConfirmed) (bool, error) {
	if err := blockchain.CheckBlockVersion(p.GetBlock().ProtocolVersion); err != nil {
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
	if existingPtr == nil || *existingPtr == nil {
		return s.pendingData.CompareAndSwap(
			existingPtr,
			utils.HeapPtr[core.PendingData](p),
		), nil
	}

	existing := *existingPtr
	if !existing.Validate(head) {
		return s.pendingData.CompareAndSwap(
			existingPtr,
			utils.HeapPtr[core.PendingData](p),
		), nil
	}

	existingB := existing.GetBlock()
	incomingB := p.GetBlock()

	// If same number and existing has >= txCount, keep existing but update attachment if needed.
	if existingB.Number == incomingB.Number &&
		existingB.TransactionCount >= incomingB.TransactionCount {
		// Try to update attachment to match p.PreLatest (can be nil to clear)
		_ = s.UpdatePreLatestAttachment(incomingB.Number, p.PreLatest)
		return false, nil
	}

	// Otherwise, swap in the better pre_confirmed.
	return s.pendingData.CompareAndSwap(
		existingPtr,
		utils.HeapPtr[core.PendingData](p),
	), nil
}

func (s *Synchronizer) storeEmptyPendingData(lastHeader *core.Header) {
	blockVer, err := core.ParseBlockVersion(lastHeader.ProtocolVersion)
	if err != nil {
		s.log.Errorw("Failed to parse block version", "err", err)
		return
	}

	if blockVer.GreaterThanEqual(core.Ver0_14_0) {
		if err := s.storeEmptyPreConfirmed(lastHeader, nil); err != nil {
			s.log.Errorw("Failed to store empty pre_confirmed block", "number", lastHeader.Number)
		}
	} else {
		if err := s.storeEmptyPending(lastHeader); err != nil {
			s.log.Errorw("Failed to store empty pending block", "number", lastHeader.Number)
		}
	}
}

func (s *Synchronizer) storeEmptyPending(latestHeader *core.Header) error {
	receipts := make([]*core.TransactionReceipt, 0)
	pendingBlock := &core.Block{
		Header: &core.Header{
			ParentHash:       latestHeader.Hash,
			SequencerAddress: latestHeader.SequencerAddress,
			Number:           latestHeader.Number + 1,
			Timestamp:        uint64(time.Now().Unix()),
			ProtocolVersion:  latestHeader.ProtocolVersion,
			EventsBloom:      core.EventsBloom(receipts),
			L1GasPriceETH:    latestHeader.L1GasPriceETH,
			L1GasPriceSTRK:   latestHeader.L1GasPriceSTRK,
			L2GasPrice:       latestHeader.L2GasPrice,
			L1DataGasPrice:   latestHeader.L1DataGasPrice,
			L1DAMode:         latestHeader.L1DAMode,
		},
		Transactions: make([]core.Transaction, 0),
		Receipts:     receipts,
	}

	stateDiff, err := makeStateDiffForEmptyBlock(s.blockchain, latestHeader.Number+1)
	if err != nil {
		return err
	}

	pending := core.Pending{
		Block: pendingBlock,
		StateUpdate: &core.StateUpdate{
			OldRoot:   latestHeader.GlobalStateRoot,
			StateDiff: stateDiff,
		},
		NewClasses: make(map[felt.Felt]core.Class, 0),
	}

	s.pendingData.Store(utils.HeapPtr[core.PendingData](&pending))
	return nil
}

// storeEmptyPreConfirmed creates a baseline pre_confirmed for head+1 and stores it.
// Pass preLatest to attach it to the baseline; pass nil to clear any attachment.
func (s *Synchronizer) storeEmptyPreConfirmed(latestHeader *core.Header, preLatest *core.PreLatest) error {
	preConfirmed, err := s.makeEmptyPreConfirmedForParent(latestHeader)
	if err != nil {
		return err
	}
	preConfirmed.WithPreLatest(preLatest)
	_, err = s.StorePreConfirmed(&preConfirmed)
	return err
}

func (s *Synchronizer) makeEmptyPreConfirmedForParent(latestHeader *core.Header) (core.PreConfirmed, error) {
	receipts := make([]*core.TransactionReceipt, 0)
	preConfirmedBlock := &core.Block{
		// pre_confirmed block does not have parent hash
		Header: &core.Header{
			SequencerAddress: latestHeader.SequencerAddress,
			Number:           latestHeader.Number + 1,
			Timestamp:        uint64(time.Now().Unix()),
			ProtocolVersion:  latestHeader.ProtocolVersion,
			EventsBloom:      core.EventsBloom(receipts),
			L1GasPriceETH:    latestHeader.L1GasPriceETH,
			L1GasPriceSTRK:   latestHeader.L1GasPriceSTRK,
			L2GasPrice:       latestHeader.L2GasPrice,
			L1DataGasPrice:   latestHeader.L1DataGasPrice,
			L1DAMode:         latestHeader.L1DAMode,
		},
		Transactions: make([]core.Transaction, 0),
		Receipts:     receipts,
	}

	stateDiff, err := makeStateDiffForEmptyBlock(s.blockchain, latestHeader.Number+1)
	if err != nil {
		return core.PreConfirmed{}, err
	}

	preConfirmed := core.PreConfirmed{
		Block: preConfirmedBlock,
		StateUpdate: &core.StateUpdate{
			StateDiff: stateDiff,
		},
		NewClasses:            make(map[felt.Felt]core.Class, 0),
		TransactionStateDiffs: make([]*core.StateDiff, 0),
		CandidateTxs:          make([]core.Transaction, 0),
	}

	return preConfirmed, nil
}

// isAtTip reports whether the given number is at or beyond the highest known header.
func (s *Synchronizer) isGreaterThanTip(blockNumber uint64) bool {
	highest := s.highestBlockHeader.Load()
	if highest == nil {
		return false
	}
	return highest.Number < blockNumber
}

// needsPreConfirmed reports whether the given protocol version string
// indicates blocks >= v0.14.0 (i.e., pre_confirmed path is required).
func needsPreConfirmed(protocolVersion string) (bool, error) {
	ver, err := core.ParseBlockVersion(protocolVersion)
	if err != nil {
		return false, err
	}
	return ver.GreaterThanEqual(core.Ver0_14_0), nil
}

// makeStateDiffForEmptyBlock constructs a minimal state diff for an empty block.
// It optionally writes a historical block hash mapping when blockNumber >= blockHashLag.
func makeStateDiffForEmptyBlock(bc blockchain.Reader, blockNumber uint64) (*core.StateDiff, error) {
	stateDiff := &core.StateDiff{
		StorageDiffs:      make(map[felt.Felt]map[felt.Felt]*felt.Felt),
		Nonces:            make(map[felt.Felt]*felt.Felt),
		DeployedContracts: make(map[felt.Felt]*felt.Felt),
		DeclaredV0Classes: make([]*felt.Felt, 0),
		DeclaredV1Classes: make(map[felt.Felt]*felt.Felt),
		ReplacedClasses:   make(map[felt.Felt]*felt.Felt),
	}

	if blockNumber < blockHashLag {
		return stateDiff, nil
	}

	header, err := bc.BlockHeaderByNumber(blockNumber - blockHashLag)
	if err != nil {
		return nil, err
	}

	blockHashStorageContract := new(felt.Felt).SetUint64(1)
	stateDiff.StorageDiffs[*blockHashStorageContract] = map[felt.Felt]*felt.Felt{
		*new(felt.Felt).SetUint64(header.Number): header.Hash,
	}
	return stateDiff, nil
}
