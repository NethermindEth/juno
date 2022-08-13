package sync

import "github.com/NethermindEth/juno/pkg/types"

// This is modified from an example in the standard library heap package
// See https://pkg.go.dev/container/heap

// An item is something we manage in a priority queue.
type item struct {
	value types.StateUpdate
	// The index is needed by update and is maintained by the heap.Interface methods.
	index int // The index of the item in the heap.
}

// A StateUpdateQueue implements heap.Interface and holds items.
type StateUpdateQueue []*item

func (pq StateUpdateQueue) Len() int { return len(pq) }

func (pq StateUpdateQueue) Less(i, j int) bool {
	// We want Pop to give us the highest, not lowest, priority so we use greater than here.
	return pq[i].value.SequenceNumber > pq[j].value.SequenceNumber
}

func (pq StateUpdateQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *StateUpdateQueue) Push(x any) {
	n := len(*pq)
	val := x.(types.StateUpdate)
	item := &item{
		value: val,
		index: n,
	}
	*pq = append(*pq, item)
}

func (pq *StateUpdateQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}
