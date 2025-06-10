package triedb

import (
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb/database"
	"github.com/NethermindEth/juno/core/trie2/triedb/hashdb"
	"github.com/NethermindEth/juno/core/trie2/triedb/pathdb"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
)

type Scheme string

const (
	PathDB Scheme = "pathdb"
	HashDB Scheme = "hashdb"
)

type Config struct {
	pathConfig *pathdb.Config
	pashConfig *hashdb.Config
}

type Database struct {
	triedb database.TrieDB
	config *Config
}

func New(disk db.KeyValueStore, config *Config) (*Database, error) {
	// Default to path config if not provided
	if config == nil {
		config = &Config{
			pathConfig: nil,
			pashConfig: &hashdb.Config{
				CleanCacheSize: 1000,
			},
		}
	}

	var triedb database.TrieDB
	var err error

	if config.pathConfig != nil {
		triedb, err = pathdb.New(disk, config.pathConfig)
		if err != nil {
			return nil, err
		}
	} else if config.pashConfig != nil {
		triedb = hashdb.New(disk, config.pashConfig)
	}

	return &Database{
		triedb: triedb,
		config: config,
	}, nil
}

func (d *Database) Update(
	root,
	parent *felt.Felt,
	blockNum uint64,
	mergeClassNodes,
	mergeContractNodes *trienode.MergeNodeSet,
) error {
	switch td := d.triedb.(type) {
	case *pathdb.Database:
		return td.Update(root, parent, blockNum, mergeClassNodes, mergeContractNodes)
	case *hashdb.Database:
		return td.Update(root, parent, blockNum, mergeClassNodes, mergeContractNodes)
	default:
		return fmt.Errorf("unsupported trie db type: %T", td)
	}
}

func (d *Database) Scheme() Scheme {
	switch d.triedb.(type) {
	case *pathdb.Database:
		return PathDB
	case *hashdb.Database:
		return HashDB
	default:
		return ""
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
