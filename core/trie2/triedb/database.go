package triedb

import (
	"bytes"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb/pathdb"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
)

var (
	_ TrieDB = (*pathdb.Database)(nil)
	_ TrieDB = (*pathdb.EmptyDatabase)(nil)
)

// TODO: may not be the best way to use this interface to split pathdb and hashdb...
type TrieDB interface {
	Get(buf *bytes.Buffer, owner felt.Felt, path trieutils.Path, isLeaf bool) (int, error)
	Put(owner felt.Felt, path trieutils.Path, blob []byte, isLeaf bool) error
	Delete(owner felt.Felt, path trieutils.Path, isLeaf bool) error
	NewIterator(owner felt.Felt) (db.Iterator, error)
}

type Config struct {
	PathConfig *pathdb.Config
	// TODO: add hashdb config here
}

type Database struct {
	backend TrieDB
	config  *Config
}

func New(database db.KeyValueStore, prefix db.Bucket, config *Config) *Database {
	return &Database{ // TODO: handle both pathdb and hashdb
		backend: pathdb.New(database, prefix, config.PathConfig),
		config:  config,
	}
}

func (d *Database) Get(buf *bytes.Buffer, owner felt.Felt, path trieutils.Path, isLeaf bool) (int, error) {
	return d.backend.Get(buf, owner, path, isLeaf)
}

func (d *Database) Put(owner felt.Felt, path trieutils.Path, blob []byte, isLeaf bool) error {
	return d.backend.Put(owner, path, blob, isLeaf)
}

func (d *Database) Delete(owner felt.Felt, path trieutils.Path, isLeaf bool) error {
	return d.backend.Delete(owner, path, isLeaf)
}

func (d *Database) NewIterator(owner felt.Felt) (db.Iterator, error) {
	return d.backend.NewIterator(owner)
}

func NewEmptyPathDatabase() *Database { // TODO: handle both pathdb and hashdb
	return &Database{backend: pathdb.EmptyDatabase{}, config: &Config{}}
}
