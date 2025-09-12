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
)

// pollPreLatest fetches the pre-latest block for each new head and forwards it to the given channel.
// The consumer of preLatestChan never receives the same preLatest twice.
func (s *Synchronizer) pollPreLatest(ctx context.Context, out chan<- *core.PreLatest) {
	defer close(out)

	if s.pendingPollInterval == 0 {
		s.log.Infow("Pre-latest block polling is disabled")
		return
	}

	sub := s.newHeads.SubscribeKeepLast()
	defer sub.Unsubscribe()

	// Cache of pre-latest blocks keyed by the hash of their parent.
	// When we receive the head with this parent hash, we emit the cached pre-latest.
	seenByParent := make(map[felt.Felt]*core.PreLatest)

	ticker := time.NewTicker(s.pendingPollInterval)
	defer ticker.Stop()

	var (
		currentHead      *core.Block
		currentHeadHash  felt.Felt
		haveHead         bool
		deliveredForHead bool // whether we've already emitted a pre-latest for the current head
	)

	for {
		// We only poll when:
		//   - we have a head
		//   - we are at the tip (or caught up) relative to highest known header
		//   - we have not yet delivered a pre-latest for this head
		shouldPoll := haveHead && s.isAtTip(currentHead.Header.Number) && !deliveredForHead

		select {
		case <-ctx.Done():
			return

		case head, ok := <-sub.Recv():
			if !ok {
				// Subscription closed; nothing more to do.
				return
			}

			currentHead = head
			currentHeadHash = *head.Hash
			haveHead = true
			deliveredForHead = false

			// If we already cached a pre-latest for this new head (by its parent hash),
			// emit it immediately and mark as delivered.
			if pl, hit := seenByParent[currentHeadHash]; hit {
				delete(seenByParent, currentHeadHash)
				pl.Block.Number = currentHead.Number + 1

				select {
				case <-ctx.Done():
					return
				case out <- pl:
					deliveredForHead = true
				}
			}

		case <-ticker.C:
			if !shouldPoll {
				continue
			}

			pending, err := s.dataSource.BlockPending(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				s.log.Debugw("Error while trying to poll pre_latest block", "err", err)
				continue
			}

			preLatest := core.PreLatest(pending)

			// If the polled pre-latest doesn't belong to the current head, cache it by its parent.
			// The cached item will be emitted when the corresponding head arrives.
			if *preLatest.Block.ParentHash != currentHeadHash {
				seenByParent[*preLatest.Block.ParentHash] = &preLatest
				continue
			}

			// The pre-latest is the predecessor of the current head's next block.
			preLatest.Block.Number = currentHead.Number + 1

			select {
			case <-ctx.Done():
				return
			case out <- &preLatest:
				deliveredForHead = true
			}
		}
	}
}

func (s *Synchronizer) pollPreConfirmed(ctx context.Context, out chan<- *core.PreConfirmed) {
	if s.preConfirmedPollInterval <= 0 {
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
			shouldPoll := targetNum > 0 && s.isAtTip(targetNum)
			if !shouldPoll {
				continue
			}

			preConfirmed, err := s.dataSource.PreConfirmedBlockByNumber(ctx, targetNum)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
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

func (s *Synchronizer) pollPending(ctx context.Context, out chan<- *core.Pending) {
	if s.pendingPollInterval <= 0 {
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

			if !s.isAtTip(head.Number) {
				continue
			}

			pending, err := s.dataSource.BlockPending(ctx)
			if err != nil {
				s.log.Debugw("Error while trying to poll pending block", "err", err)
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

// needsPreConfirmed reports whether the given protocol version string
// indicates blocks >= v0.14.0 (i.e., pre_confirmed path is required).
func needsPreConfirmed(protocolVersion string) (bool, error) {
	ver, err := core.ParseBlockVersion(protocolVersion)
	if err != nil {
		return false, err
	}
	return ver.GreaterThanEqual(core.Ver0_14_0), nil
}

// isAtTip reports whether the given number is at or beyond the highest known header.
func (s *Synchronizer) isAtTip(blockNumber uint64) bool {
	highest := s.highestBlockHeader.Load()
	if highest == nil {
		return false
	}
	return highest.Number <= blockNumber
}

// pollPendingData runs the pending pollers.
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

		case p, ok := <-pendingCh:
			if !ok {
				pendingCh = nil
				continue
			}
			need, err := needsPreConfirmed(p.Block.ProtocolVersion)
			if err != nil {
				s.log.Debugw("Failed to parse pending protocol version", "err", err)
				continue
			}
			if need {
				return true // switch to pre_confirmed
			}
			if err := s.StorePending(p); err != nil {
				s.log.Debugw("Error storing pending block", "err", err)
			}
		}
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
		targetNum       uint64
		stagedPreLatest *core.PreLatest
	)

	for {
		select {
		case <-ctx.Done():
			return

		case head, ok := <-headsSub.Recv():
			if !ok {
				return
			}
			n := head.Number + 1

			// If head catches up to a target that was raised by pre_latest, invalidate attachment.
			if n == targetNum && stagedPreLatest != nil {
				stagedPreLatest = nil

				// Re-store empty baseline without pre_latest.
				if err := s.storeEmptyPreConfirmed(head.Header, nil); err != nil {
					s.log.Debugw("Error re-storing empty pre_confirmed (clear attachment)", "err", err)
				}
			}

			// If head advances the target, reset staged pre_latest, publish target, store baseline.
			if n > targetNum {
				targetNum = n
				stagedPreLatest = nil
				s.targetPreConfirmedNum.Store(targetNum)

				if err := s.storeEmptyPreConfirmed(head.Header, nil); err != nil {
					s.log.Debugw("Error storing empty pre_confirmed (from head)", "err", err)
				}
			}

		case pl, ok := <-preLatestCh:
			if !ok {
				preLatestCh = nil
				continue
			}
			n := pl.Block.Number + 1
			if n > targetNum {
				// Pre-latest raises target; attach and baseline.
				targetNum = n
				stagedPreLatest = pl
				s.targetPreConfirmedNum.Store(targetNum)

				if err := s.storeEmptyPreConfirmed(pl.Block.Header, pl); err != nil {
					s.log.Debugw("Error storing empty pre_confirmed (with pre_latest)", "err", err)
				}
				continue
			}
			if n == targetNum && stagedPreLatest == nil {
				// Target derived from head earlier; now attach pre_latest.
				stagedPreLatest = pl
				if err := s.storeEmptyPreConfirmed(pl.Block.Header, pl); err != nil {
					s.log.Debugw("Error storing empty pre_confirmed (attach pre_latest)", "err", err)
				}
			}

		case pc, ok := <-preConfirmedCh:
			if !ok {
				preConfirmedCh = nil
				continue
			}
			if pc.Block.Number != targetNum {
				continue
			}

			// Attach whatever is currently valid (could be nil if head superseded).
			pc.WithPreLatest(stagedPreLatest)

			changed, err := s.StorePreConfirmed(pc)
			if err != nil {
				s.log.Debugw("Error while trying to store pre_confirmed block", "err", err)
				continue
			}
			if changed {
				s.pendingDataFeed.Send(pc)
			}
		}
	}
}

// StorePending stores a pending block given that it is for the next height
func (s *Synchronizer) StorePending(p *core.Pending) error {
	err := blockchain.CheckBlockVersion(p.Block.ProtocolVersion)
	if err != nil {
		return err
	}

	expectedParentHash := new(felt.Felt)
	h, err := s.blockchain.HeadsHeader()
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return err
	} else if err == nil {
		expectedParentHash = h.Hash
	}

	if !expectedParentHash.Equal(p.Block.ParentHash) {
		return fmt.Errorf("store pending: %w", blockchain.ErrParentDoesNotMatchHead)
	}

	if existingPending, err := s.PendingData(); err == nil {
		if existingPending.GetBlock().TransactionCount >= p.Block.TransactionCount {
			// ignore the incoming pending if it has fewer transactions than the one we already have
			return nil
		}
	} else if !errors.Is(err, ErrPendingBlockNotFound) {
		return err
	}

	s.pendingData.Store(utils.HeapPtr[core.PendingData](p))

	s.pendingDataFeed.Send(p)

	return nil
}

// UpdatePreLatestAttachment updates (or clears) the PreLatest attachment of the currently stored
// pre_confirmed at the given blockNumber by atomically swapping the stored pointer.
// Returns true if the store was updated, false if no matching pre_confirmed is stored
// or the attachment was already equal.
func (s *Synchronizer) UpdatePreLatestAttachment(blockNumber uint64, preLatest *core.PreLatest) (bool, error) {
	for {
		curPtr := s.pendingData.Load()
		if curPtr == nil || *curPtr == nil {
			return false, nil
		}

		cur := *curPtr
		pc, ok := cur.(*core.PreConfirmed)
		if !ok {
			// A pending is stored or another type; nothing to do.
			return false, nil
		}
		if pc.Block == nil || pc.Block.Number != blockNumber {
			// Different height stored; do not touch.
			return false, nil
		}

		if pc.PreLatest == preLatest {
			// No change.
			return false, nil
		}

		// Clone and update attachment on the clone.
		next := pc.Clone()
		next.WithPreLatest(preLatest)

		if s.pendingData.CompareAndSwap(curPtr, utils.HeapPtr[core.PendingData](next)) {
			return true, nil
		}
	}
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
		_, _ = s.UpdatePreLatestAttachment(incomingB.Number, p.PreLatest)
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
// Pass preLatest if you want the baseline to carry an attachment; pass nil to clear.
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

func makeStateDiffForEmptyBlock(bc blockchain.Reader, blockNumber uint64) (*core.StateDiff, error) {
	stateDiff := &core.StateDiff{
		StorageDiffs:      make(map[felt.Felt]map[felt.Felt]*felt.Felt),
		Nonces:            make(map[felt.Felt]*felt.Felt),
		DeployedContracts: make(map[felt.Felt]*felt.Felt),
		DeclaredV0Classes: make([]*felt.Felt, 0),
		DeclaredV1Classes: make(map[felt.Felt]*felt.Felt),
		ReplacedClasses:   make(map[felt.Felt]*felt.Felt),
	}

	const blockHashLag = 10
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
