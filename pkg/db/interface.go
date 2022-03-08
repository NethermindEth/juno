// XXX: Should probably be in the same file with all the other logic.
package db

// TODO: Document.
type Databaser interface {
	// Has returns true if the value at the provided key is in the
	// database.
	Has(key []byte) (bool, error)

	// Get returns the value associated with the provided key in the
	// database or returns an error otherwise.
	Get(key []byte) ([]byte, error)

	// Put inserts a key-value pair into the database.
	Put(key, value []byte) error

	// XXX: Document return value.
	// Delete Remove a previous inserted key, otherwise nothing happen
	Delete(key []byte) error

	// NumberOfItems returns the number of items in the database.
	NumberOfItems() (uint64, error)

	// TODO: Document.
	Begin()
	Rollback()
	Close()
}
