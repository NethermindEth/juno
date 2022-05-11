// Package store provides an interface for a key-value store and an
// ephemeral key-value store type that can be used when persistence
// beyond the running program is not required.
package store

// Storer specifies the API for a []byte key-value store.
type Storer interface {
	Delete(key []byte)
	Get(key []byte) ([]byte, bool)
	Put(key, val []byte)
	Begin()
	Rollback()
	Close()
}

// Ephemeral defines a temporary key-value store.
type Ephemeral struct {
	// NOTE: Go does not support []byte keys so as a workaround, a string
	// is used and the Struct's method will handle the conversion.
	table map[string][]byte
}

// New creates a new ephemeral storage instance which conforms to the
// [Storer] interface.
func New() Ephemeral {
	return Ephemeral{table: make(map[string][]byte)}
}

// Delete removes a key and associated value from ephemeral storage.
func (e Ephemeral) Delete(key []byte) {
	delete(e.table, string(key))
}

// Get retrieves a value associated with the given key and a bool
// indicating whether the item was found.
func (e Ephemeral) Get(key []byte) (item []byte, ok bool) {
	item, ok = e.table[string(key)]
	return
}

// Put commits a key-value pair to ephemeral storage.
func (e Ephemeral) Put(key, val []byte) {
	e.table[string(key)] = val
}

func (e Ephemeral) Begin() {

}

func (e Ephemeral) Rollback() {

}

func (e Ephemeral) Close() {

}
