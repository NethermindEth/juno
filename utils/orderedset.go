package utils

import (
	"sync"
)

// OrderedSet is a thread-safe data structure that maintains both uniqueness and insertion order of elements.
// It combines the benefits of both maps and slices:
// - Uses a map for O(1) lookups and to ensure element uniqueness
// - Uses a slice to maintain insertion order and enable ordered iteration
// The data structure is safe for concurrent access through the use of a read-write mutex.
type OrderedSet[K comparable, V any] struct {
	itemPos map[K]int // position of the node in the list
	items   []V
	size    int
	lock    sync.RWMutex
}

func NewOrderedSet[K comparable, V any]() *OrderedSet[K, V] {
	return &OrderedSet[K, V]{
		itemPos: make(map[K]int),
	}
}

func (ps *OrderedSet[K, V]) Put(key K, value V) {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	// Update existing entry
	if pos, exists := ps.itemPos[key]; exists {
		ps.items[pos] = value
		return
	}

	// Insert new entry
	ps.itemPos[key] = len(ps.items)
	ps.items = append(ps.items, value)
	ps.size++
}

func (ps *OrderedSet[K, V]) Get(key K) (V, bool) {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	if pos, ok := ps.itemPos[key]; ok {
		return ps.items[pos], true
	}
	var zero V
	return zero, false
}

func (ps *OrderedSet[K, V]) Size() int {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return ps.size
}

// List returns a shallow copy of the proof set's value list.
func (ps *OrderedSet[K, V]) List() []V {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	values := make([]V, len(ps.items))
	copy(values, ps.items)
	return values
}
