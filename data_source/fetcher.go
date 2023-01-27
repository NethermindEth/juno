package datasource

import (
	"errors"
	"sync"

	"github.com/NethermindEth/juno/core"
)

type Fetcher struct {
	source     DataSource        // source of our data
	fromHeight uint64            // height that we should start fetching from
	numWorkers uint8             // number of goroutines to use
	next       uint64            // next height to server
	wg         sync.WaitGroup    // WaitGroup on our worker goroutines
	chans      []chan NextHeight // output channels of our workers
	quit       chan struct{}     // quit signal channel
}

type NextHeight struct {
	Block  *core.Block
	Update *core.StateUpdate
}

func NewFetcher(source DataSource, fromHeight uint64, numWorkers uint8) *Fetcher {
	chans := make([]chan NextHeight, numWorkers)
	for i := uint8(0); i < numWorkers; i++ {
		chans[i] = make(chan NextHeight, 3)
	}

	return &Fetcher{
		source:     source,
		fromHeight: fromHeight,
		numWorkers: numWorkers,
		next:       fromHeight,
		chans:      chans,
		quit:       make(chan struct{}),
	}
}

// Run starts Fetcher
// calling Run on a Fetcher that was previously Shutdown is undefined behaviour
func (f *Fetcher) Run() error {
	f.wg.Add(int(f.numWorkers))

	for i := uint8(0); i < f.numWorkers; i++ {
		go f.fetch(i)
	}

	return nil
}

func (f *Fetcher) fetch(index uint8) {
	curHeight := f.fromHeight + uint64(index)
	for {
		block, err := f.source.GetBlockByNumber(curHeight)
		if err != nil {
			continue // todo: backoff
		}
		update, err := f.source.GetStateUpdate(curHeight)
		if err != nil {
			continue // todo: backoff
		}

		next := NextHeight{
			Block:  block,
			Update: update,
		}
		select {
		case <-f.quit:
			close(f.chans[index])
			f.wg.Done()
			return
		case f.chans[index] <- next:
			curHeight += uint64(f.numWorkers)
		}
	}
}

// Shutdown stops Fetcher
func (f *Fetcher) Shutdown() error {
	close(f.quit)
	f.wg.Wait()

	return nil
}

// GetNext returns the next block and resulting state update
// GetNext is not thread-safe, should be called from a single context
func (f *Fetcher) GetNext() (*NextHeight, error) {
	nextIdx := (f.next - f.fromHeight) % uint64(f.numWorkers)
	next, ok := <-f.chans[nextIdx]

	if !ok {
		return nil, errors.New("fetcher is shutdown")
	}

	f.next++
	return &next, nil
}
