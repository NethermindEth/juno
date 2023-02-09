package sync

import (
	"errors"
	"log"
	"sync/atomic"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/starknetdata"
)

// Synchronizer manages a list of StarknetData to fetch the latest blockchain updates
type Synchronizer struct {
	running uint64

	Blockchain   *blockchain.Blockchain
	StarknetData starknetdata.StarknetData

	quit chan struct{}
}

func NewSynchronizer(bc *blockchain.Blockchain, starkNetData starknetdata.StarknetData) *Synchronizer {
	return &Synchronizer{
		running:      0,
		Blockchain:   bc,
		StarknetData: starkNetData,
		quit:         make(chan struct{}),
	}
}

// Run starts the Synchronizer, returns an error if the loop is already running
func (s *Synchronizer) Run() error {
	if running := atomic.CompareAndSwapUint64(&s.running, 0, 1); !running {
		return errors.New("synchronizer is already running")
	}
	return s.SyncBlocks()
}

// Shutdown attempts to stop the Synchronizer, should block until loop acknowledges the request
func (s *Synchronizer) Shutdown() error {
	if stopped := atomic.CompareAndSwapUint64(&s.running, 1, 0); !stopped {
		return errors.New("synchronizer is stopped")
	}
	close(s.quit)
	return nil
}

func (s *Synchronizer) SyncBlocks() error {
	for {
		select {
		case <-s.quit:
			// Clean up and return
			return nil
		default:
			// Call the gateway to get blocks and state update
			nextHeight := uint64(0)
			if h, err := s.Blockchain.Height(); err == nil {
				nextHeight = h + 1
			}
			block, err := s.StarknetData.BlockByNumber(nextHeight)
			if err != nil {
				return err
			}
			log.Println()
			log.Printf("Fetched Block: Number: %d, Hash: %s", block.Number, block.Hash.Text(16))
			stateUpdate, err := s.StarknetData.StateUpdate(nextHeight)
			if err != nil {
				return err
			}
			log.Printf("Fetched StateUpdate: Hash: %s, NewRoot: %s", stateUpdate.BlockHash.Text(16),
				stateUpdate.NewRoot.Text(16))
			if err = s.Blockchain.Store(block, stateUpdate); err != nil {
				return err
			}
			log.Printf("Stored Block: Number: %d, Hash: %s", block.Number, block.Hash.Text(16))
			log.Printf("Applied StateUpdate: Hash: %s, NewRoot: %s",
				stateUpdate.BlockHash.Text(16),
				stateUpdate.NewRoot.Text(16))
		}
	}
}
