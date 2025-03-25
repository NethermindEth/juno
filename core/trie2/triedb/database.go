package triedb

import (
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb/database"
	"github.com/NethermindEth/juno/core/trie2/triedb/pathdb"
	"github.com/NethermindEth/juno/core/trie2/trienode"
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
	// Default to path config if not provided
	if config == nil {
		config = &Config{
			PathConfig: &pathdb.Config{},
		}
	}

	return &Database{ // TODO: handle both pathdb and hashdb
		triedb: pathdb.New(disk, config.PathConfig),
		config: config,
	}
}

func (d *Database) Update(
	root,
	parent felt.Felt,
	blockNum uint64,
	classNodes map[trieutils.Path]trienode.TrieNode,
	contractNodes map[felt.Felt]map[trieutils.Path]trienode.TrieNode,
) error {
	switch td := d.triedb.(type) {
	case *pathdb.Database:
		return td.Update(root, parent, blockNum, classNodes, contractNodes)
	// TODO: handle hashdb
	default:
		return fmt.Errorf("unsupported trie db type: %T", td)
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
