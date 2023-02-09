package sync

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/starknetdata"
	"github.com/NethermindEth/juno/utils"
)

// Synchronizer manages a list of StarknetData to fetch the latest blockchain updates
type Synchronizer struct {
	running uint64

	Blockchain   *blockchain.Blockchain
	StarknetData starknetdata.StarknetData

	log utils.SimpleLogger
}

func NewSynchronizer(bc *blockchain.Blockchain, starkNetData starknetdata.StarknetData, log utils.SimpleLogger) *Synchronizer {
	return &Synchronizer{
		running:      0,
		Blockchain:   bc,
		StarknetData: starkNetData,
		log:          log,
	}
}

// Run starts the Synchronizer, returns an error if the loop is already running
func (s *Synchronizer) Run(ctx context.Context) error {
	if running := atomic.CompareAndSwapUint64(&s.running, 0, 1); !running {
		return errors.New("synchronizer is already running")
	}
	return s.SyncBlocks(ctx)
}

func (s *Synchronizer) SyncBlocks(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Call the gateway to get blocks and state update
			nextHeight := uint64(0)
			if h, err := s.Blockchain.Height(); err == nil {
				nextHeight = h + 1
			}
			block, err := s.StarknetData.BlockByNumber(ctx, nextHeight)
			if err != nil {
				return err
			}
			s.log.Infow("Fetched block", "number", block.Number, "hash", block.Hash.Text(16))
			stateUpdate, err := s.StarknetData.StateUpdate(ctx, nextHeight)
			if err != nil {
				return err
			}
			s.log.Infow("Fetched state update", "newRoot", stateUpdate.NewRoot.Text(16), "hash", stateUpdate.BlockHash.Text(16))
			if err = s.Blockchain.Store(block, stateUpdate); err != nil {
				return err
			}
			s.log.Infow("Stored block", "number", block.Number, "hash", block.Hash.Text(16))
			s.log.Infow("Applied state update", "newRoot", stateUpdate.NewRoot.Text(16), "hash", stateUpdate.BlockHash.Text(16))
		}
	}
}
