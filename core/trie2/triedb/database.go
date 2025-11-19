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

const (
	PathScheme string = "path"
	HashScheme string = "hash"
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
	var triedb database.TrieDB
	var err error
	// Default to path config if not provided
	if config == nil {
		triedb, err = pathdb.New(disk, nil)
		if err != nil {
			return nil, err
		}
	} else if config.PathConfig != nil {
		triedb, err = pathdb.New(disk, config.PathConfig)
		if err != nil {
			return nil, err
		}
	} else if config.HashConfig != nil {
		triedb = hashdb.New(disk, config.HashConfig)
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

func (d *Database) Journal(root *felt.Felt) error {
	pdb, ok := d.triedb.(*pathdb.Database)
	if !ok {
		return fmt.Errorf("unsupported trie db type: %T", d.triedb)
	}
	return pdb.Journal(root)
}

func (d *Database) Scheme() string {
	if d.config == nil {
		return PathScheme
	} else if d.config.PathConfig != nil {
		return PathScheme
	}
	return HashScheme
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
