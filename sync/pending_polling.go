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
	"github.com/NethermindEth/juno/utils"
)

const (
	blockHashLag = 10
)

// needsPreConfirmed reports whether the given protocol version string
// indicates blocks >= v0.14.0 (i.e., pre_confirmed path is required).
func needsPreConfirmed(protocolVersion string) (bool, error) {
	ver, err := core.ParseBlockVersion(protocolVersion)
	if err != nil {
		return false, err
	}
	return ver.GreaterThanEqual(core.Ver0_14_0), nil
}

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

	if existingPending.GetBlock().TransactionCount < incomingPending.GetBlock().TransactionCount {
		return false
	}

	return true
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

// StorePreConfirmed stores a pre_confirmed block given that it is for the next height.
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
		return false, nil
	}

	return s.pendingData.CompareAndSwap(
		existingPtr,
		utils.HeapPtr[core.PendingData](p),
	), nil
}

// makeStateDiffForEmptyBlock constructs a minimal state diff for an empty block.
// It optionally writes a historical block hash mapping when blockNumber >= blockHashLag.
func makeStateDiffForEmptyBlock(bc blockchain.Reader, blockNumber uint64) (*core.StateDiff, error) {
	stateDiff := &core.StateDiff{
		StorageDiffs:      make(map[felt.Felt]map[felt.Felt]*felt.Felt, 1),
		Nonces:            make(map[felt.Felt]*felt.Felt, 0),
		DeployedContracts: make(map[felt.Felt]*felt.Felt, 0),
		DeclaredV0Classes: make([]*felt.Felt, 0),
		DeclaredV1Classes: make(map[felt.Felt]*felt.Felt, 0),
		ReplacedClasses:   make(map[felt.Felt]*felt.Felt, 0),
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

func (s *Synchronizer) makeEmptyPendingForParent(latestHeader *core.Header) (core.Pending, error) {
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
		return core.Pending{}, err
	}

	pending := core.Pending{
		Block: pendingBlock,
		StateUpdate: &core.StateUpdate{
			OldRoot:   latestHeader.GlobalStateRoot,
			StateDiff: stateDiff,
		},
		NewClasses: make(map[felt.Felt]core.Class, 0),
	}
	return pending, nil
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

func (s *Synchronizer) storeEmptyPending(latestHeader *core.Header) error {
	pending, err := s.makeEmptyPendingForParent(latestHeader)
	if err != nil {
		return err
	}

	_, err = s.StorePending(&pending)
	return err
}

func (s *Synchronizer) storeEmptyPreConfirmed(latestHeader *core.Header) error {
	preConfirmed, err := s.makeEmptyPreConfirmedForParent(latestHeader)
	if err != nil {
		return err
	}
	_, err = s.StorePreConfirmed(&preConfirmed)
	return err
}

func (s *Synchronizer) storeEmptyPendingData(lastHeader *core.Header) {
	blockVer, err := core.ParseBlockVersion(lastHeader.ProtocolVersion)
	if err != nil {
		s.log.Errorw("Failed to parse block version", "err", err)
		return
	}

	if blockVer.GreaterThanEqual(core.Ver0_14_0) {
		if err := s.storeEmptyPreConfirmed(lastHeader); err != nil {
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
// It computes nextHeight = head.Number + 1 and stores an empty pre_confirmed when
// advancing.
func (s *Synchronizer) handleHeadInPreConfirmedPhase(
	head *core.Block,
	targetPreConfirmedNum *atomic.Uint64,
) {
	next := head.Number + 1
	if next <= targetPreConfirmedNum.Load() {
		return
	}

	targetPreConfirmedNum.Store(next)
	if err := s.storeEmptyPreConfirmed(head.Header); err != nil {
		s.log.Debugw("Error storing empty pre_confirmed (from head)", "err", err)
	}
}

// handlePreConfirmed finalises when the polled pre_confirmed equals the target.
// It stores new pre_confirmed and broadcasts if changed.
func (s *Synchronizer) handlePreConfirmed(pc *core.PreConfirmed) {
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
	preConfirmedCh := make(chan *core.PreConfirmed, 1)
	var preConfirmedBlockNumberToPoll atomic.Uint64

	go s.pollPreConfirmed(ctx, &preConfirmedBlockNumberToPoll, preConfirmedCh)

	for {
		select {
		case <-ctx.Done():
			return

		case head, ok := <-headsSub.Recv():
			if !ok {
				return
			}

			s.handleHeadInPreConfirmedPhase(
				head,
				&preConfirmedBlockNumberToPoll,
			)

		case pc := <-preConfirmedCh:
			s.handlePreConfirmed(pc)
		}
	}
}

// pollPendingData runs synchronisation in two phases, switching when protocol >= v0.14.0:
//  1. Pending phase: store empty pending baselines and real pending blocks.
//  2. Pre_confirmed phase: manage baselines and poll pre_confirmed.
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
