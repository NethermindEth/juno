package db

import (
	"errors"
	"io"
)

// ErrKeyNotFound is returned when key isn't found on a txn.Get.
var ErrKeyNotFound = errors.New("Key not found")

// DB is a key-value database
type DB interface {
	io.Closer

	// NewTransaction returns a transaction on this database
	NewTransaction(update bool) Transaction
	// View creates a read-only transaction and calls fn with the given transaction
	// View should handle committing or discarding the transaction. Transaction should be discarded when fn
	// returns an error
	View(fn func(txn Transaction) error) error
	// Update creates a read-write transaction and calls fn with the given transaction
	// Update should handle committing or discarding the transaction. Transaction should be discarded when fn
	// returns an error
	Update(fn func(txn Transaction) error) error

	// Impl returns the underlying database object
	Impl() any
}

// Entry is a database entry consisting of a Key and Value
type Entry struct {
	Key   []byte
	Value []byte
}

// Transaction provides an interface to access the database's state at the point the transaction was created
// Updates done to the database with a transaction should be only visible to other newly created transaction after
// the transaction is committed.
type Transaction interface {
	// Discard discards all the changes done to the database with this transaction
	Discard()
	// Commit flushes all the changes pending on this transaction to the database, making the changes visible to other
	// transaction
	Commit() error

	// Set updates the value of the given key
	Set(key, val []byte) error
	// Delete removes the key from the database
	Delete(key []byte) error
	// Get fetches the value for the given key, should return ErrKeyNotFound if key is not present
	Get(key []byte) ([]byte, error)
	// Seek would seek to the provided key if present. If absent, it would seek to the next
	// key in lexicographic order
	Seek(key []byte) (*Entry, error)

	// Impl returns the underlying transaction object
	Impl() any
}
