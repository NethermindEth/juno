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
	itemPos map[K]int // position of the item in the list
	items   []V
	lock    sync.RWMutex
}

func NewOrderedSet[K comparable, V any]() *OrderedSet[K, V] {
	return &OrderedSet[K, V]{
		itemPos: make(map[K]int),
	}
}

func (o *OrderedSet[K, V]) Put(key K, value V) {
	o.lock.Lock()
	defer o.lock.Unlock()

	// Update existing entry
	if pos, exists := o.itemPos[key]; exists {
		o.items[pos] = value
		return
	}

	// Insert new entry
	o.itemPos[key] = len(o.items)
	o.items = append(o.items, value)
}

func (o *OrderedSet[K, V]) Get(key K) (V, bool) {
	o.lock.RLock()
	defer o.lock.RUnlock()

	if pos, ok := o.itemPos[key]; ok {
		return o.items[pos], true
	}
	var zero V
	return zero, false
}

func (o *OrderedSet[K, V]) Size() int {
	o.lock.RLock()
	defer o.lock.RUnlock()

	return len(o.items)
}

// List returns a shallow copy of the proof set's value list.
func (o *OrderedSet[K, V]) List() []V {
	o.lock.RLock()
	defer o.lock.RUnlock()

	values := make([]V, len(o.items))
	copy(values, o.items)
	return values
}

// Keys returns a slice of keys in their insertion order
func (o *OrderedSet[K, V]) Keys() []K {
	o.lock.RLock()
	defer o.lock.RUnlock()

	keys := make([]K, len(o.items))
	for k, pos := range o.itemPos {
		keys[pos] = k
	}
	return keys
}

func (o *OrderedSet[K, V]) Clear() {
	o.lock.Lock()
	defer o.lock.Unlock()

	o.items = nil
	o.itemPos = make(map[K]int)
}
