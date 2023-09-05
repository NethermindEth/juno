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
	"github.com/NethermindEth/juno/metrics"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/starknetdata"
	"github.com/NethermindEth/juno/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sourcegraph/conc/stream"
)

var _ service.Service = (*Synchronizer)(nil)

const (
	opVerifyLabel = "verify"
	opStoreLabel  = "store"
	opFetchLabel  = "fetch"
)

// Synchronizer manages a list of StarknetData to fetch the latest blockchain updates
type Synchronizer struct {
	Blockchain          *blockchain.Blockchain
	StarknetData        starknetdata.StarknetData
	StartingBlockNumber *uint64
	HighestBlockHeader  atomic.Pointer[core.Header]

	log utils.SimpleLogger

	pendingPollInterval time.Duration

	catchUpMode bool

	// metrics
	opTimerHistogram *prometheus.HistogramVec
	blockCount       prometheus.Counter
	chainHeightGauge prometheus.Gauge
	bestBlockGauge   prometheus.Gauge
	reorgCount       prometheus.Counter
	transactionCount prometheus.Counter
}

func New(bc *blockchain.Blockchain, starkNetData starknetdata.StarknetData,
	log utils.SimpleLogger, pendingPollInterval time.Duration,
) *Synchronizer {
	s := &Synchronizer{
		Blockchain:          bc,
		StarknetData:        starkNetData,
		log:                 log,
		pendingPollInterval: pendingPollInterval,

		opTimerHistogram: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "sync",
			Name:      "timers",
		}, []string{"op"}),
		blockCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "sync",
			Name:      "blocks",
		}),
		chainHeightGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "sync",
			Name:      "blockchain_height",
		}),
		bestBlockGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "sync",
			Name:      "best_known_block_number",
		}),
		reorgCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "sync",
			Name:      "reorganisations",
		}),
		transactionCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "sync",
			Name:      "transactions",
		}),
	}
	metrics.MustRegister(
		s.opTimerHistogram,
		s.blockCount,
		s.chainHeightGauge,
		s.bestBlockGauge,
		s.reorgCount,
		s.transactionCount,
	)
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
			block, err := s.StarknetData.BlockByNumber(ctx, height)
			if err != nil {
				continue
			}
			stateUpdate, err := s.StarknetData.StateUpdate(ctx, height)
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
	state, closer, err := s.Blockchain.HeadState()
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
			class, fetchErr := s.StarknetData.Class(ctx, classHash)
			if fetchErr == nil {
				newClasses[*classHash] = class
			}
			return fetchErr
		}
		return stateErr
	}

	for _, deployedContract := range stateUpdate.StateDiff.DeployedContracts {
		if err = fetchIfNotFound(deployedContract.ClassHash); err != nil {
			return nil, db.CloseAndWrapOnError(closer, err)
		}
	}
	for _, classHash := range stateUpdate.StateDiff.DeclaredV0Classes {
		if err = fetchIfNotFound(classHash); err != nil {
			return nil, db.CloseAndWrapOnError(closer, err)
		}
	}
	for _, declaredV1 := range stateUpdate.StateDiff.DeclaredV1Classes {
		if err = fetchIfNotFound(declaredV1.ClassHash); err != nil {
			return nil, db.CloseAndWrapOnError(closer, err)
		}
	}

	return newClasses, db.CloseAndWrapOnError(closer, nil)
}

func (s *Synchronizer) verifierTask(ctx context.Context, block *core.Block, stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.Class, resetStreams context.CancelFunc,
) stream.Callback {
	timer := prometheus.NewTimer(s.opTimerHistogram.WithLabelValues(opVerifyLabel))
	commitments, err := s.Blockchain.SanityCheckNewHeight(block, stateUpdate, newClasses)
	timer.ObserveDuration()
	return func() {
		select {
		case <-ctx.Done():
			return
		default:
			if err != nil {
				s.log.Warnw("Sanity checks failed", "number", block.Number, "hash", block.Hash.ShortString())
				resetStreams()
				return
			}
			timer := prometheus.NewTimer(s.opTimerHistogram.WithLabelValues(opStoreLabel))
			err = s.Blockchain.Store(block, commitments, stateUpdate, newClasses)
			timer.ObserveDuration()

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
			highestBlockHeader := s.HighestBlockHeader.Load()
			if highestBlockHeader == nil || highestBlockHeader.Number <= block.Number {
				highestBlock, err := s.StarknetData.BlockLatest(ctx)
				if err != nil {
					s.log.Warnw("Failed fetching latest block", "err", err)
				} else {
					s.HighestBlockHeader.Store(highestBlock.Header)
					isBehind := highestBlock.Number > block.Number+uint64(maxWorkers())
					if s.catchUpMode != isBehind {
						resetStreams()
					}
					s.catchUpMode = isBehind
				}
			}

			s.log.Infow("Stored Block", "number", block.Number, "hash",
				block.Hash.ShortString(), "root", block.GlobalStateRoot.ShortString())
			s.updateStats(block)
		}
	}
}

func (s *Synchronizer) nextHeight() uint64 {
	nextHeight := uint64(0)
	if h, err := s.Blockchain.Height(); err == nil {
		nextHeight = h + 1
	}
	return nextHeight
}

func (s *Synchronizer) syncBlocks(syncCtx context.Context) {
	defer func() {
		s.StartingBlockNumber = nil
		s.HighestBlockHeader.Store(nil)
	}()

	fetchers, verifiers := s.setupWorkers()
	streamCtx, streamCancel := context.WithCancel(syncCtx)

	nextHeight := s.nextHeight()
	startingHeight := nextHeight
	s.StartingBlockNumber = &startingHeight

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
				timer := prometheus.NewTimer(s.opTimerHistogram.WithLabelValues(opFetchLabel))
				cb := s.fetcherTask(curStreamCtx, curHeight, verifiers, curCancel)
				timer.ObserveDuration()
				return cb
			})
			nextHeight++
		}
	}
}

func maxWorkers() int {
	m, mProcs := 16, runtime.GOMAXPROCS(0)
	if mProcs > m {
		return m
	}
	return mProcs
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
	head, err := s.Blockchain.HeadsHeader()
	if err == nil {
		localHead = head.Hash
	}

	s.log.Infow("Reorg detected", "localHead", localHead, "forkHead", forkBlock.Hash)

	err = s.Blockchain.RevertHead()
	if err != nil {
		s.log.Warnw("Failed reverting HEAD", "reverted", localHead, "err", err)
	} else {
		s.log.Infow("Reverted HEAD", "reverted", localHead)
	}
	s.reorgCount.Inc()
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
					err := s.fetchAndStorePending(ctx)
					if err != nil {
						s.log.Debugw("Error while trying to poll pending block", "err", err)
					}
					<-sem
				}()
			default:
			}
		}
	}
}

func (s *Synchronizer) fetchAndStorePending(ctx context.Context) error {
	highestBlockHeader := s.HighestBlockHeader.Load()
	if highestBlockHeader == nil {
		return nil
	}

	head, err := s.Blockchain.HeadsHeader()
	if err != nil {
		return err
	}

	// not at the tip of the chain yet, no need to poll pending
	if highestBlockHeader.Number > head.Number {
		return nil
	}

	pendingBlock, err := s.StarknetData.BlockPending(ctx)
	if err != nil {
		return err
	}

	pendingStateUpdate, err := s.StarknetData.StateUpdatePending(ctx)
	if err != nil {
		return err
	}

	newClasses, err := s.fetchUnknownClasses(ctx, pendingStateUpdate)
	if err != nil {
		return err
	}

	s.log.Debugw("Found pending block", "txns", pendingBlock.TransactionCount)
	return s.Blockchain.StorePending(&blockchain.Pending{
		Block:       pendingBlock,
		StateUpdate: pendingStateUpdate,
		NewClasses:  newClasses,
	})
}

func (s *Synchronizer) updateStats(block *core.Block) {
	var (
		transactions              = block.TransactionCount
		currentHeight             = block.Number
		highestKnownHeight uint64 = 0
	)
	highestBlockHeader := s.HighestBlockHeader.Load()
	if highestBlockHeader != nil {
		highestKnownHeight = highestBlockHeader.Number
	}

	s.blockCount.Inc()
	s.chainHeightGauge.Set(float64(currentHeight))
	s.bestBlockGauge.Set(float64(highestKnownHeight))
	s.transactionCount.Add(float64(transactions))
}
