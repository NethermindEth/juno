package sync

import (
	"errors"
	"sync/atomic"

	"github.com/NethermindEth/juno/core/blockchain"
	"github.com/NethermindEth/juno/starknetdata"
)

// SyncLoop manages a list of DataSources to fetch the latest blockchain updates
type SyncLoop struct {
	running uint64

	Blockchain  *blockchain.Blockchain
	DataSources []*starknetdata.StarkNetData

	ExitChn chan struct{}
}

func NewSyncLoop(bc *blockchain.Blockchain, sources []*starknetdata.StarkNetData) *SyncLoop {
	return &SyncLoop{
		running: 0,

		Blockchain:  bc,
		DataSources: sources,
		ExitChn:     make(chan struct{}),
	}
}

// Run starts the SyncLoop, returns an error if the loop is already running
func (l *SyncLoop) Run() error {
	running := atomic.CompareAndSwapUint64(&l.running, 0, 1)
	if !running {
		return errors.New("SyncLoop is already running")
	}
	defer atomic.CompareAndSwapUint64(&l.running, 1, 0)

	<-l.ExitChn // todo: work
	return nil
}

// Shutdown attempts to stop the SyncLoop, should block until loop acknowledges the request
func (l *SyncLoop) Shutdown() error {
	l.ExitChn <- struct{}{}
	return nil
}
