package tracker

import (
	"sync"
)

// Inspired by net/http
// See https://cs.opensource.google/go/go/+/refs/tags/go1.25.1:src/net/http/server.go;l=3604
type Tracker struct {
	mu   sync.Mutex
	wg   sync.WaitGroup
	done bool
}

func (g *Tracker) Add(delta int) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.done {
		return false
	}

	g.wg.Add(delta)
	return true
}

func (g *Tracker) Done() {
	g.wg.Done()
}

func (g *Tracker) Wait() {
	g.mu.Lock()
	g.done = true
	g.mu.Unlock()

	g.wg.Wait()
}
