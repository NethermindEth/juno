package sync

import (
	"sync"

	"github.com/NethermindEth/juno/internal/sync/contracts"
)

// This is modified from an example in the standard library heap package
// See https://pkg.go.dev/container/heap

// An item is something we manage in a priority queue.
type item struct {
	value contracts.LogStateUpdate
	// The index is needed by update and is maintained by the heap.Interface methods.
	index int // The index of the item in the heap.
}

// A LogStateUpdateQueue implements heap.Interface and holds items.
// It is goroutine-safe.
type LogStateUpdateQueue struct {
	items []*item
	mu    sync.Mutex
}

func NewStateUpdateQueue() *LogStateUpdateQueue {
	return &LogStateUpdateQueue{
		items: make([]*item, 0),
	}
}

func (pq *LogStateUpdateQueue) Len() int {
	return len(pq.items)
}

func (pq *LogStateUpdateQueue) Less(i, j int) bool {
	return pq.items[i].value.BlockNumber.Cmp(pq.items[j].value.BlockNumber) == -1
}

func (pq *LogStateUpdateQueue) Swap(i, j int) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
	pq.items[i].index = i
	pq.items[j].index = j
}

func (pq *LogStateUpdateQueue) Push(x any) {
	n := len((*pq).items)
	val := x.(contracts.LogStateUpdate)
	item := &item{
		value: val,
		index: n,
	}
	pq.mu.Lock()
	(*pq).items = append((*pq).items, item)
	pq.mu.Unlock()
}

func (pq *LogStateUpdateQueue) Pop() any {
	old := (*pq).items
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	pq.mu.Lock()
	(*pq).items = old[0 : n-1]
	pq.mu.Unlock()
	return item
}

func (pq *LogStateUpdateQueue) Peek() any {
	return pq.items[0].value
}
