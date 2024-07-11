package sync

import (
	"context"
	"errors"
	"sort"
	"sync"
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
	s.newSyncBlocks(ctx)
	return nil
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

func (s *Synchronizer) nextHeight() uint64 {
	nextHeight := uint64(0)
	if h, err := s.blockchain.Height(); err == nil {
		nextHeight = h + 1
	}
	return nextHeight
}

// Get stateupdate, classes and commitments with the given block height
func (s *Synchronizer) getStateAndClasses(ctx context.Context, wg *sync.WaitGroup, blockHeight uint64,
	resultChan chan<- GetResult) {
	defer func() {
		wg.Done()
	}()

	select {
	case <-ctx.Done():
		return
	default:
		stateUpdate, block, err := s.starknetData.StateUpdateWithBlock(ctx, blockHeight)
		if err != nil {
			resultChan <- GetResult{nil, nil, nil, nil, err}
			return
		}

		newClasses, err := s.fetchUnknownClasses(ctx, stateUpdate)
		if err != nil {
			resultChan <- GetResult{nil, nil, nil, nil, err}
			return
		}

		commitments, err := s.blockchain.SanityCheckNewHeight(block, stateUpdate, newClasses)
		if err != nil {
			s.log.Warnw("Failed sanity check", "number", block.Number, "hash", block.Hash.ShortString(), "err", err, commitments)
			resultChan <- GetResult{nil, nil, nil, nil, err}
			return
		}

		resultChan <- GetResult{stateUpdate, block, newClasses, commitments, nil}
	}
}

func (s *Synchronizer) sendHeadAndLog(block *core.Block) {
	s.newHeads.Send(block.Header)
	s.log.Infow("Stored Block", "number", block.Number, "hash",
		block.Hash.ShortString(), "root", block.GlobalStateRoot.ShortString())
}

type GetResult struct {
	stateUpdate *core.StateUpdate
	block       *core.Block
	classes     map[felt.Felt]core.Class
	commitments *core.BlockCommitments
	err         error
}

// main loop for syncing blocks
// Generates worker threads for fetching blocks, classes and sanity check
// since those can be done in parallel
// Waits for all current threads to finish and then stores blocks sequentially
// Here bottleneck is waiting for all to finish but once a goroutine finishes fetching it can
// start fetching the next block

// Also it doesnt check right now if it caught the head of the chain
// It is only on cathcup mode

// Shouldn't be opening and closing threads continuously because it is costly
// Implement it for simplicity but would be better to have a pool of threads
func (s *Synchronizer) newSyncBlocks(ctx context.Context) {
	blockHeight := s.nextHeight()
	var wg sync.WaitGroup
	var workerCount = 12

	for {
		select {
		case <-ctx.Done():
			return
		default:
			resultChan := make(chan GetResult, workerCount)

			for i := 0; i < workerCount; i++ {
				wg.Add(1)
				go s.getStateAndClasses(ctx, &wg, blockHeight+uint64(i), resultChan)
			}

			wg.Wait()
			close(resultChan)

			results := []GetResult{}
			for result := range resultChan {
				results = append(results, result)
			}

			sort.Slice(results, func(i, j int) bool {
				return results[i].block.Header.Number < results[j].block.Header.Number
			})

			i := 0
			for i = 0; i < workerCount; i++ {
				block := results[i].block
				stateUpdate := results[i].stateUpdate
				newClasses := results[i].classes
				commitments := results[i].commitments
				err := results[i].err

				if err != nil {
					break
				}

				err = s.blockchain.Store(block, commitments, stateUpdate, newClasses)
				if err != nil {
					continue
				}

				s.sendHeadAndLog(block)
			}

			blockHeight += uint64(i)
		}

	}
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
