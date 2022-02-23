package db

type Databaser interface {
	// Has Check that the key provided exists in the collection
	Has(key []byte) (bool, error)

	// Get Returns the value associated to the provided key or returns an error otherwise
	Get(key []byte) ([]byte, error)

	// Put Insert the key-value pair into the collection
	Put(key, value []byte) error

	// Delete Remove a previous inserted key, otherwise nothing happen
	Delete(key []byte) error

	// NumberOfItems return the number of items in the collection
	NumberOfItems() (uint64, error)
	Begin()
	Rollback()
	Close()
}
