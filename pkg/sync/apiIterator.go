package sync

// apiIterator is an iterator that can be used to iterate over the StateDiff provided by the feeder gateway.
type apiIterator struct {
	index int
}

// hasNext returns true if there is another element in the collection.
func (u *apiIterator) hasNext() bool {
	return false

}

// getNext returns the next element in the collection.
func (u *apiIterator) getNext() StateDiff {
	if u.hasNext() {
		return StateDiff{}
	}
	return StateDiff{}
}
