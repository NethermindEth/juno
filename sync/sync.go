package sync

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/starknetdata"
	"github.com/NethermindEth/juno/utils"
	"github.com/sourcegraph/conc/stream"
)

type ErrSyncFailed struct {
	Height uint64
	Err    error
}

func (e ErrSyncFailed) Error() string {
	return fmt.Sprintf("Sync failed on block #%d with %s", e.Height, e.Err.Error())
}

// Synchronizer manages a list of StarknetData to fetch the latest blockchain updates
type Synchronizer struct {
	Blockchain   *blockchain.Blockchain
	StarknetData starknetdata.StarknetData

	log utils.SimpleLogger
}

func NewSynchronizer(bc *blockchain.Blockchain, starkNetData starknetdata.StarknetData, log utils.SimpleLogger) *Synchronizer {
	return &Synchronizer{
		Blockchain:   bc,
		StarknetData: starkNetData,
		log:          log,
	}
}

// Run starts the Synchronizer, returns an error if the loop is already running
func (s *Synchronizer) Run(ctx context.Context) error {
	return s.SyncBlocks(ctx)
}

func (s *Synchronizer) fetcherTask(ctx context.Context, height uint64, verifiers *stream.Stream, errChan chan ErrSyncFailed) stream.Callback {
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

			return func() {
				verifiers.Go(func() stream.Callback { return s.verifierTask(ctx, block, stateUpdate, errChan) })
			}
		}
	}
}

func (s *Synchronizer) verifierTask(ctx context.Context, block *core.Block, stateUpdate *core.StateUpdate, errChan chan ErrSyncFailed) stream.Callback {
	err := s.Blockchain.SanityCheckNewHeight(block, stateUpdate)
	return func() {
		select {
		case <-ctx.Done():
			return
		default:
			if err != nil {
				s.log.Warnw("Sanity checks failed", "number", block.Number, "hash", block.Hash.Text(16))
				select {
				case <-ctx.Done():
				case errChan <- ErrSyncFailed{block.Number, err}:
				}
				return
			}
			err = s.Blockchain.Store(block, stateUpdate)
			if err != nil {
				s.log.Warnw("Failed storing Block", "number", block.Number, "hash", block.Hash.Text(16),
					"err", err.Error())
				select {
				case <-ctx.Done():
				case errChan <- ErrSyncFailed{block.Number, err}:
				}
				return
			}

			s.log.Infow("Stored Block", "number", block.Number, "hash", block.Hash.Text(16),
				"root", block.GlobalStateRoot.Text(16))
		}
	}
}

func (s *Synchronizer) SyncBlocks(syncCtx context.Context) error {
	errChan := make(chan ErrSyncFailed)
	fetchers := stream.New().WithMaxGoroutines(runtime.NumCPU())
	verifiers := stream.New().WithMaxGoroutines(runtime.NumCPU())

	streamCtx, streamCancel := context.WithCancel(syncCtx)
	nextHeight := uint64(0)
	if h, err := s.Blockchain.Height(); err == nil {
		nextHeight = h + 1
	}

	for {
		select {
		case err := <-errChan:
			streamCancel() // cancel all running tasks
			streamCtx, streamCancel = context.WithCancel(syncCtx)
			nextHeight = err.Height // keep syncing from failed height
			s.log.Warnw("Rolling back sync process to failed height", "height", err.Height)
		case <-syncCtx.Done():
			fetchers.Wait()
			verifiers.Wait()
			if errors.Is(syncCtx.Err(), context.Canceled) {
				return nil
			} else {
				return syncCtx.Err()
			}
		default:
			curHeight := nextHeight
			curStreamCtx := streamCtx
			fetchers.Go(func() stream.Callback {
				return s.fetcherTask(curStreamCtx, curHeight, verifiers, errChan)
			})
			nextHeight++
		}
	}
}
