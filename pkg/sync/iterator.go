package sync

// iterator is a generic interface for iterating over a collection.
type iterator interface {
	// hasNext returns true if there is another element in the collection.
	hasNext() bool
	// getNext returns the next element in the collection.
	getNext() StateDiff
}
