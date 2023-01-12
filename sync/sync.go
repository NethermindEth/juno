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
		running: 0,

		Blockchain:   bc,
		StarkNetData: starkNetData,
		quit:         make(chan struct{}),
	}
}

// Run starts the Synchronizer, returns an error if the loop is already running
func (s *Synchronizer) Run() error {
	running := atomic.CompareAndSwapUint64(&s.running, 0, 1)
	if !running {
		return errors.New("synchronizer is already running")
	}
	defer atomic.CompareAndSwapUint64(&s.running, 1, 0)

	<-s.quit
	return nil
}

// Shutdown attempts to stop the Synchronizer, should block until loop acknowledges the request
func (s *Synchronizer) Shutdown() error {
	close(s.quit)
	return nil
}
