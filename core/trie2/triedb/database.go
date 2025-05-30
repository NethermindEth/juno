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

type Config struct {
	PathConfig *pathdb.Config
	HashConfig *hashdb.Config
}

type Database struct {
	triedb database.TrieDB
	config *Config
}

func New(disk db.KeyValueStore, config *Config) (*Database, error) {
	// Default to path config if not provided
	if config == nil {
		config = &Config{
			PathConfig: &pathdb.Config{},
			HashConfig: nil,
		}
	}

	pathdb, err := pathdb.New(disk, config.PathConfig)
	if err != nil {
		return nil, err
	}

	return &Database{ // TODO: handle both pathdb and hashdb
		triedb: pathdb,
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
		return td.Update(&root, &parent, blockNum, mergeClassNodes, mergeContractNodes)
	case *hashdb.Database:
		return td.Update(root, parent, blockNum, mergeClassNodes, mergeContractNodes)
	default:
		return fmt.Errorf("unsupported trie db type: %T", td)
	}
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
