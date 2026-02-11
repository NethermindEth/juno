package rawdb

import (
	"sort"
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb/database"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
)

var _ database.TrieDB = (*Database)(nil)

type Config struct {
	CleanCacheSize uint64 // Maximum size (in bytes) for caching clean nodes
}

type Database struct {
	disk db.KeyValueStore

	lock sync.RWMutex
	log  utils.StructuredLogger

	config     Config
	cleanCache *cleanCache
}

func New(disk db.KeyValueStore, config *Config) *Database {
	if config == nil {
		config = &Config{
			CleanCacheSize: 16 * utils.Megabyte,
		}
	}
	cleanCache := newCleanCache(config.CleanCacheSize)
	return &Database{
		disk:       disk,
		config:     *config,
		cleanCache: &cleanCache,
		log:        utils.NewNopZapLogger(),
	}
}

func (d *Database) readNode(
	id trieutils.TrieID,
	owner *felt.Address,
	path *trieutils.Path,
	isLeaf bool,
) ([]byte, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()

	isClass := id.Type() == trieutils.Class
	blob := d.cleanCache.getNode(owner, path, isClass)
	if blob != nil {
		return blob, nil
	}

	blob, err := trieutils.GetNodeByPath(d.disk, id.Bucket(), owner, path, isLeaf)
	if err != nil {
		return nil, err
	}

	d.cleanCache.putNode(owner, path, isClass, blob)
	return blob, nil
}

func (d *Database) NewIterator(id trieutils.TrieID) (db.Iterator, error) {
	key := id.Bucket().Key()
	owner := id.Owner()
	if !felt.IsZero(&owner) {
		oBytes := owner.Bytes()
		key = append(key, oBytes[:]...)
	}

	return d.disk.NewIterator(key, true)
}

func (d *Database) Update(
	root,
	parent *felt.StateRootHash,
	blockNum uint64,
	mergedClassNodes *trienode.MergeNodeSet,
	mergedContractNodes *trienode.MergeNodeSet,
	batch db.Batch,
) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	var classNodes classNodesMap
	var classOrderedPaths []trieutils.Path
	var contractNodes contractNodesMap
	var contractOrderedPaths []trieutils.Path
	var contractStorageNodes contractStorageNodesMap
	var contractStorageOrderedPaths map[felt.Address][]trieutils.Path

	if mergedClassNodes != nil {
		classNodes, classOrderedPaths, _, _ = mergedClassNodes.FlattenWithOrder()
	}
	if mergedContractNodes != nil {
		contractNodes,
			contractOrderedPaths,
			contractStorageNodes,
			contractStorageOrderedPaths = mergedContractNodes.FlattenWithOrder()
	}

	for _, path := range classOrderedPaths {
		n := classNodes[path]
		err := d.updateNode(batch, db.ClassTrie, &felt.Address{}, &path, n, true)
		if err != nil {
			return err
		}
	}

	for _, path := range contractOrderedPaths {
		n := contractNodes[path]
		err := d.updateNode(batch, db.ContractTrieContract, &felt.Address{}, &path, n, false)
		if err != nil {
			return err
		}
	}

	owners := make([]felt.Address, 0, len(contractStorageNodes))
	for owner := range contractStorageNodes {
		owners = append(owners, owner)
	}
	sort.Slice(owners, func(i, j int) bool {
		return felt.Equal(&owners[i], &owners[j])
	})
	for _, owner := range owners {
		orderedPaths := contractStorageOrderedPaths[owner]
		for _, path := range orderedPaths {
			n := contractStorageNodes[owner][path]
			err := d.updateNode(batch, db.ContractTrieStorage, &owner, &path, n, false)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *Database) updateNode(
	batch db.KeyValueWriter,
	bucket db.Bucket,
	owner *felt.Address,
	path *trieutils.Path,
	n trienode.TrieNode,
	isClass bool,
) error {
	if batch == nil {
		return nil
	}

	if _, deleted := n.(*trienode.DeletedNode); deleted {
		err := trieutils.DeleteNodeByPath(batch, bucket, owner, path, n.IsLeaf())
		if err != nil {
			return err
		}
		d.cleanCache.deleteNode(owner, path, isClass)
	} else {
		err := trieutils.WriteNodeByPath(
			batch,
			bucket,
			owner,
			path,
			n.IsLeaf(),
			n.Blob(),
		)
		if err != nil {
			return err
		}
		d.cleanCache.putNode(owner, path, isClass, n.Blob())
	}
	return nil
}

// This method was added to satisfy the TrieDB interface, but it is not used.
func (d *Database) Commit(_ *felt.StateRootHash) error {
	return nil
}

// This method was added to satisfy the TrieDB interface, but it is not used.
func (d *Database) Close() error {
	return nil
}

func (d *Database) Scheme() database.TrieDBScheme {
	return database.RawScheme
}
