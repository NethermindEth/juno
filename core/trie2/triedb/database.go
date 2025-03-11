package triedb

import (
	"bytes"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb/hashdb"
	"github.com/NethermindEth/juno/core/trie2/triedb/pathdb"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
)

var (
	_ TrieDB = (*pathdb.Database)(nil)
	_ TrieDB = (*pathdb.EmptyDatabase)(nil)
)

type Scheme int

const (
	PathScheme Scheme = iota
	HashScheme
)

func (s Scheme) String() string {
	switch s {
	case PathScheme:
		return "pathdb"
	case HashScheme:
		return "hashdb"
	default:
		return "unknown"
	}
}

type TrieDB interface {
	NewIterator(owner felt.Felt) (db.Iterator, error)
}

type Config struct {
	PathConfig *pathdb.Config
	HashConfig *hashdb.Config
}

type Database struct {
	backend TrieDB
	config  *Config
}

func New(txn db.Transaction, prefix db.Bucket, config *Config) *Database {
	scheme := HashScheme
	switch scheme {
	case PathScheme:
		return &Database{
			backend: pathdb.New(txn, prefix, config.PathConfig),
			config:  config,
		}
	case HashScheme:
		return &Database{
			backend: hashdb.New(txn, prefix, config.HashConfig),
			config:  config,
		}
	default:
		return nil
	}
}

func (d *Database) Get(buf *bytes.Buffer, owner felt.Felt, path trieutils.Path, hash felt.Felt, isLeaf bool) (int, error) {
	switch b := d.backend.(type) {
	case *pathdb.Database:
		return b.Get(buf, owner, path, isLeaf)
	case *hashdb.Database:
		return b.Get(buf, owner, path, hash, isLeaf)
	default:
		return 0, nil
	}
}

func (d *Database) Put(owner felt.Felt, path trieutils.Path, hash felt.Felt, blob []byte, isLeaf bool) error {
	switch b := d.backend.(type) {
	case *pathdb.Database:
		return b.Put(owner, path, blob, isLeaf)
	case *hashdb.Database:
		return b.Put(owner, path, hash, blob, isLeaf)
	default:
		return nil
	}
}

func (d *Database) Delete(owner felt.Felt, path trieutils.Path, hash felt.Felt, isLeaf bool) error {
	switch b := d.backend.(type) {
	case *pathdb.Database:
		return b.Delete(owner, path, isLeaf)
	case *hashdb.Database:
		return b.Delete(owner, path, hash, isLeaf)
	default:
		return nil
	}
}

func (d *Database) NewIterator(owner felt.Felt) (db.Iterator, error) {
	return d.backend.NewIterator(owner)
}

func NewEmptyPathDatabase() *Database {
	scheme := HashScheme
	switch scheme {
	case PathScheme:
		return &Database{backend: pathdb.EmptyDatabase{}, config: &Config{}}
	case HashScheme:
		return &Database{backend: hashdb.EmptyDatabase{}, config: &Config{}}
	default:
		return nil
	}
}
