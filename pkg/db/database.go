package db

// Databaser represents the interface for all kind of databasess.
type Databaser interface {
	// Has returns true if the value at the provided key is in the
	// database.
	Has(key []byte) (bool, error)

	// Get returns the value associated with the provided key in the
	// database or returns an error otherwise.
	Get(key []byte) ([]byte, error)

	// Put inserts a key-value pair into the database.
	Put(key, value []byte) error

	// Delete Remove a previous inserted key, if key doesn't exist nothing happen, if err != nil means couldn't delete.
	Delete(key []byte) error

	// NumberOfItems returns the number of items in the database.
	NumberOfItems() (uint64, error)

	// Begin create a new Transaction
	Begin()

	// Rollback the database to a previous state
	Rollback()

	// Close closes the environment.
	Close()
}
