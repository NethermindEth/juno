package sync

import (
	"context"
	"errors"
	"runtime"
	stdsync "sync"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"

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

type PreLatestDataSubscription struct {
	*feed.Subscription[*core.PreLatest]
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
	SubscribePreLatest() PreLatestDataSubscription

	PendingData() (core.PendingData, error)
	PendingBlock() *core.Block
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

func (n *NoopSynchronizer) SubscribePreLatest() PreLatestDataSubscription {
	return PreLatestDataSubscription{feed.New[*core.PreLatest]().Subscribe()}
}

func (n *NoopSynchronizer) PendingBlock() *core.Block {
	return nil
}

func (n *NoopSynchronizer) PendingData() (core.PendingData, error) {
	return nil, errors.New("PendingData() is not implemented")
}

func (n *NoopSynchronizer) PendingState() (core.CommonStateReader, func() error, error) {
	return nil, nil, errors.New("PendingState() not implemented")
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
	preLatestDataFeed   *feed.Feed[*core.PreLatest]

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
		preLatestDataFeed:        feed.New[*core.PreLatest](),
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
				if lastPossiblyValidHeight, isReorg := s.isReverting(ctx, height); isReorg {
					return func() {
						verifiers.Go(func() stream.Callback {
							return func() {
								s.revertTask(ctx, lastPossiblyValidHeight, resetStreams)
							}
						})
					}
				}
				continue
			}

			return func() {
				verifiers.Go(func() stream.Callback {
					return s.verifierTask(ctx, &committedBlock, resetStreams)
				})
			}
		}
	}
}

func (s *Synchronizer) isReverting(
	ctx context.Context,
	nextHeight uint64,
) (lastPossiblyValidHeight uint64, isReorg bool) {
	// If localHead is somehow not available, we precautionarily assume we're not reverting
	localHead, err := s.blockchain.HeadsHeader()
	if err != nil {
		return 0, false
	}
	localHeight := localHead.Number

	// Only check if we're waiting for the very next block
	if localHeight+1 != nextHeight {
		return 0, false
	}

	// If unable to fetch remoteHead block, we precautionarily assume we're not reverting
	remoteHead, err := s.dataSource.BlockLatest(ctx)
	if err != nil {
		return 0, false
	}
	remoteHeight := remoteHead.Number

	// If a newer block is available, revert will be handled in storeTask
	if remoteHeight > localHeight {
		return 0, false
	}

	// If the latest block is at the same height as the head, compare their hashes
	// If the latest block is older than the head, compare with the stored block at the same height
	if remoteHeight < localHeight {
		localHeight = remoteHeight
		if localHead, err = s.blockchain.BlockHeaderByNumber(localHeight); err != nil {
			return 0, false
		}
	}

	if *remoteHead.Hash == *localHead.Hash {
		return 0, false
	}

	return remoteHeight - 1, true
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
		&reverseStateDiff,
	)
	if err != nil {
		s.log.Errorw("Plugin RevertBlock failure:", "err", err)
	}
}

func (s *Synchronizer) verifierTask(
	ctx context.Context,
	committedBlock *CommittedBlock,
	resetStreams context.CancelFunc,
) stream.Callback {
	verifyTimer := time.Now()
	commitments, err := s.blockchain.SanityCheckNewHeight(
		committedBlock.Block,
		committedBlock.StateUpdate,
		committedBlock.NewClasses,
	)
	if err != nil {
		return func() {
			defer close(committedBlock.Persisted)
			s.log.Warnw(
				"Sanity checks failed",
				"number",
				committedBlock.Block.Number,
				"hash",
				committedBlock.Block.Hash.ShortString(),
				"err",
				err,
			)
			resetStreams()
		}
	}

	s.listener.OnSyncStepDone(OpVerify, committedBlock.Block.Number, time.Since(verifyTimer))
	return func() {
		s.storeTask(ctx, committedBlock, resetStreams, commitments)
	}
}

func (s *Synchronizer) storeTask(
	ctx context.Context,
	committedBlock *CommittedBlock,
	resetStreams context.CancelFunc,
	commitments *core.BlockCommitments,
) {
	defer close(committedBlock.Persisted)
	select {
	case <-ctx.Done():
		return
	default:
	}

	storeTimer := time.Now()
	block := committedBlock.Block
	stateUpdate := committedBlock.StateUpdate
	newClasses := committedBlock.NewClasses
	if err := s.blockchain.Store(block, commitments, stateUpdate, newClasses); err != nil {
		if errors.Is(err, blockchain.ErrParentDoesNotMatchHead) {
			// Block block.Number - 1 is the parent of this block which doesn't match
			// so we need to revert the head to block.Number - 2
			s.revertTask(ctx, block.Number-2, resetStreams)
			return
		}

		s.log.Warnw("Failed storing Block", "number", block.Number,
			"hash", block.Hash.ShortString(), "err", err)
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
			s.catchUpMode = isBehind
		}
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

func (s *Synchronizer) revertTask(ctx context.Context, lastPossiblyValidHeight uint64, resetStreams context.CancelFunc) {
	defer resetStreams()
	var lastHead *core.Header

	shouldContinue := true
	for shouldContinue {
		localHeader, err := s.blockchain.HeadsHeader()
		if err != nil {
			s.log.Errorw("Failed to retrieve the local head header", "err", err)
			break
		}
		lastHead = localHeader

		// Always reorg head newer than lastPossiblyValidHeight. Otherwise, check the hash
		if localHeader.Number <= lastPossiblyValidHeight {
			remoteBlock, err := s.dataSource.BlockByNumber(ctx, localHeader.Number)
			if err != nil {
				s.log.Errorw("Failed to retrieve the remote header", "err", err)
				break
			}
			remoteHeader := remoteBlock.Block.Header

			// Double check to avoid reverting the head if the hash is the same
			if *remoteHeader.Hash == *localHeader.Hash {
				break
			}

			// Terminate the loop if the parent hash is the same
			shouldContinue = *remoteHeader.ParentHash != *localHeader.ParentHash
		}

		// Actuallly revert the head and restart the sync process
		if s.plugin != nil {
			s.handlePluginRevertBlock()
		}
		s.revertHead(localHeader)
	}

	if lastHead != nil {
		s.storeEmptyPendingData(lastHead)
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

	if s.readOnlyBlockchain {
		s.pollLatest(syncCtx)
		return
	}

	fetchers, verifiers := s.setupWorkers()
	streamCtx, streamCancel := context.WithCancel(syncCtx)

	go s.pollLatest(syncCtx)

	pollPendingWg := &stdsync.WaitGroup{}
	pollPendingWg.Go(func() { s.pollPendingData(streamCtx) })

	for {
		select {
		case <-streamCtx.Done():
			streamCancel()
			fetchers.Wait()
			verifiers.Wait()
			pollPendingWg.Wait()

			select {
			case <-syncCtx.Done():
				return
			default:
				streamCtx, streamCancel = context.WithCancel(syncCtx)
				nextHeight = s.nextHeight()
				fetchers, verifiers = s.setupWorkers()
				pollPendingWg.Go(func() { s.pollPendingData(streamCtx) })
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
	return min(16, runtime.GOMAXPROCS(0))
}

func (s *Synchronizer) setupWorkers() (*stream.Stream, *stream.Stream) {
	numWorkers := 1
	if s.catchUpMode {
		numWorkers = maxWorkers()
	}
	return stream.New().WithMaxGoroutines(numWorkers), stream.New().WithMaxGoroutines(runtime.GOMAXPROCS(0))
}

func (s *Synchronizer) revertHead(localHeader *core.Header) {
	s.log.Infow("Reorg detected", "localHead", localHeader.Hash)
	if err := s.blockchain.RevertHead(); err != nil {
		s.log.Warnw("Failed reverting HEAD", "reverted", localHeader.Hash, "err", err)
	} else {
		s.log.Infow("Reverted HEAD", "reverted", localHeader.Hash)
	}

	if s.currReorg == nil { // first block of the reorg
		s.currReorg = &ReorgBlockRange{
			StartBlockHash: localHeader.Hash,
			StartBlockNum:  localHeader.Number,
			EndBlockHash:   localHeader.Hash,
			EndBlockNum:    localHeader.Number,
		}
	} else { // not the first block of the reorg, adjust the starting block
		s.currReorg.StartBlockHash = localHeader.Hash
		s.currReorg.StartBlockNum = localHeader.Number
	}

	s.listener.OnReorg(localHeader.Number)
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

func (s *Synchronizer) SubscribePreLatest() PreLatestDataSubscription {
	return PreLatestDataSubscription{s.preLatestDataFeed.Subscribe()}
}

func (s *Synchronizer) pollLatest(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)

	for {
		highestBlock, err := s.dataSource.BlockLatest(ctx)
		if err != nil {
			s.log.Warnw("Failed fetching latest block", "err", err)
		} else {
			s.highestBlockHeader.Store(highestBlock.Header)
		}

		select {
		case <-ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
			continue
		}
	}
}

func (s *Synchronizer) PendingData() (core.PendingData, error) {
	ptr := s.pendingData.Load()
	if ptr == nil || *ptr == nil {
		return nil, core.ErrPendingDataNotFound
	}

	head, err := s.blockchain.HeadsHeader()
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			return nil, err
		}
		head = nil
	}

	p := *ptr
	if p.Validate(head) {
		// Special handling: if the pending data is PreConfirmed and contains a
		// 'pre-latest' block attachment that is now outdated (head moved on),
		// return a copy with the pre-latest attachment discarded.
		if preConfirmed, ok := p.(*core.PreConfirmed); ok && head != nil {
			shouldDiscardPreLatest := preConfirmed.Block.Number == head.Number+1 &&
				preConfirmed.PreLatest != nil
			if shouldDiscardPreLatest {
				return preConfirmed.Copy().WithPreLatest(nil), nil
			}
		}
		return p, nil
	}

	return nil, core.ErrPendingDataNotFound
}

func (s *Synchronizer) PendingBlock() *core.Block {
	pendingData, err := s.PendingData()
	if err != nil {
		return nil
	}
	return pendingData.GetBlock()
}
