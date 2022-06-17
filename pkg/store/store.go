// Package store provides an interface for a key-value store and an
// ephemeral key-value store type that can be used when persistence
// beyond the running program is not required.
package store

import (
	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/log"
	"go.uber.org/zap"
)

// KVStorer specifies the API for a []byte key-value store.
type KVStorer interface {
	Delete(key []byte)
	Get(key []byte) ([]byte, bool)
	Put(key, val []byte)
}

// Ephemeral defines an temporary key-value store.
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

type Persistent struct {
	database *db.NamedDatabase
	logger   *zap.SugaredLogger
}

func NewPersistent() (*Persistent, error) {
	database, err := db.GetDatabase("TRIE_STORE")
	if err != nil {
		return nil, err
	}
	return &Persistent{
		database: database,
		logger:   log.Default.Named("Trie"),
	}, nil
}

func (p Persistent) Delete(key []byte) {
	err := p.database.Delete(key)
	if err != nil {
		p.logger.With("error", err, "key", string(key)).Error("Delete error")
	}
}

func (p Persistent) Get(key []byte) ([]byte, bool) {
	value, err := p.database.Get(key)
	if err != nil {
		if db.IsNotFound(err) {
			return nil, false
		}
		p.logger.With("error", err, "key", string(key)).Error("Get error")
		return nil, false
	}
	return value, true
}

func (p Persistent) Put(key, val []byte) {
	err := p.database.Put(key, val)
	if err != nil {
		p.logger.With("error", err, "key", string(key), "value", string(val)).Error("Put error")
	}
}
