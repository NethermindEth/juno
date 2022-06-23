package sync

// apiCollection is a collection of StateDiff provided from the feeder gateway that can be iterated over.
type apiCollection struct {
}

// createIterator returns an iterator for the collection.
func (a apiCollection) createIterator() iterator {
	return nil
}
