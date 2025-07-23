package sync

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/feed"
	junoplugin "github.com/NethermindEth/juno/plugin"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/utils"
	"github.com/sourcegraph/conc/stream"
)

var (
	_ service.Service = (*Synchronizer)(nil)
	_ Reader          = (*Synchronizer)(nil)

	ErrPendingBlockNotFound          = errors.New("pending block not found")
	ErrMustSwitchPollingPreConfirmed = errors.New(
		"reached starknet 0.14.0. node requires switching from pending to polling pre_confirmed blocks",
	)
)

const (
	OpVerify = "verify"
	OpStore  = "store"
	OpFetch  = "fetch"
)

// This is a work-around. mockgen chokes when the instantiated generic type is in the interface.
type NewHeadSubscription struct {
	*feed.Subscription[*core.Block]
}

type ReorgSubscription struct {
	*feed.Subscription[*ReorgBlockRange]
}

type PendingTxSubscription struct {
	*feed.Subscription[[]core.Transaction]
}

type PendingDataSubscription struct {
	*feed.Subscription[core.PendingData]
}

// ReorgBlockRange represents data about reorganised blocks, starting and ending block number and hash
type ReorgBlockRange struct {
	// StartBlockHash is the hash of the first known block of the orphaned chain
	StartBlockHash *felt.Felt
	// StartBlockNum is the number of the first known block of the orphaned chain
	StartBlockNum uint64
	// The last known block of the orphaned chain
	EndBlockHash *felt.Felt
	// Number of the last known block of the orphaned chain
	EndBlockNum uint64
}

// Todo: Since this is also going to be implemented by p2p package we should move this interface to node package
//
//go:generate mockgen -destination=../mocks/mock_synchronizer.go -package=mocks -mock_names Reader=MockSyncReader github.com/NethermindEth/juno/sync Reader
type Reader interface {
	StartingBlockNumber() (uint64, error)
	HighestBlockHeader() *core.Header
	SubscribeNewHeads() NewHeadSubscription
	SubscribeReorg() ReorgSubscription
	SubscribePendingData() PendingDataSubscription

	PendingData() (core.PendingData, error)
	PendingBlock() *core.Block
	PendingState() (core.StateReader, error)
	PendingStateBeforeIndex(index int) (core.StateReader, error)
}

// This is temporary and will be removed once the p2p synchronizer implements this interface.
type NoopSynchronizer struct{}

func (n *NoopSynchronizer) StartingBlockNumber() (uint64, error) {
	return 0, errors.New("StartingBlockNumber() not implemented")
}

func (n *NoopSynchronizer) HighestBlockHeader() *core.Header {
	return nil
}

func (n *NoopSynchronizer) SubscribeNewHeads() NewHeadSubscription {
	return NewHeadSubscription{feed.New[*core.Block]().Subscribe()}
}

func (n *NoopSynchronizer) SubscribeReorg() ReorgSubscription {
	return ReorgSubscription{feed.New[*ReorgBlockRange]().Subscribe()}
}

func (n *NoopSynchronizer) SubscribePendingData() PendingDataSubscription {
	return PendingDataSubscription{feed.New[core.PendingData]().Subscribe()}
}

func (n *NoopSynchronizer) PendingBlock() *core.Block {
	return nil
}

func (n *NoopSynchronizer) PendingData() (core.PendingData, error) {
	return nil, errors.New("PendingData() is not implemented")
}

func (n *NoopSynchronizer) PendingState() (state.StateReader, error) {
	return nil, errors.New("PendingState() not implemented")
}

func (n *NoopSynchronizer) PendingStateBeforeIndex(index int) (core.StateReader, func() error, error) {
	return nil, nil, errors.New("PendingStateBeforeIndex() not implemented")
}

// Synchronizer manages a list of StarknetData to fetch the latest blockchain updates
type Synchronizer struct {
	blockchain          *blockchain.Blockchain
	db                  db.KeyValueStore
	readOnlyBlockchain  bool
	dataSource          DataSource
	startingBlockNumber *uint64
	highestBlockHeader  atomic.Pointer[core.Header]
	newHeads            *feed.Feed[*core.Block]
	reorgFeed           *feed.Feed[*ReorgBlockRange]
	pendingDataFeed     *feed.Feed[core.PendingData]

	log      utils.SimpleLogger
	listener EventListener

	pendingData              atomic.Pointer[core.PendingData]
	pendingPollInterval      time.Duration
	preConfirmedPollInterval time.Duration

	catchUpMode bool
	plugin      junoplugin.JunoPlugin

	currReorg *ReorgBlockRange // If nil, no reorg is happening
}

func New(
	bc *blockchain.Blockchain,
	dataSource DataSource,
	log utils.SimpleLogger,
	pendingPollInterval, preConfirmedPollInterval time.Duration,
	readOnlyBlockchain bool,
	database db.KeyValueStore,
) *Synchronizer {
	s := &Synchronizer{
		blockchain:               bc,
		dataSource:               dataSource,
		db:                       database,
		log:                      log,
		newHeads:                 feed.New[*core.Block](),
		reorgFeed:                feed.New[*ReorgBlockRange](),
		pendingDataFeed:          feed.New[core.PendingData](),
		pendingPollInterval:      pendingPollInterval,
		preConfirmedPollInterval: preConfirmedPollInterval,
		listener:                 &SelectiveListener{},
		readOnlyBlockchain:       readOnlyBlockchain,
	}
	return s
}

// WithPlugin registers an plugin
func (s *Synchronizer) WithPlugin(plugin junoplugin.JunoPlugin) *Synchronizer {
	s.plugin = plugin
	return s
}

// WithListener registers an EventListener
func (s *Synchronizer) WithListener(listener EventListener) *Synchronizer {
	s.listener = listener
	return s
}

// Run starts the Synchronizer, returns an error if the loop is already running
func (s *Synchronizer) Run(ctx context.Context) error {
	s.syncBlocks(ctx)
	return nil
}

func (s *Synchronizer) fetcherTask(ctx context.Context, height uint64, verifiers *stream.Stream,
	resetStreams context.CancelFunc,
) stream.Callback {
	for {
		select {
		case <-ctx.Done():
			return func() {}
		default:
			committedBlock, err := s.dataSource.BlockByNumber(ctx, height)
			if err != nil {
				continue
			}

			return func() {
				verifiers.Go(func() stream.Callback {
					return s.verifierTask(ctx, committedBlock.Block, committedBlock.StateUpdate, committedBlock.NewClasses, resetStreams)
				})
			}
		}
	}
}

func (s *Synchronizer) handlePluginRevertBlock() {
	fromBlock, err := s.blockchain.Head()
	if err != nil {
		s.log.Warnw("Failed to retrieve the reverted blockchain head block for the plugin", "err", err)
		return
	}
	fromSU, err := s.blockchain.StateUpdateByNumber(fromBlock.Number)
	if err != nil {
		s.log.Warnw("Failed to retrieve the reverted blockchain head state-update for the plugin", "err", err)
		return
	}
	reverseStateDiff, err := s.blockchain.GetReverseStateDiff()
	if err != nil {
		s.log.Warnw("Failed to retrieve reverse state diff", "head", fromBlock.Number, "hash", fromBlock.Hash.ShortString(), "err", err)
		return
	}

	var toBlockAndStateUpdate *junoplugin.BlockAndStateUpdate
	if fromBlock.Number != 0 {
		toBlock, err := s.blockchain.BlockByHash(fromBlock.ParentHash)
		if err != nil {
			s.log.Warnw("Failed to retrieve the parent block for the plugin", "err", err)
			return
		}
		toSU, err := s.blockchain.StateUpdateByNumber(toBlock.Number)
		if err != nil {
			s.log.Warnw("Failed to retrieve the parents state-update for the plugin", "err", err)
			return
		}
		toBlockAndStateUpdate = &junoplugin.BlockAndStateUpdate{
			Block:       toBlock,
			StateUpdate: toSU,
		}
	}
	err = s.plugin.RevertBlock(
		&junoplugin.BlockAndStateUpdate{Block: fromBlock, StateUpdate: fromSU},
		toBlockAndStateUpdate,
		&reverseStateDiff)
	if err != nil {
		s.log.Errorw("Plugin RevertBlock failure:", "err", err)
	}
}

//nolint:gocyclo
func (s *Synchronizer) verifierTask(ctx context.Context, block *core.Block, stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.Class, resetStreams context.CancelFunc,
) stream.Callback {
	verifyTimer := time.Now()
	commitments, err := s.blockchain.SanityCheckNewHeight(block, stateUpdate, newClasses)
	if err == nil {
		s.listener.OnSyncStepDone(OpVerify, block.Number, time.Since(verifyTimer))
	}
	return func() {
		select {
		case <-ctx.Done():
			return
		default:
			if err != nil {
				s.log.Warnw("Sanity checks failed", "number", block.Number, "hash", block.Hash.ShortString(), "err", err)
				resetStreams()
				return
			}
			storeTimer := time.Now()
			err = s.blockchain.Store(block, commitments, stateUpdate, newClasses)
			if err != nil {
				if errors.Is(err, blockchain.ErrParentDoesNotMatchHead) {
					// revert the head and restart the sync process, hoping that the reorg is not deep
					// if the reorg is deeper, we will end up here again and again until we fully revert reorged
					// blocks
					if s.plugin != nil {
						s.handlePluginRevertBlock()
					}
					s.revertHead(block)

					// The previous head has been reverted, hence, get the current head and store empty pending block
					head, err := s.blockchain.HeadsHeader()
					if err != nil {
						s.log.Errorw("Failed to retrieve the head header", "err", err)
					}

					if head != nil {
						s.storeEmptyPendingData(head)
					}
				} else {
					s.log.Warnw("Failed storing Block", "number", block.Number,
						"hash", block.Hash.ShortString(), "err", err)
				}
				resetStreams()
				return
			}

			s.storeEmptyPendingData(block.Header)
			s.listener.OnSyncStepDone(OpStore, block.Number, time.Since(storeTimer))

			highestBlockHeader := s.highestBlockHeader.Load()
			if highestBlockHeader != nil {
				isBehind := highestBlockHeader.Number > block.Number+uint64(maxWorkers())
				if s.catchUpMode != isBehind {
					resetStreams()
				}
				s.catchUpMode = isBehind
			}

			if highestBlockHeader == nil || highestBlockHeader.Number < block.Number {
				s.highestBlockHeader.CompareAndSwap(highestBlockHeader, block.Header)
			}

			if s.currReorg != nil {
				s.reorgFeed.Send(s.currReorg)
				s.currReorg = nil // reset the reorg data
			}

			s.newHeads.Send(block)
			s.log.Infow("Stored Block", "number", block.Number, "hash",
				block.Hash.ShortString(), "root", block.GlobalStateRoot.ShortString())
			if s.plugin != nil {
				err := s.plugin.NewBlock(block, stateUpdate, newClasses)
				if err != nil {
					s.log.Errorw("Plugin NewBlock failure:", err)
				}
			}
		}
	}
}

func (s *Synchronizer) nextHeight() uint64 {
	nextHeight := uint64(0)
	if h, err := s.blockchain.Height(); err == nil {
		nextHeight = h + 1
	}
	return nextHeight
}

func (s *Synchronizer) syncBlocks(syncCtx context.Context) {
	defer func() {
		s.startingBlockNumber = nil
		s.highestBlockHeader.Store(nil)
	}()

	nextHeight := s.nextHeight()
	startingHeight := nextHeight
	s.startingBlockNumber = &startingHeight

	latestSem := make(chan struct{}, 1)
	if s.readOnlyBlockchain {
		s.pollLatest(syncCtx, latestSem)
		return
	}

	fetchers, verifiers := s.setupWorkers()
	streamCtx, streamCancel := context.WithCancel(syncCtx)

	go s.pollLatest(syncCtx, latestSem)
	pendingSem := make(chan struct{}, 1)
	go s.pollPendingData(syncCtx, pendingSem)

	for {
		select {
		case <-streamCtx.Done():
			streamCancel()
			fetchers.Wait()
			verifiers.Wait()

			select {
			case <-syncCtx.Done():
				pendingSem <- struct{}{}
				latestSem <- struct{}{}
				return
			default:
				streamCtx, streamCancel = context.WithCancel(syncCtx)
				nextHeight = s.nextHeight()
				fetchers, verifiers = s.setupWorkers()
				s.log.Warnw("Restarting sync process", "height", nextHeight, "catchUpMode", s.catchUpMode)
			}
		default:
			curHeight, curStreamCtx, curCancel := nextHeight, streamCtx, streamCancel
			fetchers.Go(func() stream.Callback {
				fetchTimer := time.Now()
				cb := s.fetcherTask(curStreamCtx, curHeight, verifiers, curCancel)
				s.listener.OnSyncStepDone(OpFetch, curHeight, time.Since(fetchTimer))
				return cb
			})
			nextHeight++
		}
	}
}

func maxWorkers() int {
	return min(16, runtime.GOMAXPROCS(0)) //nolint:mnd
}

func (s *Synchronizer) setupWorkers() (*stream.Stream, *stream.Stream) {
	numWorkers := 1
	if s.catchUpMode {
		numWorkers = maxWorkers()
	}
	return stream.New().WithMaxGoroutines(numWorkers), stream.New().WithMaxGoroutines(runtime.GOMAXPROCS(0))
}

func (s *Synchronizer) revertHead(forkBlock *core.Block) {
	var localHead *felt.Felt
	head, err := s.blockchain.HeadsHeader()
	if err == nil {
		localHead = head.Hash
	}
	s.log.Infow("Reorg detected", "localHead", localHead, "forkHead", forkBlock.Hash)
	if err := s.blockchain.RevertHead(); err != nil {
		s.log.Warnw("Failed reverting HEAD", "reverted", localHead, "err", err)
	} else {
		s.log.Infow("Reverted HEAD", "reverted", localHead)
	}

	if s.currReorg == nil { // first block of the reorg
		s.currReorg = &ReorgBlockRange{
			StartBlockHash: localHead,
			StartBlockNum:  head.Number,
			EndBlockHash:   localHead,
			EndBlockNum:    head.Number,
		}
	} else { // not the first block of the reorg, adjust the starting block
		s.currReorg.StartBlockHash = localHead
		s.currReorg.StartBlockNum = head.Number
	}

	s.listener.OnReorg(head.Number)
}

func (s *Synchronizer) pollPendingData(ctx context.Context, sem chan struct{}) {
	if s.pendingPollInterval == time.Duration(0) ||
		s.preConfirmedPollInterval == time.Duration(0) {
		s.log.Infow("Pending data polling is disabled")
		return
	}
	s.pollPending(ctx, sem)
	s.pollPreConfirmed(ctx, sem)
}

func (s *Synchronizer) pollPending(ctx context.Context, sem chan struct{}) {
	if s.pendingPollInterval == time.Duration(0) {
		s.log.Infow("Pending block polling is disabled")
		return
	}

	pendingPollTicker := time.NewTicker(s.pendingPollInterval)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			pendingPollTicker.Stop()
			return
		case <-pendingPollTicker.C:
			select {
			case sem <- struct{}{}:
				go func() {
					defer func() {
						<-sem
					}()
					err := s.fetchAndStorePending(ctx)
					if err != nil {
						if errors.Is(err, ErrMustSwitchPollingPreConfirmed) {
							s.log.Infow(
								"Detected block version 0.14.0; switching to polling mode for pre_confirmed blocks",
							)
							cancel()
							return
						}
						s.log.Debugw("Error while trying to poll pending block", "err", err)
					}
				}()
			default:
			}
		}
	}
}

func (s *Synchronizer) pollPreConfirmed(ctx context.Context, sem chan struct{}) {
	if s.preConfirmedPollInterval == time.Duration(0) {
		s.log.Infow("Pre-confirmed block polling is disabled")
		return
	}

	preConfirmedPollTicker := time.NewTicker(s.preConfirmedPollInterval)
	for {
		select {
		case <-ctx.Done():
			preConfirmedPollTicker.Stop()
			return
		case <-preConfirmedPollTicker.C:
			select {
			case sem <- struct{}{}:
				go func() {
					defer func() {
						<-sem
					}()

					err := s.fetchAndStorePreConfirmed(ctx)
					if err != nil {
						s.log.Debugw("Error while trying to poll pre_confirmed block", "err", err)
					}
				}()
			default:
			}
		}
	}
}

func (s *Synchronizer) pollLatest(ctx context.Context, sem chan struct{}) {
	poll := func() {
		select {
		case sem <- struct{}{}:
			go func() {
				defer func() {
					<-sem
				}()
				highestBlock, err := s.dataSource.BlockLatest(ctx)
				if err != nil {
					s.log.Warnw("Failed fetching latest block", "err", err)
				} else {
					s.highestBlockHeader.Store(highestBlock.Header)
				}
			}()
		default:
		}
	}

	ticker := time.NewTicker(time.Minute)
	poll()

	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
			poll()
		}
	}
}

func (s *Synchronizer) fetchAndStorePending(ctx context.Context) error {
	highestBlockHeader := s.highestBlockHeader.Load()
	if highestBlockHeader == nil {
		return nil
	}

	head, err := s.blockchain.HeadsHeader()
	if err != nil {
		return err
	}

	blockVer, err := core.ParseBlockVersion(head.ProtocolVersion)
	if err != nil {
		return err
	}

	if blockVer.GreaterThanEqual(core.Ver0_14_0) {
		return ErrMustSwitchPollingPreConfirmed
	}

	// not at the tip of the chain yet, no need to poll pending
	if highestBlockHeader.Number > head.Number {
		return nil
	}

	pending, err := s.dataSource.BlockPending(ctx)
	if err != nil {
		return err
	}

	blockVer, err = core.ParseBlockVersion(pending.Block.ProtocolVersion)
	if err != nil {
		return err
	}

	if blockVer.GreaterThanEqual(core.Ver0_14_0) {
		return ErrMustSwitchPollingPreConfirmed
	}

	return s.StorePending(&pending)
}

func (s *Synchronizer) fetchAndStorePreConfirmed(ctx context.Context) error {
	highestBlockHeader := s.highestBlockHeader.Load()
	if highestBlockHeader == nil {
		return nil
	}

	head, err := s.blockchain.HeadsHeader()
	if err != nil {
		return err
	}

	// not at the tip of the chain yet, no need to poll preconfirmed
	if highestBlockHeader.Number > head.Number {
		return nil
	}

	preConfirmedBlock, err := s.dataSource.PreConfirmedBlockByNumber(ctx, highestBlockHeader.Number+1)
	if err != nil {
		return err
	}

	return s.StorePreConfirmed(&preConfirmedBlock)
}

func (s *Synchronizer) StartingBlockNumber() (uint64, error) {
	if s.startingBlockNumber == nil {
		return 0, errors.New("not running")
	}
	return *s.startingBlockNumber, nil
}

func (s *Synchronizer) HighestBlockHeader() *core.Header {
	return s.highestBlockHeader.Load()
}

func (s *Synchronizer) SubscribeNewHeads() NewHeadSubscription {
	return NewHeadSubscription{s.newHeads.Subscribe()}
}

func (s *Synchronizer) SubscribeReorg() ReorgSubscription {
	return ReorgSubscription{s.reorgFeed.Subscribe()}
}

func (s *Synchronizer) SubscribePendingData() PendingDataSubscription {
	return PendingDataSubscription{s.pendingDataFeed.Subscribe()}
}

// StorePending stores a pending block given that it is for the next height
func (s *Synchronizer) StorePending(p *Pending) error {
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

// StorePreConfirmed stores a pre_confirmed block given that it is for the next height
func (s *Synchronizer) StorePreConfirmed(p *core.PreConfirmed) error {
	err := blockchain.CheckBlockVersion(p.Block.ProtocolVersion)
	if err != nil {
		return err
	}

	if existingPending, err := s.PendingData(); err == nil {
		existingPendingB := existingPending.GetBlock()
		if existingPendingB.Number == p.Block.Number &&
			existingPendingB.TransactionCount >= p.Block.TransactionCount {
			// ignore the incoming prec_confirmed if it has fewer transactions than the one we already have
			return nil
		}
	} else if !errors.Is(err, ErrPendingBlockNotFound) {
		return err
	}

	s.pendingData.Store(utils.HeapPtr[core.PendingData](p))

	s.pendingDataFeed.Send(p)

	return nil
}

func (s *Synchronizer) PendingData() (core.PendingData, error) {
	ptr := s.pendingData.Load()
	if ptr == nil || *ptr == nil {
		return nil, ErrPendingBlockNotFound
	}

	p := *ptr
	switch p.Variant() {
	case core.PreConfirmedBlockVariant:
		expectedOldRoot := &felt.Zero
		expectedBlockNumber := uint64(0)
		if head, err := s.blockchain.HeadsHeader(); err == nil {
			expectedOldRoot = head.GlobalStateRoot
			expectedBlockNumber = head.Number + 1
		}

		if p.GetStateUpdate().OldRoot.Equal(expectedOldRoot) &&
			p.GetBlock().Number == expectedBlockNumber {
			return p, nil
		}

	case core.PendingBlockVariant:
		expectedParentHash := &felt.Zero
		if head, err := s.blockchain.HeadsHeader(); err == nil {
			expectedParentHash = head.Hash
		}

		if p.GetBlock().ParentHash.Equal(expectedParentHash) {
			return p, nil
		}
	}
	return nil, ErrPendingBlockNotFound
}

func (s *Synchronizer) PendingBlock() *core.Block {
	pendingData, err := s.PendingData()
	if err != nil {
		return nil
	}
	return pendingData.GetBlock()
}

// PendingState returns the state resulting from execution of the pending block
func (s *Synchronizer) PendingState() (state.StateReader, error) {
	pending, err := s.PendingData()
	if err != nil {
		return nil, err
	}

	pendingStateUpdate := pending.GetStateUpdate()
	state, err := state.New(pendingStateUpdate.OldRoot, s.blockchain.StateDB)
	if err != nil {
		return nil, err
	}

	return NewPendingState(pendingStateUpdate.StateDiff, pending.GetNewClasses(), state), nil
}

// PendingStateAfterIndex returns the state obtained by applying all transaction state diffs
// up to given index in the pre-confirmed block.
func (s *Synchronizer) PendingStateBeforeIndex(index int) (core.StateReader, func() error, error) {
	txn := s.db.NewIndexedBatch()

	pending, err := s.PendingData()
	if err != nil {
		return nil, nil, err
	}

	if pending.Variant() != core.PreConfirmedBlockVariant {
		return nil, nil, errors.New("only supported for pre_confirmed block")
	}

	stateDiff := core.EmptyStateDiff()
	// Transaction state diffs size must always match Transactions
	txStateDiffs := pending.GetTransactionStateDiffs()
	for _, txStateDiff := range txStateDiffs[:index] {
		stateDiff.Merge(txStateDiff)
	}

	return NewPendingState(&stateDiff, pending.GetNewClasses(), core.NewState(txn)), noop, nil
}

func (s *Synchronizer) storeEmptyPendingData(lastHeader *core.Header) {
	blockVer, err := core.ParseBlockVersion(lastHeader.ProtocolVersion)
	if err == nil {
		if blockVer.GreaterThanEqual(core.Ver0_14_0) {
			if err := s.storeEmptyPreConfirmed(lastHeader); err != nil {
				s.log.Errorw("Failed to store empty pre_confirmed block", "number", lastHeader.Number)
			}
		} else {
			if err := s.storeEmptyPending(lastHeader); err != nil {
				s.log.Errorw("Failed to store empty pending block", "number", lastHeader.Number)
			}
		}
	} else {
		s.log.Errorw("Failed to parse block version", "err", err)
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

	pending := Pending{
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

func (s *Synchronizer) storeEmptyPreConfirmed(latestHeader *core.Header) error {
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
		return err
	}

	preConfirmed := core.PreConfirmed{
		Block: preConfirmedBlock,
		StateUpdate: &core.StateUpdate{
			OldRoot:   latestHeader.GlobalStateRoot,
			StateDiff: stateDiff,
		},
		NewClasses:            make(map[felt.Felt]core.Class, 0),
		TransactionStateDiffs: make([]*core.StateDiff, 0),
		CandidateTxs:          make([]core.Transaction, 0),
	}

	s.pendingData.Store(utils.HeapPtr[core.PendingData](&preConfirmed))
	return nil
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
