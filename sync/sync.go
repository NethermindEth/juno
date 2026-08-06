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
	"github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/feed"
	junoplugin "github.com/NethermindEth/juno/plugin"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/sync/preconfirmed"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/sourcegraph/conc/stream"
	"go.uber.org/zap"
)

var (
	_ service.Service = (*Synchronizer)(nil)
	_ Reader          = (*Synchronizer)(nil)
)

const (
	OpVerify = "verify"
	OpStore  = "store"
	OpFetch  = "fetch"

	// Reorg-check ops, one per exit level of isReverting: resolved from the
	// local height alone, after fetching the remote head, or only after also
	// reading the local header for a hash comparison.
	OpReorgCheckFast   = "reorgCheckFast"
	OpReorgCheckRemote = "reorgCheckRemote"
	OpReorgCheckLocal  = "reorgCheckLocal"
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

type PreConfirmedDataSubscription struct {
	*feed.Subscription[*pending.PreConfirmed]
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
	// StartingBlockHeader returns the header for the first block of the current sync run.
	// Implementations may return a partial header containing only Number and Hash.
	StartingBlockHeader() (*core.Header, error)
	HighestBlockHeader() *core.Header
	SubscribeNewHeads() NewHeadSubscription
	SubscribeReorg() ReorgSubscription
	SubscribePreConfirmed() PreConfirmedDataSubscription
	PreConfirmedChain() (preconfirmed.ChainReader, error)
}

// This is temporary and will be removed once the p2p synchronizer implements this interface.
type NoopSynchronizer struct{}

func (n *NoopSynchronizer) StartingBlockHeader() (*core.Header, error) {
	return nil, errors.New("StartingBlockHeader() not implemented")
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

func (n *NoopSynchronizer) SubscribePreConfirmed() PreConfirmedDataSubscription {
	return PreConfirmedDataSubscription{feed.New[*pending.PreConfirmed]().Subscribe()}
}

func (n *NoopSynchronizer) PreConfirmedChain() (preconfirmed.ChainReader, error) {
	return preconfirmed.ChainReader{}, errors.New("PreConfirmedChain() is not implemented")
}

// Synchronizer manages a list of StarknetData to fetch the latest blockchain updates
type Synchronizer struct {
	blockchain           *blockchain.Blockchain
	db                   db.KeyValueStore
	readOnlyBlockchain   bool
	dataSource           DataSource
	startingBlockNumber  atomic.Pointer[uint64]
	startingBlockHeader  atomic.Pointer[core.Header]
	highestBlockHeader   atomic.Pointer[core.Header]
	newHeads             *feed.Feed[*core.Block]
	reorgFeed            *feed.Feed[*ReorgBlockRange]
	preConfirmedDataFeed *feed.Feed[*pending.PreConfirmed]

	logger   log.StructuredLogger
	listener EventListener

	preConfirmed             *preconfirmed.ChainStorage
	preConfirmedPollInterval time.Duration

	catchUpMode bool
	plugin      junoplugin.JunoPlugin

	currReorg *ReorgBlockRange // If nil, no reorg is happening
}

func New(
	bc *blockchain.Blockchain,
	dataSource DataSource,
	logger log.StructuredLogger,
	preConfirmedPollInterval time.Duration,
	readOnlyBlockchain bool,
	database db.KeyValueStore,
) *Synchronizer {
	s := &Synchronizer{
		blockchain:               bc,
		dataSource:               dataSource,
		db:                       database,
		logger:                   logger,
		newHeads:                 feed.New[*core.Block](),
		reorgFeed:                feed.New[*ReorgBlockRange](),
		preConfirmedDataFeed:     feed.New[*pending.PreConfirmed](),
		preConfirmedPollInterval: preConfirmedPollInterval,
		listener:                 &SelectiveListener{},
		readOnlyBlockchain:       readOnlyBlockchain,
		preConfirmed:             preconfirmed.NewChainStorage(),
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
	checkTimer := time.Now()

	// If localHeight is somehow not available, we precautionarily assume we're not reverting
	localHeight, err := s.blockchain.Height()
	if err != nil {
		return 0, false
	}

	// Only check if we're waiting for the very next block
	if localHeight+1 != nextHeight {
		s.listener.OnSyncStepDone(OpReorgCheckFast, nextHeight, time.Since(checkTimer))
		return 0, false
	}

	// If unable to fetch remoteHead block, we precautionarily assume we're not reverting
	remoteHead, err := s.dataSource.BlockHeaderLatest(ctx)
	if err != nil {
		s.listener.OnSyncStepDone(OpReorgCheckRemote, nextHeight, time.Since(checkTimer))
		return 0, false
	}
	remoteHeight := remoteHead.Number

	// If a newer block is available, revert will be handled in storeTask
	if remoteHeight > localHeight {
		s.listener.OnSyncStepDone(OpReorgCheckRemote, nextHeight, time.Since(checkTimer))
		return 0, false
	}

	// If the latest block is at the same height as the head, compare their hashes
	// If the latest block is older than the head, compare with the stored block at the same height
	if remoteHeight < localHeight {
		localHeight = remoteHeight
	}

	localHead, err := s.blockchain.BlockHeaderByNumber(localHeight)
	if err != nil {
		return 0, false
	}
	s.listener.OnSyncStepDone(OpReorgCheckLocal, nextHeight, time.Since(checkTimer))

	if *remoteHead.Hash == *localHead.Hash {
		return 0, false
	}

	return remoteHeight - 1, true
}

func (s *Synchronizer) handlePluginRevertBlock() {
	fromBlock, err := s.blockchain.Head()
	if err != nil {
		s.logger.Warn(
			"Failed to retrieve the reverted blockchain head block for the plugin",
			zap.Error(err),
		)
		return
	}
	fromSU, err := s.blockchain.StateUpdateByNumber(fromBlock.Number)
	if err != nil {
		s.logger.Warn("Failed to retrieve the reverted blockchain head state-update for the plugin",
			zap.Error(err),
		)
		return
	}
	reverseStateDiff, err := s.blockchain.GetReverseStateDiff()
	if err != nil {
		s.logger.Warn("Failed to retrieve reverse state diff",
			zap.Uint64("head", fromBlock.Number),
			zap.String("hash", fromBlock.Hash.ShortString()),
			zap.Error(err),
		)
		return
	}

	var toBlockAndStateUpdate *junoplugin.BlockAndStateUpdate
	if fromBlock.Number != 0 {
		toBlock, err := s.blockchain.BlockByHash(fromBlock.ParentHash)
		if err != nil {
			s.logger.Warn("Failed to retrieve the parent block for the plugin", zap.Error(err))
			return
		}
		toSU, err := s.blockchain.StateUpdateByNumber(toBlock.Number)
		if err != nil {
			s.logger.Warn("Failed to retrieve the parents state-update for the plugin", zap.Error(err))
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
		s.logger.Error("Plugin RevertBlock failure:", zap.Error(err))
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
			committedBlock.Persisted <- err
			s.logger.Warn(
				"Sanity checks failed",
				zap.Uint64("number", committedBlock.Block.Number),
				zap.String("hash", committedBlock.Block.Hash.ShortString()),
				zap.Error(err),
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
	select {
	case <-ctx.Done():
		committedBlock.Persisted <- ctx.Err()
		return
	default:
	}

	storeTimer := time.Now()
	block := committedBlock.Block
	stateUpdate := committedBlock.StateUpdate
	newClasses := committedBlock.NewClasses
	if err := s.blockchain.Store(block, commitments, stateUpdate, newClasses); err != nil {
		committedBlock.Persisted <- err
		if errors.Is(err, blockchain.ErrParentDoesNotMatchHead) {
			// Block block.Number - 1 is the parent of this block which doesn't match
			// so we need to revert the head to block.Number - 2
			s.revertTask(ctx, block.Number-2, resetStreams)
			return
		}

		s.logger.Warn("Failed storing Block", zap.Uint64("number", block.Number),
			zap.String("hash", block.Hash.ShortString()), zap.Error(err))
		resetStreams()
		return
	}
	committedBlock.Persisted <- nil

	startingBlockNumber := s.startingBlockNumber.Load()
	if startingBlockNumber != nil && block.Number == *startingBlockNumber {
		s.startingBlockHeader.Store(block.Header)
	}

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
	s.logger.Info(
		"Stored Block",
		zap.Uint64("number", block.Number),
		zap.String("hash", block.Hash.ShortString()),
		zap.String("root", block.GlobalStateRoot.ShortString()),
	)
	if s.plugin != nil {
		err := s.plugin.NewBlock(block, stateUpdate, newClasses)
		if err != nil {
			s.logger.Error("Plugin NewBlock failure:", zap.Error(err))
		}
	}
}

func (s *Synchronizer) revertTask(
	ctx context.Context, lastPossiblyValidHeight uint64, resetStreams context.CancelFunc,
) {
	defer resetStreams()
	shouldContinue := true
	for shouldContinue {
		localHeader, err := s.blockchain.HeadsHeader()
		if err != nil {
			s.logger.Error("Failed to retrieve the local head header", zap.Error(err))
			break
		}

		// Always reorg head newer than lastPossiblyValidHeight. Otherwise, check the hash
		if localHeader.Number <= lastPossiblyValidHeight {
			remoteBlock, err := s.dataSource.BlockByNumber(ctx, localHeader.Number)
			if err != nil {
				s.logger.Error("Failed to retrieve the remote header", zap.Error(err))
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
}

func (s *Synchronizer) nextHeight() uint64 {
	if height, err := s.blockchain.Height(); err == nil {
		return height + 1
	}
	return 0
}

func (s *Synchronizer) syncBlocks(syncCtx context.Context) {
	defer func() {
		s.startingBlockNumber.Store(nil)
		s.startingBlockHeader.Store(nil)
		s.highestBlockHeader.Store(nil)
	}()

	nextHeight := s.nextHeight()
	startingHeight := nextHeight
	s.startingBlockNumber.Store(&startingHeight)

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
				s.logger.Warn("Restarting sync process",
					zap.Uint64("height", nextHeight),
					zap.Bool("catchUpMode", s.catchUpMode),
				)
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
	s.logger.Info("Reorg detected", zap.String("localHead", localHeader.Hash.String()))
	if err := s.blockchain.RevertHead(); err != nil {
		s.logger.Warn("Failed reverting HEAD",
			zap.String("reverted", localHeader.Hash.String()),
			zap.Error(err),
		)
	} else {
		s.logger.Info("Reverted HEAD", zap.String("reverted", localHeader.Hash.String()))
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

func (s *Synchronizer) StartingBlockHeader() (*core.Header, error) {
	startingBlockNumber := s.startingBlockNumber.Load()
	if startingBlockNumber == nil {
		return nil, errors.New("not running")
	}

	header := s.startingBlockHeader.Load()
	if header != nil {
		return header, nil
	}

	hash, err := core.GetBlockHeaderHashByNumber(s.db, *startingBlockNumber)
	if err != nil {
		return nil, err
	}
	header = &core.Header{
		Number: *startingBlockNumber,
		Hash:   hash,
	}
	// The sync loop may stop or restart while the fallback DB read is in flight.
	// Only cache the fallback header if it still belongs to the same sync run.
	if s.startingBlockNumber.Load() != startingBlockNumber {
		return nil, errors.New("not running")
	}
	// Avoid overwriting a full header cached by storeTask while this fallback was reading from DB.
	if !s.startingBlockHeader.CompareAndSwap(nil, header) {
		header = s.startingBlockHeader.Load()
		if header == nil {
			return nil, errors.New("not running")
		}
	}
	return header, nil
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

func (s *Synchronizer) SubscribePreConfirmed() PreConfirmedDataSubscription {
	return PreConfirmedDataSubscription{s.preConfirmedDataFeed.Subscribe()}
}

func (s *Synchronizer) pollLatest(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)

	for {
		header, err := s.dataSource.BlockHeaderLatest(ctx)
		if err != nil {
			s.logger.Warn("Failed fetching latest block", zap.Error(err))
		} else {
			s.highestBlockHeader.Store(header)
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

func (s *Synchronizer) PreConfirmedChain() (preconfirmed.ChainReader, error) {
	height, err := s.blockchain.Height()
	if err != nil {
		return preconfirmed.ChainReader{}, err
	}

	snapshot := s.preConfirmed.SnapshotForBlock(height + 1)
	if snapshot.Length() > 0 {
		return snapshot, nil
	}

	head, err := s.blockchain.HeadsHeader()
	if err != nil {
		return preconfirmed.ChainReader{}, err
	}

	emptyPreConfirmed, err := MakeEmptyPreConfirmedForParent(s.blockchain, head)
	if err != nil {
		return preconfirmed.ChainReader{}, err
	}

	return preconfirmed.NewChain(&emptyPreConfirmed)
}

// pollPendingData launches the pre_confirmed chain poller.
func (s *Synchronizer) pollPendingData(ctx context.Context) {
	if s.preConfirmedPollInterval == 0 {
		s.logger.Info("Pre-confirmed block polling is disabled")
		return
	}

	poller := preconfirmed.NewPoller(
		s.dataSource,
		s.preConfirmed,
		s.blockchain,
		s.preConfirmedDataFeed,
		&s.highestBlockHeader,
		s.preConfirmedPollInterval,
		s.logger,
	)
	poller.Run(ctx)
}
