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
)

var ErrCallEmptyDatabase = errors.New("call to empty database")

type Database struct {
	disk   db.KeyValueStore
	prefix db.Bucket
	config *Config

	CleanCache CleanCache
	DirtyCache DirtyCache

	dirtyCacheSize int

	log utils.SimpleLogger
}

const (
	maxCacheSize         = 100 * 1024 * 1024 // 100 MB
	idealBatchSize       = 100 * 1024
	flushInterval        = 5 * time.Minute
	storageKeySize       = 75
	contractClassKeySize = 44
)

func New(disk db.KeyValueStore, prefix db.Bucket, config *Config) *Database {
	if config == nil {
		config = DefaultConfig
	}
	return &Database{
		disk:       disk,
		prefix:     prefix,
		config:     config,
		CleanCache: NewCleanCache(config.CleanCacheType, config.CleanCacheSize),
		DirtyCache: NewDirtyCache(config.DirtyCacheType, config.DirtyCacheSize),
	}
}

func (d *Database) get(owner felt.Felt, path trieutils.Path, hash felt.Felt, isLeaf bool) ([]byte, error) {
	key := trieutils.NodeKeyByHash(d.prefix, owner, path, hash, isLeaf)

	if blob, hit := d.CleanCache.Get(key); hit {
		return blob, nil
	}

	if blob, hit := d.DirtyCache.Get(key); hit {
		return blob, nil
	}

	var blob []byte
	err := d.disk.Get(key, func(blob []byte) error {
		blob = blob
		return nil
	})
	if err != nil {
		return nil, err
	}

	if d.CleanCache != nil && len(blob) > 0 {
		d.CleanCache.Set(key, blob)
	}

	return blob, nil
}

func (d *Database) insert(owner felt.Felt, path trieutils.Path, hash felt.Felt, blob []byte, isLeaf bool) error {
	key := trieutils.NodeKeyByHash(d.prefix, owner, path, hash, isLeaf)

	d.DirtyCache.Set(key, blob)
	d.dirtyCacheSize += len(blob) + d.hashLen()
	return nil
}

func (d *Database) NewIterator(owner felt.Felt) (db.Iterator, error) {
	key := d.prefix.Key()

	if owner != (felt.Felt{}) {
		oBytes := owner.Bytes()
		key = append(key, oBytes[:]...)
	}

	return d.disk.NewIterator(key, true)
}

func (db *Database) Update(root felt.Felt, parent felt.Felt, block uint64, nodes *trienode.MergeNodeSet) error {
	for owner := range nodes.Sets {
		subset := nodes.Sets[owner]
		subset.ForEach(true, func(path trieutils.Path, n trienode.TrieNode) error {
			db.insert(owner, path, n.Hash(), n.Blob(), n.IsLeaf())
			return nil
		})
	}
	return nil
}

func (d *Database) Commit() error {
	key, value, cacheNotEmpty := d.DirtyCache.GetOldest()
	batch := d.disk.NewBatch()

	for cacheNotEmpty {
		batch.Put(key, value)

		if batch.Size() > idealBatchSize {
			if err := batch.Write(); err != nil {
				return err
			}
			batch.Reset()
		}

		d.dirtyCacheSize -= (len(value) + d.hashLen())
		key, value, cacheNotEmpty = d.DirtyCache.GetOldest()
		ok := d.DirtyCache.RemoveOldest()
		if !ok {
			return fmt.Errorf("oldest element in dirty cache not found")
		}
		d.CleanCache.Set(key, value)
	}
	return nil
}

func (d *Database) Cap(limit uint64) error {
	key, value, cacheNotEmpty := d.DirtyCache.GetOldest()
	batch := d.disk.NewBatch()
	startTime := time.Now()
	nodes := d.DirtyCache.Len()
	dirtyCacheSize := d.dirtyCacheSize

	for uint64(d.dirtyCacheSize) > limit && cacheNotEmpty {
		batch.Put(key, value)

		if batch.Size() > idealBatchSize {
			if err := batch.Write(); err != nil {
				d.log.Errorw("Failed to write flush list to disk", "error", err)
				return err
			}
			batch.Reset()
		}

		d.dirtyCacheSize -= (len(value) + d.hashLen())
		ok := d.DirtyCache.RemoveOldest()
		if !ok {
			return fmt.Errorf("oldest element in dirty cache not found")
		}
		key, value, cacheNotEmpty = d.DirtyCache.GetOldest()
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

func (d *Database) hashLen() int {
	switch d.prefix {
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
	db *Database
}

func (r *reader) Node(owner felt.Felt, path trieutils.Path, hash felt.Felt, isLeaf bool) ([]byte, error) {
	return trieutils.GetNodeByPath(r.db.disk, r.id.Bucket(), owner, path, isLeaf)
}

func (d *Database) NodeReader(id trieutils.TrieID) (database.NodeReader, error) {
	return &reader{db: d, id: id}, nil
}
