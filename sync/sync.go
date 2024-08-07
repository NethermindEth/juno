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

// Todo: Since this is also going to be implemented by p2p package we should move this interface to node package
//
//go:generate mockgen -destination=../mocks/mock_synchronizer.go -package=mocks -mock_names Reader=MockSyncReader github.com/NethermindEth/juno/sync Reader
type Reader interface {
	StartingBlockNumber() (uint64, error)
	HighestBlockHeader() *core.Header
	SubscribeNewHeads() HeaderSubscription
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

// Synchronizer manages a list of StarknetData to fetch the latest blockchain updates
type Synchronizer struct {
	blockchain          *blockchain.Blockchain
	readOnlyBlockchain  bool
	starknetData        starknetdata.StarknetData
	startingBlockNumber *uint64
	highestBlockHeader  atomic.Pointer[core.Header]
	newHeads            *feed.Feed[*core.Header]

	log      utils.SimpleLogger
	listener EventListener

	pendingPollInterval time.Duration
	catchUpMode         bool
}

func New(bc *blockchain.Blockchain, starkNetData starknetdata.StarknetData,
	log utils.SimpleLogger, pendingPollInterval time.Duration, readOnlyBlockchain bool,
) *Synchronizer {
	s := &Synchronizer{
		blockchain:          bc,
		starknetData:        starkNetData,
		log:                 log,
		newHeads:            feed.New[*core.Header](),
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
				
					s.log.Warnw("Failed storing Block", "number", block.Number,
						"hash", block.Hash.ShortString(), "err", err)
				// }
				resetStreams()
				return
			}
			s.listener.OnSyncStepDone(OpStore, block.Number, time.Since(storeTimer))

			highestBlockHeader := s.highestBlockHeader.Load()

			if highestBlockHeader != nil {
				m, mProcs := 16, runtime.GOMAXPROCS(0)
				maxWorkers := mProcs
				if mProcs >  m{
					maxWorkers = m
				} else {
					maxWorkers = mProcs 
				}

				isBehind := highestBlockHeader.Number > block.Number+uint64(maxWorkers)
				if s.catchUpMode != isBehind {
					resetStreams()
				}
				s.catchUpMode = isBehind
			}

			if highestBlockHeader == nil || highestBlockHeader.Number < block.Number {
				s.highestBlockHeader.CompareAndSwap(highestBlockHeader, block.Header)
			}

			s.newHeads.Send(block.Header)
			s.log.Infow("Stored Block", "number", block.Number, "hash",
				block.Hash.ShortString(), "root", block.GlobalStateRoot.ShortString())
		}
	}
}



func (s *Synchronizer) syncBlocks(syncCtx context.Context) {
	defer func() {
		s.startingBlockNumber = nil
		s.highestBlockHeader.Store(nil)
	}()

var nextHeight uint64
	if h, err := s.blockchain.Height(); err == nil {
		nextHeight = h + 1
	} else {
		nextHeight = 0
	} 
	startingHeight := nextHeight
	s.startingBlockNumber = &startingHeight
pendingSem := make(chan struct{}, 1)
	latestSem := make(chan struct{}, 1)
	if s.readOnlyBlockchain {
		// Polling for the latest block in read-only mode
		go func() {
			poll := func() {
				select {
				case latestSem <- struct{}{}:
					go func() {
						defer func() { <-latestSem }()
						highestBlock, err := s.starknetData.BlockLatest(syncCtx)
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
			defer ticker.Stop()
			poll()

			for {
				select {
				case <-syncCtx.Done():
					return
				case <-ticker.C:
					poll()
				}
			}
		}()
		return
	}
numWorkers := 1
	if s.catchUpMode {
		m, mProcs := 16, runtime.GOMAXPROCS(0)
		if mProcs > m {
			numWorkers = m
			} else {
				numWorkers = mProcs
			}
		}
	fetchers, verifiers := stream.New().WithMaxGoroutines(numWorkers), stream.New().WithMaxGoroutines(runtime.GOMAXPROCS(0))
	streamCtx, streamCancel := context.WithCancel(syncCtx)

	go func() {
		pollLatest := func() {
			select {
			case latestSem <- struct{}{}:
				go func() {
					defer func() { <-latestSem }()
					highestBlock, err := s.starknetData.BlockLatest(syncCtx)
					if err != nil {
						s.log.Warnw("Failed fetching latest block", "err", err)
					} else {
						s.highestBlockHeader.Store(highestBlock.Header)
					}
				}()
			default:
			}
		}

		latestTicker := time.NewTicker(time.Minute)
		defer latestTicker.Stop()
		pollLatest()

		for {
			select {
			case <-syncCtx.Done():
				return
			case <-latestTicker.C:
				pollLatest()
			}
		}
	}()

	// Polling for pending blocks
	go func() {
		if s.pendingPollInterval == time.Duration(0) {
			return
		}

		pendingPollTicker := time.NewTicker(s.pendingPollInterval)
		defer pendingPollTicker.Stop()

		for {
			select {
			case <-syncCtx.Done():
				return
			case <-pendingPollTicker.C:
				select {
				case pendingSem <- struct{}{}:
					go func() {
						defer func() { <-pendingSem }()
						err := s.fetchAndStorePending(syncCtx)
						if err != nil {
							s.log.Debugw("Error while trying to poll pending block", "err", err)
						}
					}()
				default:
				}
			}
		}
	}()

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
				var t_nextHeight uint64
	if h, err := s.blockchain.Height(); err == nil {
		t_nextHeight = h + 1
	} else {
		t_nextHeight = 0
	} 
	

	
				nextHeight = t_nextHeight
				fetchers, verifiers = stream.New().WithMaxGoroutines(numWorkers), stream.New().WithMaxGoroutines(runtime.GOMAXPROCS(0))
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
