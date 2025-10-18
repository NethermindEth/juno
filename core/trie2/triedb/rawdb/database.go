package rawdb

import (
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
)

type Config struct {
	CleanCacheSize uint64 // Maximum size (in bytes) for caching clean nodes
}

type Database struct {
	disk db.KeyValueStore

	lock sync.RWMutex
	log  utils.SimpleLogger

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

func (d *Database) readNode(id trieutils.TrieID, owner *felt.Felt, path *trieutils.Path, isLeaf bool) ([]byte, error) {
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
	if !owner.Equal(&felt.Zero) {
		oBytes := owner.Bytes()
		key = append(key, oBytes[:]...)
	}

	return d.disk.NewIterator(key, true)
}

func (d *Database) Commit(_ *felt.Felt) error {
	return nil
}

func (d *Database) Update(
	root,
	parent *felt.Felt,
	blockNum uint64,
	mergedClassNodes *trienode.MergeNodeSet,
	mergedContractNodes *trienode.MergeNodeSet,
	batch db.KeyValueWriter,
) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	var classNodes classNodesMap
	var contractNodes contractNodesMap
	var contractStorageNodes contractStorageNodesMap

	if mergedClassNodes != nil {
		classNodes, _ = mergedClassNodes.Flatten()
	}
	if mergedContractNodes != nil {
		contractNodes, contractStorageNodes = mergedContractNodes.Flatten()
	}

	for path, n := range classNodes {
		if _, deleted := n.(*trienode.DeletedNode); deleted {
			if err := trieutils.DeleteNodeByPath(batch, db.ClassTrie, &felt.Zero, &path, n.IsLeaf()); err != nil {
				return err
			}
			d.cleanCache.deleteNode(&felt.Zero, &path, true)
		} else {
			if err := trieutils.WriteNodeByPath(batch, db.ClassTrie, &felt.Zero, &path, n.IsLeaf(), n.Blob()); err != nil {
				return err
			}
			d.cleanCache.putNode(&felt.Zero, &path, true, n.Blob())
		}
	}

	for path, n := range contractNodes {
		if _, deleted := n.(*trienode.DeletedNode); deleted {
			if err := trieutils.DeleteNodeByPath(batch, db.ContractTrieContract, &felt.Zero, &path, n.IsLeaf()); err != nil {
				return err
			}
			d.cleanCache.deleteNode(&felt.Zero, &path, false)
		} else {
			if err := trieutils.WriteNodeByPath(batch, db.ContractTrieContract, &felt.Zero, &path, n.IsLeaf(), n.Blob()); err != nil {
				return err
			}
			d.cleanCache.putNode(&felt.Zero, &path, false, n.Blob())
		}
	}

	for owner, nodes := range contractStorageNodes {
		for path, n := range nodes {
			if _, deleted := n.(*trienode.DeletedNode); deleted {
				if err := trieutils.DeleteNodeByPath(batch, db.ContractTrieStorage, &owner, &path, n.IsLeaf()); err != nil {
					return err
				}
				d.cleanCache.deleteNode(&owner, &path, false)
			} else {
				if err := trieutils.WriteNodeByPath(batch, db.ContractTrieStorage, &owner, &path, n.IsLeaf(), n.Blob()); err != nil {
					return err
				}
				d.cleanCache.putNode(&owner, &path, false, n.Blob())
			}
		}
	}

	return nil
}

func (d *Database) Close() error {
	return nil
}
