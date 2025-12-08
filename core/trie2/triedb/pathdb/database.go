package pathdb

import (
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb/database"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
)

const (
	maxDiffLayers           = 128 // TODO(weiihann): might want to make this configurable
	contractClassTrieHeight = 251
)

var _ database.TrieDB = (*Database)(nil)

type Config struct {
	CleanCacheSize  uint64 // Maximum size (in bytes) for caching clean nodes
	WriteBufferSize int    // Maximum size (in bytes) for buffering writes before flushing
}

// Represents the path-based database which contains a in-memory layer tree (cache) + disk layer (database)
type Database struct {
	disk   db.KeyValueStore
	tree   *layerTree
	config Config
	lock   sync.RWMutex
}

// Creates a new path-based database. It will load the journal from the disk and recreate the layer tree.
// If the journal is not found, it will create a new disk layer only. If the config is not provided, it will use the default config,
// which is 16MB for clean cache and 64MB for dirty cache.
func New(disk db.KeyValueStore, config *Config) (*Database, error) {
	if config == nil {
		config = &Config{
			CleanCacheSize:  16 * utils.Megabyte,
			WriteBufferSize: 64 * utils.Megabyte,
		}
	}
	db := &Database{disk: disk, config: *config}
	head, err := db.loadJournal()
	if err != nil {
		return nil, err
	}
	db.tree = newLayerTree(head)
	return db, nil
}

func (d *Database) Close() error {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.tree.diskLayer().resetCache()
	return nil
}

// Forces the commit of all the in-memory diff layers to the disk layer
func (d *Database) Commit(root *felt.StateRootHash) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	return d.tree.cap(root, 0)
}

// Creates a new diff layer on top of the current layer tree.
// If the creation of the diff layer exceeds the given height of the layer tree, the bottom-most layer
// will be merged to the disk layer.
func (d *Database) Update(
	root,
	parent *felt.StateRootHash,
	blockNum uint64,
	mergeClassNodes,
	mergeContractNodes *trienode.MergeNodeSet,
) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	if err := d.tree.add(root, parent, blockNum, mergeClassNodes, mergeContractNodes); err != nil {
		return err
	}

	return d.tree.cap(root, maxDiffLayers)
}

// TODO(weiihann): how to deal with state comm and the layer tree?
func (d *Database) NewIterator(id trieutils.TrieID) (db.Iterator, error) {
	var (
		idBytes    = id.Bucket().Key()
		ownerBytes []byte
	)

	owner := id.Owner()
	if !felt.IsZero(&owner) {
		ob := owner.Bytes()
		ownerBytes = ob[:]
	}

	prefix := make([]byte, 0, len(idBytes)+len(ownerBytes))
	prefix = append(prefix, idBytes...)
	prefix = append(prefix, ownerBytes...)

	return d.disk.NewIterator(prefix, true)
}

func (d *Database) Scheme() database.TrieDBScheme {
	return database.PathScheme
}
