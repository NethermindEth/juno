package db

import (
	"errors"
	"fmt"
	"io"
)

// ErrKeyNotFound is returned when key isn't found on a txn.Get.
var ErrKeyNotFound = errors.New("key not found")

// DB is a key-value database
type DB interface {
	io.Closer

	// NewTransaction returns a transaction on this database, it should block if an update transaction is requested
	// while another one is still in progress
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

// Iterator is an iterator over a DB's key/value pairs.
type Iterator interface {
	io.Closer

	// Valid returns true if the iterator is positioned at a valid key/value pair.
	Valid() bool

	// Next moves the iterator to the next key/value pair. It returns whether the
	// iterator is valid after the call. Once invalid, the iterator remains
	// invalid.
	Next() bool

	// Key returns the key at the current position.
	Key() []byte

	// Value returns the value at the current position.
	Value() ([]byte, error)

	// Seek would seek to the provided key if present. If absent, it would seek to the next
	// key in lexicographical order
	Seek(key []byte) bool
}

// Transaction provides an interface to access the database's state at the point the transaction was created
// Updates done to the database with a transaction should be only visible to other newly created transaction after
// the transaction is committed.
type Transaction interface {
	// NewIterator returns an iterator over the database's key/value pairs.
	NewIterator() (Iterator, error)
	// Discard discards all the changes done to the database with this transaction
	Discard() error
	// Commit flushes all the changes pending on this transaction to the database, making the changes visible to other
	// transaction
	Commit() error

	// Set updates the value of the given key
	Set(key, val []byte) error
	// Delete removes the key from the database
	Delete(key []byte) error
	// Get fetches the value for the given key, should return ErrKeyNotFound if key is not present
	// Caller should not assume that the slice would stay valid after the call to cb
	Get(key []byte, cb func([]byte) error) error

	// Impl returns the underlying transaction object
	Impl() any
}

func CloseAndWrapOnError(closeFn func() error, existingErr *error) {
	if closeErr := closeFn(); closeErr != nil {
		if *existingErr != nil {
			*existingErr = fmt.Errorf(`failed to close because "%v" with existing err "%w"`, closeErr, *existingErr)
		} else {
			*existingErr = closeErr
		}
	}
}
