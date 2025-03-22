package triedb

import (
	"github.com/NethermindEth/juno/core/trie2/triedb/database"
	"github.com/NethermindEth/juno/core/trie2/triedb/pathdb"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
)

type Config struct {
	PathConfig *pathdb.Config
	// TODO: add hashdb config here
}

type Database struct {
	triedb database.TrieDB
	config *Config
}

func New(disk db.KeyValueStore, config *Config) *Database {
	return &Database{ // TODO: handle both pathdb and hashdb
		triedb: pathdb.New(disk, config.PathConfig),
		config: config,
	}
}

func (d *Database) NodeReader(id trieutils.TrieID) (database.NodeReader, error) {
	return d.triedb.NodeReader(id)
}

func (d *Database) Close() error {
	return d.triedb.Close()
}

func (d *Database) NewIterator(id trieutils.TrieID) (db.Iterator, error) {
	return d.triedb.NewIterator(id)
}
