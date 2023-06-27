package sync

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/starknetdata"
	"github.com/NethermindEth/juno/utils"
	"github.com/sourcegraph/conc/stream"
)

var _ service.Service = (*Synchronizer)(nil)

// Synchronizer manages a list of StarknetData to fetch the latest blockchain updates
type Synchronizer struct {
	Blockchain          *blockchain.Blockchain
	StarknetData        starknetdata.StarknetData
	StartingBlockNumber *uint64
	HighestBlockHeader  *core.Header

	log utils.SimpleLogger

	pendingPollInterval time.Duration
}

func New(bc *blockchain.Blockchain, starkNetData starknetdata.StarknetData,
	log utils.SimpleLogger, pendingPollInterval time.Duration,
) *Synchronizer {
	return &Synchronizer{
		Blockchain:          bc,
		StarknetData:        starkNetData,
		log:                 log,
		pendingPollInterval: pendingPollInterval,
	}
}

// Run starts the Synchronizer, returns an error if the loop is already running
func (s *Synchronizer) Run(ctx context.Context) error {
	s.syncBlocks(ctx)
	return nil
}

func (s *Synchronizer) fetcherTask(ctx context.Context, height uint64, verifiers *stream.Stream,
	resetStreams context.CancelFunc,
) stream.Callback {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("fetcher paniced", r)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return func() {}
		default:
			block, err := s.StarknetData.BlockByNumber(ctx, height)
			if err != nil {
				fmt.Printf("Error %s", err)
				continue
			}

			if block == nil {
				select {
				case <-ctx.Done():
				case <-time.After(time.Second):
				}
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
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("verifier paniced", r)
		}
	}()
	starttime := time.Now()
	defer func() {
		fmt.Printf("check new height %.2f\n", time.Now().Sub(starttime).Seconds())
	}()
	err := s.Blockchain.SanityCheckNewHeight(block, stateUpdate, newClasses)
	return func() {
		starttime := time.Now()
		defer func() {
			fmt.Printf("store  %.2f\n", time.Now().Sub(starttime).Seconds())
		}()
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("verifier callback paniced", r)
			}
		}()
		select {
		case <-ctx.Done():
			return
		default:
			if err != nil {
				s.log.Warnw("Sanity checks failed", "number", block.Number, "hash", block.Hash.ShortString())
				resetStreams()
				return
			}
			err = s.Blockchain.Store(block, stateUpdate, newClasses)
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

			if s.HighestBlockHeader == nil || s.HighestBlockHeader.Number < block.Number {
				highestBlock, err := s.StarknetData.BlockLatest(ctx)
				if err != nil {
					s.log.Warnw("Failed fetching latest block", "err", err)
				} else {
					s.HighestBlockHeader = highestBlock.Header
				}
			}

			s.log.Infow("Stored Block", "number", block.Number, "hash",
				block.Hash.ShortString(), "root", block.GlobalStateRoot.ShortString())
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
		s.HighestBlockHeader = nil
	}()

	fetchers := stream.New().WithMaxGoroutines(runtime.NumCPU())
	verifiers := stream.New().WithMaxGoroutines(runtime.NumCPU())

	streamCtx, streamCancel := context.WithCancel(syncCtx)

	nextHeight := s.nextHeight()
	startingHeight := nextHeight
	s.StartingBlockNumber = &startingHeight

	pendingSem := make(chan struct{}, 1)
	go s.pollPending(syncCtx, pendingSem)

	for {
		select {
		case <-streamCtx.Done():
			select {
			case <-syncCtx.Done():
				streamCancel()
				fetchers.Wait()
				verifiers.Wait()
				pendingSem <- struct{}{}
				return
			default:
				streamCtx, streamCancel = context.WithCancel(syncCtx)
				nextHeight = s.nextHeight()
				s.log.Warnw("Rolling back sync process", "height", nextHeight)
			}
		default:
			curHeight, curStreamCtx, curCancel := nextHeight, streamCtx, streamCancel
			fetchers.Go(func() stream.Callback {
				return s.fetcherTask(curStreamCtx, curHeight, verifiers, curCancel)
			})
			nextHeight++
		}
	}
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
	if s.HighestBlockHeader == nil {
		return nil
	}

	head, err := s.Blockchain.HeadsHeader()
	if err != nil {
		return err
	}

	// not at the tip of the chain yet, no need to poll pending
	if s.HighestBlockHeader.Number > head.Number {
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
