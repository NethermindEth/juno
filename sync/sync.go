package sync

import (
	"context"
	"errors"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/starknetdata"
	"github.com/NethermindEth/juno/utils"
	"github.com/sourcegraph/conc/stream"
)

var (
	_ service.Service = (*Synchronizer)(nil)
	_ Reader          = (*Synchronizer)(nil)
)

const (
	OpVerify = "verify"
	OpStore  = "store"
	OpFetch  = "fetch"
)

// This is a work-around. mockgen chokes when the instantiated generic type is in the interface.
type HeaderSubscription struct {
	*feed.Subscription[*core.Header]
}

type ReorgSubscription struct {
	*feed.Subscription[*ReorgData]
}

// Todo: Since this is also going to be implemented by p2p package we should move this interface to node package
//
//go:generate mockgen -destination=../mocks/mock_synchronizer.go -package=mocks -mock_names Reader=MockSyncReader github.com/NethermindEth/juno/sync Reader
type Reader interface {
	StartingBlockNumber() (uint64, error)
	HighestBlockHeader() *core.Header
	SubscribeNewHeads() HeaderSubscription
	SubscribeReorg() ReorgSubscription
}

// This is temporary and will be removed once the p2p synchronizer implements this interface.
type NoopSynchronizer struct{}

func (n *NoopSynchronizer) StartingBlockNumber() (uint64, error) {
	return 0, nil
}

func (n *NoopSynchronizer) HighestBlockHeader() *core.Header {
	return nil
}

func (n *NoopSynchronizer) SubscribeNewHeads() HeaderSubscription {
	return HeaderSubscription{feed.New[*core.Header]().Subscribe()}
}

func (n *NoopSynchronizer) SubscribeReorg() ReorgSubscription {
	return ReorgSubscription{feed.New[*ReorgData]().Subscribe()}
}

// ReorgData represents data about reorganised blocks, starting and ending block number and hash
type ReorgData struct {
	// StartBlockHash is the hash of the first known block of the orphaned chain
	StartBlockHash *felt.Felt `json:"starting_block_hash"`
	// StartBlockNum is the number of the first known block of the orphaned chain
	StartBlockNum uint64 `json:"starting_block_number"`
	// The last known block of the orphaned chain
	EndBlockHash *felt.Felt `json:"ending_block_hash"`
	// Number of the last known block of the orphaned chain
	EndBlockNum uint64 `json:"ending_block_number"`
}

// Synchronizer manages a list of StarknetData to fetch the latest blockchain updates
type Synchronizer struct {
	blockchain          *blockchain.Blockchain
	readOnlyBlockchain  bool
	starknetData        starknetdata.StarknetData
	startingBlockNumber *uint64
	highestBlockHeader  atomic.Pointer[core.Header]
	newHeads            *feed.Feed[*core.Header]
	reorgFeed           *feed.Feed[*ReorgData]

	log      utils.SimpleLogger
	listener EventListener

	pendingPollInterval time.Duration
	catchUpMode         bool

	currReorg *ReorgData // If nil, no reorg is happening
}

func New(bc *blockchain.Blockchain, starkNetData starknetdata.StarknetData,
	log utils.SimpleLogger, pendingPollInterval time.Duration, readOnlyBlockchain bool,
) *Synchronizer {
	s := &Synchronizer{
		blockchain:          bc,
		starknetData:        starkNetData,
		log:                 log,
		newHeads:            feed.New[*core.Header](),
		reorgFeed:           feed.New[*ReorgData](),
		pendingPollInterval: pendingPollInterval,
		listener:            &SelectiveListener{},
		readOnlyBlockchain:  readOnlyBlockchain,
	}
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
			stateUpdate, block, err := s.starknetData.StateUpdateWithBlock(ctx, height)
			if err != nil {
				continue
			}

			newClasses, err := s.fetchUnknownClasses(ctx, stateUpdate)
			if err != nil {
				continue
			}

			return func() {
				verifiers.Go(func() stream.Callback {
					return s.verifierTask(ctx, block, stateUpdate, newClasses, resetStreams)
				})
			}
		}
	}
}

func (s *Synchronizer) fetchUnknownClasses(ctx context.Context, stateUpdate *core.StateUpdate) (map[felt.Felt]core.Class, error) {
	state, closer, err := s.blockchain.HeadState()
	if err != nil {
		// if err is db.ErrKeyNotFound we are on an empty DB
		if !errors.Is(err, db.ErrKeyNotFound) {
			return nil, err
		}
		closer = func() error {
			return nil
		}
	}

	newClasses := make(map[felt.Felt]core.Class)
	fetchIfNotFound := func(classHash *felt.Felt) error {
		if _, ok := newClasses[*classHash]; ok {
			return nil
		}

		stateErr := db.ErrKeyNotFound
		if state != nil {
			_, stateErr = state.Class(classHash)
		}

		if errors.Is(stateErr, db.ErrKeyNotFound) {
			class, fetchErr := s.starknetData.Class(ctx, classHash)
			if fetchErr == nil {
				newClasses[*classHash] = class
			}
			return fetchErr
		}
		return stateErr
	}

	for _, classHash := range stateUpdate.StateDiff.DeployedContracts {
		if err = fetchIfNotFound(classHash); err != nil {
			return nil, utils.RunAndWrapOnError(closer, err)
		}
	}
	for _, classHash := range stateUpdate.StateDiff.DeclaredV0Classes {
		if err = fetchIfNotFound(classHash); err != nil {
			return nil, utils.RunAndWrapOnError(closer, err)
		}
	}
	for classHash := range stateUpdate.StateDiff.DeclaredV1Classes {
		if err = fetchIfNotFound(&classHash); err != nil {
			return nil, utils.RunAndWrapOnError(closer, err)
		}
	}

	return newClasses, closer()
}

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
					s.revertHead(block)
				} else {
					s.log.Warnw("Failed storing Block", "number", block.Number,
						"hash", block.Hash.ShortString(), "err", err)
				}
				resetStreams()
				return
			}
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

			s.newHeads.Send(block.Header)
			s.log.Infow("Stored Block", "number", block.Number, "hash",
				block.Hash.ShortString(), "root", block.GlobalStateRoot.ShortString())
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
	go s.pollPending(syncCtx, pendingSem)

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

	err = s.blockchain.RevertHead()
	if err != nil {
		s.log.Warnw("Failed reverting HEAD", "reverted", localHead, "err", err)
	} else {
		s.log.Infow("Reverted HEAD", "reverted", localHead)
	}

	if s.currReorg == nil { // first block of the reorg
		s.currReorg = &ReorgData{
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

func (s *Synchronizer) pollPending(ctx context.Context, sem chan struct{}) {
	if s.pendingPollInterval == time.Duration(0) {
		return
	}

	pendingPollTicker := time.NewTicker(s.pendingPollInterval)
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
						s.log.Debugw("Error while trying to poll pending block", "err", err)
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
				highestBlock, err := s.starknetData.BlockLatest(ctx)
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

	// not at the tip of the chain yet, no need to poll pending
	if highestBlockHeader.Number > head.Number {
		return nil
	}

	pendingStateUpdate, pendingBlock, err := s.starknetData.StateUpdatePendingWithBlock(ctx)
	if err != nil {
		return err
	}

	newClasses, err := s.fetchUnknownClasses(ctx, pendingStateUpdate)
	if err != nil {
		return err
	}

	s.log.Debugw("Found pending block", "txns", pendingBlock.TransactionCount)
	return s.blockchain.StorePending(&blockchain.Pending{
		Block:       pendingBlock,
		StateUpdate: pendingStateUpdate,
		NewClasses:  newClasses,
	})
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

func (s *Synchronizer) SubscribeNewHeads() HeaderSubscription {
	return HeaderSubscription{
		Subscription: s.newHeads.Subscribe(),
	}
}

func (s *Synchronizer) SubscribeReorg() ReorgSubscription {
	return ReorgSubscription{
		Subscription: s.reorgFeed.Subscribe(),
	}
}
