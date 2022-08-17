package sync

import (
	"sync"

	"github.com/NethermindEth/juno/pkg/types"
)

// This is modified from an example in the standard library heap package
// See https://pkg.go.dev/container/heap

// An item is something we manage in a priority queue.
type item struct {
	value types.StateUpdate
	// The index is needed by update and is maintained by the heap.Interface methods.
	index int // The index of the item in the heap.
}

// A StateUpdateQueue implements heap.Interface and holds items.
// It is goroutine-safe.
type StateUpdateQueue struct {
	items []*item
	mu    sync.Mutex
}

func NewStateUpdateQueue() *StateUpdateQueue {
	return &StateUpdateQueue{
		items: make([]*item, 0),
	}
}

func (pq *StateUpdateQueue) Len() int {
	return len(pq.items)
}

func (pq *StateUpdateQueue) Less(i, j int) bool {
	return pq.items[i].value.SequenceNumber < pq.items[j].value.SequenceNumber
}

func (pq *StateUpdateQueue) Swap(i, j int) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
	pq.items[i].index = i
	pq.items[j].index = j
}

func (pq *StateUpdateQueue) Push(x any) {
	n := len((*pq).items)
	val := x.(types.StateUpdate)
	item := &item{
		value: val,
		index: n,
	}
	pq.mu.Lock()
	(*pq).items = append((*pq).items, item)
	pq.mu.Unlock()
}

func (pq *StateUpdateQueue) Pop() any {
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

func (pq *StateUpdateQueue) Peek() any {
	return pq.items[0]
}
