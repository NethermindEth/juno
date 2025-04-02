package hashdb

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb/database"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
)

var ErrCallEmptyDatabase = errors.New("call to empty database")

var dbBufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

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

func (d *Database) get(buf *bytes.Buffer, owner felt.Felt, path trieutils.Path, hash felt.Felt, isLeaf bool) (int, error) {
	dbBuf := dbBufferPool.Get().(*bytes.Buffer)
	dbBuf.Reset()
	defer func() {
		dbBuf.Reset()
		dbBufferPool.Put(dbBuf)
	}()

	if err := d.dbKey(dbBuf, owner, path, hash, isLeaf); err != nil {
		return 0, err
	}

	if hit := d.CleanCache.Get(buf, dbBuf.Bytes()); hit {
		return buf.Len(), nil
	}

	if hit := d.DirtyCache.Get(buf, dbBuf.Bytes()); hit {
		return buf.Len(), nil
	}

	err := d.disk.Get(dbBuf.Bytes(), func(blob []byte) error {
		buf.Write(blob)
		return nil
	})
	if err != nil {
		return 0, err
	}

	if d.CleanCache != nil && buf.Len() > 0 {
		d.CleanCache.Set(dbBuf.Bytes(), buf.Bytes())
	}

	return buf.Len(), nil
}

func (d *Database) insert(owner felt.Felt, path trieutils.Path, hash felt.Felt, blob []byte, isLeaf bool) error {
	buffer := dbBufferPool.Get().(*bytes.Buffer)
	buffer.Reset()
	defer func() {
		buffer.Reset()
		dbBufferPool.Put(buffer)
	}()

	if err := d.dbKey(buffer, owner, path, hash, isLeaf); err != nil {
		return err
	}

	d.DirtyCache.Set(buffer.Bytes(), blob)
	d.dirtyCacheSize += len(blob) + d.hashLen()
	return nil
}

func (d *Database) NewIterator(owner felt.Felt) (db.Iterator, error) {
	buffer := dbBufferPool.Get().(*bytes.Buffer)
	buffer.Reset()
	defer func() {
		buffer.Reset()
		dbBufferPool.Put(buffer)
	}()

	_, err := buffer.Write(d.prefix.Key())
	if err != nil {
		return nil, err
	}

	if owner != (felt.Felt{}) {
		oBytes := owner.Bytes()
		_, err := buffer.Write(oBytes[:])
		if err != nil {
			return nil, err
		}
	}

	return d.disk.NewIterator(buffer.Bytes(), true)
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
