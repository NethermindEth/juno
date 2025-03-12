package db

import "io"

// Represents a data store that can read from the database
type KeyValueReader interface {
	// Checks if a key exists in the data store
	Has(key []byte) (bool, error)
	// Retrieves a value for a given key if it exists
	Get2(key []byte) ([]byte, error) // TODO(weiihann): deal with this
}

// Represents a data store that can write to the database
type KeyValueWriter interface {
	// Inserts a given value into the data store
	Put(key []byte, value []byte) error
	// Deletes a given key from the data store
	Delete(key []byte) error
}

// Represents a data store that can delete a range of keys from the database
type KeyValueRangeDeleter interface {
	// Deletes a range of keys from start (inclusive) to end (exclusive)
	DeleteRange(start, end []byte) error
}

// Helper interface for atomic operations
type Helper interface {
	Update2(func(Batch) error) error
	// This will create a read-only snapshot and apply the callback to it
	View2(func(Snapshot) error) error
}

// Represents a key-value data store that can handle different operations
type KeyValueStore interface {
	KeyValueReader
	KeyValueWriter
	KeyValueRangeDeleter
	Batcher
	IndexedBatcher
	Snapshotter
	Iterable
	Helper
	Listener
	io.Closer
}
