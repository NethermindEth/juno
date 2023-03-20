package sync

import (
	"context"
	"errors"
	"runtime"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknetdata"
	"github.com/NethermindEth/juno/utils"
	"github.com/sourcegraph/conc/stream"
)

// Synchronizer manages a list of StarknetData to fetch the latest blockchain updates
type Synchronizer struct {
	Blockchain   *blockchain.Blockchain
	StarknetData starknetdata.StarknetData

	log utils.SimpleLogger
}

func New(bc *blockchain.Blockchain, starkNetData starknetdata.StarknetData, log utils.SimpleLogger) *Synchronizer {
	return &Synchronizer{
		Blockchain:   bc,
		StarknetData: starkNetData,
		log:          log,
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

			// There are classes in deployed transactions which refer to class hash that are no present in declared
			// classes. Thus, we need to fetch all the classes which are referenced in deployed contracts
			referencedClasses := make(map[felt.Felt]*core.Class)
			for _, deployedContract := range stateUpdate.StateDiff.DeployedContracts {
				referencedClasses[*deployedContract.ClassHash] = nil
			}
			for _, classHash := range stateUpdate.StateDiff.DeclaredClasses {
				referencedClasses[*classHash] = nil
			}
			for classHash := range referencedClasses {
				class, err := s.StarknetData.Class(ctx, &classHash)
				if err != nil {
					continue
				}
				referencedClasses[classHash] = class
			}

			return func() {
				verifiers.Go(func() stream.Callback {
					return s.verifierTask(ctx, block, stateUpdate, referencedClasses, resetStreams)
				})
			}
		}
	}
}

func (s *Synchronizer) verifierTask(ctx context.Context, block *core.Block, stateUpdate *core.StateUpdate,
	declaredClasses map[felt.Felt]*core.Class, resetStreams context.CancelFunc,
) stream.Callback {
	err := s.Blockchain.SanityCheckNewHeight(block, stateUpdate)
	return func() {
		select {
		case <-ctx.Done():
			return
		default:
			if err != nil {
				if errors.As(err, new(core.ErrCantVerifyTransactionHash)) {
					for ; err != nil; err = errors.Unwrap(err) {
						s.log.Debugw("Sanity checks failed", "number", block.Number, "hash",
							block.Hash.ShortString(), "error", err.Error())
					}
				} else {
					s.log.Warnw("Sanity checks failed", "number", block.Number, "hash", block.Hash.ShortString())
					resetStreams()
					return
				}
			}
			err := s.Blockchain.Store(block, stateUpdate, declaredClasses)
			if err != nil {
				s.log.Warnw("Failed storing Block", "number", block.Number,
					"hash", block.Hash.ShortString(), "err", err.Error())
				resetStreams()
				return
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
	fetchers := stream.New().WithMaxGoroutines(runtime.NumCPU())
	verifiers := stream.New().WithMaxGoroutines(runtime.NumCPU())

	streamCtx, streamCancel := context.WithCancel(syncCtx)
	nextHeight := s.nextHeight()

	for {
		select {
		case <-streamCtx.Done():
			select {
			case <-syncCtx.Done():
				streamCancel()
				fetchers.Wait()
				verifiers.Wait()
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
