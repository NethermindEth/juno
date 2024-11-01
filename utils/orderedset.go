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
	nodeSet  map[K]V
	nodePos  map[K]int // position of the node in the list
	nodeList []V
	size     int
	lock     sync.RWMutex
}

func NewOrderedSet[K comparable, V any]() *OrderedSet[K, V] {
	return &OrderedSet[K, V]{
		nodeSet: make(map[K]V),
		nodePos: make(map[K]int),
	}
}

func (ps *OrderedSet[K, V]) Put(key K, value V) {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	// Update existing entry
	if _, exists := ps.nodeSet[key]; exists {
		ps.nodeSet[key] = value
		pos := ps.nodePos[key]
		ps.nodeList[pos] = value
		return
	}

	// Insert new entry
	ps.nodeSet[key] = value
	ps.nodePos[key] = len(ps.nodeList) - 1
	ps.nodeList = append(ps.nodeList, value)
	ps.size++
}

func (ps *OrderedSet[K, V]) Get(key K) (V, bool) {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	value, ok := ps.nodeSet[key]
	return value, ok
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

	values := make([]V, len(ps.nodeList))
	copy(values, ps.nodeList)
	return values
}
