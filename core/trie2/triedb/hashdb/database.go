package hashdb

import (
	"errors"
	"sync"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb/database"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
	"github.com/VictoriaMetrics/fastcache"
)

var ErrCallEmptyDatabase = errors.New("call to empty database")

type Config struct {
	CleanCacheSize int
}

var DefaultConfig = &Config{
	CleanCacheSize: 1024 * 1024 * 64,
}

type Database struct {
	disk   db.KeyValueStore
	config *Config

	cleanCache *fastcache.Cache
	dirtyCache *DirtyCache

	dirtyCacheSize int

	log  utils.SimpleLogger
	lock sync.RWMutex
}

const (
	idealBatchSize = 100 * 1024
)

func New(disk db.KeyValueStore, config *Config) *Database {
	if config == nil {
		config = DefaultConfig
	}
	return &Database{
		disk:       disk,
		config:     config,
		cleanCache: fastcache.New(config.CleanCacheSize),
		dirtyCache: NewDirtyCache(),
		log:        utils.NewNopZapLogger(),
	}
}

func (d *Database) insert(bucket db.Bucket, owner felt.Felt, path trieutils.Path, hash felt.Felt, blob []byte, isLeaf bool) {
	key := trieutils.NodeKeyByHash(bucket, owner, path, hash, isLeaf)
	_, found := d.dirtyCache.Get(key, bucketToTrieType(bucket), owner)
	if found {
		return
	}
	d.dirtyCache.Set(key, blob, bucketToTrieType(bucket), owner)
	d.dirtyCacheSize += nodeSize(key, blob)
}

func (d *Database) node(bucket db.Bucket, owner *felt.Felt, path trieutils.Path, hash *felt.Felt, isLeaf bool) ([]byte, error) {
	key := trieutils.NodeKeyByHash(bucket, *owner, path, *hash, isLeaf)
	if blob := d.cleanCache.Get(nil, key); blob != nil {
		return blob, nil
	}

	nodeBlob, found := d.dirtyCache.Get(key, bucketToTrieType(bucket), *owner)
	if found {
		return nodeBlob, nil
	}

	var blob []byte
	err := d.disk.Get(key, func(value []byte) error {
		blob = value
		return nil
	})
	if err != nil {
		return nil, err
	}
	if blob == nil {
		return nil, db.ErrKeyNotFound
	}

	d.cleanCache.Set(key, blob)

	return blob, nil
}

func (d *Database) remove(bucket db.Bucket, owner felt.Felt, path trieutils.Path, hash felt.Felt, blob []byte, isLeaf bool) error {
	key := trieutils.NodeKeyByHash(bucket, owner, path, hash, isLeaf)
	if err := d.dirtyCache.Remove(key, bucketToTrieType(bucket), owner); err != nil {
		d.log.Errorw("Failed to remove node from dirty cache", "error", err)
		return err
	}
	d.dirtyCacheSize -= nodeSize(key, blob)
	return nil
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

func (d *Database) Commit(_ felt.Felt) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	batch := d.disk.NewBatch()
	nodes, dirtyCacheSize, startTime := d.dirtyCache.Len(), d.dirtyCacheSize, time.Now()

	for key, nodeBlob := range d.dirtyCache.classNodes {
		if err := d.writeNodeToBatch(batch, key, nodeBlob, trieutils.Class, felt.Zero); err != nil {
			return err
		}
	}

	if err := d.clearBatch(batch); err != nil {
		return err
	}

	for key, nodeBlob := range d.dirtyCache.contractNodes {
		if err := d.writeNodeToBatch(batch, key, nodeBlob, trieutils.Contract, felt.Zero); err != nil {
			return err
		}
	}

	if err := d.clearBatch(batch); err != nil {
		return err
	}

	for owner, nodes := range d.dirtyCache.contractStorageNodes {
		for key, nodeBlob := range nodes {
			if err := d.writeNodeToBatch(batch, key, nodeBlob, trieutils.ContractStorage, owner); err != nil {
				return err
			}
		}
	}

	if err := d.clearBatch(batch); err != nil {
		return err
	}

	d.log.Debugw("Flushed dirty cache to disk",
		"size", dirtyCacheSize-d.dirtyCacheSize,
		"nodes", nodes-d.dirtyCache.Len(),
		"duration", time.Since(startTime),
		"liveNodes", d.dirtyCache.Len(),
		"liveSize", d.dirtyCacheSize,
	)
	return nil
}

func (d *Database) writeNodeToBatch(batch db.Batch, key string, nodeBlob []byte, trieType trieutils.TrieType, owner felt.Felt) error {
	if err := batch.Put([]byte(key), nodeBlob); err != nil {
		d.log.Errorw("Failed to write flush list to disk", "error", err)
		return err
	}

	if batch.Size() > idealBatchSize {
		if err := batch.Write(); err != nil {
			d.log.Errorw("Failed to write flush list to disk", "error", err)
			return err
		}
		batch.Reset()
	}

	d.dirtyCacheSize -= nodeSize([]byte(key), nodeBlob)
	if err := d.dirtyCache.Remove([]byte(key), trieType, owner); err != nil {
		d.log.Errorw("Failed to remove node from dirty cache", "error", err)
		return err
	}
	return nil
}

func (d *Database) clearBatch(batch db.Batch) error {
	if batch.Size() > 0 {
		if err := batch.Write(); err != nil {
			d.log.Errorw("Failed to write flush list to disk", "error", err)
			return err
		}
	}
	return nil
}

func (d *Database) Update(
	root,
	parent felt.Felt,
	blockNum uint64,
	mergedClassNodes *trienode.MergeNodeSet,
	mergedContractNodes *trienode.MergeNodeSet,
) error {
	classNodes, _ := mergedClassNodes.Flatten()
	contractNodes, contractStorageNodes := mergedContractNodes.Flatten()

	for path, node := range classNodes {
		if deletedNode, ok := node.(*trienode.DeletedNode); ok {
			if err := d.remove(db.ClassTrie, felt.Zero, path, deletedNode.Hash(), deletedNode.Blob(), deletedNode.IsLeaf()); err != nil {
				return err
			}
		} else {
			d.insert(db.ClassTrie, felt.Zero, path, node.Hash(), node.Blob(), node.IsLeaf())
		}
	}

	for path, node := range contractNodes {
		if _, ok := node.(*trienode.DeletedNode); ok {
			if err := d.remove(db.ContractTrieContract, felt.Zero, path, node.Hash(), node.Blob(), node.IsLeaf()); err != nil {
				return err
			}
		} else {
			d.insert(db.ContractTrieContract, felt.Zero, path, node.Hash(), node.Blob(), node.IsLeaf())
		}
	}

	for owner, nodes := range contractStorageNodes {
		for path, node := range nodes {
			if _, ok := node.(*trienode.DeletedNode); ok {
				if err := d.remove(db.ContractTrieStorage, owner, path, node.Hash(), node.Blob(), node.IsLeaf()); err != nil {
					return err
				}
			} else {
				d.insert(db.ContractTrieStorage, owner, path, node.Hash(), node.Blob(), node.IsLeaf())
			}
		}
	}
	return nil
}

func bucketToTrieType(bucket db.Bucket) trieutils.TrieType {
	switch bucket {
	case db.ClassTrie:
		return trieutils.Class
	case db.ContractTrieContract:
		return trieutils.Contract
	case db.ContractTrieStorage:
		return trieutils.ContractStorage
	default:
		panic("unknown bucket")
	}
}

type reader struct {
	id trieutils.TrieID
	d  *Database
}

func (r *reader) Node(owner *felt.Felt, path trieutils.Path, hash *felt.Felt, isLeaf bool) ([]byte, error) {
	return r.d.node(r.id.Bucket(), owner, path, hash, isLeaf)
}

func (d *Database) NodeReader(id trieutils.TrieID) (database.NodeReader, error) {
	return &reader{d: d, id: id}, nil
}

func (d *Database) Close() error {
	return nil
}

func nodeSize(key, node []byte) int {
	return len(key) + len(node)
}
