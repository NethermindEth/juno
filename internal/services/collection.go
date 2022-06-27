package services

// StateDiffCollector is a collection of StateDiff provided from the feeder gateway that can be iterated over.
type StateDiffCollector interface {
	// CreateIterator returns an StateDiffIterator for the collection.
	CreateIterator() StateDiffIterator
}
