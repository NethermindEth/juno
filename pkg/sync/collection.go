package sync

// collection is a collection of items that can be iterated over.
type collection interface {
	// createIterator returns an iterator for the collection.
	createIterator() iterator
}
