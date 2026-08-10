package db

import "io"

// Exposes a read-only interface to the database
type KeyValueReader interface {
	// Checks if a key exists in the data store
	Has(key []byte) (bool, error)
	// If a given key exists, the callback will be called with the value
	// Example:
	//
	//	var value []byte
	//	db.Get([]byte("key"), func(v []byte) error {
	//		value = v
	//		return nil
	//	})
	Get(key []byte, cb func(value []byte) error) error
	// Creates iterators over a database's key/value pairs
	NewIterator(prefix []byte, withUpperBound bool) (Iterator, error)
}

// Exposes a write-only interface to the database
type KeyValueWriter interface {
	// Inserts a given value into the data store
	Put(key []byte, value []byte) error
	// Deletes a given key from the data store
	Delete(key []byte) error
}

// Exposes a range-deletion interface to the database
type KeyValueRangeDeleter interface {
	// Deletes a range of keys from start (inclusive) to end (exclusive)
	DeleteRange(start, end []byte) error
}

// Helper interface
type Helper interface {
	// This will create a read-write transaction, apply the callback to it, and flush the changes
	//
	// The callback may read from and write to the same store, including nested
	// Update/Write calls.
	//
	// It must not start a transaction from inside a Get or Has value callback:
	// implementations may hold a read lock across those callbacks, so committing
	// the nested transaction deadlocks.
	//
	// It must not call Close either. What happens is implementation-dependent:
	// pebble-backed stores reject new transactions once Close begins and wait for
	// the in-flight ones, so Close from inside a callback deadlocks; memory and
	// remote abort in-flight transactions instead. For the same reason Close is
	// only a flush barrier on pebble-backed stores — elsewhere, writes still in
	// flight when Close lands are dropped.
	Update(func(IndexedBatch) error) error
	// This will create a write-only batch, apply the callback to it, and flush the changes.
	// Use this instead of Update when you don't need to read from the batch.
	//
	// The same callback rules as Update apply.
	Write(func(Batch) error) error
	// TODO(weiihann): honestly this doesn't make sense, but it's currently needed for the metrics
	// remove this once the metrics are refactored
	// Returns the underlying database
	Impl() any
	// Returns a local filesystem path for consensus WAL files.
	// Empty means the DB has no local path.
	Path() string
}

// Represents a key-value data store that can handle different operations
type KeyValueStore interface {
	KeyValueReader
	KeyValueWriter
	KeyValueRangeDeleter
	Batcher
	IndexedBatcher
	Snapshotter
	Helper
	Listener
	io.Closer
}
