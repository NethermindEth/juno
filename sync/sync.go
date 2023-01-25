package sync

import (
	"errors"
	"sync/atomic"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/starknetdata"
)

// Synchronizer manages a list of StarkNetData to fetch the latest blockchain updates
type Synchronizer struct {
	running uint64

	Blockchain   *blockchain.Blockchain
	StarkNetData starknetdata.StarkNetData

	quit chan struct{}
}

func NewSynchronizer(bc *blockchain.Blockchain, starkNetData starknetdata.StarkNetData) *Synchronizer {
	return &Synchronizer{
		running:      0,
		Blockchain:   bc,
		StarkNetData: starkNetData,
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
			var nextHeight uint64
			if h := s.Blockchain.Height(); h != nil {
				nextHeight = *h + 1
			}
			block, err := s.StarkNetData.BlockByNumber(nextHeight)
			if err != nil {
				return err
			}
			stateUpdate, err := s.StarkNetData.StateUpdate(nextHeight)
			if err != nil {
				return err
			}
			if err = s.Blockchain.Store(block, stateUpdate); err != nil {
				return err
			}
		}
	}
}
