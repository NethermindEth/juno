// Package store provides an interface for a key-value store and an
// ephemeral key-value store type that can be used for testing purposes.
package store

import "fmt"

// Storer specifies the API for a [[]byte] key-value store.
type Storer interface {
	// Delete(key []byte)
	Get(key []byte) ([]byte, bool)
	Put(key, val []byte)
}

// Ephemeral defines an temporary key-value store.
type Ephemeral struct {
	// NOTE: Go does not support []byte keys so as a workaround, a string
	// is used and the struct's method will handle the conversion.
	table map[string][]byte
}

// New creates a new ephemeral storage instance which conforms to the
// [Storer] interface.
func New() Ephemeral {
	return Ephemeral{table: make(map[string][]byte)}
}

// DEBUG.
func (e *Ephemeral) Contents() {
	for k, v := range e.table {
		fmt.Printf("key = %s, val = %s\n", k, v)
	}
}

// Get retrieves a value associated with the given key and a bool
// indicating whether the item was found.
func (e Ephemeral) Get(key []byte) (item []byte, ok bool) {
	item, ok = e.table[string(key)]
	return
}

// Put commits the value val with given key to the ephemeral storage.
func (e Ephemeral) Put(key, val []byte) {
	e.table[string(key)] = val
}

// TODO: Implement Delete.
