package sync

import (
	"context"
	"errors"
	"fmt"
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
	PendingState() (core.StateReader, func() error, error)
	PendingStateBeforeIndex(index int) (core.StateReader, func() error, error)
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

func (n *NoopSynchronizer) PendingState() (core.StateReader, func() error, error) {
	return nil, nil, errors.New("PendingState() not implemented")
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
		reverseStateDiff)
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
	pollLatestWg := &stdsync.WaitGroup{}
	if s.readOnlyBlockchain {
		pollLatestWg.Go(func() { s.pollLatest(syncCtx) })
		pollLatestWg.Wait()
		return
	}

	fetchers, verifiers := s.setupWorkers()
	streamCtx, streamCancel := context.WithCancel(syncCtx)

	pollLatestWg.Go(func() { s.pollLatest(syncCtx) })

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
				pollLatestWg.Wait()
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
	return min(16, runtime.GOMAXPROCS(0)) //nolint:mnd
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

func (s *Synchronizer) pollLatest(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	fetchWg := &stdsync.WaitGroup{}
	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			fetchWg.Wait()
			return
		case <-ticker.C:
			fetchWg.Wait()
			fetchWg.Go(func() {
				highestBlock, err := s.dataSource.BlockLatest(ctx)
				if err != nil {
					s.log.Warnw("Failed fetching latest block", "err", err)
				} else {
					s.highestBlockHeader.Store(highestBlock.Header)
				}
			})
		}
	}
}

func (s *Synchronizer) pollPendingData(ctx context.Context) {
	if s.pendingPollInterval == time.Duration(0) ||
		s.preConfirmedPollInterval == time.Duration(0) {
		s.log.Infow("Pending data polling is disabled")
		return
	}
	s.pollPending(ctx)

	// If pollPending is cancelled by parent context
	// do not initiate pre_confirmed polling.
	select {
	case <-ctx.Done():
		return
	default:
		s.pollPreConfirmed(ctx)
	}
}

func (s *Synchronizer) pollPending(ctx context.Context) {
	if s.pendingPollInterval == time.Duration(0) {
		s.log.Infow("Pending block polling is disabled")
		return
	}
	pendingPollTicker := time.NewTicker(s.pendingPollInterval)

	ctx, cancel := context.WithCancel(ctx)
	wg := &stdsync.WaitGroup{}
	for {
		select {
		case <-ctx.Done():
			pendingPollTicker.Stop()
			cancel()
			wg.Wait()
			return
		case <-pendingPollTicker.C:
			wg.Wait()
			wg.Go(func() {
				err := s.fetchAndStorePending(ctx)
				if err != nil {
					if errors.Is(err, ErrMustSwitchPollingPreConfirmed) {
						s.log.Infow(
							"Detected block version >= 0.14.0; polling for pre_confirmed blocks",
						)
						cancel()
						return
					}
					s.log.Debugw("Error while trying to poll pending block", "err", err)
				}
			})
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
	pending.Block.Number = head.Number + 1

	blockVer, err = core.ParseBlockVersion(pending.Block.ProtocolVersion)
	if err != nil {
		return err
	}

	if blockVer.GreaterThanEqual(core.Ver0_14_0) {
		return ErrMustSwitchPollingPreConfirmed
	}

	return s.StorePending(&pending)
}

// pollPreLatest fetches preLatest block for each new head and forwards the prelatest to given chanel.
//
// Consumer of preLatestChan never receives the same preLatest twice.
func (s *Synchronizer) pollPreLatest(ctx context.Context, preLatestChan chan *core.PreLatest) {
	if s.pendingPollInterval == time.Duration(0) {
		s.log.Infow("Pre-latest block polling is disabled")
		return
	}

	newHeadSub := s.newHeads.SubscribeKeepLast()
	defer newHeadSub.Unsubscribe()
	// When fetching pre-latest it is possible that pre-latest we fetched
	// belongs to l2 finalised block node not fetched yet.
	// In such scenario we cache the pre-latest for the future block.
	// Upon receiving the corresponding head for the cached pre-latest,
	// it will be removed from cache. However in case of reorg this key might not get evicted,
	// and incase of reorg, this goroutine must be restarted.
	seenBlockHashes := make(map[felt.Felt]*core.PreLatest)

	var subCtx context.Context
	var cancel context.CancelFunc = func() {}
	var fetchWg stdsync.WaitGroup

	for {
		select {
		case <-ctx.Done():
			cancel()
			fetchWg.Wait()
			return
		case head := <-newHeadSub.Recv():
			cancel()
			fetchWg.Wait()
			subCtx, cancel = context.WithCancel(ctx)
			fetchWg.Go(func() { s.fetchPreLatest(subCtx, head, seenBlockHashes, preLatestChan) })
		}
	}
}

// fetchPreLatest fetches the prelatest block for a given head and forwards the value to preLatestChan.
// Consumers of preLatestChan will receive each pre-latest once. It is possible for a pre-latest block to 
// be skipped if:
//  - the pre-latest has already become the latest block of the chain.
//  - the pre-latest is part of the future of the chain. In this case, it will be cached and served later.
func (s *Synchronizer) fetchPreLatest(
	ctx context.Context,
	head *core.Block,
	seenBlockHashes map[felt.Felt]*core.PreLatest,
	preLatestChan chan *core.PreLatest,
) {
	highestBlockHeader := s.highestBlockHeader.Load()
	if highestBlockHeader == nil || highestBlockHeader.Number > head.Number {
		// not at the tip of the chain yet, no need to poll pre_latest
		return
	}

	if val, hit := seenBlockHashes[*head.Hash]; hit {
		// Already seen this pre_latest for this head.
		val.Block.Number = head.Number + 1
		delete(seenBlockHashes, *head.Hash)
		select {
		case <-ctx.Done():
			return
		case preLatestChan <- val:
			return
		}
	}

	preLatestPollTicker := time.NewTicker(s.pendingPollInterval)
	defer preLatestPollTicker.Stop()

	for {
		pending, err := s.dataSource.BlockPending(ctx)
		if err == nil {
			preLatest := core.PreLatest(pending)
			// Seen future hash, new latest is on flight
			// Cache the pre-latest to use when its predecessor is available
			if *preLatest.Block.ParentHash != *head.Hash {
				seenBlockHashes[*preLatest.Block.ParentHash] = &preLatest
				return
			}

			preLatest.Block.Number = head.Number + 1
			select {
			case <-ctx.Done():
				return
			case preLatestChan <- &preLatest:
				return
			}
		}

		s.log.Debugw("Error while trying to poll pre_latest block", "err", err)
		select {
		case <-ctx.Done():
			return
		case <-preLatestPollTicker.C:
			continue
		}
	}
}

// pollPreConfirmed fetches and stores pre_confirmed blocks.
func (s *Synchronizer) pollPreConfirmed(ctx context.Context) {
	if s.preConfirmedPollInterval == time.Duration(0) {
		s.log.Infow("Pre-confirmed block polling is disabled")
		return
	}

	newHeadSub := s.newHeads.SubscribeKeepLast()
	defer newHeadSub.Unsubscribe()

	preLatestChan := make(chan *core.PreLatest)
	defer close(preLatestChan)
	var preLatestWg stdsync.WaitGroup
	preLatestWg.Go(func() { s.pollPreLatest(ctx, preLatestChan) })

	mostRecentPredecessor := int64(-1)
	var subCtx context.Context
	var cancel context.CancelFunc = func() {}
	var fetchWg stdsync.WaitGroup

	for {
		select {
		case <-ctx.Done():
			cancel()
			fetchWg.Wait()
			preLatestWg.Wait()
			return
		case preLatest := <-preLatestChan:
			if int64(preLatest.Block.Number) <= mostRecentPredecessor {
				// already got more recent data
				continue
			}
			mostRecentPredecessor = int64(preLatest.Block.Number)

			cancel()
			fetchWg.Wait()
			subCtx, cancel = context.WithCancel(ctx)

			// Store new preLatest as base and serve most recent state till pre_confirmed is fetched.
			// No-op if actual preConfirmed is more recent.
			if err := s.storeEmptyPreConfirmed(preLatest.Block.Header, preLatest); err != nil {
				s.log.Debugw("Error while storing empty pre_confirmed", "error", err)
			}

			fetchWg.Go(func() {
				s.fetchAndStorePreConfirmed(
					subCtx,
					uint64(mostRecentPredecessor+1),
					preLatest,
				)
			})
		case head := <-newHeadSub.Recv():
			if int64(head.Number) <= mostRecentPredecessor {
				// already got more recent data for this head
				continue
			}

			// Head is more recent than pre-latest,
			// use head as base and poll for latest + 1
			mostRecentPredecessor = int64(head.Number)
			cancel()
			fetchWg.Wait()
			subCtx, cancel = context.WithCancel(ctx)
			// Store new head as base and serve most recent state till pre_confirmed is fetched.
			// No-op if actual preConfirmed is more recent.
			if err := s.storeEmptyPreConfirmed(head.Header, nil); err != nil {
				s.log.Debugw("Error while storing empty pre_confirmed", "error", err)
			}

			fetchWg.Go(func() {
				s.fetchAndStorePreConfirmed(
					subCtx,
					uint64(mostRecentPredecessor+1),
					nil,
				)
			})
		}
	}
}

func (s *Synchronizer) fetchAndStorePreConfirmed(
	ctx context.Context,
	preConfirmedNumber uint64,
	preLatest *core.PreLatest,
) {
	preConfirmedPollTicker := time.NewTicker(s.preConfirmedPollInterval)
	defer preConfirmedPollTicker.Stop()

	for {
		highestBlockHeader := s.highestBlockHeader.Load()
		if highestBlockHeader == nil || highestBlockHeader.Number >= preConfirmedNumber {
			// not at the tip of the chain yet, no need to poll preconfirmed
			return
		}

		preConfirmed, err := s.dataSource.PreConfirmedBlockByNumber(ctx, preConfirmedNumber)
		if err == nil {
			preConfirmed.WithPreLatest(preLatest)
			isChanged, err := s.StorePreConfirmed(&preConfirmed)
			if err == nil && isChanged {
				s.log.Tracew(
					"pre_confirmed block stored",
					"number", preConfirmed.Block.Number,
					"txCount", preConfirmed.Block.TransactionCount,
				)
				s.pendingDataFeed.Send(&preConfirmed)
			}

			if err != nil {
				s.log.Debugw("Error while trying to store pre_confirmed block", "err", err)
			}
		} else {
			s.log.Debugw("Error while trying to poll pre_confirmed block", "err", err)
		}

		select {
		case <-ctx.Done():
			preConfirmedPollTicker.Stop()
			return
		case <-preConfirmedPollTicker.C:
			continue
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

// StorePreConfirmed stores a pre_confirmed block given that it is for the next height.
func (s *Synchronizer) StorePreConfirmed(p *core.PreConfirmed) (bool, error) {
	err := blockchain.CheckBlockVersion(p.GetBlock().ProtocolVersion)
	if err != nil {
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

	existingPending := *existingPtr

	if !existingPending.Validate(head) {
		return s.pendingData.CompareAndSwap(
			existingPtr,
			utils.HeapPtr[core.PendingData](p),
		), nil
	}

	existingPendingB := existingPending.GetBlock()
	if existingPendingB.Number == p.GetBlock().Number &&
		existingPendingB.TransactionCount >= p.GetBlock().TransactionCount {
		// ignore the incoming pre_confirmed if it has fewer transactions than the one we already have
		return false, nil
	}

	return s.pendingData.CompareAndSwap(
		existingPtr,
		utils.HeapPtr[core.PendingData](p),
	), nil
}

func (s *Synchronizer) PendingData() (core.PendingData, error) {
	ptr := s.pendingData.Load()
	if ptr == nil || *ptr == nil {
		return nil, ErrPendingBlockNotFound
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
		return p, nil
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

var noop = func() error { return nil }

// PendingState returns the state resulting from execution of the pending block
func (s *Synchronizer) PendingState() (core.StateReader, func() error, error) {
	txn := s.db.NewIndexedBatch()

	pendingPtr := s.pendingData.Load()
	if pendingPtr == nil || *pendingPtr == nil {
		return nil, nil, ErrPendingBlockNotFound
	}

	head, err := s.blockchain.HeadsHeader()
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			return nil, nil, err
		}
		head = nil
	}

	pending := *pendingPtr

	if !pending.Validate(head) {
		return nil, nil, ErrPendingBlockNotFound
	}

	stateDiff := core.EmptyStateDiff()
	newClasses := make(map[felt.Felt]core.Class)
	switch pending.Variant() {
	case core.PreConfirmedBlockVariant:
		preLatest := pending.GetPreLatest()
		// Built pre_confirmed state top on pre_latest if
		// pre_confirmed is 2 blocks ahead of latest
		if preLatest != nil && preLatest.Block.Number == head.Number+1 {
			stateDiff.Merge(preLatest.StateUpdate.StateDiff)
			newClasses = preLatest.NewClasses
		}
		stateDiff.Merge(pending.GetStateUpdate().StateDiff)
	case core.PendingBlockVariant:
		newClasses = pending.GetNewClasses()
		stateDiff.Merge(pending.GetStateUpdate().StateDiff)
	default:
		return nil, nil, errors.New("unsupported pending data variant")
	}

	return NewPendingState(&stateDiff, newClasses, core.NewState(txn)), noop, nil
}

// PendingStateAfterIndex returns the state obtained by applying all transaction state diffs
// up to given index in the pre-confirmed block.
func (s *Synchronizer) PendingStateBeforeIndex(index int) (core.StateReader, func() error, error) {
	txn := s.db.NewIndexedBatch()

	pendingPtr := s.pendingData.Load()
	if pendingPtr == nil || *pendingPtr == nil {
		return nil, nil, ErrPendingBlockNotFound
	}

	pending := *pendingPtr
	if pending.Variant() != core.PreConfirmedBlockVariant {
		return nil, nil, errors.New("only supported for pre_confirmed block")
	}

	head, err := s.blockchain.HeadsHeader()
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			return nil, nil, err
		}
		head = nil
	}

	if !pending.Validate(head) {
		return nil, nil, ErrPendingBlockNotFound
	}

	if index > len(pending.GetTransactions()) {
		return nil, nil, errors.New("transaction index out of bounds")
	}

	stateDiff := core.EmptyStateDiff()
	newClasses := make(map[felt.Felt]core.Class)
	preLatest := pending.GetPreLatest()
	// Built pre_confirmed state top on pre_latest if
	// pre_confirmed is 2 blocks ahead of latest
	if preLatest != nil {
		if preLatest.Block.Number == head.Number+1 {
			stateDiff.Merge(preLatest.StateUpdate.StateDiff)
			newClasses = preLatest.NewClasses
		}
	}

	// Transaction state diffs size must always match Transactions
	txStateDiffs := pending.GetTransactionStateDiffs()
	for _, txStateDiff := range txStateDiffs[:index] {
		stateDiff.Merge(txStateDiff)
	}

	return NewPendingState(&stateDiff, newClasses, core.NewState(txn)), noop, nil
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
