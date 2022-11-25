package db

import (
	"io"
)

type TxOp func(Transaction) error

// Database represents a complete database environment, including all
// buckets (or "sub-databases").
type Database interface {
	io.Closer

	// Update creates a read+write transaction on the database.
	Update(TxOp) error
	// Update creates a read-only transaction on the database.
	View(TxOp) error
}

// Transaction represents a batch of reads and writes to a database.
// The underlying database may batch transactions in-memory before
// commiting them to disk, though details vary by implementation.
// Transactions can also add new buckets to the database.
type Transaction interface {
	CreateBucketIfNotExists(bucketName string) error
	Cursor(bucketName string) (Cursor, error)
}

// Cursor performs lookups and modifications on a bucket.
type Cursor interface {
	io.Closer

	// Get retrieves a value for a provided key. Returns nil if the
	// value does not exist for the key.
	Get(key []byte) ([]byte, error)
	// Put inserts a given key-value pair in the bucket,
	// overwriting previously inserted pairs if necessary.
	Put(key []byte, value []byte) error
	// Seek moves the cursor to the first key with the provided
	// prefix. Returns nil if no key has the provided prefix. If the
	// prefix is nil or empty, the first key in the bucket is
	// returned.
	Seek(key []byte) ([]byte, []byte, error)
	// First returns the first key-value pair in the bucket.
	First() ([]byte, []byte, error)
	// Last returns the last key-value pair in the bucket.
	Last() ([]byte, []byte, error)
	// Next returns the next key-value pair in the bucket relative
	// to the cursor's current position.
	Next() ([]byte, []byte, error)
	// Prev returns the previous key-value pair in the bucket
	// relative to the cursor's current position.
	Prev() ([]byte, []byte, error)
}
