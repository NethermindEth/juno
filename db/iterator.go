package db

import "io"

// Provides functionality to iterate over a database's key/value pairs in ascending order.
// It must be closed after use. A single iterator cannot be used concurrently. Multiple iterators can be used
// concurrently.
type Iterator interface {
	io.Closer

	// Valid returns true if the iterator is positioned at a valid key/value pair.
	Valid() bool

	// First moves the iterator to the first key/value pair.
	First() bool

	// Prev moves the iterator to the previous key/value pair
	Prev() bool

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
