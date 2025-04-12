package hashdb

import (
	"errors"
	"fmt"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb/database"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/ethdb"
)

var ErrCallEmptyDatabase = errors.New("call to empty database")

type Database struct {
	disk   db.KeyValueStore
	bucket db.Bucket
	config *Config

	CleanCache CleanCache
	DirtyCache DirtyCache

	dirtyCacheSize int

	log utils.SimpleLogger
}

const (
	maxCacheSize         = 100 * 1024 * 1024
	idealBatchSize       = 100 * 1024
	flushInterval        = 5 * time.Minute
	storageKeySize       = 75
	contractClassKeySize = 44
)

func New(disk db.KeyValueStore, config *Config) *Database {
	if config == nil {
		config = DefaultConfig
	}
	return &Database{
		disk:       disk,
		config:     config,
		CleanCache: NewCleanCache(config.CleanCacheType, config.CleanCacheSize),
		DirtyCache: NewDirtyCache(config.DirtyCacheType, config.DirtyCacheSize),
		log:        utils.NewNopZapLogger(),
	}
}

func (d *Database) insert(bucket db.Bucket, owner felt.Felt, path trieutils.Path, hash felt.Felt, blob []byte, isLeaf bool) error {
	key := trieutils.NodeKeyByHash(bucket, owner, path, hash, isLeaf)
	_, found := d.DirtyCache.Get(key)
	if found {
		return nil
	}
	d.DirtyCache.Set(key, cachedNode{
		blob:     blob,
		parents:  0,
		external: make(map[string]struct{}),
	})
	d.dirtyCacheSize += len(blob) + d.hashLen()
	return nil
}

func (d *Database) Node(bucket db.Bucket, owner felt.Felt, path trieutils.Path, hash felt.Felt, isLeaf bool) ([]byte, error) {
	key := trieutils.NodeKeyByHash(bucket, owner, path, hash, isLeaf)
	if d.CleanCache != nil {
		blob, found := d.CleanCache.Get(key)
		if found {
			return blob, nil
		}
	}

	if d.DirtyCache != nil {
		node, found := d.DirtyCache.Get(key)
		if found {
			return node.blob, nil
		}
	}

	var blob []byte
	err := d.disk.Get(key, func(value []byte) error {
		blob = value
		return nil
	})
	if err != nil {
		return nil, err
	}

	if d.CleanCache != nil {
		d.CleanCache.Set(key, blob)
	}

	return blob, nil
}

func (d *Database) remove(bucket db.Bucket, owner felt.Felt, path trieutils.Path, hash felt.Felt, blob []byte, isLeaf bool) error {
	key := trieutils.NodeKeyByHash(bucket, owner, path, hash, isLeaf)
	d.DirtyCache.Remove(key)
	d.dirtyCacheSize -= len(blob) + d.hashLen()
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

func (d *Database) Cap(limit uint64) error {
	batch := d.disk.NewBatch()
	key, node, cacheNotEmpty := d.DirtyCache.GetOldest()
	nodes, dirtyCacheSize, startTime := d.DirtyCache.Len(), d.dirtyCacheSize, time.Now()

	for uint64(d.dirtyCacheSize) > limit && cacheNotEmpty {
		batch.Put(key, node.blob)

		if batch.Size() > idealBatchSize {
			if err := batch.Write(); err != nil {
				d.log.Errorw("Failed to write flush list to disk", "error", err)
				return err
			}
			batch.Reset()
		}

		d.dirtyCacheSize -= (len(node.blob) + d.hashLen())
		ok := d.DirtyCache.RemoveOldest()
		if !ok {
			return fmt.Errorf("oldest element in dirty cache not found")
		}
		key, node, cacheNotEmpty = d.DirtyCache.GetOldest()
	}

	if batch.Size() > 0 {
		if err := batch.Write(); err != nil {
			d.log.Errorw("Failed to write flush list to disk", "error", err)
			return err
		}
	}

	d.log.Debugw("Flushed dirty cache to disk",
		"size", dirtyCacheSize-d.dirtyCacheSize,
		"nodes", nodes-d.DirtyCache.Len(),
		"duration", time.Since(startTime),
		"liveNodes", d.DirtyCache.Len(),
		"liveSize", d.dirtyCacheSize,
	)
	return nil
}

func (d *Database) Commit(root felt.Felt) error {
	batch := d.disk.NewBatch()
	key := trieutils.NodeKeyByHash(d.bucket, felt.Zero, trieutils.Path{}, root, false)
	type stackEntry struct {
		key       []byte
		processed bool
	}
	stack := []stackEntry{{key: key, processed: false}}

	for len(stack) > 0 {
		current := &stack[len(stack)-1]

		if !current.processed {
			node, ok := d.DirtyCache.Get(current.key)
			if !ok {
				stack = stack[:len(stack)-1]
				continue
			}

			current.processed = true

			//TODO: Fix this with actual trie traversal
			for childKey := range node.external {
				stack = append(stack, stackEntry{key: []byte(childKey), processed: false})
			}
			continue
		}

		node, ok := d.DirtyCache.Get(current.key)
		if !ok {
			stack = stack[:len(stack)-1]
			continue
		}

		// Uncache the node
		d.DirtyCache.Remove(current.key)
		d.CleanCache.Set(current.key, node.blob)

		batch.Put(current.key, node.blob)

		if batch.Size() >= ethdb.IdealBatchSize {
			if err := batch.Write(); err != nil {
				return err
			}
			batch.Reset()
		}

		stack = stack[:len(stack)-1]
	}

	// Write the remaining batch
	if batch.Size() > 0 {
		if err := batch.Write(); err != nil {
			return err
		}
	}

	return nil
}

func (d *Database) Update(
	root,
	parent felt.Felt,
	blockNum uint64,
	classNodes map[trieutils.Path]trienode.TrieNode,
	contractNodes map[felt.Felt]map[trieutils.Path]trienode.TrieNode,
) {
	for path, node := range classNodes {
		if _, ok := node.(*trienode.DeletedNode); ok {
			d.remove(db.ContractStorage, felt.Zero, path, node.Hash(), node.Blob(), node.IsLeaf())
		} else {
			d.insert(db.ContractStorage, felt.Zero, path, node.Hash(), node.Blob(), node.IsLeaf())
		}
	}

	for owner, nodes := range contractNodes {
		bucket := db.ContractTrieStorage
		if owner.Equal(&felt.Zero) {
			bucket = db.ContractTrieContract
		}

		for path, node := range nodes {
			if _, ok := node.(*trienode.DeletedNode); ok {
				d.remove(bucket, owner, path, node.Hash(), node.Blob(), node.IsLeaf())
			} else {
				d.insert(bucket, owner, path, node.Hash(), node.Blob(), node.IsLeaf())
			}
		}
	}
}

func (d *Database) hashLen() int {
	switch d.bucket {
	case db.ContractStorage:
		return storageKeySize
	case db.ContractTrieContract, db.ClassTrie:
		return contractClassKeySize
	default:
		return 0
	}
}

type reader struct {
	id trieutils.TrieID
	d  *Database
}

func (r *reader) Node(owner felt.Felt, path trieutils.Path, hash felt.Felt, isLeaf bool) ([]byte, error) {
	return r.d.Node(r.id.Bucket(), owner, path, hash, isLeaf)
}

func (d *Database) NodeReader(id trieutils.TrieID) (database.NodeReader, error) {
	return &reader{d: d, id: id}, nil
}

func (d *Database) Close() error {
	return nil
}
