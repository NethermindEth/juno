package tracker

import (
	"sync"
)

// Tracker is a replacement for sync.WaitGroup that properly handles the case
// when a goroutine attempts to call Add after Wait has been called. For example:
//
// Goroutine A (server shutdown)
// wg.Wait()  // waits for all work to finish
//
// Goroutine B (handling request) - might run AFTER Wait() starts
// wg.Add(1)  // PANIC or undefined behaviour
//
// With Tracker, Add returns false instead and Goroutine B can handle the rejection gracefully.
//
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
