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
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/feed"
	junoplugin "github.com/NethermindEth/juno/plugin"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/starknetdata"
	"github.com/NethermindEth/juno/utils"
	"github.com/sourcegraph/conc/stream"
)

var (
	_ service.Service = (*Synchronizer)(nil)
	_ Reader          = (*Synchronizer)(nil)

	ErrPendingBlockNotFound = errors.New("pending block not found")
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

type PendingSubscription struct {
	*feed.Subscription[*core.Block]
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
	SubscribePending() PendingSubscription

	Pending() (*Pending, error)
	PendingBlock() *core.Block
	PendingState() (core.StateReader, func() error, error)
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

func (n *NoopSynchronizer) SubscribePending() PendingSubscription {
	return PendingSubscription{feed.New[*core.Block]().Subscribe()}
}

func (n *NoopSynchronizer) PendingBlock() *core.Block {
	return nil
}

func (n *NoopSynchronizer) Pending() (*Pending, error) {
	return nil, errors.New("Pending() is not implemented")
}

func (n *NoopSynchronizer) PendingState() (core.StateReader, func() error, error) {
	return nil, nil, errors.New("PendingState() not implemented")
}

// Synchronizer manages a list of StarknetData to fetch the latest blockchain updates
type Synchronizer struct {
	blockchain          *blockchain.Blockchain
	db                  db.DB
	readOnlyBlockchain  bool
	starknetData        starknetdata.StarknetData
	startingBlockNumber *uint64
	highestBlockHeader  atomic.Pointer[core.Header]
	newHeads            *feed.Feed[*core.Block]
	reorgFeed           *feed.Feed[*ReorgBlockRange]
	pendingFeed         *feed.Feed[*core.Block]

	log      utils.SimpleLogger
	listener EventListener

	pending             atomic.Pointer[Pending]
	pendingPollInterval time.Duration
	catchUpMode         bool
	plugin              junoplugin.JunoPlugin

	currReorg *ReorgBlockRange // If nil, no reorg is happening
}

func New(bc *blockchain.Blockchain, starkNetData starknetdata.StarknetData, log utils.SimpleLogger,
	pendingPollInterval time.Duration, readOnlyBlockchain bool, database db.DB,
) *Synchronizer {
	s := &Synchronizer{
		blockchain:          bc,
		db:                  database,
		starknetData:        starkNetData,
		log:                 log,
		newHeads:            feed.New[*core.Block](),
		reorgFeed:           feed.New[*ReorgBlockRange](),
		pendingFeed:         feed.New[*core.Block](),
		pendingPollInterval: pendingPollInterval,
		listener:            &SelectiveListener{},
		readOnlyBlockchain:  readOnlyBlockchain,
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
						if err := s.storeEmptyPending(head); err != nil {
							s.log.Errorw("Failed to store empty pending block", "number", block.Number)
						}
					}
				} else {
					s.log.Warnw("Failed storing Block", "number", block.Number,
						"hash", block.Hash.ShortString(), "err", err)
				}
				resetStreams()
				return
			}

			if err := s.storeEmptyPending(block.Header); err != nil {
				s.log.Errorw("Failed to store empty pending block", "number", block.Number)
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

	pendingBlock.Number = head.Number + 1
	newClasses, err := s.fetchUnknownClasses(ctx, pendingStateUpdate)
	if err != nil {
		return err
	}
	s.log.Debugw("Found pending block", "txns", pendingBlock.TransactionCount)
	return s.StorePending(&Pending{
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

func (s *Synchronizer) SubscribeNewHeads() NewHeadSubscription {
	return NewHeadSubscription{s.newHeads.Subscribe()}
}

func (s *Synchronizer) SubscribeReorg() ReorgSubscription {
	return ReorgSubscription{s.reorgFeed.Subscribe()}
}

func (s *Synchronizer) SubscribePending() PendingSubscription {
	return PendingSubscription{s.pendingFeed.Subscribe()}
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

	if existingPending, err := s.Pending(); err == nil {
		if existingPending.Block.TransactionCount >= p.Block.TransactionCount {
			// ignore the incoming pending if it has fewer transactions than the one we already have
			return nil
		}
	} else if !errors.Is(err, ErrPendingBlockNotFound) {
		return err
	}
	s.pending.Store(p)

	s.pendingFeed.Send(p.Block)

	return nil
}

func (s *Synchronizer) Pending() (*Pending, error) {
	p := s.pending.Load()
	if p == nil {
		return nil, ErrPendingBlockNotFound
	}

	expectedParentHash := &felt.Zero
	if head, err := s.blockchain.HeadsHeader(); err == nil {
		expectedParentHash = head.Hash
	}
	if p.Block.ParentHash.Equal(expectedParentHash) {
		return p, nil
	}

	return nil, ErrPendingBlockNotFound
}

func (s *Synchronizer) PendingBlock() *core.Block {
	pending, err := s.Pending()
	if err != nil {
		return nil
	}
	return pending.Block
}

// PendingState returns the state resulting from execution of the pending block
func (s *Synchronizer) PendingState() (core.StateReader, func() error, error) {
	txn, err := s.db.NewTransaction(false)
	if err != nil {
		return nil, nil, err
	}

	pending, err := s.Pending()
	if err != nil {
		return nil, nil, utils.RunAndWrapOnError(txn.Discard, err)
	}

	return NewPendingState(pending.StateUpdate.StateDiff, pending.NewClasses, core.NewState(txn)), txn.Discard, nil
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

	emptyPending := &Pending{
		Block: pendingBlock,
		StateUpdate: &core.StateUpdate{
			OldRoot:   latestHeader.GlobalStateRoot,
			StateDiff: stateDiff,
		},
		NewClasses: make(map[felt.Felt]core.Class, 0),
	}

	s.pending.Store(emptyPending)
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
